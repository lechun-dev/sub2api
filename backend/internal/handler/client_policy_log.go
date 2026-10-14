package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func recordClientPolicyDenial(ops *service.OpsService, c *gin.Context, event service.ClientPolicyDenialEvent) {
	if ops == nil || !ops.IsMonitoringEnabled(c.Request.Context()) {
		return
	}
	enqueueOpsErrorLog(ops, buildClientPolicyDenialLog(c, event))
}

// 2026-10-14 coder(lq): 白名单字段单独构建，拒绝读取原始 UA、凭据、请求正文和 URL query；WS 关闭后按拒绝发生时间入队。
func buildClientPolicyDenialLog(c *gin.Context, event service.ClientPolicyDenialEvent) *service.OpsInsertErrorLogInput {
	body, _ := json.Marshal(event.Snapshot)
	requestID, _ := c.Request.Context().Value(ctxkey.RequestID).(string)
	if requestID == "" {
		requestID = c.Writer.Header().Get("X-Request-Id")
	}
	entry := &service.OpsInsertErrorLogInput{
		CreatedAt: event.CreatedAt, GroupID: &event.GroupID, RequestID: truncateString(requestID, 128),
		RequestPath: truncateString(c.FullPath(), 512), StatusCode: http.StatusForbidden,
		ErrorPhase: "request", ErrorType: service.ClientPolicyDenialType, ErrorOwner: "client", ErrorSource: "gateway", Severity: "info",
		IsBusinessLimited: true, ErrorMessage: event.Snapshot.Code, ErrorBody: string(body),
	}
	if key, ok := middleware2.GetAPIKeyFromContext(c); ok && key != nil {
		if key.ID > 0 {
			id := key.ID
			entry.APIKeyID = &id
		}
		if key.User != nil && key.User.ID > 0 {
			id := key.User.ID
			entry.UserID = &id
		}
		if key.Group != nil {
			entry.Platform = key.Group.Platform
		}
	}
	if value := strings.TrimSpace(ip.GetClientIP(c)); value != "" {
		entry.ClientIP = &value
	}
	return entry
}
