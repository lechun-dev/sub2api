package service

import (
	"bytes"
	"encoding/json"
	"strings"
)

// ConversationTokenUsage 是响应中解析出的 token 用量；nil 表示上游未返回该字段。
type ConversationTokenUsage struct {
	InputTokens       *int64
	OutputTokens      *int64
	CachedInputTokens *int64
}

type anthropicUsage struct {
	InputTokens          *int64 `json:"input_tokens"`
	OutputTokens         *int64 `json:"output_tokens"`
	CacheReadInputTokens *int64 `json:"cache_read_input_tokens"`
}

type openAIUsageDetails struct {
	CachedTokens *int64 `json:"cached_tokens"`
}

type openAIUsage struct {
	PromptTokens       *int64              `json:"prompt_tokens"`
	CompletionTokens   *int64              `json:"completion_tokens"`
	InputTokens        *int64              `json:"input_tokens"`
	OutputTokens       *int64              `json:"output_tokens"`
	PromptTokensDetail *openAIUsageDetails `json:"prompt_tokens_details"`
	InputTokensDetail  *openAIUsageDetails `json:"input_tokens_details"`
}

type geminiUsage struct {
	PromptTokenCount        *int64 `json:"promptTokenCount"`
	CandidatesTokenCount    *int64 `json:"candidatesTokenCount"`
	CachedContentTokenCount *int64 `json:"cachedContentTokenCount"`
}

// ExtractConversationUsage 从客户端收到的响应中解析 token 用量，兼容 SSE 与非流式 JSON。
// 只解析响应中显式携带的用量；上游未返回时保持 nil。
func ExtractConversationUsage(protocol string, stream bool, body []byte) ConversationTokenUsage {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return ConversationTokenUsage{}
	}
	if stream || bytes.HasPrefix(body, []byte("event:")) || bytes.HasPrefix(body, []byte("data:")) {
		return extractConversationUsageSSE(protocol, body)
	}
	return extractConversationUsageJSON(protocol, body)
}

func extractConversationUsageJSON(protocol string, body []byte) ConversationTokenUsage {
	switch protocol {
	case ConversationArchiveProtocolAnthropicMessages:
		var response struct {
			Usage anthropicUsage `json:"usage"`
		}
		if json.Unmarshal(body, &response) != nil {
			return ConversationTokenUsage{}
		}
		return anthropicUsageToTokenUsage(response.Usage)
	case ConversationArchiveProtocolOpenAIChat, ConversationArchiveProtocolOpenAIResponses:
		var response struct {
			Usage openAIUsage `json:"usage"`
		}
		if json.Unmarshal(body, &response) != nil {
			return ConversationTokenUsage{}
		}
		return openAIUsageToTokenUsage(response.Usage)
	case ConversationArchiveProtocolGemini:
		var response struct {
			UsageMetadata geminiUsage `json:"usageMetadata"`
		}
		if json.Unmarshal(body, &response) != nil {
			return ConversationTokenUsage{}
		}
		return geminiUsageToTokenUsage(response.UsageMetadata)
	default:
		return ConversationTokenUsage{}
	}
}

func extractConversationUsageSSE(protocol string, body []byte) ConversationTokenUsage {
	usage := ConversationTokenUsage{}
	for _, rawLine := range bytes.Split(body, []byte("\n")) {
		line := strings.TrimSpace(string(rawLine))
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}
		mergeTokenUsage(&usage, extractConversationUsageEvent(protocol, []byte(payload)))
	}
	return usage
}

func extractConversationUsageEvent(protocol string, payload []byte) ConversationTokenUsage {
	switch protocol {
	case ConversationArchiveProtocolAnthropicMessages:
		var event struct {
			Message *struct {
				Usage anthropicUsage `json:"usage"`
			} `json:"message"`
			Usage *anthropicUsage `json:"usage"`
		}
		if json.Unmarshal(payload, &event) != nil {
			return ConversationTokenUsage{}
		}
		usage := ConversationTokenUsage{}
		if event.Message != nil {
			mergeTokenUsage(&usage, anthropicUsageToTokenUsage(event.Message.Usage))
		}
		if event.Usage != nil {
			mergeTokenUsage(&usage, anthropicUsageToTokenUsage(*event.Usage))
		}
		return usage
	case ConversationArchiveProtocolOpenAIChat:
		var event struct {
			Usage *openAIUsage `json:"usage"`
		}
		if json.Unmarshal(payload, &event) != nil || event.Usage == nil {
			return ConversationTokenUsage{}
		}
		return openAIUsageToTokenUsage(*event.Usage)
	case ConversationArchiveProtocolOpenAIResponses:
		var event struct {
			Response *struct {
				Usage *openAIUsage `json:"usage"`
			} `json:"response"`
			Usage *openAIUsage `json:"usage"`
		}
		if json.Unmarshal(payload, &event) != nil {
			return ConversationTokenUsage{}
		}
		usage := ConversationTokenUsage{}
		if event.Response != nil && event.Response.Usage != nil {
			mergeTokenUsage(&usage, openAIUsageToTokenUsage(*event.Response.Usage))
		}
		if event.Usage != nil {
			mergeTokenUsage(&usage, openAIUsageToTokenUsage(*event.Usage))
		}
		return usage
	case ConversationArchiveProtocolGemini:
		var event struct {
			UsageMetadata geminiUsage `json:"usageMetadata"`
		}
		if json.Unmarshal(payload, &event) != nil {
			return ConversationTokenUsage{}
		}
		return geminiUsageToTokenUsage(event.UsageMetadata)
	default:
		return ConversationTokenUsage{}
	}
}

func anthropicUsageToTokenUsage(usage anthropicUsage) ConversationTokenUsage {
	return ConversationTokenUsage{
		InputTokens:       usage.InputTokens,
		OutputTokens:      usage.OutputTokens,
		CachedInputTokens: usage.CacheReadInputTokens,
	}
}

func openAIUsageToTokenUsage(usage openAIUsage) ConversationTokenUsage {
	input := usage.InputTokens
	if input == nil {
		input = usage.PromptTokens
	}
	output := usage.OutputTokens
	if output == nil {
		output = usage.CompletionTokens
	}
	var cached *int64
	if usage.InputTokensDetail != nil && usage.InputTokensDetail.CachedTokens != nil {
		cached = usage.InputTokensDetail.CachedTokens
	} else if usage.PromptTokensDetail != nil {
		cached = usage.PromptTokensDetail.CachedTokens
	}
	return ConversationTokenUsage{InputTokens: input, OutputTokens: output, CachedInputTokens: cached}
}

func geminiUsageToTokenUsage(usage geminiUsage) ConversationTokenUsage {
	return ConversationTokenUsage{
		InputTokens:       usage.PromptTokenCount,
		OutputTokens:      usage.CandidatesTokenCount,
		CachedInputTokens: usage.CachedContentTokenCount,
	}
}

func mergeTokenUsage(dst *ConversationTokenUsage, src ConversationTokenUsage) {
	if src.InputTokens != nil {
		dst.InputTokens = src.InputTokens
	}
	if src.OutputTokens != nil {
		dst.OutputTokens = src.OutputTokens
	}
	if src.CachedInputTokens != nil {
		dst.CachedInputTokens = src.CachedInputTokens
	}
}
