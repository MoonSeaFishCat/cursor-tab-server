package config

import (
	"strings"
	"testing"
)

func TestLoadRejectsMissingAdministratorCredentials(t *testing.T) {
	t.Setenv("ADMIN_USERNAME", "")
	t.Setenv("ADMIN_PASSWORD", "password")
	_, err := Load(writeTempConfig(t, "token: cursor-token\n"))
	if err == nil || !strings.Contains(err.Error(), "ADMIN_USERNAME") {
		t.Fatalf("err = %v", err)
	}
}
