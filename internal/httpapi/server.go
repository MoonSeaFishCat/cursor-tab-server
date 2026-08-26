package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"cursor-tab-server/internal/audit"
	"cursor-tab-server/internal/auth"
	"cursor-tab-server/internal/captcha"
	"cursor-tab-server/internal/config"
	"cursor-tab-server/internal/proxy"
	"cursor-tab-server/internal/ratelimit"
	"cursor-tab-server/internal/store"
)

const (
	defaultCaptchaRatePerMinute = 20
	defaultLoginRatePerMinute   = 10
	defaultLogRetentionDays     = 30
)

type Dependencies struct {
	Config    config.Config
	Store     *store.Store
	Proxy     *proxy.Handler
	Audit     *audit.Service
	StartedAt time.Time
}

type Server struct {
	deps                                 Dependencies
	proxyLimit, adminLimit, captchaLimit *ratelimit.Limiter
	loginLimit                           *ratelimit.Limiter
	logRetentionDays                     int
	mux                                  *http.ServeMux
}

func New(d Dependencies) *Server {
	s := &Server{
		deps:             d,
		proxyLimit:       ratelimit.New(d.Config.ProxyRatePerMinute, time.Minute),
		adminLimit:       ratelimit.New(d.Config.AdminRatePerMinute, time.Minute),
		captchaLimit:     ratelimit.New(defaultCaptchaRatePerMinute, time.Minute),
		loginLimit:       ratelimit.New(defaultLoginRatePerMinute, time.Minute),
		logRetentionDays: defaultLogRetentionDays,
		mux:              http.NewServeMux(),
	}
	s.applyStoredSettings()
	s.routes()
	return s
}

// LogRetentionDays reports the currently configured audit log retention window.
func (s *Server) LogRetentionDays() int { return s.logRetentionDays }

// applyStoredSettings loads online-editable overrides persisted in the
// database so runtime configuration survives restarts. Invalid or missing
// overrides keep their defaults.
func (s *Server) applyStoredSettings() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	overrides := []struct {
		name  string
		apply func(int)
	}{
		{store.SettingProxyRatePerMinute, func(v int) { s.proxyLimit.SetLimit(v) }},
		{store.SettingAdminRatePerMinute, func(v int) { s.adminLimit.SetLimit(v) }},
		{store.SettingCaptchaRatePerMinute, func(v int) { s.captchaLimit.SetLimit(v) }},
		{store.SettingLoginRatePerMinute, func(v int) { s.loginLimit.SetLimit(v) }},
		{store.SettingLogRetentionDays, func(v int) { s.logRetentionDays = v }},
	}
	for _, override := range overrides {
		value, ok, err := s.deps.Store.SettingInt(ctx, override.name)
		if err == nil && ok {
			override.apply(value)
		}
	}
	if token, ok, err := s.deps.Store.SettingString(ctx, store.SettingCursorToken); err == nil && ok && strings.TrimSpace(token) != "" {
		s.deps.Proxy.SetToken(token)
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.mux.ServeHTTP(w, r) }

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	s.mux.HandleFunc("GET /admin/captcha", s.createCaptcha)
	s.mux.HandleFunc("POST /admin/session", s.login)
	s.mux.HandleFunc("GET /admin/session", s.adminOnly(s.sessionStatus))
	s.mux.HandleFunc("DELETE /admin/session", s.adminOnly(s.logout))
	s.mux.HandleFunc("GET /admin/status", s.adminOnly(s.status))
	s.mux.HandleFunc("GET /admin/dashboard", s.adminOnly(s.dashboard))
	s.mux.HandleFunc("GET /admin/settings", s.adminOnly(s.settings))
	s.mux.HandleFunc("PUT /admin/settings", s.adminOnly(s.updateSettings))
	s.mux.HandleFunc("GET /admin/api-keys", s.adminOnly(s.listKeys))
	s.mux.HandleFunc("POST /admin/api-keys", s.adminOnly(s.createKey))
	s.mux.HandleFunc("POST /admin/api-keys/{id}/disable", s.adminOnly(s.disableKey))
	s.mux.HandleFunc("GET /admin/audit-logs", s.adminOnly(s.listAudit))
	s.mux.HandleFunc("/", s.serveApplication)
}

