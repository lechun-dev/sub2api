package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestRepairOpenAIResponsesInputToolPairing(t *testing.T) {
	t.Run("orphan outputs become user messages and preserve content", func(t *testing.T) {
		reqBody := map[string]any{"input": []any{
			map[string]any{"type": "function_call_output", "call_id": "missing_string", "output": "string result"},
			map[string]any{"type": "tool_search_output", "call_id": "missing_parts", "output": []any{
				map[string]any{"type": "output_text", "text": "first part"},
				map[string]any{"type": "output_text", "text": "second part"},
			}},
			map[string]any{"type": "custom_tool_call_output", "call_id": "missing_object", "output": map[string]any{"ok": true}},
		}}

		require.True(t, repairOpenAIResponsesInputToolPairing(reqBody))
		got := reqBody["input"].([]any)
		require.Equal(t, "message", got[0].(map[string]any)["type"])
		require.Equal(t, "user", got[0].(map[string]any)["role"])
		require.Equal(t, "[Tool output from an earlier turn, call_id missing_string]\nstring result", got[0].(map[string]any)["content"].([]any)[0].(map[string]any)["text"])
		require.Equal(t, "[Tool output from an earlier turn, call_id missing_parts]\nfirst part\nsecond part", got[1].(map[string]any)["content"].([]any)[0].(map[string]any)["text"])
		require.Equal(t, "[Tool output from an earlier turn, call_id missing_object]\n{\"ok\":true}", got[2].(map[string]any)["content"].([]any)[0].(map[string]any)["text"])
	})

	t.Run("named delegation output without call id is preserved", func(t *testing.T) {
		named := map[string]any{"type": "function_call_output", "name": "send_message_to_thread", "namespace": "codex_app", "output": "delegation"}
		input := []any{named}
		reqBody := map[string]any{"input": input}

		require.False(t, repairOpenAIResponsesInputToolPairing(reqBody))
		require.Equal(t, input, reqBody["input"])
	})

	t.Run("missing supported call outputs get placeholders", func(t *testing.T) {
		input := []any{
			map[string]any{"type": "function_call", "call_id": "function_1", "name": "lookup", "arguments": "{}"},
			map[string]any{"type": "custom_tool_call", "call_id": "custom_1", "name": "apply_patch"},
			map[string]any{"type": "tool_search_call", "call_id": "search_1", "query": "docs"},
			map[string]any{"type": "mcp_tool_call", "call_id": "mcp_1", "name": "read"},
		}
		reqBody := map[string]any{"input": input}

		require.True(t, repairOpenAIResponsesInputToolPairing(reqBody))
		got := reqBody["input"].([]any)
		require.Len(t, got, 6)
		require.Equal(t, input, got[:len(input)], "existing items keep their positions")
		require.Equal(t, map[string]any{"type": "function_call_output", "call_id": "function_1", "output": openAIResponsesMissingToolOutputPlaceholder}, got[4])
		require.Equal(t, map[string]any{"type": "custom_tool_call_output", "call_id": "custom_1", "output": openAIResponsesMissingToolOutputPlaceholder}, got[5])

		first := reqBody["input"]
		require.False(t, repairOpenAIResponsesInputToolPairing(reqBody))
		require.Equal(t, first, reqBody["input"])
	})

	t.Run("duplicate call ids get one placeholder output", func(t *testing.T) {
		input := []any{
			map[string]any{"type": "function_call", "call_id": "duplicate", "name": "first", "arguments": "{}"},
			map[string]any{"type": "function_call", "call_id": "duplicate", "name": "second", "arguments": "{}"},
		}
		reqBody := map[string]any{"input": input}

		require.True(t, repairOpenAIResponsesInputToolPairing(reqBody))
		got := reqBody["input"].([]any)
		require.Len(t, got, 3)
		require.Equal(t, input[0], got[0])
		require.Equal(t, input[1], got[1])
		require.Equal(t, map[string]any{"type": "function_call_output", "call_id": "duplicate", "output": openAIResponsesMissingToolOutputPlaceholder}, got[2])
	})

	t.Run("orphan output triggers placeholders for supported call types", func(t *testing.T) {
		reqBody := map[string]any{"input": []any{
			map[string]any{"type": "function_call", "call_id": "function_1", "name": "lookup", "arguments": "{}"},
			map[string]any{"type": "custom_tool_call", "call_id": "custom_1", "name": "apply_patch"},
			map[string]any{"type": "tool_search_call", "call_id": "search_1", "query": "docs"},
			map[string]any{"type": "mcp_tool_call", "call_id": "mcp_1", "name": "read"},
			map[string]any{"type": "function_call_output", "call_id": "orphan_1", "output": "keep this result"},
		}}

		require.True(t, repairOpenAIResponsesInputToolPairing(reqBody))
		got := reqBody["input"].([]any)
		require.Len(t, got, 7)
		require.Equal(t, "tool_search_call", got[2].(map[string]any)["type"])
		require.Equal(t, "mcp_tool_call", got[3].(map[string]any)["type"])
		require.Equal(t, "message", got[4].(map[string]any)["type"])
		require.Contains(t, got[4].(map[string]any)["content"].([]any)[0].(map[string]any)["text"], "keep this result")
		require.Equal(t, map[string]any{"type": "function_call_output", "call_id": "function_1", "output": openAIResponsesMissingToolOutputPlaceholder}, got[5])
		require.Equal(t, map[string]any{"type": "custom_tool_call_output", "call_id": "custom_1", "output": openAIResponsesMissingToolOutputPlaceholder}, got[6])
	})

	t.Run("paired calls and outputs are preserved regardless of order", func(t *testing.T) {
		input := []any{
			map[string]any{"type": "tool_search_output", "call_id": "search_1", "output": "first"},
			map[string]any{"type": "tool_search_call", "id": "search_1", "query": "docs"},
			map[string]any{"type": "function_call_output", "call_id": "function_1", "output": "second"},
			map[string]any{"type": "function_call", "call_id": "function_1", "name": "lookup", "arguments": "{}"},
			map[string]any{"type": "custom_tool_call_output", "call_id": "custom_1", "output": "third"},
			map[string]any{"type": "custom_tool_call", "call_id": "custom_1", "name": "apply_patch"},
			map[string]any{"type": "mcp_tool_call_output", "call_id": "mcp_1", "output": "fourth"},
			map[string]any{"type": "mcp_tool_call", "call_id": "mcp_1", "name": "read"},
		}
		reqBody := map[string]any{"input": input}

		require.False(t, repairOpenAIResponsesInputToolPairing(reqBody))
		require.Equal(t, input, reqBody["input"])
	})

	t.Run("item reference proves call context", func(t *testing.T) {
		input := []any{
			map[string]any{"type": "function_call_output", "call_id": "referenced", "output": "ok"},
			map[string]any{"type": "item_reference", "id": "referenced"},
		}
		reqBody := map[string]any{"input": input}

		require.False(t, repairOpenAIResponsesInputToolPairing(reqBody))
		require.Equal(t, input, reqBody["input"])
	})

	t.Run("previous response proves call context", func(t *testing.T) {
		input := []any{map[string]any{"type": "function_call_output", "call_id": "remote", "output": "ok"}}
		reqBody := map[string]any{"input": input, "previous_response_id": "resp_1"}

		require.False(t, repairOpenAIResponsesInputToolPairing(reqBody))
		require.Equal(t, input, reqBody["input"])
	})

	t.Run("previous response still repairs missing call output", func(t *testing.T) {
		reqBody := map[string]any{
			"previous_response_id": "resp_1",
			"input": []any{
				map[string]any{"type": "function_call", "call_id": "call_1", "name": "lookup", "arguments": "{}"},
			},
		}

		require.True(t, repairOpenAIResponsesInputToolPairing(reqBody))
		got := reqBody["input"].([]any)
		require.Len(t, got, 2)
		require.Equal(t, map[string]any{"type": "function_call_output", "call_id": "call_1", "output": openAIResponsesMissingToolOutputPlaceholder}, got[1])
	})

	t.Run("repair is idempotent", func(t *testing.T) {
		reqBody := map[string]any{"input": []any{
			map[string]any{"type": "function_call", "call_id": "call_1", "name": "lookup", "arguments": "{}"},
			map[string]any{"type": "function_call_output", "call_id": "orphan", "output": "keep"},
		}}

		require.True(t, repairOpenAIResponsesInputToolPairing(reqBody))
		first := reqBody["input"]
		require.False(t, repairOpenAIResponsesInputToolPairing(reqBody))
		require.Equal(t, first, reqBody["input"])
	})

	t.Run("bytes facade leaves invalid and unchanged bodies untouched", func(t *testing.T) {
		invalid := []byte(`{"input":`)
		got, changed := RepairOpenAIResponsesInputToolPairingBytes(invalid)
		require.False(t, changed)
		require.Equal(t, invalid, got)

		unchanged := []byte(`{"input":[{"type":"function_call_output","name":"send_message_to_thread","output":"delegation"}]}`)
		got, changed = RepairOpenAIResponsesInputToolPairingBytes(unchanged)
		require.False(t, changed)
		require.Equal(t, unchanged, got)
	})

	t.Run("bytes facade encodes repaired input", func(t *testing.T) {
		body := []byte(`{"input":[{"type":"function_call_output","call_id":"orphan","output":"keep this result"}]}`)
		got, changed := RepairOpenAIResponsesInputToolPairingBytes(body)
		require.True(t, changed)
		require.True(t, gjson.GetBytes(got, `input.0.content.0.text`).String() == "[Tool output from an earlier turn, call_id orphan]\nkeep this result")
		require.False(t, gjson.GetBytes(got, `input.0.call_id`).Exists())
	})

	t.Run("bytes facade encodes missing call output", func(t *testing.T) {
		body := []byte(`{"input":[{"type":"function_call","call_id":"call_1","name":"lookup","arguments":"{}"}]}`)
		got, changed := RepairOpenAIResponsesInputToolPairingBytes(body)
		require.True(t, changed)
		require.Equal(t, "function_call_output", gjson.GetBytes(got, `input.1.type`).String())
		require.Equal(t, "call_1", gjson.GetBytes(got, `input.1.call_id`).String())
		require.Equal(t, openAIResponsesMissingToolOutputPlaceholder, gjson.GetBytes(got, `input.1.output`).String())
	})
}

