package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	DefaultListenAddr     = "127.0.0.1:8041"
	DefaultDatabasePath   = "./data/cursor-tab-server.db"
	defaultProxyRateLimit = 120
	defaultAdminRateLimit = 30
)

type fileConfig struct {
	Token string `yaml:"token"`
}

type Config struct {
	CursorToken        string
	AdminUsername      string
	AdminPassword      string
	ListenAddr         string
	DatabasePath       string
	RedisURL           string
	RedisPrefix        string
	TokenKeyPath       string
	ProxyRatePerMinute int
	AdminRatePerMinute int
}

func Load(path string) (Config, error) {
	if err := loadDotEnv(filepath.Join(filepath.Dir(path), ".env")); err != nil {
		return Config{}, err
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read configuration: %w", err)
	}
	var file fileConfig
	if err := yaml.Unmarshal(contents, &file); err != nil {
		return Config{}, fmt.Errorf("parse configuration: %w", err)
	}
	if strings.TrimSpace(file.Token) == "" {
		return Config{}, fmt.Errorf("token cannot be empty")
	}
	username := strings.TrimSpace(os.Getenv("ADMIN_USERNAME"))
	if username == "" {
		return Config{}, fmt.Errorf("ADMIN_USERNAME cannot be empty")
	}
	password := os.Getenv("ADMIN_PASSWORD")
	if password == "" {
		return Config{}, fmt.Errorf("ADMIN_PASSWORD cannot be empty")
	}
	databasePath := envOrDefault("DATABASE_PATH", DefaultDatabasePath)
	return Config{
		CursorToken:        strings.TrimSpace(file.Token),
		AdminUsername:      username,
		AdminPassword:      password,
		ListenAddr:         envOrDefault("LISTEN_ADDR", DefaultListenAddr),
		DatabasePath:       databasePath,
		RedisURL:           strings.TrimSpace(os.Getenv("REDIS_URL")),
		RedisPrefix:        envOrDefault("REDIS_PREFIX", "cursor-tab"),
		TokenKeyPath:       envOrDefault("TOKEN_KEY_PATH", filepath.Join(filepath.Dir(databasePath), "token.key")),
		ProxyRatePerMinute: envPositiveInt("PROXY_RATE_PER_MINUTE", defaultProxyRateLimit),
		AdminRatePerMinute: envPositiveInt("ADMIN_RATE_PER_MINUTE", defaultAdminRateLimit),
	}, nil
}

func loadDotEnv(path string) error {
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read .env: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		name, value, found := strings.Cut(line, "=")
		name = strings.TrimSpace(name)
		if !found || name == "" {
			return fmt.Errorf("parse .env line %d", lineNumber)
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
			value = value[1 : len(value)-1]
		}
		if strings.TrimSpace(os.Getenv(name)) == "" {
			if err := os.Setenv(name, value); err != nil {
				return fmt.Errorf("set .env value %q: %w", name, err)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read .env: %w", err)
	}
	return nil
}

func envOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func envPositiveInt(name string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	var parsed int
	if _, err := fmt.Sscanf(value, "%d", &parsed); err != nil || parsed < 1 {
		return fallback
	}
	return parsed
}
