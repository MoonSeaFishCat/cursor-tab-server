package httpapi

import (
	"embed"
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed assets/*
var embeddedAssets embed.FS

func pathAPIKey(requestPath string) (apiKey, upstreamPath string, ok bool) {
	parts := strings.Split(strings.TrimPrefix(requestPath, "/"), "/")
	if len(parts) < 2 {
		return "", "", false
	}
	apiKey = strings.TrimPrefix(parts[0], "key=")
	if !strings.HasPrefix(apiKey, "cts_") || len(apiKey) <= len("cts_") {
		return "", "", false
	}
	return apiKey, "/" + strings.Join(parts[1:], "/"), true
}

func (s *Server) serveApplication(w http.ResponseWriter, r *http.Request) {
	if s.deps.Proxy.Allowed(r.URL.Path) {
		s.proxyOnly(w, r)
		return
	}
	if apiKey, upstreamPath, ok := pathAPIKey(r.URL.Path); ok && s.deps.Proxy.Allowed(upstreamPath) {
		proxyRequest := r.Clone(r.Context())
		proxyRequest.URL.Path = upstreamPath
		proxyRequest.URL.RawPath = ""
		proxyRequest.Header.Set("X-API-Key", apiKey)
		s.proxyOnly(w, proxyRequest)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.NotFound(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/admin/") || r.URL.Path == "/healthz" {
		http.NotFound(w, r)
		return
	}
	assets, err := fs.Sub(embeddedAssets, "assets")
	if err != nil {
		http.Error(w, "application assets unavailable", http.StatusInternalServerError)
		return
	}
	requested := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if requested != "" {
		if _, err := fs.Stat(assets, requested); err == nil {
			requestCopy := r.Clone(r.Context())
			requestCopy.URL.Path = "/" + requested
			http.FileServer(http.FS(assets)).ServeHTTP(w, requestCopy)
			return
		}
	}
	index, err := assets.Open("index.html")
	if err != nil {
		http.Error(w, "application assets unavailable", http.StatusInternalServerError)
		return
	}
	defer index.Close()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.Copy(w, index)
}