func (s *Server) createCaptcha(w http.ResponseWriter, r *http.Request) {
	now := time.Now().UTC()
	if !s.captchaLimit.Allow(clientIP(r), now) {
		writeError(w, http.StatusTooManyRequests, "rate_limited")
		return
	}
	if err := s.deps.Store.DeleteExpiredLoginCaptchas(r.Context(), now); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	challenge, err := captcha.Generate(now)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	if err := s.deps.Store.CreateLoginCaptcha(r.Context(), store.LoginCaptcha{
		ID: challenge.ID, AnswerHash: challenge.AnswerHash, CreatedAt: now, ExpiresAt: challenge.ExpiresAt,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"captcha_id": challenge.ID,
		"image":      captcha.DataURL(challenge.Image),
		"expires_at": challenge.ExpiresAt,
	})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Username      string `json:"username"`
		Password      string `json:"password"`
		CaptchaID     string `json:"captcha_id"`
		CaptchaAnswer string `json:"captcha_answer"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input) != nil ||
		strings.TrimSpace(input.Username) == "" || input.Password == "" ||
		strings.TrimSpace(input.CaptchaID) == "" || strings.TrimSpace(input.CaptchaAnswer) == "" {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}

	now := time.Now().UTC()
	if !s.loginLimit.Allow(clientIP(r), now) {
		writeError(w, http.StatusTooManyRequests, "rate_limited")
		return
	}
	challenge, err := s.deps.Store.ConsumeLoginCaptcha(r.Context(), input.CaptchaID, now)
	if err != nil || !auth.VerifySecret(strings.ToUpper(strings.TrimSpace(input.CaptchaAnswer)), challenge.AnswerHash) ||
		!verifyCredentials(input.Username, input.Password, s.deps.Config.AdminUsername, s.deps.Config.AdminPassword) {
		writeError(w, http.StatusUnauthorized, "invalid_credentials")
		return
	}

	plain, hash, err := auth.NewSession()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	expires := now.Add(8 * time.Hour)
	if err = s.deps.Store.CreateAdminSession(r.Context(), store.AdminSession{ID: randomID(), SecretHash: hash, CreatedAt: now, ExpiresAt: expires}); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	http.SetCookie(w, auth.SessionCookie(plain, expires))
	w.WriteHeader(http.StatusNoContent)
}

func verifyCredentials(username, password, expectedUsername, expectedPassword string) bool {
	usernameMatches := subtle.ConstantTimeCompare([]byte(username), []byte(expectedUsername)) == 1
	passwordMatches := auth.VerifySecret(password, auth.HashSecret(expectedPassword))
	return usernameMatches && passwordMatches
}

func (s *Server) adminOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if !s.adminLimit.Allow(ip, time.Now()) {
			writeError(w, http.StatusTooManyRequests, "rate_limited")
			return
		}
		cookie, err := r.Cookie(auth.SessionCookieName)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if _, err = s.deps.Store.FindActiveAdminSession(r.Context(), auth.HashSecret(cookie.Value), time.Now()); err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next(w, r)
	}
}

func (s *Server) sessionStatus(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	cookie, _ := r.Cookie(auth.SessionCookieName)
	_ = s.deps.Store.DeleteAdminSession(r.Context(), auth.HashSecret(cookie.Value))
	http.SetCookie(w, auth.ClearSessionCookie())
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	if err := s.deps.Store.PingContext(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, "database_unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"started_at": s.deps.StartedAt.UTC(), "database": "ok", "proxy_rate_per_minute": s.proxyLimit.Limit(), "admin_rate_per_minute": s.adminLimit.Limit(), "log_retention_days": s.logRetentionDays})
}

func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	now := time.Now().UTC()
	summary, err := s.deps.Audit.Summary(r.Context(), now)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"requests_24h":        summary.Requests24H,
		"errors_24h":          summary.Errors24H,
		"average_latency_ms":  summary.AverageLatencyMS,
		"success_rate":        summary.SuccessRate,
		"active_keys_24h":     summary.ActiveKeys24H,
		"status_distribution": summary.StatusDistribution,
		"server":              map[string]any{"started_at": s.deps.StartedAt.UTC(), "listen_addr": s.deps.Config.ListenAddr},
	})
}

func (s *Server) settings(w http.ResponseWriter, r *http.Request) {
	token := s.deps.Proxy.Token()
	writeJSON(w, http.StatusOK, map[string]any{
		"listen_addr":             s.deps.Config.ListenAddr,
		"database_path":           s.deps.Config.DatabasePath,
		"proxy_rate_per_minute":   s.proxyLimit.Limit(),
		"admin_rate_per_minute":   s.adminLimit.Limit(),
		"log_retention_days":      s.logRetentionDays,
		"captcha_rate_per_minute": s.captchaLimit.Limit(),
		"login_rate_per_minute":   s.loginLimit.Limit(),
		"cursor_token_set":        token != "",
		"cursor_token_masked":     maskToken(token),
	})
}

// maskToken keeps only the outer edges of a credential visible so the admin
// UI can confirm which token is active without exposing it.
func maskToken(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	if len(token) <= 8 {
		return "••••"
	}
	return token[:4] + "••••" + token[len(token)-4:]
}

// updateSettings persists online configuration changes and applies them to
// the running server immediately.
func (s *Server) updateSettings(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ProxyRatePerMinute   *int    `json:"proxy_rate_per_minute"`
		AdminRatePerMinute   *int    `json:"admin_rate_per_minute"`
		CaptchaRatePerMinute *int    `json:"captcha_rate_per_minute"`
		LoginRatePerMinute   *int    `json:"login_rate_per_minute"`
		LogRetentionDays     *int    `json:"log_retention_days"`
		CursorToken          *string `json:"cursor_token"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&input) != nil {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	if input.CursorToken != nil {
		token := strings.TrimSpace(*input.CursorToken)
		if len(token) < 10 || len(token) > 4096 {
			writeError(w, http.StatusBadRequest, "invalid_cursor_token")
			return
		}
		if err := s.deps.Store.SaveSettingString(r.Context(), store.SettingCursorToken, token); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error")
			return
		}
		s.deps.Proxy.SetToken(token)
	}
	type editable struct {
		name  string
		value *int
		min   int
		max   int
		apply func(int)
	}
	fields := []editable{
		{store.SettingProxyRatePerMinute, input.ProxyRatePerMinute, 1, 100000, func(v int) { s.proxyLimit.SetLimit(v) }},
		{store.SettingAdminRatePerMinute, input.AdminRatePerMinute, 1, 100000, func(v int) { s.adminLimit.SetLimit(v) }},
		{store.SettingCaptchaRatePerMinute, input.CaptchaRatePerMinute, 1, 100000, func(v int) { s.captchaLimit.SetLimit(v) }},
		{store.SettingLoginRatePerMinute, input.LoginRatePerMinute, 1, 1000, func(v int) { s.loginLimit.SetLimit(v) }},
		{store.SettingLogRetentionDays, input.LogRetentionDays, 1, 3650, func(v int) { s.logRetentionDays = v }},
	}
	for _, field := range fields {
		if field.value != nil && (*field.value < field.min || *field.value > field.max) {
			writeError(w, http.StatusBadRequest, "invalid_"+field.name)
			return
		}
	}
	for _, field := range fields {
		if field.value == nil {
			continue
		}
		if err := s.deps.Store.SaveSettingInt(r.Context(), field.name, *field.value); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error")
			return
		}
		field.apply(*field.value)
	}
	s.settings(w, r)
}

