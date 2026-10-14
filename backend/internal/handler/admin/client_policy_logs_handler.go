package admin

import (
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *OpsHandler) ListClientPolicyDenials(c *gin.Context) {
	if h.opsService == nil {
		response.Error(c, http.StatusServiceUnavailable, "日志服务不可用")
		return
	}
	if err := h.opsService.RequireMonitoringEnabled(c.Request.Context()); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	id, err := strconv.ParseInt(c.Query("group_id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "group_id 必须为正整数")
		return
	}
	start, end, err := parseOpsTimeRange(c, "24h")
	if err != nil || !end.After(start) {
		response.BadRequest(c, "日期范围无效，最多查询 30 天")
		return
	}
	page, size := response.ParsePagination(c)
	if page > 100000 {
		response.BadRequest(c, "日志分页超出范围")
		return
	}
	result, err := h.opsService.ListClientPolicyDenials(c.Request.Context(), service.ClientPolicyDenialFilter{GroupID: id, StartTime: start, EndTime: end, Page: page, PageSize: size})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, result.Items, int64(result.Total), result.Page, result.PageSize)
}
