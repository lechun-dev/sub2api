package admin

import (
	"errors"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *SettingHandler) GetClientAccess(c *gin.Context) {
	cfg, err := h.settingService.GetClientAccessConfig(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusServiceUnavailable, "客户端配置暂不可用，请联系管理员检查")
		return
	}
	response.Success(c, cfg)
}

func (h *SettingHandler) SaveClientAccess(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1024*1024)
	var cfg service.ClientAccessConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		response.BadRequest(c, "客户端配置格式无效或超过大小限制")
		return
	}
	if err := service.ValidateClientAccessConfig(&cfg); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.settingService.SaveClientAccessConfig(c.Request.Context(), cfg); err != nil {
		switch {
		case errors.Is(err, service.ErrClientAccessConflict):
			response.Error(c, http.StatusConflict, err.Error())
		case errors.Is(err, service.ErrGroupNotFound):
			response.BadRequest(c, "配置引用了已删除或不存在的分组，请重新加载分组列表")
		default:
			response.Error(c, http.StatusServiceUnavailable, "客户端配置保存失败，请稍后重试")
		}
		return
	}
	response.Success(c, gin.H{"saved": true})
}
