package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTempConfig(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadRejectsMissingAdminCredentials(t *testing.T) {
	t.Setenv("ADMIN_USERNAME", "")
	t.Setenv("ADMIN_PASSWORD", "password")
	_, err := Load(writeTempConfig(t, "token: cursor-token\n"))
	if err == nil || !strings.Contains(err.Error(), "ADMIN_USERNAME") {
		t.Fatalf("err = %v", err)
	}
}

func TestLoadReadsAdministratorCredentialsFromDotEnvBesideConfiguration(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "config.yaml")
	if err := os.WriteFile(path, []byte("token: cursor-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, ".env"), []byte("ADMIN_USERNAME=dotenv-admin\nADMIN_PASSWORD=dotenv-password\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ADMIN_USERNAME", "")
	t.Setenv("ADMIN_PASSWORD", "")

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AdminUsername != "dotenv-admin" || cfg.AdminPassword != "dotenv-password" {
		t.Fatalf("administrator credentials = %q, %q", cfg.AdminUsername, cfg.AdminPassword)
	}
}

func TestLoadUsesSecureDefaults(t *testing.T) {
	t.Setenv("ADMIN_USERNAME", "admin")
	t.Setenv("ADMIN_PASSWORD", "password")
	cfg, err := Load(writeTempConfig(t, "token: cursor-token\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ListenAddr != DefaultListenAddr {
		t.Fatalf("ListenAddr = %q", cfg.ListenAddr)
	}
	if cfg.DatabasePath != DefaultDatabasePath {
		t.Fatalf("DatabasePath = %q", cfg.DatabasePath)
	}
}
