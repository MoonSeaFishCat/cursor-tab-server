package proxy

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const maxRequestBody = 10 << 20

var hopByHop = map[string]struct{}{"connection": {}, "proxy-connection": {}, "keep-alive": {}, "proxy-authenticate": {}, "proxy-authorization": {}, "te": {}, "trailer": {}, "transfer-encoding": {}, "upgrade": {}}

func DefaultTargets() map[string]string {
	return map[string]string{
		"/aiserver.v1.AiService/StreamCpp": "https://api4.cursor.sh:443/aiserver.v1.AiService/StreamCpp", "/aiserver.v1.AiService/StreamNextCursorPrediction": "https://api4.cursor.sh:443/aiserver.v1.AiService/StreamNextCursorPrediction", "/aiserver.v1.AiService/GetCppEditClassification": "https://api4.cursor.sh:443/aiserver.v1.AiService/GetCppEditClassification", "/aiserver.v1.AiService/RefreshTabContext": "https://api2.cursor.sh:443/aiserver.v1.AiService/RefreshTabContext", "/aiserver.v1.AiService/CppConfig": "https://api4.cursor.sh:443/aiserver.v1.AiService/CppConfig", "/aiserver.v1.AiService/CppEditHistoryStatus": "https://api2.cursor.sh:443/aiserver.v1.AiService/CppEditHistoryStatus", "/aiserver.v1.AiService/CppAppend": "https://api3.cursor.sh:443/aiserver.v1.AiService/CppAppend", "/aiserver.v1.AiService/CppEditHistoryAppend": "https://api3.cursor.sh:443/aiserver.v1.AiService/CppEditHistoryAppend", "/aiserver.v1.CppService/AvailableModels": "https://api3.cursor.sh:443/aiserver.v1.CppService/AvailableModels", "/aiserver.v1.CppService/RecordCppFate": "https://api2.cursor.sh:443/aiserver.v1.CppService/RecordCppFate", "/aiserver.v1.AiService/ReportAiCodeChangeMetrics": "https://api2.cursor.sh:443/aiserver.v1.AiService/ReportAiCodeChangeMetrics", "/aiserver.v1.AiService/WriteGitCommitMessage": "https://api2.cursor.sh:443/aiserver.v1.AiService/WriteGitCommitMessage", "/aiserver.v1.AiService/WriteGitBranchName": "https://api2.cursor.sh:443/aiserver.v1.AiService/WriteGitBranchName", "/aiserver.v1.FileSyncService/FSSyncFile": "https://api4.cursor.sh:443/aiserver.v1.FileSyncService/FSSyncFile", "/aiserver.v1.FileSyncService/FSIsEnabledForUser": "https://api4.cursor.sh:443/aiserver.v1.FileSyncService/FSIsEnabledForUser", "/aiserver.v1.FileSyncService/FSConfig": "https://api4.cursor.sh:443/aiserver.v1.FileSyncService/FSConfig", "/aiserver.v1.FileSyncService/FSUploadFile": "https://api4.cursor.sh:443/aiserver.v1.FileSyncService/FSUploadFile", "/aiserver.v1.DashboardService/GetEffectiveUserPlugins": "https://api2.cursor.sh:443/aiserver.v1.DashboardService/GetEffectiveUserPlugins",
	}
}

type Pool interface {
	Acquire(context.Context, string, string) (id, token string, release func(), err error)
	MarkSuccess(context.Context, string)
	MarkFailure(context.Context, string, int)
}

type staticPool struct{ token string }

func (p *staticPool) Acquire(context.Context, string, string) (string, string, func(), error) {
	return "static", p.token, func() {}, nil
}
func (*staticPool) MarkSuccess(context.Context, string)      {}
func (*staticPool) MarkFailure(context.Context, string, int) {}

type Handler struct {
	pool    Pool
	client  *http.Client
	targets map[string]string
}

func New(token string, client *http.Client, targets map[string]string) *Handler {
	return NewWithPool(&staticPool{token: strings.TrimSpace(token)}, client, targets)
}

func NewWithPool(pool Pool, client *http.Client, targets map[string]string) *Handler {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	clone := map[string]string{}
	for key, value := range targets {
		clone[key] = value
	}
	return &Handler{pool: pool, client: client, targets: clone}
}

func (h *Handler) Allowed(path string) bool { _, ok := h.targets[path]; return ok }

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.ServeForSubject(w, r, "anonymous")
}

