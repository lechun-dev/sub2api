package middleware

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// ConversationArchiveSkipHeader 是客户端声明「本次请求不要进入会话存档」的请求头。
//
// 典型用法是本机调试、压测、内部巡检流量：只影响当前请求，不需要改动全局开关，
// 因此也不会让真正需要分析的线上会话漏档。
const ConversationArchiveSkipHeader = "X-Sub2API-Archive"

// conversationArchiveSkipValues 视为「跳过归档」的取值，大小写不敏感。
// on / true / 1 等取值保持默认行为（正常归档），未识别的取值同样按默认归档处理。
var conversationArchiveSkipValues = map[string]struct{}{
	"skip":  {},
	"off":   {},
	"no":    {},
	"none":  {},
	"local": {},
	"false": {},
	"0":     {},
}

// conversationArchiveSkipRequested 判断请求头是否要求跳过归档。
// 支持逗号/分号/空格分隔的多个取值，例如 "local,skip"。
func conversationArchiveSkipRequested(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	for _, token := range strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == ' '
	}) {
		if _, ok := conversationArchiveSkipValues[strings.ToLower(token)]; ok {
			return true
		}
	}
	return false
}

// ConversationArchive 在网关文本端点上捕获客户端请求与响应，异步归档用户与 AI 的会话。
//
// 只在开关打开时包装请求/响应；关闭时完全跳过，不产生额外内存拷贝。
// 请求带 ConversationArchiveSkipHeader 标记时同样直接放行，请求/响应体不会被复制，也不会落库。
// WebSocket（Responses WS）不走此中间件：连接被 hijack，字节流无法在这里捕获。
func ConversationArchive(archive *service.ConversationArchiveService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 标记头只供网关内部判定，读完即删，避免随客户端请求一路转发到上游影响上游指纹。
		rawSkip := strings.TrimSpace(c.GetHeader(ConversationArchiveSkipHeader))
		if rawSkip != "" {
			c.Request.Header.Del(ConversationArchiveSkipHeader)
		}
		// 通用客户端标识头与标记头一样，只供网关内部判定，读完即删。
		rawClientID := strings.TrimSpace(c.GetHeader(service.ConversationArchiveClientIDHeader))
		if rawClientID != "" {
			c.Request.Header.Del(service.ConversationArchiveClientIDHeader)
		}
		if archive == nil || conversationArchiveSkipRequested(rawSkip) || isWebSocketUpgradeRequest(c) {
			c.Next()
			return
		}
		protocol, pathModel, ok := service.ConversationArchiveTarget(c.Request.Method, c.Request.URL.Path)
		if !ok || !archive.Enabled(c.Request.Context()) {
			c.Next()
			return
		}
		// DSH 等客户端会自带一个稳定的本机标识；能自定义请求头的客户端（OpenClaw 等）
		// 则用通用的 x-sub2api-archive-client-id 上报。任一命中排除名单即直接放行，
		// 这样"本机调试流量不归档"不需要改服务端开关。
		// 放在开关判定之后，关闭存档或非归档端点时不会多读一次设置。
		if archive.ExcludedClient(c.Request.Context(),
			c.GetHeader(service.ConversationArchiveHarnessUserIDHeader), rawClientID) {
			c.Next()
			return
		}

		requestBuffer := service.NewBoundedBuffer(service.ConversationArchiveRequestLimitBytes)
		if c.Request.Body != nil {
			c.Request.Body = service.NewTeeReadCloser(c.Request.Body, requestBuffer)
		}
		responseBuffer := service.NewBoundedBuffer(service.ConversationArchiveResponseLimitBytes)
		originalWriter := c.Writer
		c.Writer = &conversationArchiveResponseWriter{ResponseWriter: originalWriter, buffer: responseBuffer}

		c.Next()

		c.Writer = originalWriter
		status := originalWriter.Status()
		if status == 0 {
			status = 200
		}
		if status < 200 || status >= 300 {
			return // 只归档成功响应，失败/拦截请求不进入分析表
		}
		userID, apiKeyID, groupID := conversationArchiveIdentity(c)
		requestID, _ := c.Request.Context().Value(ctxkey.ClientRequestID).(string)
		record := service.BuildConversationArchive(service.ConversationArchiveCaptureInput{
			RequestID:     requestID,
			UserID:        userID,
			APIKeyID:      apiKeyID,
			GroupID:       groupID,
			Protocol:      protocol,
			Endpoint:      c.Request.URL.Path,
			PathModel:     pathModel,
			StatusCode:    status,
			ContentType:   originalWriter.Header().Get("Content-Type"),
			RequestBytes:  requestBuffer.Bytes(),
			ResponseBytes: responseBuffer.Bytes(),
			RequestLimit:  service.ConversationArchiveRequestLimitBytes,
			ResponseLimit: service.ConversationArchiveResponseLimitBytes,
			Headers:       conversationArchiveSessionHeaders(c),
		})
		if record != nil {
			archive.Record(record)
		}
	}
}

func conversationArchiveIdentity(c *gin.Context) (userID, apiKeyID int64, groupID *int64) {
	if apiKey, ok := GetAPIKeyFromContext(c); ok && apiKey != nil {
		return apiKey.UserID, apiKey.ID, apiKey.GroupID
	}
	if subject, ok := GetAuthSubjectFromContext(c); ok {
		return subject.UserID, 0, nil
	}
	return 0, 0, nil
}

func conversationArchiveSessionHeaders(c *gin.Context) map[string]string {
	headers := make(map[string]string, 4)
	for _, name := range []string{"session_id", "x-session-id", "conversation_id", "x-conversation-id"} {
		if value := c.GetHeader(name); value != "" {
			headers[name] = value
		}
	}
	return headers
}

type conversationArchiveResponseWriter struct {
	gin.ResponseWriter
	buffer *service.BoundedBuffer
}

func (w *conversationArchiveResponseWriter) Write(p []byte) (int, error) {
	n, err := w.ResponseWriter.Write(p)
	if n > 0 {
		w.buffer.Write(p[:n])
	}
	return n, err
}

func (w *conversationArchiveResponseWriter) WriteString(s string) (int, error) {
	n, err := w.ResponseWriter.WriteString(s)
	if n > 0 {
		w.buffer.Write([]byte(s[:n]))
	}
	return n, err
}
