package admin

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

const conversationArchiveMaxPageSize = 100

// ConversationArchiveHandler 会话存档只读查询接口，供管理员或带权限的管理密钥（含 AI）使用。
type ConversationArchiveHandler struct {
	archive *service.ConversationArchiveService
}

// NewConversationArchiveHandler 创建会话存档查询处理器。
func NewConversationArchiveHandler(archive *service.ConversationArchiveService) *ConversationArchiveHandler {
	return &ConversationArchiveHandler{archive: archive}
}

type conversationArchiveItem struct {
	ID                int64                                `json:"id"`
	RequestID         string                               `json:"request_id"`
	UserID            int64                                `json:"user_id"`
	Username          string                               `json:"username,omitempty"`
	Email             string                               `json:"email,omitempty"`
	APIKeyID          int64                                `json:"api_key_id"`
	GroupID           *int64                               `json:"group_id,omitempty"`
	GroupName         string                               `json:"group_name,omitempty"`
	GroupPlatform     string                               `json:"group_platform,omitempty"`
	Protocol          string                               `json:"protocol"`
	Endpoint          string                               `json:"endpoint"`
	Model             string                               `json:"model"`
	Stream            bool                                 `json:"stream"`
	StatusCode        int                                  `json:"status_code"`
	SessionKey        string                               `json:"session_key,omitempty"`
	MessageCount      int                                  `json:"message_count"`
	ResponseChars     int                                  `json:"response_chars"`
	InputTokens       *int64                               `json:"input_tokens,omitempty"`
	OutputTokens      *int64                               `json:"output_tokens,omitempty"`
	CachedInputTokens *int64                               `json:"cached_input_tokens,omitempty"`
	Truncated         bool                                 `json:"truncated"`
	CreatedAt         time.Time                            `json:"created_at"`
	RequestMessages   []service.ConversationArchiveMessage `json:"request_messages,omitempty"`
	ResponseText      *string                              `json:"response_text,omitempty"`
}

