package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func requireTokenUsage(t *testing.T, usage ConversationTokenUsage, input, output, cached int64) {
	t.Helper()
	require.NotNil(t, usage.InputTokens)
	require.Equal(t, input, *usage.InputTokens)
	require.NotNil(t, usage.OutputTokens)
	require.Equal(t, output, *usage.OutputTokens)
	require.NotNil(t, usage.CachedInputTokens)
	require.Equal(t, cached, *usage.CachedInputTokens)
}

func TestExtractConversationUsageAnthropic(t *testing.T) {
	t.Parallel()

	jsonUsage := ExtractConversationUsage(ConversationArchiveProtocolAnthropicMessages, false, []byte(
		`{"usage":{"input_tokens":10,"output_tokens":20,"cache_read_input_tokens":5}}`))
	requireTokenUsage(t, jsonUsage, 10, 20, 5)

	sseUsage := ExtractConversationUsage(ConversationArchiveProtocolAnthropicMessages, true, []byte(
		"event: message_start\n"+
			"data: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":10,\"cache_read_input_tokens\":5}}}\n\n"+
			"event: message_delta\n"+
			"data: {\"type\":\"message_delta\",\"usage\":{\"output_tokens\":20}}\n\n"))
	requireTokenUsage(t, sseUsage, 10, 20, 5)
}

func TestExtractConversationUsageOpenAI(t *testing.T) {
	t.Parallel()

	chatUsage := ExtractConversationUsage(ConversationArchiveProtocolOpenAIChat, false, []byte(
		`{"usage":{"prompt_tokens":10,"completion_tokens":20,"prompt_tokens_details":{"cached_tokens":5}}}`))
	requireTokenUsage(t, chatUsage, 10, 20, 5)

	streamUsage := ExtractConversationUsage(ConversationArchiveProtocolOpenAIChat, true, []byte(
		"data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\n"+
			"data: {\"choices\":[],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":20,\"prompt_tokens_details\":{\"cached_tokens\":5}}}\n\n"+
			"data: [DONE]\n\n"))
	requireTokenUsage(t, streamUsage, 10, 20, 5)

	responsesUsage := ExtractConversationUsage(ConversationArchiveProtocolOpenAIResponses, false, []byte(
		`{"usage":{"input_tokens":10,"output_tokens":20,"input_tokens_details":{"cached_tokens":5}}}`))
	requireTokenUsage(t, responsesUsage, 10, 20, 5)

	responsesStream := ExtractConversationUsage(ConversationArchiveProtocolOpenAIResponses, true, []byte(
		"data: {\"type\":\"response.output_text.delta\",\"delta\":\"hi\"}\n\n"+
			"data: {\"type\":\"response.completed\",\"response\":{\"usage\":{\"input_tokens\":10,\"output_tokens\":20,\"input_tokens_details\":{\"cached_tokens\":5}}}}\n\n"))
	requireTokenUsage(t, responsesStream, 10, 20, 5)
}

func TestExtractConversationUsageGemini(t *testing.T) {
	t.Parallel()

	jsonUsage := ExtractConversationUsage(ConversationArchiveProtocolGemini, false, []byte(
		`{"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":20,"cachedContentTokenCount":5}}`))
	requireTokenUsage(t, jsonUsage, 10, 20, 5)

	sseUsage := ExtractConversationUsage(ConversationArchiveProtocolGemini, true, []byte(
		"data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"hi\"}]}}],\"usageMetadata\":{\"promptTokenCount\":10,\"candidatesTokenCount\":5}}\n\n"+
			"data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"!\"}]}}],\"usageMetadata\":{\"promptTokenCount\":10,\"candidatesTokenCount\":20,\"cachedContentTokenCount\":5}}\n\n"))
	requireTokenUsage(t, sseUsage, 10, 20, 5)
}

func TestExtractConversationUsageMissing(t *testing.T) {
	t.Parallel()

	for _, protocol := range []string{
		ConversationArchiveProtocolAnthropicMessages,
		ConversationArchiveProtocolOpenAIChat,
		ConversationArchiveProtocolOpenAIResponses,
		ConversationArchiveProtocolGemini,
	} {
		usage := ExtractConversationUsage(protocol, false, []byte(`{"choices":[{"message":{"content":"hi"}}]}`))
		require.Nil(t, usage.InputTokens, protocol)
		require.Nil(t, usage.OutputTokens, protocol)
		require.Nil(t, usage.CachedInputTokens, protocol)
	}
}

func TestBuildConversationArchiveStoresTokenUsage(t *testing.T) {
	t.Parallel()

	record := BuildConversationArchive(ConversationArchiveCaptureInput{
		Protocol:      ConversationArchiveProtocolOpenAIChat,
		Endpoint:      "/v1/chat/completions",
		StatusCode:    200,
		ContentType:   "application/json",
		RequestBytes:  []byte(`{"model":"gpt-5","messages":[{"role":"user","content":"hi"}]}`),
		ResponseBytes: []byte(`{"choices":[{"message":{"content":"hello"}}],"usage":{"prompt_tokens":10,"completion_tokens":20,"prompt_tokens_details":{"cached_tokens":5}}}`),
	})
	require.NotNil(t, record)
	require.NotNil(t, record.InputTokens)
	require.Equal(t, int64(10), *record.InputTokens)
	require.Equal(t, int64(20), *record.OutputTokens)
	require.Equal(t, int64(5), *record.CachedInputTokens)
}
