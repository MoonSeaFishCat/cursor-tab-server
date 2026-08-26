package proxy

import (
	"bytes"
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
	"sync"
	"time"
)

const maxRequestBody = 10 << 20

var hopByHop = map[string]struct{}{"connection": {}, "proxy-connection": {}, "keep-alive": {}, "proxy-authenticate": {}, "proxy-authorization": {}, "te": {}, "trailer": {}, "transfer-encoding": {}, "upgrade": {}}

func DefaultTargets() map[string]string {
	return map[string]string{
		"/aiserver.v1.AiService/StreamCpp": "https://api4.cursor.sh:443/aiserver.v1.AiService/StreamCpp", "/aiserver.v1.AiService/StreamNextCursorPrediction": "https://api4.cursor.sh:443/aiserver.v1.AiService/StreamNextCursorPrediction", "/aiserver.v1.AiService/GetCppEditClassification": "https://api4.cursor.sh:443/aiserver.v1.AiService/GetCppEditClassification", "/aiserver.v1.AiService/RefreshTabContext": "https://api2.cursor.sh:443/aiserver.v1.AiService/RefreshTabContext", "/aiserver.v1.AiService/CppConfig": "https://api4.cursor.sh:443/aiserver.v1.AiService/CppConfig", "/aiserver.v1.AiService/CppEditHistoryStatus": "https://api2.cursor.sh:443/aiserver.v1.AiService/CppEditHistoryStatus", "/aiserver.v1.AiService/CppAppend": "https://api3.cursor.sh:443/aiserver.v1.AiService/CppAppend", "/aiserver.v1.AiService/CppEditHistoryAppend": "https://api3.cursor.sh:443/aiserver.v1.AiService/CppEditHistoryAppend", "/aiserver.v1.CppService/AvailableModels": "https://api3.cursor.sh:443/aiserver.v1.CppService/AvailableModels", "/aiserver.v1.CppService/RecordCppFate": "https://api2.cursor.sh:443/aiserver.v1.CppService/RecordCppFate", "/aiserver.v1.AiService/ReportAiCodeChangeMetrics": "https://api2.cursor.sh:443/aiserver.v1.AiService/ReportAiCodeChangeMetrics", "/aiserver.v1.AiService/WriteGitCommitMessage": "https://api2.cursor.sh:443/aiserver.v1.AiService/WriteGitCommitMessage", "/aiserver.v1.AiService/WriteGitBranchName": "https://api2.cursor.sh:443/aiserver.v1.AiService/WriteGitBranchName", "/aiserver.v1.FileSyncService/FSSyncFile": "https://api4.cursor.sh:443/aiserver.v1.FileSyncService/FSSyncFile", "/aiserver.v1.FileSyncService/FSIsEnabledForUser": "https://api4.cursor.sh:443/aiserver.v1.FileSyncService/FSIsEnabledForUser", "/aiserver.v1.FileSyncService/FSConfig": "https://api4.cursor.sh:443/aiserver.v1.FileSyncService/FSConfig", "/aiserver.v1.FileSyncService/FSUploadFile": "https://api4.cursor.sh:443/aiserver.v1.FileSyncService/FSUploadFile", "/aiserver.v1.DashboardService/GetEffectiveUserPlugins": "https://api2.cursor.sh:443/aiserver.v1.DashboardService/GetEffectiveUserPlugins",
	}
}

type Handler struct {
	mu      sync.RWMutex
	token   string
	client  *http.Client
	targets map[string]string
}

func New(token string, client *http.Client, targets map[string]string) *Handler {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	clone := map[string]string{}
	for k, v := range targets {
		clone[k] = v
	}
	return &Handler{token: strings.TrimSpace(token), client: client, targets: clone}
}
func (h *Handler) Allowed(path string) bool { _, ok := h.targets[path]; return ok }

// Token returns the Cursor access token currently used for upstream requests.
func (h *Handler) Token() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.token
}

// SetToken swaps the Cursor access token at runtime so credential rotation
// does not require a restart.
func (h *Handler) SetToken(token string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.token = strings.TrimSpace(token)
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	target, ok := h.targets[r.URL.Path]
	if !ok {
		http.NotFound(w, r)
		return
	}
	u, err := url.Parse(target)
	if err != nil {
		http.Error(w, "bad upstream", 502)
		return
	}
	u.RawQuery = r.URL.RawQuery
	query := u.Query()
	query.Del("key")
	u.RawQuery = query.Encode()
	var body []byte
	if r.Body != nil && r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodDelete {
		data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxRequestBody))
		if err != nil {
			http.Error(w, "request body too large", 413)
			return
		}
		body = data
	}
	req, err := http.NewRequestWithContext(r.Context(), r.Method, u.String(), bytes.NewReader(body))
	if err != nil {
		http.Error(w, "create upstream request", 502)
		return
	}
	copyHeaders(req.Header, r.Header)
	auth := bearer(h.Token())
	req.Header.Set("Authorization", auth)
	req.Header.Set("x-cursor-checksum", BuildChecksum(auth, time.Now()))
	req.Header.Del("X-API-Key")
	req.Host = u.Host
	if len(body) > 0 {
		req.Header.Set("Content-Length", strconv.Itoa(len(body)))
	}
	resp, err := h.client.Do(req)
	if err != nil {
		http.Error(w, "upstream request failed", 502)
		return
	}
	defer resp.Body.Close()
	copyHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	_, _ = copyStream(w, resp.Body)
}
func copyHeaders(dst, src http.Header) {
	connection := map[string]struct{}{}
	for _, v := range src.Values("Connection") {
		for _, name := range strings.Split(v, ",") {
			connection[strings.ToLower(strings.TrimSpace(name))] = struct{}{}
		}
	}
	for k, values := range src {
		lower := strings.ToLower(k)
		if _, skip := hopByHop[lower]; skip {
			continue
		}
		if _, skip := connection[lower]; skip {
			continue
		}
		for _, v := range values {
			dst.Add(k, v)
		}
	}
}
func copyStream(w io.Writer, r io.Reader) (int64, error) {
	buf := make([]byte, 32*1024)
	var total int64
	for {
		n, e := r.Read(buf)
		if n > 0 {
			m, we := w.Write(buf[:n])
			total += int64(m)
			if we != nil {
				return total, we
			}
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
		if e != nil {
			if errors.Is(e, io.EOF) {
				return total, nil
			}
			return total, e
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
	for i := range bytes {
		shift := uint((len(bytes) - 1 - i) * 8)
		bytes[i] = byte(new(big.Int).Rsh(value, shift).Uint64() & 0xff)
	}
	seed := 165
	for i := range bytes {
		current := (int(bytes[i]^byte(seed)) + i) & 0xff
		bytes[i] = byte(current)
		seed = current
	}
	prefix := strings.TrimRight(base64.StdEncoding.EncodeToString(bytes), "=")
	sum := sha256.Sum256([]byte(strings.TrimSpace(authorization)))
	return prefix + fmt.Sprintf("%x", sum)[:32]
}
