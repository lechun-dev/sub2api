package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterInternalRoutes 注册内部系统使用的只读接口。
// 认证由签名身份断言中间件完成；路由组只暴露会话存档查询，不能执行管理写操作。
func RegisterInternalRoutes(v1 *gin.RouterGroup, h *handler.Handlers, internalAuth gin.HandlerFunc, auditLog middleware.AuditLogMiddleware) {
	internal := v1.Group("/internal")
	internal.Use(internalAuth)
	internal.Use(gin.HandlerFunc(auditLog))
	{
		internal.GET("/conversation-archives", h.Admin.ConversationArchive.List)
		internal.GET("/conversation-archives/:id", h.Admin.ConversationArchive.Get)
	}
}
