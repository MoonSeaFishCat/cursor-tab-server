package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProxyOverridesClientAuthorizationAndStreamsUpstream(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer cursor-token" {
			t.Fatalf("authorization=%q", got)
		}
		if r.URL.Query().Get("key") != "" {
			t.Fatal("API key query parameter was forwarded upstream")
		}
		if r.URL.Query().Get("context") != "editor" {
			t.Fatalf("context=%q", r.URL.Query().Get("context"))
		}
		if r.Header.Get("x-cursor-checksum") == "" {
			t.Fatal("checksum")
		}
		_, _ = io.WriteString(w, "streamed")
	}))
	defer up.Close()
	h := New("cursor-token", up.Client(), map[string]string{"/allowed": up.URL + "/allowed"})
	out := httptest.NewRecorder()
	h.ServeHTTP(out, httptest.NewRequest(http.MethodPost, "/allowed?context=editor&key=client-api-key", strings.NewReader("body")))
	if out.Code != 200 || out.Body.String() != "streamed" {
		t.Fatalf("%d %q", out.Code, out.Body.String())
	}
}
