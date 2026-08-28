package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cursor-tab-server/internal/audit"
	"cursor-tab-server/internal/config"
	"cursor-tab-server/internal/coordination"
	"cursor-tab-server/internal/httpapi"
	"cursor-tab-server/internal/proxy"
	"cursor-tab-server/internal/secret"
	"cursor-tab-server/internal/store"
	"cursor-tab-server/internal/tokenpool"
)

const configPath = "./config.yaml"

func main() {
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatal(err)
	}
	database, err := store.Open(cfg.DatabasePath)
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()
	if err := database.Migrate(context.Background()); err != nil {
		log.Fatal(err)
	}

	coordinator, err := coordination.New(cfg.RedisURL, cfg.RedisPrefix)
	if err != nil {
		log.Fatal(err)
	}
	defer coordinator.Close()
	tokenCipher, err := secret.LoadOrCreate(cfg.TokenKeyPath)
	if err != nil {
		log.Fatal(err)
	}
	tokens := tokenpool.New(database, tokenCipher, coordinator)
	legacyToken := cfg.CursorToken
	if stored, ok, readErr := database.SettingString(context.Background(), store.SettingCursorToken); readErr == nil && ok && stored != "" {
		legacyToken = stored
	}
	if err := tokens.Bootstrap(context.Background(), legacyToken); err != nil {
		log.Fatal(err)
	}

	auditService := audit.New(database)
	stopCleanup := startCleanup(database, auditService)
	defer stopCleanup()
	proxyHandler := proxy.NewWithPool(tokens, &http.Client{Timeout: 30 * time.Second}, proxy.DefaultTargets())
	server := &http.Server{
		Addr: cfg.ListenAddr,
		Handler: httpapi.New(httpapi.Dependencies{
			Config: cfg, Store: database, Proxy: proxyHandler, TokenPool: tokens,
			Coordinator: coordinator, Audit: auditService, StartedAt: time.Now().UTC(),
		}),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		log.Printf("cursor-tab-server listening on %s", cfg.ListenAddr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("server error: %v", err)
		}
	}()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
	<-interrupt
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "shutdown error:", err)
	}
}

func startCleanup(database *store.Store, service *audit.Service) func() {
	stopped := make(chan struct{})
	cleanup := func() {
		retentionDays := 30
		if value, ok, err := database.SettingInt(context.Background(), store.SettingLogRetentionDays); err == nil && ok {
			retentionDays = value
		}
		if _, err := service.DeleteOlderThan(context.Background(), time.Now().Add(-time.Duration(retentionDays)*24*time.Hour)); err != nil {
			log.Printf("audit cleanup failed: %v", err)
		}
	}
	cleanup()
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				cleanup()
			case <-stopped:
				return
			}
		}
	}()
	return func() { close(stopped) }
}
