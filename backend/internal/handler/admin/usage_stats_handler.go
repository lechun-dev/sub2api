package admin

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

const (
	internalUsageStatsDefaultRange = 30 * 24 * time.Hour
	internalUsageStatsMaxRange     = 366 * 24 * time.Hour
)

type internalUsageStatsItem struct {
	Key                 string  `json:"key"`
	UserID              *int64  `json:"user_id,omitempty"`
	Username            string  `json:"username,omitempty"`
	Email               string  `json:"email,omitempty"`
	GroupID             *int64  `json:"group_id,omitempty"`
	GroupName           string  `json:"group_name,omitempty"`
	APIKeyID            *int64  `json:"api_key_id,omitempty"`
	Model               string  `json:"model,omitempty"`
	Date                string  `json:"date,omitempty"`
	Requests            int64   `json:"requests"`
	InputTokens         int64   `json:"input_tokens"`
	OutputTokens        int64   `json:"output_tokens"`
	CacheCreationTokens int64   `json:"cache_creation_tokens"`
	CacheReadTokens     int64   `json:"cache_read_tokens"`
	TotalTokens         int64   `json:"total_tokens"`
	TotalCost           float64 `json:"total_cost"`
}

// InternalStats 是内部签名接口使用的 usage_logs 汇总，口径与计费一致。
// GET /api/v1/internal/usage/stats
func (h *UsageHandler) InternalStats(c *gin.Context) {
	filters := usagestats.UsageLogFilters{
		Model:    strings.TrimSpace(c.Query("model")),
		UserName: strings.TrimSpace(c.Query("username")),
	}

	var err error
	if filters.UserID, err = parseInternalUsageID(c, "user_id"); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if filters.APIKeyID, err = parseInternalUsageID(c, "api_key_id"); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if filters.GroupID, err = parseInternalUsageID(c, "group_id"); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	end, err := parseInternalUsageTime(c, "end_time")
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	start, err := parseInternalUsageTime(c, "start_time")
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if end == nil {
		now := time.Now().UTC()
		end = &now
	}
	if start == nil {
		value := end.Add(-internalUsageStatsDefaultRange)
		start = &value
	}
	if !start.Before(*end) {
		response.BadRequest(c, "start_time must be before end_time")
		return
	}
	if end.Sub(*start) > internalUsageStatsMaxRange {
		response.BadRequest(c, "time range must not exceed 366 days")
		return
	}
	filters.StartTime = start
	filters.EndTime = end

	groupBy := strings.ToLower(strings.TrimSpace(c.Query("group_by")))
	if groupBy == "" {
		groupBy = service.UsageStatsGroupNone
	}
	order := strings.ToLower(strings.TrimSpace(c.Query("order")))
	if order == "" {
		order = "desc"
	}
	if order != "asc" && order != "desc" {
		response.BadRequest(c, "Invalid order, expect asc or desc")
		return
	}
	limit := 0
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		parsed, parseErr := strconv.Atoi(raw)
		if parseErr != nil || parsed <= 0 {
			response.BadRequest(c, "Invalid limit")
			return
		}
		limit = parsed
	}

	items, totals, err := h.usageService.GetAggregatedStats(c.Request.Context(), filters, groupBy, limit, order != "asc")
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]internalUsageStatsItem, 0, len(items))
	for _, item := range items {
		out = append(out, internalUsageStatsItem{
			Key:                 item.Key,
			UserID:              item.UserID,
			Username:            item.Username,
			Email:               item.Email,
			GroupID:             item.GroupID,
			GroupName:           item.GroupName,
			APIKeyID:            item.APIKeyID,
			Model:               item.Model,
			Date:                item.Date,
			Requests:            item.Requests,
			InputTokens:         item.InputTokens,
			OutputTokens:        item.OutputTokens,
			CacheCreationTokens: item.CacheCreationTokens,
			CacheReadTokens:     item.CacheReadTokens,
			TotalTokens:         item.TotalTokens,
			TotalCost:           item.TotalCost,
		})
	}
	response.Success(c, gin.H{
		"group_by":   groupBy,
		"order":      order,
		"start_time": start.UTC().Format(time.RFC3339),
		"end_time":   end.UTC().Format(time.RFC3339),
		"items":      out,
		"totals":     totals,
	})
}

func parseInternalUsageID(c *gin.Context, name string) (int64, error) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return 0, errors.New("Invalid " + name)
	}
	return value, nil
}

// parseInternalUsageTime 接受 RFC3339 或 YYYY-MM-DD；date-only 的上界按次日 00:00（UTC）计算。
func parseInternalUsageTime(c *gin.Context, name string) (*time.Time, error) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return nil, nil
	}
	if value, err := time.Parse(time.RFC3339, raw); err == nil {
		value = value.UTC()
		return &value, nil
	}
	if value, err := time.Parse("2006-01-02", raw); err == nil {
		value = value.UTC()
		if name == "end_time" {
			value = value.AddDate(0, 0, 1)
		}
		return &value, nil
	}
	return nil, errors.New("Invalid " + name + ", expect RFC3339 or YYYY-MM-DD")
}
