package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestConversationArchiveTarget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		method   string
		path     string
		protocol string
		model    string
		ok       bool
	}{
		{name: "anthropic messages", method: "POST", path: "/v1/messages", protocol: ConversationArchiveProtocolAnthropicMessages, ok: true},
		{name: "antigravity messages", method: "POST", path: "/antigravity/v1/messages", protocol: ConversationArchiveProtocolAnthropicMessages, ok: true},
		{name: "chat completions", method: "POST", path: "/v1/chat/completions", protocol: ConversationArchiveProtocolOpenAIChat, ok: true},
		{name: "responses", method: "POST", path: "/v1/responses", protocol: ConversationArchiveProtocolOpenAIResponses, ok: true},
		{name: "codex responses", method: "POST", path: "/backend-api/codex/responses", protocol: ConversationArchiveProtocolOpenAIResponses, ok: true},
		{name: "gemini generate", method: "POST", path: "/v1beta/models/gemini-2.5-pro:generateContent", protocol: ConversationArchiveProtocolGemini, model: "gemini-2.5-pro", ok: true},
		{name: "gemini stream", method: "POST", path: "/v1beta/models/gemini-2.5-pro:streamGenerateContent", protocol: ConversationArchiveProtocolGemini, model: "gemini-2.5-pro", ok: true},
		{name: "count tokens excluded", method: "POST", path: "/v1/messages/count_tokens", ok: false},
		{name: "gemini count tokens excluded", method: "POST", path: "/v1beta/models/gemini-2.5-pro:countTokens", ok: false},
		{name: "embeddings excluded", method: "POST", path: "/v1/embeddings", ok: false},
		{name: "images excluded", method: "POST", path: "/v1/images/generations", ok: false},
		{name: "get excluded", method: "GET", path: "/v1/messages", ok: false},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			protocol, model, ok := ConversationArchiveTarget(tc.method, tc.path)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.protocol, protocol)
			require.Equal(t, tc.model, model)
		})
	}
}

func TestExtractConversationRequestAnthropic(t *testing.T) {
	t.Parallel()

	body := []byte(`{
		"model": "claude-sonnet-4",
		"system": [{"type":"text","text":"be nice"},{"type":"image","source":{}}],
		"messages": [
			{"role":"user","content":[{"type":"text","text":"hello"},{"type":"image","source":{"data":"xx"}}]},
			{"role":"assistant","content":"hi there"},
			{"role":"user","content":[{"type":"tool_result","tool_use_id":"t","content":"ok"}]}
		]
	}`)
	messages, model, ok := ExtractConversationRequest(ConversationArchiveProtocolAnthropicMessages, body)
	require.True(t, ok)
	require.Equal(t, "claude-sonnet-4", model)
	require.Equal(t, []ConversationArchiveMessage{
		{Role: "system", Content: "be nice"},
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi there"},
	}, messages)
}

func TestExtractConversationRequestChat(t *testing.T) {
	t.Parallel()

	body := []byte(`{
		"model": "gpt-5",
		"messages": [
			{"role":"developer","content":"rules"},
			{"role":"user","content":[{"type":"text","text":"hi"},{"type":"image_url","image_url":{"url":"data:..."}}]},
			{"role":"assistant","content":"hello"}
		]
	}`)
	messages, model, ok := ExtractConversationRequest(ConversationArchiveProtocolOpenAIChat, body)
	require.True(t, ok)
	require.Equal(t, "gpt-5", model)
	require.Equal(t, []ConversationArchiveMessage{
		{Role: "system", Content: "rules"},
		{Role: "user", Content: "hi"},
		{Role: "assistant", Content: "hello"},
	}, messages)
}

func TestExtractConversationRequestResponses(t *testing.T) {
	t.Parallel()

	body := []byte(`{
		"model": "gpt-5.1",
		"instructions": "be brief",
		"previous_response_id": "resp_123",
		"input": [
			{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]},
			{"type":"function_call_output","call_id":"c1","output":"secret"},
			{"role":"assistant","content":"hello"}
		]
	}`)
	messages, model, ok := ExtractConversationRequest(ConversationArchiveProtocolOpenAIResponses, body)
	require.True(t, ok)
	require.Equal(t, "gpt-5.1", model)
	require.Equal(t, []ConversationArchiveMessage{
		{Role: "system", Content: "be brief"},
		{Role: "user", Content: "hi"},
		{Role: "assistant", Content: "hello"},
	}, messages)
}

