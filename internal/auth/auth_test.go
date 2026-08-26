package auth

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestCreateAPIKeyReturnsOnlyVerifiableHash(t *testing.T) {
	plain, prefix, hash, err := CreateAPIKey("production")
	if err != nil || !strings.HasPrefix(plain, "cts_") || !strings.HasPrefix(plain, prefix) {
		t.Fatalf("invalid key result: plain=%q prefix=%q err=%v", plain, prefix, err)
	}
	if !VerifySecret(plain, hash) || VerifySecret("cts_invalid", hash) {
		t.Fatal("hash verification mismatch")
	}
}

func TestSessionCookieIsSecureHTTPOnlyAndExpiresInEightHours(t *testing.T) {
	now := time.Date(2026, 8, 26, 8, 0, 0, 0, time.UTC)
	cookie := SessionCookie("session-value", now.Add(8*time.Hour))
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode {
		t.Fatal("insecure cookie")
	}
	if !cookie.Expires.Equal(now.Add(8 * time.Hour)) {
		t.Fatalf("expires = %s", cookie.Expires)
	}
}