func TestWebSocketCompatibilityPreservesNamedDelegation(t *testing.T) {
	for _, toolName := range []string{"create_thread", "send_message_to_thread"} {
		for _, withHistory := range []bool{false, true} {
			for _, previousResponseID := range []string{"", "resp_previous"} {
				name := fmt.Sprintf("%s/history=%t/previous=%t", toolName, withHistory, previousResponseID != "")
				t.Run(name, func(t *testing.T) {
					input := []any{}
					if withHistory {
						input = append(input,
							map[string]any{"type": "function_call", "call_id": "fc_history", "name": "lookup", "arguments": "{}"},
							map[string]any{"type": "function_call_output", "call_id": "fc_history", "output": "historical result"},
						)
					}
					input = append(input,
						map[string]any{"type": "message", "role": "user", "content": []any{map[string]any{"type": "input_text", "text": "Updated environment context."}}},
						map[string]any{"type": "function_call_output", "name": toolName, "namespace": "codex_app", "output": "<codex_delegation>\n  <source_thread_id>source-task</source_thread_id>\n  <input>Reply with DELEGATION_OK.</input>\n</codex_delegation>"},
					)
					reqBody := map[string]any{"type": "response.create", "model": "gpt-5.5", "input": input}
					if previousResponseID != "" {
						reqBody["previous_response_id"] = previousResponseID
					}
					body, err := json.Marshal(reqBody)
					require.NoError(t, err)
					account := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth}

					normalized, _, err := normalizeOpenAIResponsesWebSocketCompatibilityBody(body, account, false)
					require.NoError(t, err)
					var got map[string]any
					require.NoError(t, json.Unmarshal(normalized, &got))
					require.Equal(t, input, got["input"], "preserve the delegation envelope, native type and history in order")
					require.Equal(t, reqBody["previous_response_id"], got["previous_response_id"])

					again, changed, err := normalizeOpenAIResponsesWebSocketCompatibilityBody(normalized, account, false)
					require.NoError(t, err)
					require.False(t, changed, "normalization must be idempotent")
					require.JSONEq(t, string(normalized), string(again))
				})
			}
		}
	}
}

