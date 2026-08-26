package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBrowserRouteFallsBackToEmbeddedIndex(t *testing.T) {
	server := testServer(t)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/audit-logs", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), `<div id="root">`) {
		t.Fatal("embedded application index was not served")
	}
}
