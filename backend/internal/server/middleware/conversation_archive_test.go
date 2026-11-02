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

func (r *conversationArchiveTestRepo) Stats(context.Context, service.ConversationArchiveFilter, string, int, bool) ([]*service.ConversationArchiveAggregate, *service.ConversationArchiveTotals, error) {
	return nil, &service.ConversationArchiveTotals{}, nil
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

func TestConversationArchiveSkipRequested(t *testing.T) {
	cases := []struct {
		raw  string
		skip bool
	}{
		{raw: "", skip: false},
		{raw: "on", skip: false},
		{raw: "1", skip: false},
		{raw: "true", skip: false},
		{raw: "archive", skip: false},
		{raw: "skip", skip: true},
		{raw: "SKIP", skip: true},
		{raw: " off ", skip: true},
		{raw: "local", skip: true},
		{raw: "false", skip: true},
		{raw: "0", skip: true},
		{raw: "local,skip", skip: true},
		{raw: "on, local", skip: true},
		{raw: "on;off", skip: true},
	}
	for _, tc := range cases {
		require.Equal(t, tc.skip, conversationArchiveSkipRequested(tc.raw), "header value %q", tc.raw)
	}
}

func TestConversationArchiveMiddlewareSkipsArchivingWhenMarked(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &conversationArchiveTestRepo{}
	svc := newConversationArchiveTestService(t, repo, "true")

	var upstreamHeader string
	var upstreamBody string
	engine := gin.New()
	engine.Use(ConversationArchive(svc))
	engine.POST("/v1/messages", func(c *gin.Context) {
		// 标记头只给网关自己看，不能随请求转发到上游。
		upstreamHeader = c.GetHeader(ConversationArchiveSkipHeader)
		raw, err := io.ReadAll(c.Request.Body)
		require.NoError(t, err)
		upstreamBody = string(raw)
		c.JSON(http.StatusOK, gin.H{"content": []gin.H{{"type": "text", "text": "hi there"}}})
	})

	for _, value := range []string{"skip", "local", "OFF"} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/v1/messages",
			strings.NewReader(`{"model":"claude-sonnet-4","messages":[{"role":"user","content":"hello"}]}`))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set(ConversationArchiveSkipHeader, value)
		engine.ServeHTTP(recorder, request)

		require.Equal(t, http.StatusOK, recorder.Code, "客户端的正常请求不应受影响")
		require.Contains(t, recorder.Body.String(), "hi there")
		require.Empty(t, upstreamHeader, "标记头不应转发到上游")
		require.Contains(t, upstreamBody, "hello", "请求体应原样透传给上游")
	}

	svc.Stop()
	require.Empty(t, repo.snapshot(), "带标记的请求不应落库")
}

func TestConversationArchiveMiddlewareArchivesWhenMarkerIsNotSkipValue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &conversationArchiveTestRepo{}
	svc := newConversationArchiveTestService(t, repo, "true")

	var upstreamHeader string
	engine := gin.New()
	engine.Use(ConversationArchive(svc))
	engine.POST("/v1/messages", func(c *gin.Context) {
		upstreamHeader = c.GetHeader(ConversationArchiveSkipHeader)
		_, _ = io.ReadAll(c.Request.Body)
		c.JSON(http.StatusOK, gin.H{"content": []gin.H{{"type": "text", "text": "hi there"}}})
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/messages",
		strings.NewReader(`{"model":"claude-sonnet-4","messages":[{"role":"user","content":"hello"}]}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(ConversationArchiveSkipHeader, "on")
	engine.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Empty(t, upstreamHeader, "标记头不应转发到上游")

	svc.Stop()
	records := repo.snapshot()
	require.Len(t, records, 1, "未命中跳过取值的请求仍应归档")
	require.Equal(t, "claude-sonnet-4", records[0].Model)
}

type conversationArchiveMultiSettingRepo struct {
	service.SettingRepository
	values map[string]string
}

func (r conversationArchiveMultiSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	return r.values[key], nil
}

func newConversationArchiveTestServiceWithSettings(t *testing.T, repo service.ConversationArchiveRepository, values map[string]string) *service.ConversationArchiveService {
	t.Helper()
	settings := service.NewSettingService(conversationArchiveMultiSettingRepo{values: values}, nil)
	svc := service.NewConversationArchiveService(repo, settings)
	svc.Start()
	t.Cleanup(svc.Stop)
	return svc
}

func TestConversationArchiveMiddlewareSkipsExcludedHarnessMachine(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const machineID = "a11a320c-7a36-4aee-89ba-92f917953065"
	repo := &conversationArchiveTestRepo{}
	svc := newConversationArchiveTestServiceWithSettings(t, repo, map[string]string{
		service.SettingKeyConversationArchiveEnabled:                "true",
		service.SettingKeyConversationArchiveExcludedHarnessUserIDs: `["` + machineID + `"]`,
	})

	engine := gin.New()
	engine.Use(ConversationArchive(svc))
	engine.POST("/v1/messages", func(c *gin.Context) {
		_, err := io.ReadAll(c.Request.Body)
		require.NoError(t, err)
		c.JSON(http.StatusOK, gin.H{"content": []gin.H{{"type": "text", "text": "hi there"}}})
	})

	send := func(machine string) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/v1/messages",
			strings.NewReader(`{"model":"claude-sonnet-4","messages":[{"role":"user","content":"hello"}]}`))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-DeepSeek-Harness-User-Id", machine)
		engine.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusOK, recorder.Code, "排除只影响归档，不影响正常转发")
	}

	send(machineID)       // 本机：不归档
	send("other-machine") // 同事的机器：照常归档

	svc.Stop()
	records := repo.snapshot()
	require.Len(t, records, 1, "只有未被排除的机器应留下存档")
	require.Equal(t, "claude-sonnet-4", records[0].Model)
}

// 能自定义请求头的客户端（OpenClaw 等）用通用标识头上报机器名，同样命中排除名单。
func TestConversationArchiveMiddlewareSkipsExcludedClientIDHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const clientID = "openclaw-lq-mbp"
	repo := &conversationArchiveTestRepo{}
	svc := newConversationArchiveTestServiceWithSettings(t, repo, map[string]string{
		service.SettingKeyConversationArchiveEnabled:                "true",
		service.SettingKeyConversationArchiveExcludedHarnessUserIDs: `["` + clientID + `"]`,
	})

	var upstreamClientID string
	engine := gin.New()
	engine.Use(ConversationArchive(svc))
	engine.POST("/v1/messages", func(c *gin.Context) {
		upstreamClientID = c.GetHeader(service.ConversationArchiveClientIDHeader)
		_, err := io.ReadAll(c.Request.Body)
		require.NoError(t, err)
		c.JSON(http.StatusOK, gin.H{"content": []gin.H{{"type": "text", "text": "hi there"}}})
	})

	send := func(id string) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/v1/messages",
			strings.NewReader(`{"model":"claude-sonnet-4","messages":[{"role":"user","content":"hello"}]}`))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set(service.ConversationArchiveClientIDHeader, id)
		engine.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusOK, recorder.Code, "排除只影响归档，不影响正常转发")
	}

	send(clientID)       // 命中名单：不归档
	send("another-host") // 未命中：照常归档

	svc.Stop()
	require.Empty(t, upstreamClientID, "客户端标识头不应转发到上游")
	records := repo.snapshot()
	require.Len(t, records, 1, "只有未命中名单的客户端应留下存档")
	require.Equal(t, "claude-sonnet-4", records[0].Model)
}
