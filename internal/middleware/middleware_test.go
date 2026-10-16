package middleware_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/farhanhrywn/task-management-service/internal/domain"
	"github.com/farhanhrywn/task-management-service/internal/middleware"
	"github.com/farhanhrywn/task-management-service/pkg/jwtutil"
	"github.com/farhanhrywn/task-management-service/pkg/logger"
	"github.com/farhanhrywn/task-management-service/pkg/response"
	"github.com/farhanhrywn/task-management-service/test/mocks"
)

func init() { gin.SetMode(gin.TestMode) }

// newEngine builds the production middleware chain with logs captured in buf.
func newEngine(buf *bytes.Buffer) *gin.Engine {
	r := gin.New()
	r.Use(
		middleware.RequestID(logger.New(buf, slog.LevelInfo)),
		middleware.Logger(),
		middleware.Recovery(),
		middleware.ErrorHandler(),
	)
	return r
}

func decodeError(t *testing.T, w *httptest.ResponseRecorder) response.ErrorBody {
	t.Helper()
	var body response.ErrorBody
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not the JSON error envelope: %v\n%s", err, w.Body.String())
	}
	return body
}

// UT-12: a panic in a handler becomes a formatted 500 without internals; the
// stack trace goes to the log only.
func TestRecovery_PanicReturns500WithoutStackTrace(t *testing.T) {
	var logs bytes.Buffer
	r := newEngine(&logs)
	r.GET("/boom", func(*gin.Context) { panic("secret internal detail") })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/boom", nil))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
	body := decodeError(t, w)
	if body.Status != 500 || body.Code != "INTERNAL_ERROR" || body.Timestamp == "" {
		t.Fatalf("unexpected error body: %+v", body)
	}
	if body.RequestID == "" || body.RequestID != w.Header().Get(middleware.HeaderRequestID) {
		t.Fatalf("request_id %q must match X-Request-ID %q", body.RequestID, w.Header().Get(middleware.HeaderRequestID))
	}
	raw := w.Body.String()
	for _, leak := range []string{"secret internal detail", "goroutine", ".go:"} {
		if strings.Contains(raw, leak) {
			t.Fatalf("response leaks %q: %s", leak, raw)
		}
	}

	lines := strings.Split(strings.TrimSpace(logs.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("want exactly one access log line, got %d:\n%s", len(lines), logs.String())
	}
	var entry map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &entry); err != nil {
		t.Fatalf("log is not JSON: %v", err)
	}
	if entry["level"] != "ERROR" || entry["request_id"] != body.RequestID || entry["status"] != float64(500) {
		t.Fatalf("unexpected access log: %v", entry)
	}
	if !strings.Contains(entry["error"].(string), "secret internal detail") || !strings.Contains(entry["stack"].(string), "goroutine") {
		t.Fatalf("log must carry the panic and stack: %v", entry)
	}

	// The server keeps serving after a panic.
	r.GET("/ok", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ok", nil))
	if w.Code != http.StatusNoContent {
		t.Fatalf("after panic: status = %d, want 204", w.Code)
	}
}

func TestLogger_LevelFollowsStatus(t *testing.T) {
	var logs bytes.Buffer
	r := newEngine(&logs)
	r.GET("/ok", func(c *gin.Context) { c.Status(http.StatusOK) })

	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/ok", nil))
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/missing", nil))

	lines := strings.Split(strings.TrimSpace(logs.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("want 2 log lines, got %d", len(lines))
	}
	for i, want := range []string{"INFO", "WARN"} {
		var entry map[string]any
		_ = json.Unmarshal([]byte(lines[i]), &entry)
		if entry["level"] != want {
			t.Fatalf("line %d level = %v, want %s", i, entry["level"], want)
		}
	}
}

// UT-11 (token): missing, malformed, expired, wrongly signed and alg-none
// tokens are rejected with 401; RequireRole returns 403 for regular users.
func TestAuth(t *testing.T) {
	clk := mocks.NewFakeClock(time.Date(2026, 10, 15, 8, 0, 0, 0, time.UTC))
	manager := jwtutil.NewManager("test-secret", 24*time.Hour, clk)
	userID := uuid.New()

	validUser, _ := manager.Generate(userID, string(domain.RoleUser))
	validAdmin, _ := manager.Generate(uuid.New(), string(domain.RoleSuperAdmin))
	wrongSecret, _ := jwtutil.NewManager("other-secret", 24*time.Hour, clk).Generate(userID, "user")
	algNone, _ := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{
		"sub": userID.String(), "role": "super_admin", "exp": clk.Now().Add(time.Hour).Unix(),
	}).SignedString(jwt.UnsafeAllowNoneSignatureType)

	var logs bytes.Buffer
	r := newEngine(&logs)
	protected := r.Group("/", middleware.Auth(manager))
	protected.GET("/me", func(c *gin.Context) { c.String(http.StatusOK, c.GetString("user_id")) })
	protected.GET("/admin", middleware.RequireRole(domain.RoleSuperAdmin), func(c *gin.Context) { c.Status(http.StatusOK) })

	call := func(path, authHeader string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if authHeader != "" {
			req.Header.Set("Authorization", authHeader)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	if w := call("/me", "Bearer "+validUser); w.Code != http.StatusOK || w.Body.String() != userID.String() {
		t.Fatalf("valid token: status=%d body=%s", w.Code, w.Body.String())
	}

	for name, header := range map[string]string{
		"missing header":  "",
		"wrong scheme":    "Basic " + validUser,
		"malformed token": "Bearer not-a-jwt",
		"wrong secret":    "Bearer " + wrongSecret,
		"alg none":        "Bearer " + algNone,
	} {
		w := call("/me", header)
		if w.Code != http.StatusUnauthorized || decodeError(t, w).Code != "UNAUTHORIZED" {
			t.Fatalf("%s: status=%d body=%s, want 401 UNAUTHORIZED", name, w.Code, w.Body.String())
		}
	}

	if w := call("/admin", "Bearer "+validUser); w.Code != http.StatusForbidden || decodeError(t, w).Code != "FORBIDDEN" {
		t.Fatalf("user on admin route: status=%d, want 403", w.Code)
	}
	if w := call("/admin", "Bearer "+validAdmin); w.Code != http.StatusOK {
		t.Fatalf("admin on admin route: status=%d, want 200", w.Code)
	}

	clk.Advance(24*time.Hour + time.Second)
	if w := call("/me", "Bearer "+validUser); w.Code != http.StatusUnauthorized {
		t.Fatalf("expired token: status=%d, want 401", w.Code)
	}
}
