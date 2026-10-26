package middleware

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// ConversationArchive 在网关文本端点上捕获客户端请求与响应，异步归档用户与 AI 的会话。
//
// 只在开关打开时包装请求/响应；关闭时完全跳过，不产生额外内存拷贝。
// WebSocket（Responses WS）不走此中间件：连接被 hijack，字节流无法在这里捕获。
func ConversationArchive(archive *service.ConversationArchiveService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if archive == nil || isWebSocketUpgradeRequest(c) {
			c.Next()
			return
		}
		protocol, pathModel, ok := service.ConversationArchiveTarget(c.Request.Method, c.Request.URL.Path)
		if !ok || !archive.Enabled(c.Request.Context()) {
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
