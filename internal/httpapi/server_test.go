package httpapi

import (
	"context"
	"cursor-tab-server/internal/audit"
	"cursor-tab-server/internal/auth"
	"cursor-tab-server/internal/config"
	"cursor-tab-server/internal/proxy"
	"cursor-tab-server/internal/store"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	testAdminUsername = "admin"
	testAdminPassword = "correct-password"
)

func testServer(t *testing.T) *Server {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err = db.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	return New(Dependencies{Config: config.Config{
		AdminUsername:      testAdminUsername,
		AdminPassword:      testAdminPassword,
		ProxyRatePerMinute: 120,
		AdminRatePerMinute: 120,
	}, Store: db, Proxy: proxy.New("token", nil, map[string]string{"/allowed": "http://example.invalid/allowed"}), Audit: audit.New(db), StartedAt: time.Now()})
}

func TestListKeysReturnsActivityForUnusedKey(t *testing.T) {
	s := testServer(t)
	plain, prefix, hash, err := auth.CreateAPIKey("unused")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.deps.Store.CreateAPIKey(context.Background(), store.APIKey{ID: "unused", Name: "unused", Prefix: prefix, SecretHash: hash, CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	createTestCaptcha(t, s, "list-keys", "ABC123")
	login := performLogin(s, testAdminUsername, testAdminPassword, "list-keys", "ABC123")
	request := httptest.NewRequest(http.MethodGet, "/admin/api-keys", nil)
	request.AddCookie(login.Result().Cookies()[0])
	out := httptest.NewRecorder()
	s.ServeHTTP(out, request)
	if out.Code != http.StatusOK {
		t.Fatalf("list keys code=%d body=%s key=%s", out.Code, out.Body.String(), plain)
	}
}

func TestProxyRejectsUnauthenticatedRequest(t *testing.T) {
	s := testServer(t)
	out := httptest.NewRecorder()
	s.ServeHTTP(out, httptest.NewRequest(http.MethodPost, "/allowed", nil))
	if out.Code != http.StatusUnauthorized {
		t.Fatalf("code=%d", out.Code)
	}
}

func TestProxyAcceptsAPIKeyInQueryParameter(t *testing.T) {
	s := testServer(t)
	plain, prefix, hash, err := auth.CreateAPIKey("query parameter")
	if err != nil {
		t.Fatal(err)
	}
	key := store.APIKey{ID: "query-key", Name: "query parameter", Prefix: prefix, SecretHash: hash, CreatedAt: time.Now().UTC()}
	if err := s.deps.Store.CreateAPIKey(context.Background(), key); err != nil {
		t.Fatal(err)
	}

	out := httptest.NewRecorder()
	s.ServeHTTP(out, httptest.NewRequest(http.MethodPost, "/allowed?key="+plain, nil))
	if out.Code == http.StatusUnauthorized {
		t.Fatalf("query API key was rejected: %s", out.Body.String())
	}
	stored, err := s.deps.Store.FindActiveAPIKeyByHash(context.Background(), auth.HashSecret(plain))
	if err != nil || stored.LastUsedAt == nil {
		t.Fatalf("query API key was not recorded as used: key=%+v err=%v", stored, err)
	}
}

func TestProxyAcceptsAPIKeyInBasePath(t *testing.T) {
	s := testServer(t)
	plain, prefix, hash, err := auth.CreateAPIKey("base path")
	if err != nil {
		t.Fatal(err)
	}
	key := store.APIKey{ID: "path-key", Name: "base path", Prefix: prefix, SecretHash: hash, CreatedAt: time.Now().UTC()}
	if err := s.deps.Store.CreateAPIKey(context.Background(), key); err != nil {
		t.Fatal(err)
	}

	for _, basePath := range []string{"/" + plain, "/key=" + plain} {
		out := httptest.NewRecorder()
		s.ServeHTTP(out, httptest.NewRequest(http.MethodPost, basePath+"/allowed", nil))
		if out.Code == http.StatusUnauthorized || out.Code == http.StatusNotFound {
			t.Fatalf("path %q was rejected with status %d: %s", basePath, out.Code, out.Body.String())
		}
	}

	stored, err := s.deps.Store.FindActiveAPIKeyByHash(context.Background(), auth.HashSecret(plain))
	if err != nil || stored.LastUsedAt == nil {
		t.Fatalf("base path API key was not recorded as used: key=%+v err=%v", stored, err)
	}
}

func TestCaptchaResponseDoesNotExposeAnswer(t *testing.T) {
	s := testServer(t)
	now := time.Now().UTC()
	if err := s.deps.Store.CreateLoginCaptcha(context.Background(), store.LoginCaptcha{ID: "expired", AnswerHash: []byte("hash"), CreatedAt: now.Add(-10 * time.Minute), ExpiresAt: now.Add(-5 * time.Minute)}); err != nil {
		t.Fatal(err)
	}
	out := httptest.NewRecorder()
	s.ServeHTTP(out, httptest.NewRequest(http.MethodGet, "/admin/captcha", nil))
	if out.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", out.Code, out.Body.String())
	}
	var expiredCount int
	if err := s.deps.Store.QueryRow(`SELECT COUNT(*) FROM login_captchas WHERE id = 'expired'`).Scan(&expiredCount); err != nil {
		t.Fatal(err)
	}
	if expiredCount != 0 {
		t.Fatalf("expired captcha count = %d", expiredCount)
	}
	var response struct {
		CaptchaID string    `json:"captcha_id"`
		Image     string    `json:"image"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	if err := json.NewDecoder(out.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response.CaptchaID == "" || !strings.HasPrefix(response.Image, "data:image/png;base64,") || response.ExpiresAt.Before(time.Now()) {
		t.Fatalf("unexpected captcha response: %+v", response)
	}
	if strings.Contains(strings.ToLower(out.Body.String()), "answer") {
		t.Fatal("captcha response exposed an answer field")
	}
}

func TestAdminLoginCreatesSessionForCorrectCredentialsAndCaptcha(t *testing.T) {
	s := testServer(t)
	createTestCaptcha(t, s, "captcha-id", "ABC123")

	login := performLogin(s, testAdminUsername, testAdminPassword, "captcha-id", "ABC123")
	if login.Code != http.StatusNoContent || len(login.Result().Cookies()) != 1 {
		t.Fatalf("login code=%d cookies=%d body=%s", login.Code, len(login.Result().Cookies()), login.Body.String())
	}
	cookie := login.Result().Cookies()[0]
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode {
		t.Fatal("login did not set a secure session cookie")
	}

	out := httptest.NewRecorder()
	check := httptest.NewRequest(http.MethodGet, "/admin/session", nil)
	check.AddCookie(cookie)
	s.ServeHTTP(out, check)
	if out.Code != http.StatusNoContent {
		t.Fatalf("session check = %d", out.Code)
	}
}

func TestAdminLoginRejectsAllInvalidCredentialsIdenticallyAndConsumesCaptcha(t *testing.T) {
	s := testServer(t)
	for _, test := range []struct {
		name     string
		username string
		password string
		answer   string
	}{
		{"username", "wrong", testAdminPassword, "ABC123"},
		{"password", testAdminUsername, "wrong", "ABC123"},
		{"answer", testAdminUsername, testAdminPassword, "wrong"},
	} {
		t.Run(test.name, func(t *testing.T) {
			captchaID := "captcha-" + test.name
			createTestCaptcha(t, s, captchaID, "ABC123")
			response := performLogin(s, test.username, test.password, captchaID, test.answer)
			if response.Code != http.StatusUnauthorized || response.Body.String() != "{\"error\":\"invalid_credentials\"}\n" {
				t.Fatalf("unexpected response: %d %s", response.Code, response.Body.String())
			}
		})
	}

	createTestCaptcha(t, s, "single-use", "ABC123")
	if response := performLogin(s, testAdminUsername, "wrong", "single-use", "ABC123"); response.Code != http.StatusUnauthorized {
		t.Fatalf("first use = %d", response.Code)
	}
	if response := performLogin(s, testAdminUsername, testAdminPassword, "single-use", "ABC123"); response.Code != http.StatusUnauthorized {
		t.Fatalf("reused captcha = %d", response.Code)
	}
}

func TestAdminLoginRateLimitsAfterTenAttemptsPerIP(t *testing.T) {
	s := testServer(t)
	for index := 0; index < 11; index++ {
		captchaID := "rate-" + string(rune('a'+index))
		createTestCaptcha(t, s, captchaID, "ABC123")
		response := performLogin(s, testAdminUsername, testAdminPassword, captchaID, "wrong")
		if index < 10 && response.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d = %d", index+1, response.Code)
		}
		if index == 10 && response.Code != http.StatusTooManyRequests {
			t.Fatalf("attempt %d = %d", index+1, response.Code)
		}
	}
}

func TestAdminLoginThenLogout(t *testing.T) {
	s := testServer(t)
	createTestCaptcha(t, s, "logout", "ABC123")
	login := performLogin(s, testAdminUsername, testAdminPassword, "logout", "ABC123")
	out := httptest.NewRecorder()
	logout := httptest.NewRequest(http.MethodDelete, "/admin/session", nil)
	logout.AddCookie(login.Result().Cookies()[0])
	s.ServeHTTP(out, logout)
	if out.Code != http.StatusNoContent {
		t.Fatalf("logout=%d", out.Code)
	}
}

func createTestCaptcha(t *testing.T, s *Server, id, answer string) {
	t.Helper()
	now := time.Now().UTC()
	if err := s.deps.Store.CreateLoginCaptcha(context.Background(), store.LoginCaptcha{
		ID: id, AnswerHash: auth.HashSecret(answer), CreatedAt: now, ExpiresAt: now.Add(5 * time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
}

func performLogin(s *Server, username, password, captchaID, answer string) *httptest.ResponseRecorder {
	body := `{"username":` + strconvQuote(username) + `,"password":` + strconvQuote(password) + `,"captcha_id":` + strconvQuote(captchaID) + `,"captcha_answer":` + strconvQuote(answer) + `}`
	out := httptest.NewRecorder()
	s.ServeHTTP(out, httptest.NewRequest(http.MethodPost, "/admin/session", strings.NewReader(body)))
	return out
}

func strconvQuote(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func TestUpdateSettingsPersistsAppliesAndSurvivesRestart(t *testing.T) {
	s := testServer(t)
	createTestCaptcha(t, s, "settings-captcha", "ABC123")
	login := performLogin(s, testAdminUsername, testAdminPassword, "settings-captcha", "ABC123")
	if login.Code != http.StatusNoContent {
		t.Fatalf("login code=%d", login.Code)
	}
	cookie := login.Result().Cookies()[0]

	update := httptest.NewRequest(http.MethodPut, "/admin/settings", strings.NewReader(`{"proxy_rate_per_minute":240,"log_retention_days":14}`))
	update.AddCookie(cookie)
	out := httptest.NewRecorder()
	s.ServeHTTP(out, update)
	if out.Code != http.StatusOK {
		t.Fatalf("update code=%d body=%s", out.Code, out.Body.String())
	}
	if s.proxyLimit.Limit() != 240 || s.logRetentionDays != 14 {
		t.Fatalf("runtime settings not applied: proxy=%d retention=%d", s.proxyLimit.Limit(), s.logRetentionDays)
	}
	var body struct {
		ProxyRatePerMinute int `json:"proxy_rate_per_minute"`
		LogRetentionDays   int `json:"log_retention_days"`
		AdminRatePerMinute int `json:"admin_rate_per_minute"`
	}
	if err := json.NewDecoder(out.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.ProxyRatePerMinute != 240 || body.LogRetentionDays != 14 || body.AdminRatePerMinute != 120 {
		t.Fatalf("unexpected settings response: %+v", body)
	}

	restarted := New(s.deps)
	if restarted.proxyLimit.Limit() != 240 || restarted.logRetentionDays != 14 {
		t.Fatalf("stored overrides not loaded on restart: proxy=%d retention=%d", restarted.proxyLimit.Limit(), restarted.logRetentionDays)
	}

	invalid := httptest.NewRequest(http.MethodPut, "/admin/settings", strings.NewReader(`{"proxy_rate_per_minute":0}`))
	invalid.AddCookie(cookie)
	out = httptest.NewRecorder()
	s.ServeHTTP(out, invalid)
	if out.Code != http.StatusBadRequest {
		t.Fatalf("invalid update code=%d", out.Code)
	}
}

func TestUpdateCursorTokenPersistsAppliesAndMasks(t *testing.T) {
	s := testServer(t)
	createTestCaptcha(t, s, "token-captcha", "ABC123")
	login := performLogin(s, testAdminUsername, testAdminPassword, "token-captcha", "ABC123")
	cookie := login.Result().Cookies()[0]

	update := httptest.NewRequest(http.MethodPut, "/admin/settings", strings.NewReader(`{"cursor_token":"cursor-token-abcdef123456"}`))
	update.AddCookie(cookie)
	out := httptest.NewRecorder()
	s.ServeHTTP(out, update)
	if out.Code != http.StatusOK {
		t.Fatalf("update code=%d body=%s", out.Code, out.Body.String())
	}
	if s.deps.Proxy.Token() != "cursor-token-abcdef123456" {
		t.Fatalf("runtime token = %q", s.deps.Proxy.Token())
	}
	if strings.Contains(out.Body.String(), "cursor-token-abcdef123456") {
		t.Fatal("settings response leaked the full token")
	}
	var body struct {
		CursorTokenSet    bool   `json:"cursor_token_set"`
		CursorTokenMasked string `json:"cursor_token_masked"`
	}
	if err := json.NewDecoder(out.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if !body.CursorTokenSet || body.CursorTokenMasked != "curs••••3456" {
		t.Fatalf("unexpected masked token: %+v", body)
	}

	restartDeps := s.deps
	restartDeps.Proxy = proxy.New("stale-token", nil, map[string]string{})
	restarted := New(restartDeps)
	if restarted.deps.Proxy.Token() != "cursor-token-abcdef123456" {
		t.Fatalf("stored token not loaded on restart: %q", restarted.deps.Proxy.Token())
	}

	invalid := httptest.NewRequest(http.MethodPut, "/admin/settings", strings.NewReader(`{"cursor_token":"short"}`))
	invalid.AddCookie(cookie)
	out = httptest.NewRecorder()
	s.ServeHTTP(out, invalid)
	if out.Code != http.StatusBadRequest {
		t.Fatalf("invalid token update code=%d", out.Code)
	}
}