func TestOpenAIResponsesInputTextIsNeverSilentlyTruncated(t *testing.T) {
	atLimit := strings.Repeat("z", openAIResponsesInputTextMaxChars)
	oversized := strings.Repeat("a", openAIResponsesInputTextMaxChars) + "中"
	input := []any{
		map[string]any{"type": "function_call_output", "call_id": "limit", "output": atLimit},
		map[string]any{"type": "function_call_output", "call_id": "a", "output": oversized},
		map[string]any{"type": "tool_search_output", "call_id": "b", "output": oversized},
		map[string]any{"type": "custom_tool_call_output", "call_id": "c", "output": oversized},
		map[string]any{"type": "mcp_tool_call_output", "call_id": "d", "output": oversized},
		map[string]any{
			"role": "user",
			"content": []any{
				map[string]any{"type": "input_text", "text": "short"},
				map[string]any{"type": "input_text", "text": oversized},
			},
		},
	}
	reqBody := map[string]any{"input": input}

	require.False(t, truncateOpenAIResponsesInputText(reqBody))
	first, ok := input[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, atLimit, first["output"])
	for _, rawItem := range input[1:5] {
		item, ok := rawItem.(map[string]any)
		require.True(t, ok)
		require.Equal(t, oversized, item["output"])
	}
	last, ok := input[5].(map[string]any)
	require.True(t, ok)
	content, ok := last["content"].([]any)
	require.True(t, ok)
	require.Len(t, content, 2)
	shortPart, ok := content[0].(map[string]any)
	require.True(t, ok)
	oversizedPart, ok := content[1].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "short", shortPart["text"])
	require.Equal(t, oversized, oversizedPart["text"])
}

