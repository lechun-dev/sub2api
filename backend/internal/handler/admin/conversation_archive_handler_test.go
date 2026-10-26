package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type conversationArchiveHandlerRepo struct {
	records    []*service.ConversationArchive
	total      int64
	lastFilter service.ConversationArchiveFilter
	listCalls  int
	record     *service.ConversationArchive
}

func (r *conversationArchiveHandlerRepo) CreateBatch(context.Context, []*service.ConversationArchive) error {
	return nil
}

func (r *conversationArchiveHandlerRepo) DeleteBefore(context.Context, time.Time, int) (int64, error) {
	return 0, nil
}

func (r *conversationArchiveHandlerRepo) List(_ context.Context, filter service.ConversationArchiveFilter) ([]*service.ConversationArchive, int64, error) {
	r.listCalls++
	r.lastFilter = filter
	return r.records, r.total, nil
}

func (r *conversationArchiveHandlerRepo) GetByID(context.Context, int64) (*service.ConversationArchive, error) {
	return r.record, nil
}

func newConversationArchiveHandler(repo *conversationArchiveHandlerRepo) *ConversationArchiveHandler {
	return NewConversationArchiveHandler(service.NewConversationArchiveService(repo, nil))
}

func decodeResponseData(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body struct {
		Code int            `json:"code"`
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, 0, body.Code)
	return body.Data
}

func TestConversationArchiveHandlerListParsesFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	createdAt := time.Date(2026, 10, 20, 8, 0, 0, 0, time.UTC)
	repo := &conversationArchiveHandlerRepo{
		total: 1,
		records: []*service.ConversationArchive{{
			ID:            1,
			RequestID:     "req-1",
			UserID:        7,
			APIKeyID:      9,
			Protocol:      service.ConversationArchiveProtocolAnthropicMessages,
			Endpoint:      "/v1/messages",
			Model:         "claude",
			StatusCode:    200,
			MessageCount:  1,
			ResponseChars: 5,
			Messages:      []service.ConversationArchiveMessage{{Role: "user", Content: "hi"}},
			ResponseText:  "hello",
			CreatedAt:     createdAt,
		}},
	}
	handler := newConversationArchiveHandler(repo)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet,
		"/api/v1/admin/conversation-archives?user_id=7&api_key_id=9&protocol=anthropic_messages&model=claude&keyword=hi&include_content=true&page=2&page_size=10&order=asc", nil)
	handler.List(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	data := decodeResponseData(t, recorder)
	require.Equal(t, float64(1), data["total"])
	require.Equal(t, float64(2), data["page"])
	require.Equal(t, float64(10), data["page_size"])
	items, ok := data["items"].([]any)
	require.True(t, ok)
	require.Len(t, items, 1)
	item := items[0].(map[string]any)
	require.Equal(t, "hello", item["response_text"])
	require.Len(t, item["request_messages"], 1)

	require.Equal(t, 1, repo.listCalls)
	require.Equal(t, int64(7), *repo.lastFilter.UserID)
	require.Equal(t, int64(9), *repo.lastFilter.APIKeyID)
	require.Equal(t, "anthropic_messages", repo.lastFilter.Protocol)
	require.Equal(t, "claude", repo.lastFilter.Model)
	require.Equal(t, "hi", repo.lastFilter.Keyword)
	require.True(t, repo.lastFilter.IncludeContent)
	require.False(t, repo.lastFilter.Descending)
	require.Equal(t, 2, repo.lastFilter.Page)
	require.Equal(t, 10, repo.lastFilter.PageSize)
}

func TestConversationArchiveHandlerListOmitsContentByDefault(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &conversationArchiveHandlerRepo{
		total: 1,
		records: []*service.ConversationArchive{{
			ID:           2,
			RequestID:    "req-2",
			MessageCount: 3,
			CreatedAt:    time.Now().UTC(),
		}},
	}
	handler := newConversationArchiveHandler(repo)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/conversation-archives", nil)
	handler.List(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	data := decodeResponseData(t, recorder)
	items := data["items"].([]any)
	item := items[0].(map[string]any)
	require.NotContains(t, item, "response_text")
	require.NotContains(t, item, "request_messages")
	require.Equal(t, float64(3), item["message_count"])
	require.False(t, repo.lastFilter.IncludeContent)
}

func TestConversationArchiveHandlerListRejectsInvalidFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &conversationArchiveHandlerRepo{}
	handler := newConversationArchiveHandler(repo)

	for _, target := range []string{
		"/api/v1/admin/conversation-archives?protocol=unknown",
		"/api/v1/admin/conversation-archives?start_time=not-a-time",
		"/api/v1/admin/conversation-archives?user_id=0",
		"/api/v1/admin/conversation-archives?stream=maybe",
		"/api/v1/admin/conversation-archives?order=random",
		"/api/v1/admin/conversation-archives?start_time=2026-10-21&end_time=2026-10-20",
	} {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodGet, target, nil)
		handler.List(c)
		require.Equal(t, http.StatusBadRequest, recorder.Code, target)
	}
	require.Equal(t, 0, repo.listCalls)
}

func TestConversationArchiveHandlerGetReturnsFullContent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &conversationArchiveHandlerRepo{record: &service.ConversationArchive{
		ID:           5,
		RequestID:    "req-5",
		MessageCount: 1,
		Messages:     []service.ConversationArchiveMessage{{Role: "user", Content: "hi"}},
		ResponseText: "hello",
		CreatedAt:    time.Now().UTC(),
	}}
	handler := newConversationArchiveHandler(repo)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/conversation-archives/5", nil)
	c.Params = gin.Params{{Key: "id", Value: "5"}}
	handler.Get(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	data := decodeResponseData(t, recorder)
	require.Equal(t, float64(5), data["id"])
	require.Equal(t, "hello", data["response_text"])
}

func TestConversationArchiveHandlerGetNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &conversationArchiveHandlerRepo{}
	handler := newConversationArchiveHandler(repo)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/conversation-archives/99", nil)
	c.Params = gin.Params{{Key: "id", Value: "99"}}
	handler.Get(c)

	require.Equal(t, http.StatusNotFound, recorder.Code)
}

func TestConversationArchiveHandlerGetRejectsInvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := newConversationArchiveHandler(&conversationArchiveHandlerRepo{})

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/conversation-archives/abc", nil)
	c.Params = gin.Params{{Key: "id", Value: "abc"}}
	handler.Get(c)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
}