func (s *Server) createKey(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name string `json:"name"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input) != nil || strings.TrimSpace(input.Name) == "" {
		writeError(w, http.StatusBadRequest, "name_required")
		return
	}
	plain, prefix, hash, err := auth.CreateAPIKey(input.Name)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_key")
		return
	}
	key := store.APIKey{ID: randomID(), Name: strings.TrimSpace(input.Name), Prefix: prefix, SecretHash: hash, CreatedAt: time.Now().UTC()}
	if err = s.deps.Store.CreateAPIKey(r.Context(), key); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": key.ID, "name": key.Name, "prefix": key.Prefix, "secret": plain, "created_at": key.CreatedAt})
}

func (s *Server) listKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := s.deps.Store.ListAPIKeys(r.Context(), queryLimit(r), queryOffset(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	activity, err := s.deps.Audit.ActivityByKey(r.Context(), time.Now().UTC())
	if err != nil {
		// Activity is supplementary console data. A failed aggregate must not
		// prevent administrators from listing or disabling their API keys.
		log.Printf("load API key activity: %v", err)
		activity = map[string]store.APIKeyActivity{}
	}
	items := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		items = append(items, map[string]any{
			"id": key.ID, "name": key.Name, "prefix": key.Prefix, "created_at": key.CreatedAt,
			"disabled_at": key.DisabledAt, "last_used_at": key.LastUsedAt, "activity": activity[key.ID],
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "limit": queryLimit(r), "offset": queryOffset(r)})
}

func (s *Server) disableKey(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "key_id_required")
		return
	}
	if err := s.deps.Store.DisableAPIKey(r.Context(), id, time.Now()); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listAudit(w http.ResponseWriter, r *http.Request) {
	status, _ := strconv.Atoi(r.URL.Query().Get("status"))
	page, err := s.deps.Audit.QueryPage(r.Context(), audit.Query{APIKeyID: r.URL.Query().Get("key_id"), Path: r.URL.Query().Get("path"), StatusCode: status, Limit: queryLimit(r), Offset: queryOffset(r)})
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_query")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": page.Items, "total": page.Total, "limit": queryLimit(r), "offset": queryOffset(r)})
}

func (s *Server) proxyOnly(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	apiKey := r.Header.Get("X-API-Key")
	if apiKey == "" {
		apiKey = r.URL.Query().Get("key")
	}
	key, err := s.deps.Store.FindActiveAPIKeyByHash(r.Context(), auth.HashSecret(apiKey))
	status, errorKind := 0, ""
	if err != nil {
		status, errorKind = http.StatusUnauthorized, "unauthorized"
		writeError(w, status, errorKind)
		s.record(r, key.ID, status, start, errorKind, 0, 0)
		return
	}
	subject := key.ID + "|" + clientIP(r)
	if !s.proxyLimit.Allow(subject, start) {
		status, errorKind = http.StatusTooManyRequests, "rate_limited"
		writeError(w, status, errorKind)
		s.record(r, key.ID, status, start, errorKind, 0, 0)
		return
	}
	_ = s.deps.Store.MarkAPIKeyUsed(r.Context(), key.ID, start)
	rec := &captureWriter{ResponseWriter: w, status: http.StatusOK}
	s.deps.Proxy.ServeHTTP(rec, r)
	s.record(r, key.ID, rec.status, start, "", r.ContentLength, rec.bytes)
}

func (s *Server) record(r *http.Request, keyID string, status int, start time.Time, errorKind string, requestBytes, responseBytes int64) {
	_ = s.deps.Audit.Record(context.Background(), audit.Record{OccurredAt: time.Now(), APIKeyID: keyID, SourceIP: clientIP(r), Method: r.Method, Path: r.URL.Path, StatusCode: status, DurationMS: time.Since(start).Milliseconds(), RequestBytes: max(requestBytes, 0), ResponseBytes: responseBytes, ErrorKind: errorKind})
}

type captureWriter struct {
	http.ResponseWriter
	status int
	bytes  int64
}

func (c *captureWriter) WriteHeader(code int) { c.status = code; c.ResponseWriter.WriteHeader(code) }
func (c *captureWriter) Write(data []byte) (int, error) {
	n, e := c.ResponseWriter.Write(data)
	c.bytes += int64(n)
	return n, e
}
func (c *captureWriter) Flush() {
	if f, ok := c.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"error": code})
}
func queryLimit(r *http.Request) int {
	v, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if v < 1 {
		v = 50
	}
	if v > 100 {
		v = 100
	}
	return v
}
func queryOffset(r *http.Request) int {
	v, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if v < 0 {
		v = 0
	}
	return v
}
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}
func randomID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
