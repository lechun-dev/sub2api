package service

import (
	"bytes"
	"encoding/json"
	"strings"
)

// ExtractConversationRequest 从客户端请求体中抽取文本消息与模型名。
// 只保留文本内容：图片、附件、工具调用与思考块会被丢弃。
func ExtractConversationRequest(protocol string, body []byte) ([]ConversationArchiveMessage, string, bool) {
	body = bytes.TrimSpace(body)
	if len(body) == 0 || body[0] != '{' {
		return nil, "", false
	}
	switch protocol {
	case ConversationArchiveProtocolAnthropicMessages:
		return extractAnthropicRequest(body)
	case ConversationArchiveProtocolOpenAIChat:
		return extractChatRequest(body)
	case ConversationArchiveProtocolOpenAIResponses:
		return extractResponsesRequest(body)
	case ConversationArchiveProtocolGemini:
		return extractGeminiRequest(body)
	default:
		return nil, "", false
	}
}

// ExtractConversationResponse 从客户端收到的响应中抽取助手文本；兼容 SSE 与非流式 JSON。
func ExtractConversationResponse(protocol string, stream bool, body []byte) (string, bool) {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return "", false
	}
	if stream || bytes.HasPrefix(body, []byte("event:")) || bytes.HasPrefix(body, []byte("data:")) {
		return extractConversationSSE(protocol, body)
	}
	return extractConversationJSONResponse(protocol, body)
}

type conversationContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func conversationContentText(raw json.RawMessage) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return ""
	}
	switch raw[0] {
	case '"':
		var text string
		if json.Unmarshal(raw, &text) == nil {
			return text
		}
	case '[':
		var blocks []conversationContentBlock
		if json.Unmarshal(raw, &blocks) == nil {
			parts := make([]string, 0, len(blocks))
			for _, block := range blocks {
				if conversationTextBlockType(block.Type) && block.Text != "" {
					parts = append(parts, block.Text)
				}
			}
			return strings.Join(parts, "\n")
		}
	case '{':
		var block conversationContentBlock
		if json.Unmarshal(raw, &block) == nil && conversationTextBlockType(block.Type) {
			return block.Text
		}
	}
	return ""
}

func conversationTextBlockType(blockType string) bool {
	switch strings.ToLower(strings.TrimSpace(blockType)) {
	case "text", "input_text", "output_text":
		return true
	default:
		return false
	}
}

func normalizeConversationRole(role string) string {
	normalized := strings.ToLower(strings.TrimSpace(role))
	switch normalized {
	case "model":
		return "assistant"
	case "developer":
		return "system"
	case "":
		return "user"
	default:
		return normalized
	}
}

func appendConversationMessage(messages []ConversationArchiveMessage, role, content string) []ConversationArchiveMessage {
	content = strings.TrimSpace(content)
	if content == "" {
		return messages
	}
	return append(messages, ConversationArchiveMessage{Role: normalizeConversationRole(role), Content: content})
}

func extractAnthropicRequest(body []byte) ([]ConversationArchiveMessage, string, bool) {
	var request struct {
		Model    string          `json:"model"`
		System   json.RawMessage `json:"system"`
		Messages []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &request); err != nil {
		return nil, "", false
	}
	messages := make([]ConversationArchiveMessage, 0, len(request.Messages)+1)
	messages = appendConversationMessage(messages, "system", conversationContentText(request.System))
	for _, message := range request.Messages {
		messages = appendConversationMessage(messages, message.Role, conversationContentText(message.Content))
	}
	return messages, strings.TrimSpace(request.Model), true
}

func extractChatRequest(body []byte) ([]ConversationArchiveMessage, string, bool) {
	var request struct {
		Model    string `json:"model"`
		Messages []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &request); err != nil || len(request.Messages) == 0 {
		return nil, "", false
	}
	messages := make([]ConversationArchiveMessage, 0, len(request.Messages))
	for _, message := range request.Messages {
		messages = appendConversationMessage(messages, message.Role, conversationContentText(message.Content))
	}
	return messages, strings.TrimSpace(request.Model), true
}

