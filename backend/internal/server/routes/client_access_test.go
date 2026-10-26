package routes

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type clientAccessRouteRepo struct {
	service.SettingRepository
	raw string
}

func (r clientAccessRouteRepo) GetValue(context.Context, string) (string, error) {
	if r.raw == "" {
		return "", service.ErrSettingNotFound
	}
	return r.raw, nil
}

func unrestrictedClientAccessTestSettings() *service.SettingService {
	return service.NewSettingService(clientAccessRouteRepo{}, nil)
}

func TestClientAccessGatewayEntrypoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := service.DefaultClientAccessConfig()
	cfg.Groups["38"] = service.GroupClientPolicy{Mode: "allow", ClientIDs: []string{"deepseek_harness"}}
	raw, err := json.Marshal(cfg)
	require.NoError(t, err)
	router := gin.New()
	RegisterGatewayRoutes(router, &handler.Handlers{Gateway: &handler.GatewayHandler{}, OpenAIGateway: &handler.OpenAIGatewayHandler{}, AsyncImage: handler.NewAsyncImageHandler(nil, nil)}, servermiddleware.APIKeyAuthMiddleware(func(c *gin.Context) {
		id := int64(38)
		c.Set(string(servermiddleware.ContextKeyAPIKey), &service.APIKey{GroupID: &id, Group: &service.Group{ID: id, Platform: service.PlatformDeepseek}})
		c.Next()
	}), nil, nil, nil, service.NewSettingService(clientAccessRouteRepo{raw: string(raw)}, nil), nil, nil, &config.Config{Gateway: config.GatewayConfig{MaxBodySize: 1048576, TextMaxBodySize: 1048576}})
	for _, tc := range []struct{ method, path string }{
		{"POST", "/v1/chat/completions"}, {"POST", "/chat/completions"}, {"POST", "/v1/messages"},
		{"POST", "/v1/responses"}, {"POST", "/responses"}, {"POST", "/backend-api/codex/responses"},
		{"GET", "/v1/responses"}, {"GET", "/responses"}, {"GET", "/backend-api/codex/responses"},
		{"GET", "/v1/models"}, {"GET", "/models"}, {"GET", "/backend-api/codex/models"}, {"GET", "/antigravity/models"},
		{"POST", "/antigravity/v1/messages"},
	} {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{"model":"deepseek-chat","messages":[]}`))
			req.Header.Set("User-Agent", "codex_cli_rs/0.150.0")
			req.Header.Set("Upgrade", "websocket")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			require.Equal(t, 403, w.Code, w.Body.String())
			require.Contains(t, w.Body.String(), "client_not_allowed")
			require.Contains(t, w.Body.String(), "DeepSeek Harness")
		})
	}
}