func TestOpenAIResponsesInputNeverRequestsPreemptiveTruncation(t *testing.T) {
	short := []byte(`{"input":[{"type":"function_call_output","output":"ok"}]}`)
	largeUnrelated := []byte(`{"input":"` + strings.Repeat("x", openAIResponsesInputTextMaxChars+1) + `"}`)
	largeOutput := []byte(`{"input":[{"type":"function_call_output","output":"` + strings.Repeat("x", openAIResponsesInputTextMaxChars+1) + `"}]}`)

	require.False(t, openAIResponsesInputMayNeedTruncation(short))
	require.False(t, openAIResponsesInputMayNeedTruncation(largeUnrelated))
	require.False(t, openAIResponsesInputMayNeedTruncation(largeOutput))
}

func TestOpenAIGatewayService_OAuthDropsOrphanAfterDroppingPreviousResponse(t *testing.T) {
	body := []byte(`{"model":"gpt-5.5","stream":false,"previous_response_id":"resp_missing","input":[{"type":"function_call_output","call_id":"call_missing","output":"keep this result"}]}`)
	upstream := &httpUpstreamRecorder{responses: []*http.Response{
		newOpenAIRejectedFieldTestResponse(http.StatusOK, `{"id":"resp_ok","output":[],"usage":{"input_tokens":1,"output_tokens":1,"input_tokens_details":{"cached_tokens":0}}}`),
	}}

	result, err := newOpenAIRejectedFieldTestService(upstream).Forward(
		context.Background(),
		newOpenAIRejectedFieldTestContext(body),
		newOpenAIOAuthNamespaceTestAccount(),
		body,
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, upstream.bodies, 1)
	require.False(t, gjson.GetBytes(upstream.bodies[0], "previous_response_id").Exists())
	require.Equal(t, "user", gjson.GetBytes(upstream.bodies[0], "input.0.role").String())
	require.Contains(t, gjson.GetBytes(upstream.bodies[0], "input.0.content.0.text").String(), "keep this result")
}

func TestOpenAIGatewayService_PreservesOversizedToolOutputForUpstream(t *testing.T) {
	oversized := strings.Repeat("x", openAIResponsesInputTextMaxChars) + "中"
	body := []byte(`{"model":"gpt-5.5","stream":false,"input":[{"type":"function_call","call_id":"call_1","name":"lookup","arguments":"{}"},{"type":"function_call_output","call_id":"call_1","output":"` + oversized + `"}]}`)
	upstream := &httpUpstreamRecorder{responses: []*http.Response{
		newOpenAIRejectedFieldTestResponse(http.StatusOK, `{"id":"resp_ok","output":[],"usage":{"input_tokens":1,"output_tokens":1,"input_tokens_details":{"cached_tokens":0}}}`),
	}}

	result, err := newOpenAIRejectedFieldTestService(upstream).Forward(
		context.Background(),
		newOpenAIRejectedFieldTestContext(body),
		newOpenAIRejectedFieldTestAccount(),
		body,
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, upstream.bodies, 1)
	require.Equal(t, oversized, gjson.GetBytes(upstream.bodies[0], "input.1.output").String())
}