func TestExtractConversationRequestResponsesStringInput(t *testing.T) {
	t.Parallel()

	body := []byte(`{"model":"gpt-5","input":"hey"}`)
	messages, _, ok := ExtractConversationRequest(ConversationArchiveProtocolOpenAIResponses, body)
	require.True(t, ok)
	require.Equal(t, []ConversationArchiveMessage{{Role: "user", Content: "hey"}}, messages)
}

func TestExtractConversationRequestGemini(t *testing.T) {
	t.Parallel()

	body := []byte(`{
		"systemInstruction": {"parts":[{"text":"be kind"}]},
		"contents": [
			{"role":"user","parts":[{"text":"hi"},{"inlineData":{"mimeType":"image/png","data":"xx"}}]},
			{"role":"model","parts":[{"text":"hello"}]}
		]
	}`)
	messages, model, ok := ExtractConversationRequest(ConversationArchiveProtocolGemini, body)
	require.True(t, ok)
	require.Equal(t, "", model)
	require.Equal(t, []ConversationArchiveMessage{
		{Role: "system", Content: "be kind"},
		{Role: "user", Content: "hi"},
		{Role: "assistant", Content: "hello"},
	}, messages)
}

func TestExtractConversationResponseJSON(t *testing.T) {
	t.Parallel()

	anthropic, ok := ExtractConversationResponse(ConversationArchiveProtocolAnthropicMessages, false,
		[]byte(`{"content":[{"type":"text","text":"hello"},{"type":"tool_use","id":"t","name":"x"}]}`))
	require.True(t, ok)
	require.Equal(t, "hello", anthropic)

	chat, ok := ExtractConversationResponse(ConversationArchiveProtocolOpenAIChat, false,
		[]byte(`{"choices":[{"message":{"role":"assistant","content":"hello"}}]}`))
	require.True(t, ok)
	require.Equal(t, "hello", chat)

	responses, ok := ExtractConversationResponse(ConversationArchiveProtocolOpenAIResponses, false,
		[]byte(`{"output":[{"type":"message","content":[{"type":"output_text","text":"hello"}]},{"type":"function_call"}]}`))
	require.True(t, ok)
	require.Equal(t, "hello", responses)

	gemini, ok := ExtractConversationResponse(ConversationArchiveProtocolGemini, false,
		[]byte(`{"candidates":[{"content":{"parts":[{"text":"hel"},{"text":"lo"}]}}]}`))
	require.True(t, ok)
	require.Equal(t, "hello", gemini)
}

func TestExtractConversationResponseSSE(t *testing.T) {
	t.Parallel()

	anthropic, ok := ExtractConversationResponse(ConversationArchiveProtocolAnthropicMessages, true, []byte(
		"event: content_block_delta\n"+
			"data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"Hel\"}}\n\n"+
			"data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"lo\"}}\n\n"))
	require.True(t, ok)
	require.Equal(t, "Hello", anthropic)

	chat, ok := ExtractConversationResponse(ConversationArchiveProtocolOpenAIChat, true, []byte(
		"data: {\"choices\":[{\"delta\":{\"content\":\"Hi\"}}]}\n\n"+
			"data: {\"choices\":[{\"delta\":{\"content\":\"!\"}}]}\n\n"+
			"data: [DONE]\n\n"))
	require.True(t, ok)
	require.Equal(t, "Hi!", chat)

	responses, ok := ExtractConversationResponse(ConversationArchiveProtocolOpenAIResponses, true, []byte(
		"data: {\"type\":\"response.output_text.delta\",\"delta\":\"Hi\"}\n\n"+
			"data: {\"type\":\"response.output_text.delta\",\"delta\":\"!\"}\n\n"))
	require.True(t, ok)
	require.Equal(t, "Hi!", responses)

	gemini, ok := ExtractConversationResponse(ConversationArchiveProtocolGemini, true, []byte(
		"data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"Hi\"}]}}]}\n\n"+
			"data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"!\"}]}}]}\n\n"))
	require.True(t, ok)
	require.Equal(t, "Hi!", gemini)
}

func TestExtractConversationResponseSSEResponsesCompletedFallback(t *testing.T) {
	t.Parallel()

	body := []byte("data: {\"type\":\"response.completed\",\"response\":{\"output\":[{\"type\":\"message\",\"content\":[{\"type\":\"output_text\",\"text\":\"final\"}]}]}}\n\n")
	text, ok := ExtractConversationResponse(ConversationArchiveProtocolOpenAIResponses, true, body)
	require.True(t, ok)
	require.Equal(t, "final", text)
}

