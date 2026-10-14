package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestClientPolicyDenialLogPrivacyAndOriginalAttribution(t *testing.T) {
	setupOpsErrorLogTestQueue(t, 4)
	gin.SetMode(gin.TestMode)
	ops := service.NewOpsService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	when := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
	router := gin.New()
	router.Use(OpsErrorLoggerMiddleware(ops))
	router.POST("/v1beta/models/:model", func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{ID: 41, Key: "sk-private-test", User: &service.User{ID: 72}, Group: &service.Group{ID: 99}})
		middleware2.MarkIngressRejected(c, middleware2.IngressRejectClientPolicyDenied)
		c.Set(service.ClientPolicyDenialContextKey, service.ClientPolicyDenialEvent{CreatedAt: when, GroupID: 38, Snapshot: service.ClientPolicySnapshot{Code: "client_unrecognized", Mode: "allow", SelectedClients: []service.ClientPolicyClient{{ID: "codex", Name: "Codex"}}, SelectedCount: 1}})
		c.JSON(403, gin.H{"secret": "private-response-test"})
	})
	request := httptest.NewRequest(http.MethodPost, "/v1beta/models/private-path-test?token=private-query-test", strings.NewReader("private-body-test"))
	request.Header.Set("Authorization", "Bearer sk-private-test")
	request.Header.Set("User-Agent", "private-ua-test")
	request.Header.Set("Cookie", "private-cookie-test")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, request)
	require.Equal(t, 403, w.Code)
	require.Equal(t, int64(1), OpsErrorLogQueueLength())
	entry := (<-opsErrorLogQueue).entry
	require.Equal(t, when, entry.CreatedAt)
	require.Equal(t, int64(38), *entry.GroupID)
	require.Equal(t, int64(41), *entry.APIKeyID)
	require.Equal(t, int64(72), *entry.UserID)
	require.Equal(t, "/v1beta/models/:model", entry.RequestPath)
	require.True(t, entry.IsBusinessLimited)
	require.Equal(t, service.ClientPolicyDenialType, entry.ErrorType)
	bytes, err := json.Marshal(entry)
	require.NoError(t, err)
	for _, secret := range []string{"sk-private-test", "private-path-test", "private-query-test", "private-body-test", "private-ua-test", "private-cookie-test", "private-response-test"} {
		require.NotContains(t, string(bytes), secret)
	}
	var snapshot service.ClientPolicySnapshot
	require.NoError(t, json.Unmarshal([]byte(entry.ErrorBody), &snapshot))
	require.Equal(t, "client_unrecognized", snapshot.Code)
}

func TestClientPolicyDenialLogDisabledSkipsQueue(t *testing.T) {
	setupOpsErrorLogTestQueue(t, 4)
	ops := service.NewOpsService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	ops.SetMonitoringEnabled(false)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/responses", nil)
	recordClientPolicyDenial(ops, c, service.ClientPolicyDenialEvent{CreatedAt: time.Now(), GroupID: 38, Snapshot: service.ClientPolicySnapshot{Code: "client_forbidden"}})
	require.Zero(t, OpsErrorLogQueueLength())
}

func TestClientPolicyDenialLogUsesBoundedEventUserAgent(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/responses", nil)
	c.Request.Header.Set("User-Agent", "changed-after-denial")
	event := service.ClientPolicyDenialEvent{UserAgent: "curl/8 Bearer event-secret", CreatedAt: time.Now(), GroupID: 38}
	entry := buildClientPolicyDenialLog(c, event)
	require.Contains(t, entry.UserAgent, "curl/8")
	require.NotContains(t, entry.UserAgent, "event-secret")
	require.NotContains(t, entry.UserAgent, "changed-after-denial")
	require.LessOrEqual(t, len(entry.UserAgent), 512)
}