// List 分页查询会话存档。
// GET /api/v1/admin/conversation-archives
func (h *ConversationArchiveHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	if pageSize > conversationArchiveMaxPageSize {
		pageSize = conversationArchiveMaxPageSize
	}
	filter, err := parseConversationArchiveFilter(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	filter.Page = page
	filter.PageSize = pageSize

	records, total, err := h.archive.List(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	items := make([]conversationArchiveItem, 0, len(records))
	for _, record := range records {
		items = append(items, mapConversationArchiveItem(record, filter.IncludeContent))
	}
	response.Paginated(c, items, total, page, pageSize)
}

type conversationArchiveStatsItem struct {
	Key               string `json:"key"`
	UserID            *int64 `json:"user_id,omitempty"`
	Username          string `json:"username,omitempty"`
	Email             string `json:"email,omitempty"`
	GroupID           *int64 `json:"group_id,omitempty"`
	GroupName         string `json:"group_name,omitempty"`
	APIKeyID          *int64 `json:"api_key_id,omitempty"`
	Model             string `json:"model,omitempty"`
	Protocol          string `json:"protocol,omitempty"`
	Date              string `json:"date,omitempty"`
	Records           int64  `json:"records"`
	RecordsWithUsage  int64  `json:"records_with_usage"`
	InputTokens       int64  `json:"input_tokens"`
	OutputTokens      int64  `json:"output_tokens"`
	CachedInputTokens int64  `json:"cached_input_tokens"`
}

// Stats 按用户/日期/模型等维度汇总 token 用量。
// GET /api/v1/internal/conversation-archives/stats
func (h *ConversationArchiveHandler) Stats(c *gin.Context) {
	filter, err := parseConversationArchiveFilter(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	groupBy := strings.ToLower(strings.TrimSpace(c.Query("group_by")))
	if groupBy == "" {
		groupBy = service.ConversationArchiveGroupNone
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

	items, totals, err := h.archive.UsageStats(c.Request.Context(), filter, groupBy, limit, order != "asc")
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]conversationArchiveStatsItem, 0, len(items))
	for _, item := range items {
		out = append(out, conversationArchiveStatsItem{
			Key:               item.Key,
			UserID:            item.UserID,
			Username:          item.Username,
			Email:             item.Email,
			GroupID:           item.GroupID,
			GroupName:         item.GroupName,
			APIKeyID:          item.APIKeyID,
			Model:             item.Model,
			Protocol:          item.Protocol,
			Date:              item.Date,
			Records:           item.Records,
			RecordsWithUsage:  item.RecordsWithUsage,
			InputTokens:       item.InputTokens,
			OutputTokens:      item.OutputTokens,
			CachedInputTokens: item.CachedInputTokens,
		})
	}
	response.Success(c, gin.H{
		"group_by": groupBy,
		"order":    order,
		"items":    out,
		"totals":   totals,
	})
}

// Get 查询单条存档详情（始终包含完整文本）。
// GET /api/v1/admin/conversation-archives/:id
func (h *ConversationArchiveHandler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid conversation archive id")
		return
	}
	record, err := h.archive.GetByID(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, mapConversationArchiveItem(record, true))
}

func mapConversationArchiveItem(record *service.ConversationArchive, includeContent bool) conversationArchiveItem {
	item := conversationArchiveItem{
		ID:                record.ID,
		RequestID:         record.RequestID,
		UserID:            record.UserID,
		Username:          record.Username,
		Email:             record.Email,
		APIKeyID:          record.APIKeyID,
		GroupID:           record.GroupID,
		GroupName:         record.GroupName,
		GroupPlatform:     record.GroupPlatform,
		Protocol:          record.Protocol,
		Endpoint:          record.Endpoint,
		Model:             record.Model,
		Stream:            record.Stream,
		StatusCode:        record.StatusCode,
		SessionKey:        record.SessionKey,
		MessageCount:      record.MessageCount,
		ResponseChars:     record.ResponseChars,
		InputTokens:       record.InputTokens,
		OutputTokens:      record.OutputTokens,
		CachedInputTokens: record.CachedInputTokens,
		Truncated:         record.Truncated,
		CreatedAt:         record.CreatedAt,
	}
	if includeContent {
		item.RequestMessages = record.Messages
		text := record.ResponseText
		item.ResponseText = &text
	}
	return item
}

func parseConversationArchiveFilter(c *gin.Context) (service.ConversationArchiveFilter, error) {
	filter := service.ConversationArchiveFilter{}
	var err error

	if filter.UserID, err = parseConversationArchiveOptionalID(c, "user_id"); err != nil {
		return filter, err
	}
	if filter.APIKeyID, err = parseConversationArchiveOptionalID(c, "api_key_id"); err != nil {
		return filter, err
	}
	if filter.GroupID, err = parseConversationArchiveOptionalID(c, "group_id"); err != nil {
		return filter, err
	}
	filter.UserName = strings.TrimSpace(c.Query("username"))
	if filter.UserName == "" {
		filter.UserName = strings.TrimSpace(c.Query("name"))
	}
	if len([]rune(filter.UserName)) > 100 {
		return filter, errors.New("username is too long")
	}
	filter.RequestID = strings.TrimSpace(c.Query("request_id"))
	filter.SessionKey = strings.TrimSpace(c.Query("session_key"))
	filter.Protocol = strings.ToLower(strings.TrimSpace(c.Query("protocol")))
	switch filter.Protocol {
	case "", service.ConversationArchiveProtocolAnthropicMessages, service.ConversationArchiveProtocolOpenAIChat,
		service.ConversationArchiveProtocolOpenAIResponses, service.ConversationArchiveProtocolGemini:
	default:
		return filter, errors.New("Invalid protocol")
	}
	filter.Model = strings.TrimSpace(c.Query("model"))
	filter.Endpoint = strings.TrimSpace(c.Query("endpoint"))
	filter.Keyword = strings.TrimSpace(c.Query("keyword"))
	if filter.Keyword == "" {
		filter.Keyword = strings.TrimSpace(c.Query("q"))
	}

	if v := strings.TrimSpace(c.Query("status_code")); v != "" {
		status, parseErr := strconv.Atoi(v)
		if parseErr != nil || status < 100 || status > 599 {
			return filter, errors.New("Invalid status_code")
		}
		filter.StatusCode = &status
	}
	if filter.Stream, err = parseConversationArchiveOptionalBool(c, "stream"); err != nil {
		return filter, err
	}
	if filter.Truncated, err = parseConversationArchiveOptionalBool(c, "truncated"); err != nil {
		return filter, err
	}
	if filter.StartTime, err = parseConversationArchiveOptionalTime(c, "start_time"); err != nil {
		return filter, err
	}
	if filter.EndTime, err = parseConversationArchiveOptionalTime(c, "end_time"); err != nil {
		return filter, err
	}
	if filter.StartTime != nil && filter.EndTime != nil && filter.EndTime.Before(*filter.StartTime) {
		return filter, errors.New("end_time must not be before start_time")
	}

	filter.IncludeContent = strings.EqualFold(strings.TrimSpace(c.Query("include_content")), "true")
	filter.Descending = true
	switch strings.ToLower(strings.TrimSpace(c.Query("order"))) {
	case "", "desc", "newest":
	case "asc", "oldest":
		filter.Descending = false
	default:
		return filter, errors.New("Invalid order, expect asc or desc")
	}
	return filter, nil
}

func parseConversationArchiveOptionalID(c *gin.Context, name string) (*int64, error) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return nil, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return nil, errors.New("Invalid " + name)
	}
	return &value, nil
}

func parseConversationArchiveOptionalBool(c *gin.Context, name string) (*bool, error) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return nil, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return nil, errors.New("Invalid " + name)
	}
	return &value, nil
}

func parseConversationArchiveOptionalTime(c *gin.Context, name string) (*time.Time, error) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return nil, nil
	}
	if value, err := time.Parse(time.RFC3339, raw); err == nil {
		value := value.UTC()
		return &value, nil
	}
	if value, err := time.Parse("2006-01-02", raw); err == nil {
		value := value.UTC()
		return &value, nil
	}
	return nil, errors.New("Invalid " + name + ", expect RFC3339 or YYYY-MM-DD")
}
