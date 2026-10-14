package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const clientAccessRecheckKey = "group_client_access_recheck"

type clientAccessRecheck func(context.Context) (service.ClientAccessDecision, error)

// 2026-10-13 coder(lq): 长连接只固定客户端身份和原始分组 ID，不固定权限配置；Responses 每轮重新读取规则。
func CheckWebSocketClientAccess(c *gin.Context) (service.ClientAccessDecision, error) {
	value, exists := c.Get(clientAccessRecheckKey)
	if !exists {
		return service.ClientAccessDecision{Allowed: true}, nil
	}
	return value.(clientAccessRecheck)(c.Request.Context())
}

// 2026-10-13 coder(lq): 只看鉴权 Key 的原始分组，放在路由改写前；不读请求体且 WS 握手也不得跳过。
func GroupClientAccess(settings *service.SettingService) gin.HandlerFunc {
	return func(c *gin.Context) {
		key, ok := GetAPIKeyFromContext(c)
		if !ok || key == nil || key.Group == nil {
			c.Next()
			return
		}
		cfg, err := settings.RuntimeClientAccessConfig(c.Request.Context())
		if err != nil {
			writeClientAccessError(c, http.StatusServiceUnavailable, "client_policy_unavailable", "客户端访问规则暂不可用，请稍后重试。")
			return
		}
		decision := service.EvaluateClientAccess(cfg, key.Group.ID, c.Request.Header)
		if decision.Allowed {
			if isResponsesWebSocketRoute(c) {
				groupID, headers := key.Group.ID, c.Request.Header.Clone()
				userAgent := service.SanitizeClientPolicyUserAgent(headers.Get("User-Agent"), key.Key)
				for _, name := range []string{"Authorization", "X-Api-Key", "X-Goog-Api-Key", "Cookie"} {
					headers.Del(name)
				}
				c.Set(clientAccessRecheckKey, clientAccessRecheck(func(ctx context.Context) (service.ClientAccessDecision, error) {
					next, err := settings.RuntimeClientAccessConfig(ctx)
					if err != nil {
						return service.ClientAccessDecision{}, err
					}
					decision := service.EvaluateClientAccess(next, groupID, headers)
					markClientPolicyDenial(c, groupID, decision, userAgent)
					return decision, nil
				}))
			}
			c.Next()
			return
		}
		markClientPolicyDenial(c, key.Group.ID, decision, service.SanitizeClientPolicyUserAgent(c.GetHeader("User-Agent"), key.Key))
		service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalPolicyDenied)
		MarkIngressRejected(c, IngressRejectClientPolicyDenied)
		writeClientAccessError(c, http.StatusForbidden, decision.Code, decision.Message)
	}
}

func writeClientAccessError(c *gin.Context, status int, code, message string) {
	// 2026-10-13 coder(lq): 使用既有协议 writer 保持 Gemini 原生错误格式；OpenAI/Anthropic 补充稳定机器码。
	if strings.HasPrefix(c.Request.URL.Path, "/v1beta") || strings.HasPrefix(c.Request.URL.Path, "/antigravity/v1beta") {
		GoogleErrorWriter(c, status, message)
		c.Abort()
		return
	}
	errorType := "permission_error"
	if status >= 500 {
		errorType = "api_error"
	}
	c.AbortWithStatusJSON(status, gin.H{"type": "error", "error": gin.H{"type": errorType, "code": code, "message": message}})
}

// 2026-10-14 coder(lq): 只登记已确定的策略拒绝和脱敏 UA，不记录请求体/其他头；WS 使用握手时原始分组，避免路由改写后归错组。
func markClientPolicyDenial(c *gin.Context, groupID int64, decision service.ClientAccessDecision, userAgent string) {
	if decision.Allowed || decision.Denial == nil {
		return
	}
	c.Set(service.ClientPolicyDenialContextKey, service.ClientPolicyDenialEvent{UserAgent: userAgent, CreatedAt: time.Now().UTC(), GroupID: groupID, Snapshot: *decision.Denial})
}