func (h *Handler) ServeForSubject(w http.ResponseWriter, r *http.Request, subject string) {
	target, ok := h.targets[r.URL.Path]
	if !ok {
		http.NotFound(w, r)
		return
	}
	body, err := readBody(w, r)
	if err != nil {
		return
	}
	leaseID, token, release, err := h.pool.Acquire(r.Context(), subject, "")
	if err != nil {
		http.Error(w, "no healthy cursor token available", http.StatusServiceUnavailable)
		return
	}
	response, err := h.send(r, target, body, token)
	if err != nil {
		release()
		http.Error(w, "upstream request failed", http.StatusBadGateway)
		return
	}
	if retryableTokenStatus(response.StatusCode) {
		failedID := leaseID
		h.pool.MarkFailure(r.Context(), failedID, response.StatusCode)
		release()
		_ = response.Body.Close()
		leaseID, token, release, err = h.pool.Acquire(r.Context(), subject, failedID)
		if err != nil {
			http.Error(w, "no healthy cursor token available", http.StatusServiceUnavailable)
			return
		}
		response, err = h.send(r, target, body, token)
		if err != nil {
			release()
			http.Error(w, "upstream request failed", http.StatusBadGateway)
			return
		}
	}
	defer release()
	defer response.Body.Close()
	if retryableTokenStatus(response.StatusCode) {
		h.pool.MarkFailure(r.Context(), leaseID, response.StatusCode)
	} else {
		h.pool.MarkSuccess(r.Context(), leaseID)
	}
	copyHeaders(w.Header(), response.Header)
	w.WriteHeader(response.StatusCode)
	_, _ = copyStream(w, response.Body)
}

func (h *Handler) send(r *http.Request, target string, body []byte, token string) (*http.Response, error) {
	u, err := url.Parse(target)
	if err != nil {
		return nil, err
	}
	u.RawQuery = r.URL.RawQuery
	query := u.Query()
	query.Del("key")
	u.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(r.Context(), r.Method, u.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	copyHeaders(req.Header, r.Header)
	authorization := bearer(token)
	req.Header.Set("Authorization", authorization)
	req.Header.Set("x-cursor-checksum", BuildChecksum(authorization, time.Now()))
	req.Header.Del("X-API-Key")
	req.Host = u.Host
	if len(body) > 0 {
		req.Header.Set("Content-Length", strconv.Itoa(len(body)))
	}
	return h.client.Do(req)
}

func readBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	if r.Body == nil || r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodDelete {
		return nil, nil
	}
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxRequestBody))
	if err != nil {
		http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
		return nil, err
	}
	return data, nil
}

func retryableTokenStatus(status int) bool { return status == 401 || status == 403 || status == 429 }

func copyHeaders(dst, src http.Header) {
	connection := map[string]struct{}{}
	for _, value := range src.Values("Connection") {
		for _, name := range strings.Split(value, ",") {
			connection[strings.ToLower(strings.TrimSpace(name))] = struct{}{}
		}
	}
	for key, values := range src {
		lower := strings.ToLower(key)
		if _, skip := hopByHop[lower]; skip {
			continue
		}
		if _, skip := connection[lower]; skip {
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func copyStream(w io.Writer, r io.Reader) (int64, error) {
	buffer := make([]byte, 32*1024)
	var total int64
	for {
		read, readErr := r.Read(buffer)
		if read > 0 {
			written, writeErr := w.Write(buffer[:read])
			total += int64(written)
			if writeErr != nil {
				return total, writeErr
			}
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return total, nil
			}
			return total, readErr
		}
	}
}

func bearer(token string) string {
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		return token
	}
	return "Bearer " + token
}

func BuildChecksum(authorization string, now time.Time) string {
	timestamp := now.UnixMilli() / 1_000_000
	bytes := make([]byte, 6)
	value := big.NewInt(timestamp)
	for index := range bytes {
		shift := uint((len(bytes) - 1 - index) * 8)
		bytes[index] = byte(new(big.Int).Rsh(value, shift).Uint64() & 0xff)
	}
	seed := 165
	for index := range bytes {
		current := (int(bytes[index]^byte(seed)) + index) & 0xff
		bytes[index] = byte(current)
		seed = current
	}
	prefix := strings.TrimRight(base64.StdEncoding.EncodeToString(bytes), "=")
	sum := sha256.Sum256([]byte(strings.TrimSpace(authorization)))
	return prefix + fmt.Sprintf("%x", sum)[:32]
}
