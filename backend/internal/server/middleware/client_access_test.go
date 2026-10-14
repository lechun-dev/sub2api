package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type clientAccessMiddlewareRepo struct {
	service.SettingRepository
	raw string
}

type mutableClientAccessRepo struct {
	service.SettingRepository
	raw string
}

func (r *mutableClientAccessRepo) GetValue(context.Context, string) (string, error) {
	return r.raw, nil
}
func (r *mutableClientAccessRepo) CompareAndSetClientAccess(_ context.Context, old, next string, _ []int64) error {
	if r.raw != old {
		return service.ErrClientAccessConflict
	}
	r.raw = next
	return nil
}

func TestClientAccessWebSocketRechecksLatestPolicyForOriginalGroup(t *testing.T) {
	cfg := service.DefaultClientAccessConfig()
	cfg.Groups["38"] = service.GroupClientPolicy{Mode: "allow", ClientIDs: []string{"codex"}}
	raw, err := json.Marshal(cfg)
	require.NoError(t, err)
	repo := &mutableClientAccessRepo{raw: string(raw)}
	settings := service.NewSettingService(repo, nil)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(ContextKeyAPIKey), &service.APIKey{Group: &service.Group{ID: 38}})
		c.Next()
	})
	router.Use(GroupClientAccess(settings))
	router.GET("/v1/responses", func(c *gin.Context) {
		decision, err := CheckWebSocketClientAccess(c)
		require.NoError(t, err)
		require.True(t, decision.Allowed)
		next, err := settings.GetClientAccessConfig(context.Background())
		require.NoError(t, err)
		next.Groups["38"] = service.GroupClientPolicy{Mode: "deny", ClientIDs: []string{"codex"}}
		require.NoError(t, settings.SaveClientAccessConfig(context.Background(), *next))
		// 2026-10-13 coder(lq): 模拟合成路由改写上下文，长连接仍须检查 Key 建连时的原始分组。
		c.Set(string(ContextKeyAPIKey), &service.APIKey{Group: &service.Group{ID: 99}})
		decision, err = CheckWebSocketClientAccess(c)
		require.NoError(t, err)
		require.False(t, decision.Allowed)
		require.Equal(t, "client_forbidden", decision.Code)
		value, exists := c.Get(service.ClientPolicyDenialContextKey)
		require.True(t, exists)
		event := value.(service.ClientPolicyDenialEvent)
		require.Equal(t, int64(38), event.GroupID)
		require.False(t, event.CreatedAt.IsZero())
		require.Equal(t, "client_forbidden", event.Snapshot.Code)
		c.Status(204)
	})
	req := httptest.NewRequest("GET", "/v1/responses", nil)
	req.Header.Set("User-Agent", "codex_cli_rs/0.150.0")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, 204, w.Code)
}

func (r clientAccessMiddlewareRepo) GetValue(context.Context, string) (string, error) {
	return r.raw, nil
}

func TestClientAccessMiddlewareBeforeDispatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := service.DefaultClientAccessConfig()
	cfg.Groups["38"] = service.GroupClientPolicy{Mode: "allow", ClientIDs: []string{"deepseek_harness"}}
	raw, err := json.Marshal(cfg)
	require.NoError(t, err)
	for _, path := range []string{"/v1/chat/completions", "/responses", "/backend-api/codex/responses", "/v1beta/models/m:generateContent", "/antigravity/v1/messages"} {
		for _, ua := range []string{"deepseek-harness/1", "codex_cli_rs/1", "curl/8"} {
			t.Run(path+ua, func(t *testing.T) {
				calls := 0
				router := gin.New()
				router.Use(func(c *gin.Context) {
					c.Set(string(ContextKeyAPIKey), &service.APIKey{Group: &service.Group{ID: 38}})
					c.Next()
				})
				router.Use(GroupClientAccess(service.NewSettingService(clientAccessMiddlewareRepo{raw: string(raw)}, nil)))
				router.POST(path, func(c *gin.Context) { calls++; c.Status(204) })
				req := httptest.NewRequest("POST", path, strings.NewReader(`{"model":"test"}`))
				req.Header.Set("User-Agent", ua)
				req.Header.Set("Upgrade", "websocket")
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)
				if strings.HasPrefix(ua, "deepseek-harness/") {
					require.Equal(t, 204, w.Code)
					require.Equal(t, 1, calls)
				} else {
					require.Equal(t, 403, w.Code)
					require.Zero(t, calls)
					require.Contains(t, w.Body.String(), "DeepSeek Harness")
				}
			})
		}
	}
}

func TestClientAccessMiddlewareUnavailable(t *testing.T) {
	router := gin.New()
	calls := 0
	router.Use(func(c *gin.Context) {
		c.Set(string(ContextKeyAPIKey), &service.APIKey{Group: &service.Group{ID: 38}})
		c.Next()
	})
	router.Use(GroupClientAccess(service.NewSettingService(clientAccessMiddlewareRepo{raw: "invalid"}, nil)))
	router.GET("/v1/models", func(c *gin.Context) { calls++; c.Status(http.StatusOK) })
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/v1/models", nil))
	require.Equal(t, 503, w.Code)
	require.Zero(t, calls)
	require.Contains(t, w.Body.String(), "client_policy_unavailable")
}