func TestBuildConversationArchive(t *testing.T) {
	t.Parallel()

	record := BuildConversationArchive(ConversationArchiveCaptureInput{
		RequestID:     "req-1",
		UserID:        7,
		APIKeyID:      9,
		Protocol:      ConversationArchiveProtocolAnthropicMessages,
		Endpoint:      "/v1/messages",
		StatusCode:    200,
		ContentType:   "application/json",
		RequestBytes:  []byte(`{"model":"claude","messages":[{"role":"user","content":"hi"}]}`),
		ResponseBytes: []byte(`{"content":[{"type":"text","text":"hello"}]}`),
		Headers:       map[string]string{"session_id": "s-1"},
	})
	require.NotNil(t, record)
	require.Equal(t, int64(7), record.UserID)
	require.Equal(t, "claude", record.Model)
	require.Equal(t, "s-1", record.SessionKey)
	require.Equal(t, []ConversationArchiveMessage{{Role: "user", Content: "hi"}}, record.Messages)
	require.Equal(t, "hello", record.ResponseText)
	require.False(t, record.Truncated)
}

func TestBuildConversationArchiveUsesPathModel(t *testing.T) {
	t.Parallel()

	record := BuildConversationArchive(ConversationArchiveCaptureInput{
		Protocol:     ConversationArchiveProtocolGemini,
		Endpoint:     "/v1beta/models/gemini-2.5-pro:generateContent",
		PathModel:    "gemini-2.5-pro",
		StatusCode:   200,
		RequestBytes: []byte(`{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`),
	})
	require.NotNil(t, record)
	require.Equal(t, "gemini-2.5-pro", record.Model)
	require.Equal(t, "hi", record.Messages[0].Content)
}

func TestBuildConversationArchiveRejectsOversizedRequest(t *testing.T) {
	t.Parallel()

	record := BuildConversationArchive(ConversationArchiveCaptureInput{
		Protocol:     ConversationArchiveProtocolAnthropicMessages,
		RequestBytes: []byte(`{"model":"claude","messages":[{"role":"user","content":"hi"}]}`),
		RequestLimit: 10,
	})
	require.Nil(t, record)
}

func TestBuildConversationArchiveMarksTruncation(t *testing.T) {
	t.Parallel()

	record := BuildConversationArchive(ConversationArchiveCaptureInput{
		Protocol:      ConversationArchiveProtocolAnthropicMessages,
		RequestBytes:  []byte(`{"model":"claude","messages":[{"role":"user","content":"hi"}]}`),
		ResponseBytes: []byte(`{"content":[{"type":"text","text":"hello"}]}`),
		ResponseLimit: 5,
	})
	require.NotNil(t, record)
	require.True(t, record.Truncated)
}

func TestParseConversationArchiveRetentionDays(t *testing.T) {
	t.Parallel()

	require.Equal(t, defaultConversationArchiveRetentionDays, parseConversationArchiveRetentionDays(""))
	require.Equal(t, defaultConversationArchiveRetentionDays, parseConversationArchiveRetentionDays("abc"))
	require.Equal(t, 30, parseConversationArchiveRetentionDays("30"))
	require.Equal(t, 0, parseConversationArchiveRetentionDays("-1"))
}

type recordingConversationArchiveRepo struct {
	mu              sync.Mutex
	records         []*ConversationArchive
	statsItems      []*ConversationArchiveAggregate
	statsTotals     *ConversationArchiveTotals
	statsGroupBy    string
	statsLimit      int
	statsDescending bool
}

func (r *recordingConversationArchiveRepo) CreateBatch(_ context.Context, records []*ConversationArchive) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, record := range records {
		r.records = append(r.records, record)
	}
	return nil
}

func (r *recordingConversationArchiveRepo) DeleteBefore(context.Context, time.Time, int) (int64, error) {
	return 0, nil
}

func (r *recordingConversationArchiveRepo) List(context.Context, ConversationArchiveFilter) ([]*ConversationArchive, int64, error) {
	return nil, 0, nil
}

func (r *recordingConversationArchiveRepo) GetByID(context.Context, int64) (*ConversationArchive, error) {
	return nil, nil
}

func (r *recordingConversationArchiveRepo) Stats(_ context.Context, _ ConversationArchiveFilter, groupBy string, limit int, descending bool) ([]*ConversationArchiveAggregate, *ConversationArchiveTotals, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.statsGroupBy = groupBy
	r.statsLimit = limit
	r.statsDescending = descending
	return r.statsItems, r.statsTotals, nil
}

