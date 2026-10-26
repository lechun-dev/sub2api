package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type internalIdentityUsers struct {
	user      *service.User
	err       error
	seenEmail string
}

func (f *internalIdentityUsers) GetByEmail(_ context.Context, email string) (*service.User, error) {
	f.seenEmail = email
	return f.user, f.err
}

func internalIdentityConfig() *config.Config {
	return &config.Config{InternalIdentity: config.InternalIdentityConfig{
		Enabled: true, Secret: "test-internal-secret", MaxSkewSeconds: 300,
	}}
}

func internalIdentitySignature(secret, email, timestamp, method, path string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(email + "\n" + timestamp + "\n" + method + "\n" + path))
	return hex.EncodeToString(mac.Sum(nil))
}

func newInternalIdentityRouter(users InternalIdentityUserLookup, cfg *config.Config) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(InternalIdentityAuth(users, cfg))
	engine.GET("/api/v1/internal/conversation-archives", func(c *gin.Context) {
		subject, _ := GetAuthSubjectFromContext(c)
		c.JSON(http.StatusOK, gin.H{"user_id": subject.UserID, "auth_method": c.GetString("auth_method")})
	})
	return engine
}

func signedInternalRequest(t *testing.T, cfg *config.Config, email, path string, at time.Time) *http.Request {
	t.Helper()
	timestamp := strconv.FormatInt(at.Unix(), 10)
	request := httptest.NewRequest(http.MethodGet, path, nil)
	request.Header.Set(InternalIdentityEmailHeader, email)
	request.Header.Set(InternalIdentityTimestampHeader, timestamp)
	request.Header.Set(InternalIdentitySignatureHeader,
		internalIdentitySignature(cfg.InternalIdentity.Secret, email, timestamp, http.MethodGet, path))
	return request
}

func TestInternalIdentityAuthAllowsAdmin(t *testing.T) {
	cfg := internalIdentityConfig()
	users := &internalIdentityUsers{user: &service.User{
		ID: 7, Email: "admin@example.com", Role: service.RoleAdmin, Status: service.StatusActive,
	}}
	engine := newInternalIdentityRouter(users, cfg)

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, signedInternalRequest(t, cfg, "admin@example.com", "/api/v1/internal/conversation-archives", time.Now()))

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "admin@example.com", users.seenEmail)
	require.Contains(t, recorder.Body.String(), "\"user_id\":7")
	require.Contains(t, recorder.Body.String(), "internal_signed")
}

func TestInternalIdentityAuthRejectsNonAdmin(t *testing.T) {
	cfg := internalIdentityConfig()
	users := &internalIdentityUsers{user: &service.User{
		ID: 8, Email: "user@example.com", Role: "user", Status: service.StatusActive,
	}}
	engine := newInternalIdentityRouter(users, cfg)

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, signedInternalRequest(t, cfg, "user@example.com", "/api/v1/internal/conversation-archives", time.Now()))

	require.Equal(t, http.StatusForbidden, recorder.Code)
}

func TestInternalIdentityAuthRejectsDisabledAndBadRequests(t *testing.T) {
	disabled := &config.Config{InternalIdentity: config.InternalIdentityConfig{Enabled: false}}
	users := &internalIdentityUsers{}

	engine := newInternalIdentityRouter(users, disabled)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/internal/conversation-archives", nil))
	require.Equal(t, http.StatusNotFound, recorder.Code)

	cfg := internalIdentityConfig()
	engine = newInternalIdentityRouter(users, cfg)

	// 缺少头
	recorder = httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/internal/conversation-archives", nil))
	require.Equal(t, http.StatusUnauthorized, recorder.Code)

	// 过期时间戳
	expired := signedInternalRequest(t, cfg, "admin@example.com", "/api/v1/internal/conversation-archives", time.Now().Add(-time.Hour))
	recorder = httptest.NewRecorder()
	engine.ServeHTTP(recorder, expired)
	require.Equal(t, http.StatusUnauthorized, recorder.Code)

	// 签名绑定路径：用另一条路径的签名访问当前路径
	request := signedInternalRequest(t, cfg, "admin@example.com", "/api/v1/internal/conversation-archives/1", time.Now())
	request.URL.Path = "/api/v1/internal/conversation-archives"
	recorder = httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusUnauthorized, recorder.Code)

	// 用户不存在
	missing := &internalIdentityUsers{err: service.ErrUserNotFound}
	engine = newInternalIdentityRouter(missing, cfg)
	recorder = httptest.NewRecorder()
	engine.ServeHTTP(recorder, signedInternalRequest(t, cfg, "missing@example.com", "/api/v1/internal/conversation-archives", time.Now()))
	require.Equal(t, http.StatusUnauthorized, recorder.Code)

	// 用户被禁用
	inactive := &internalIdentityUsers{user: &service.User{
		ID: 9, Email: "inactive@example.com", Role: service.RoleAdmin, Status: "disabled",
	}}
	engine = newInternalIdentityRouter(inactive, cfg)
	recorder = httptest.NewRecorder()
	engine.ServeHTTP(recorder, signedInternalRequest(t, cfg, "inactive@example.com", "/api/v1/internal/conversation-archives", time.Now()))
	require.Equal(t, http.StatusUnauthorized, recorder.Code)

}