func extractResponsesRequest(body []byte) ([]ConversationArchiveMessage, string, bool) {
	var request struct {
		Model        string          `json:"model"`
		Instructions json.RawMessage `json:"instructions"`
		Input        json.RawMessage `json:"input"`
	}
	if err := json.Unmarshal(body, &request); err != nil {
		return nil, "", false
	}
	messages := make([]ConversationArchiveMessage, 0, 8)
	messages = appendConversationMessage(messages, "system", conversationContentText(request.Instructions))

	input := bytes.TrimSpace(request.Input)
	if len(input) == 0 || bytes.Equal(input, []byte("null")) {
		return messages, strings.TrimSpace(request.Model), true
	}
	if input[0] == '"' {
		var text string
		if json.Unmarshal(input, &text) == nil {
			messages = appendConversationMessage(messages, "user", text)
		}
		return messages, strings.TrimSpace(request.Model), true
	}
	var items []struct {
		Type    string          `json:"type"`
		Role    string          `json:"role"`
		Text    string          `json:"text"`
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(input, &items); err != nil {
		return messages, strings.TrimSpace(request.Model), true
	}
	for _, item := range items {
		if content := conversationContentText(item.Content); content != "" {
			messages = appendConversationMessage(messages, item.Role, content)
			continue
		}
		if strings.EqualFold(item.Type, "input_text") || strings.EqualFold(item.Type, "text") {
			messages = appendConversationMessage(messages, "user", item.Text)
		}
	}
	return messages, strings.TrimSpace(request.Model), true
}

func extractGeminiRequest(body []byte) ([]ConversationArchiveMessage, string, bool) {
	type geminiParts struct {
		Parts []struct {
			Text    string `json:"text"`
			Thought bool   `json:"thought"`
		} `json:"parts"`
	}
	var request struct {
		Model       string       `json:"model"`
		System      *geminiParts `json:"systemInstruction"`
		SystemSnake *geminiParts `json:"system_instruction"`
		Contents    []struct {
			Role  string `json:"role"`
			Parts []struct {
				Text    string `json:"text"`
				Thought bool   `json:"thought"`
			} `json:"parts"`
		} `json:"contents"`
	}
	if err := json.Unmarshal(body, &request); err != nil || len(request.Contents) == 0 {
		return nil, "", false
	}
	messages := make([]ConversationArchiveMessage, 0, len(request.Contents)+1)
	system := request.System
	if system == nil {
		system = request.SystemSnake
	}
	if system != nil {
		parts := make([]string, 0, len(system.Parts))
		for _, part := range system.Parts {
			if strings.TrimSpace(part.Text) != "" && !part.Thought {
				parts = append(parts, part.Text)
			}
		}
		messages = appendConversationMessage(messages, "system", strings.Join(parts, "\n"))
	}
	for _, content := range request.Contents {
		parts := make([]string, 0, len(content.Parts))
		for _, part := range content.Parts {
			if strings.TrimSpace(part.Text) != "" && !part.Thought {
				parts = append(parts, part.Text)
			}
		}
		messages = appendConversationMessage(messages, content.Role, strings.Join(parts, "\n"))
	}
	return messages, strings.TrimSpace(request.Model), true
}

func extractConversationJSONResponse(protocol string, body []byte) (string, bool) {
	if body[0] != '{' {
		return "", false
	}
	switch protocol {
	case ConversationArchiveProtocolAnthropicMessages:
		var response struct {
			Content []conversationContentBlock `json:"content"`
		}
		if json.Unmarshal(body, &response) != nil {
			return "", false
		}
		return joinConversationTextBlocks(response.Content)
	case ConversationArchiveProtocolOpenAIChat:
		var response struct {
			Choices []struct {
				Message struct {
					Content json.RawMessage `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if json.Unmarshal(body, &response) != nil || len(response.Choices) == 0 {
			return "", false
		}
		text := conversationContentText(response.Choices[0].Message.Content)
		return text, text != ""
	case ConversationArchiveProtocolOpenAIResponses:
		var response struct {
			Output []struct {
				Type    string                     `json:"type"`
				Content []conversationContentBlock `json:"content"`
			} `json:"output"`
		}
		if json.Unmarshal(body, &response) != nil {
			return "", false
		}
		parts := make([]string, 0, len(response.Output))
		for _, item := range response.Output {
			if strings.EqualFold(item.Type, "message") {
				if text, ok := joinConversationTextBlocks(item.Content); ok {
					parts = append(parts, text)
				}
			}
		}
		text := strings.TrimSpace(strings.Join(parts, "\n"))
		return text, text != ""
	case ConversationArchiveProtocolGemini:
		var response struct {
			Candidates []struct {
				Content struct {
					Parts []struct {
						Text    string `json:"text"`
						Thought bool   `json:"thought"`
					} `json:"parts"`
				} `json:"content"`
			} `json:"candidates"`
		}
		if json.Unmarshal(body, &response) != nil || len(response.Candidates) == 0 {
			return "", false
		}
		parts := make([]string, 0, len(response.Candidates[0].Content.Parts))
		for _, part := range response.Candidates[0].Content.Parts {
			if part.Text != "" && !part.Thought {
				parts = append(parts, part.Text)
			}
		}
		text := strings.Join(parts, "")
		return text, text != ""
	default:
		return "", false
	}
}

func joinConversationTextBlocks(blocks []conversationContentBlock) (string, bool) {
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if conversationTextBlockType(block.Type) && block.Text != "" {
			parts = append(parts, block.Text)
		}
	}
	text := strings.Join(parts, "\n")
	return text, text != ""
}

func extractConversationSSE(protocol string, body []byte) (string, bool) {
	var builder strings.Builder
	var fallback strings.Builder
	found := false
	for _, rawLine := range bytes.Split(body, []byte("\n")) {
		line := strings.TrimSpace(string(rawLine))
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}
		text, ok := extractConversationStreamEvent(protocol, []byte(payload))
		if !ok {
			continue
		}
		if text != "" {
			builder.WriteString(text)
			found = true
		}
		if protocol == ConversationArchiveProtocolOpenAIResponses && fallback.Len() == 0 {
			if completed, ok := extractResponsesCompletedText([]byte(payload)); ok {
				fallback.WriteString(completed)
			}
		}
	}
	if builder.Len() > 0 {
		return builder.String(), true
	}
	if fallback.Len() > 0 {
		return fallback.String(), true
	}
	return "", found
}

func extractConversationStreamEvent(protocol string, payload []byte) (string, bool) {
	switch protocol {
	case ConversationArchiveProtocolAnthropicMessages:
		var event struct {
			Type  string `json:"type"`
			Delta struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"delta"`
			ContentBlock struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content_block"`
		}
		if json.Unmarshal(payload, &event) != nil {
			return "", false
		}
		if event.Type == "content_block_delta" && event.Delta.Type == "text_delta" {
			return event.Delta.Text, true
		}
		if event.Type == "content_block_start" && event.ContentBlock.Type == "text" {
			return event.ContentBlock.Text, true
		}
		return "", true
	case ConversationArchiveProtocolOpenAIChat:
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content json.RawMessage `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if json.Unmarshal(payload, &chunk) != nil || len(chunk.Choices) == 0 {
			return "", false
		}
		return conversationContentText(chunk.Choices[0].Delta.Content), true
	case ConversationArchiveProtocolOpenAIResponses:
		var event struct {
			Type  string `json:"type"`
			Delta string `json:"delta"`
		}
		if json.Unmarshal(payload, &event) != nil {
			return "", false
		}
		if event.Type == "response.output_text.delta" {
			return event.Delta, true
		}
		return "", true
	case ConversationArchiveProtocolGemini:
		var chunk struct {
			Candidates []struct {
				Content struct {
					Parts []struct {
						Text    string `json:"text"`
						Thought bool   `json:"thought"`
					} `json:"parts"`
				} `json:"content"`
			} `json:"candidates"`
		}
		if json.Unmarshal(payload, &chunk) != nil || len(chunk.Candidates) == 0 {
			return "", false
		}
		parts := make([]string, 0, len(chunk.Candidates[0].Content.Parts))
		for _, part := range chunk.Candidates[0].Content.Parts {
			if part.Thought {
				continue
			}
			parts = append(parts, part.Text)
		}
		return strings.Join(parts, ""), true
	default:
		return "", false
	}
}

func extractResponsesCompletedText(payload []byte) (string, bool) {
	var event struct {
		Type     string `json:"type"`
		Response struct {
			Output []struct {
				Type    string                     `json:"type"`
				Content []conversationContentBlock `json:"content"`
			} `json:"output"`
		} `json:"response"`
	}
	if json.Unmarshal(payload, &event) != nil || event.Type != "response.completed" {
		return "", false
	}
	parts := make([]string, 0, len(event.Response.Output))
	for _, item := range event.Response.Output {
		if text, ok := joinConversationTextBlocks(item.Content); ok {
			parts = append(parts, text)
		}
	}
	text := strings.Join(parts, "\n")
	return text, text != ""
}
