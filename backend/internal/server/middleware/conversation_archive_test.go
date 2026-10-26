package middleware

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type conversationArchiveTestSettingRepo struct {
	service.SettingRepository
	value string
}

func (r conversationArchiveTestSettingRepo) GetValue(context.Context, string) (string, error) {
	return r.value, nil
}

type conversationArchiveTestRepo struct {
	mu      sync.Mutex
	records []*service.ConversationArchive
}

func (r *conversationArchiveTestRepo) CreateBatch(_ context.Context, records []*service.ConversationArchive) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records = append(r.records, records...)
	return nil
}

func (r *conversationArchiveTestRepo) DeleteBefore(context.Context, time.Time, int) (int64, error) {
	return 0, nil
}

func (r *conversationArchiveTestRepo) List(context.Context, service.ConversationArchiveFilter) ([]*service.ConversationArchive, int64, error) {
	return nil, 0, nil
}

func (r *conversationArchiveTestRepo) GetByID(context.Context, int64) (*service.ConversationArchive, error) {
	return nil, nil
}

func (r *conversationArchiveTestRepo) snapshot() []*service.ConversationArchive {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]*service.ConversationArchive(nil), r.records...)
}

func newConversationArchiveTestService(t *testing.T, repo service.ConversationArchiveRepository, enabled string) *service.ConversationArchiveService {
	t.Helper()
	settings := service.NewSettingService(conversationArchiveTestSettingRepo{value: enabled}, nil)
	svc := service.NewConversationArchiveService(repo, settings)
	svc.Start()
	t.Cleanup(svc.Stop)
	return svc
}

func TestConversationArchiveMiddlewareCapturesTextConversation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &conversationArchiveTestRepo{}
	svc := newConversationArchiveTestService(t, repo, "true")

	engine := gin.New()
	engine.Use(ConversationArchive(svc))
	engine.POST("/v1/messages", func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		require.NoError(t, err)
		require.Contains(t, string(body), "hello")
		c.JSON(http.StatusOK, gin.H{"content": []gin.H{{"type": "text", "text": "hi there"}}})
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/messages",
		strings.NewReader(`{"model":"claude-sonnet-4","messages":[{"role":"user","content":"hello"}]}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("session_id", "session-1")
	engine.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	svc.Stop()

	records := repo.snapshot()
	require.Len(t, records, 1)
	require.Equal(t, "claude-sonnet-4", records[0].Model)
	require.Equal(t, "session-1", records[0].SessionKey)
	require.Equal(t, []service.ConversationArchiveMessage{{Role: "user", Content: "hello"}}, records[0].Messages)
	require.Equal(t, "hi there", records[0].ResponseText)
	require.Equal(t, 200, records[0].StatusCode)
	require.False(t, records[0].Stream)
}

func TestConversationArchiveMiddlewareDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &conversationArchiveTestRepo{}
	svc := newConversationArchiveTestService(t, repo, "false")

	engine := gin.New()
	engine.Use(ConversationArchive(svc))
	engine.POST("/v1/messages", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"content": []gin.H{{"type": "text", "text": "hi"}}})
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/messages",
		strings.NewReader(`{"model":"claude","messages":[{"role":"user","content":"hello"}]}`))
	engine.ServeHTTP(recorder, request)

	svc.Stop()
	require.Empty(t, repo.snapshot())
}

func TestConversationArchiveMiddlewareSkipsNonTargetAndErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &conversationArchiveTestRepo{}
	svc := newConversationArchiveTestService(t, repo, "true")

	engine := gin.New()
	engine.Use(ConversationArchive(svc))
	engine.POST("/v1/embeddings", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"data": []gin.H{}})
	})
	engine.POST("/v1/messages", func(c *gin.Context) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "boom"})
	})

	for _, target := range []string{"/v1/embeddings", "/v1/messages"} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, target,
			strings.NewReader(`{"model":"claude","messages":[{"role":"user","content":"hello"}]}`))
		engine.ServeHTTP(recorder, request)
	}

	svc.Stop()
	require.Empty(t, repo.snapshot())
}