func (r *recordingConversationArchiveRepo) snapshot() []*ConversationArchive {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]*ConversationArchive(nil), r.records...)
}

func TestConversationArchiveServiceFlushesQueueOnStop(t *testing.T) {
	t.Parallel()

	repo := &recordingConversationArchiveRepo{}
	svc := NewConversationArchiveService(repo, nil)
	svc.Start()
	svc.Record(&ConversationArchive{RequestID: "req-1", Model: "claude"})
	svc.Record(&ConversationArchive{RequestID: "req-2", Model: "claude"})
	svc.Stop()

	records := repo.snapshot()
	require.Len(t, records, 2)
	written, dropped, failed := svc.Stats()
	require.Equal(t, uint64(2), written)
	require.Equal(t, uint64(0), dropped)
	require.Equal(t, uint64(0), failed)
}

type conversationArchiveSettingRepo struct {
	SettingRepository
	value string
	err   error
}

func (r conversationArchiveSettingRepo) GetValue(context.Context, string) (string, error) {
	return r.value, r.err
}

func TestConversationArchiveServiceUsageStats(t *testing.T) {
	t.Parallel()

	repo := &recordingConversationArchiveRepo{
		statsItems:  []*ConversationArchiveAggregate{{Key: "7", Records: 2, InputTokens: 10}},
		statsTotals: &ConversationArchiveTotals{Records: 2, InputTokens: 10},
	}
	svc := NewConversationArchiveService(repo, nil)

	items, totals, err := svc.UsageStats(context.Background(), ConversationArchiveFilter{}, "user", 500, false)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, int64(10), totals.InputTokens)
	require.Equal(t, "user", repo.statsGroupBy)
	require.Equal(t, conversationArchiveStatsMaxLimit, repo.statsLimit)
	require.False(t, repo.statsDescending)
}

func TestConversationArchiveServiceUsageStatsRejectsInvalidGroup(t *testing.T) {
	t.Parallel()

	repo := &recordingConversationArchiveRepo{}
	svc := NewConversationArchiveService(repo, nil)

	_, _, err := svc.UsageStats(context.Background(), ConversationArchiveFilter{}, "unknown", 0, true)
	require.Error(t, err)
	require.Empty(t, repo.statsGroupBy)
}

func TestConversationArchiveServiceEnabled(t *testing.T) {
	t.Parallel()

	settings := NewSettingService(conversationArchiveSettingRepo{value: "true"}, nil)
	svc := NewConversationArchiveService(&recordingConversationArchiveRepo{}, settings)
	require.True(t, svc.Enabled(context.Background()))

	disabled := NewConversationArchiveService(&recordingConversationArchiveRepo{},
		NewSettingService(conversationArchiveSettingRepo{value: "false"}, nil))
	require.False(t, disabled.Enabled(context.Background()))

	failed := NewConversationArchiveService(&recordingConversationArchiveRepo{},
		NewSettingService(conversationArchiveSettingRepo{err: errors.New("boom")}, nil))
	require.False(t, failed.Enabled(context.Background()))
}

func TestExtractConversationGeminiSkipsThoughtParts(t *testing.T) {
	t.Parallel()

	body := []byte(`{
		"contents": [
			{"role":"user","parts":[{"text":"hi"}]},
			{"role":"model","parts":[{"text":"internal reasoning","thought":true},{"text":"hello"}]}
		]
	}`)
	messages, _, ok := ExtractConversationRequest(ConversationArchiveProtocolGemini, body)
	require.True(t, ok)
	require.Equal(t, []ConversationArchiveMessage{
		{Role: "user", Content: "hi"},
		{Role: "assistant", Content: "hello"},
	}, messages)

	response, ok := ExtractConversationResponse(ConversationArchiveProtocolGemini, false,
		[]byte(`{"candidates":[{"content":{"parts":[{"text":"secret thought","thought":true},{"text":"answer"}]}}]}`))
	require.True(t, ok)
	require.Equal(t, "answer", response)

	streamed, ok := ExtractConversationResponse(ConversationArchiveProtocolGemini, true,
		[]byte("data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"think\",\"thought\":true}]}}]}\n\n"+
			"data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"answer\"}]}}]}\n\n"))
	require.True(t, ok)
	require.Equal(t, "answer", streamed)
}
