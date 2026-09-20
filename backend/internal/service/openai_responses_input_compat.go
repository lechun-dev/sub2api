package service

import (
	"strings"
)

const openAIResponsesInputTextMaxChars = 10000000

const openAIResponsesMissingToolOutputPlaceholder = "[tool output was not recorded]"

// sanitizeOpenAIResponsesOrphanToolOutputs removes tool-output items that have
// no matching call or item reference anywhere in the current input. Named
// function outputs without a call ID are standalone inputs, not orphan results.
func sanitizeOpenAIResponsesOrphanToolOutputs(reqBody map[string]any, input []any, hasPreviousResponseID bool) bool {
	if len(input) == 0 || hasPreviousResponseID {
		return false
	}

	toolCallIDs := make(map[string]struct{}, len(input))
	referenceIDs := make(map[string]struct{}, len(input))
	for _, rawItem := range input {
		item, ok := rawItem.(map[string]any)
		if !ok {
			continue
		}
		itemType := strings.TrimSpace(firstNonEmptyString(item["type"]))
		if itemType == "item_reference" {
			if id := strings.TrimSpace(firstNonEmptyString(item["id"])); id != "" {
				referenceIDs[id] = struct{}{}
			}
			continue
		}
		if !isCodexToolCallContextItemType(itemType) {
			continue
		}
		if id := strings.TrimSpace(firstNonEmptyString(item["call_id"], item["id"])); id != "" {
			toolCallIDs[id] = struct{}{}
		}
	}

	modified := false
	normalized := make([]any, 0, len(input))
	for _, rawItem := range input {
		item, ok := rawItem.(map[string]any)
		if !ok || !isCodexToolCallOutputItemType(strings.TrimSpace(firstNonEmptyString(item["type"]))) {
			normalized = append(normalized, rawItem)
			continue
		}

		callID := strings.TrimSpace(firstNonEmptyString(item["call_id"]))
		// Codex sends externally supplied inputs (such as task delegation) as
		// named function outputs without a preceding function call. Preserve
		// their native type, namespace and output instead of silently dropping
		// the request that started the turn.
		if callID == "" && strings.TrimSpace(firstNonEmptyString(item["type"])) == "function_call_output" &&
			strings.TrimSpace(firstNonEmptyString(item["name"])) != "" {
			normalized = append(normalized, rawItem)
			continue
		}
		_, hasToolCall := toolCallIDs[callID]
		_, hasReference := referenceIDs[callID]
		if callID != "" && (hasToolCall || hasReference) {
			normalized = append(normalized, rawItem)
			continue
		}

		modified = true
	}
	if !modified {
		return false
	}
	reqBody["input"] = normalized
	return true
}

type openAIResponsesToolPairingScan struct {
	toolCallIDs          map[string]struct{}
	referenceIDs         map[string]struct{}
	outputIDs            map[string]struct{}
	hasOrphanOutput      bool
	hasMissingCallOutput bool
}

// repairOpenAIResponsesInputToolPairing repairs Responses input[] items whose
// call/output pairing was lost by client-side context trimming or summarization.
// Unmatched outputs become user messages so their text is preserved. Calls that
// are still present are given a placeholder output only for types whose output
// envelope is unambiguous.
func repairOpenAIResponsesInputToolPairing(reqBody map[string]any) bool {
	if reqBody == nil {
		return false
	}
	input, ok := reqBody["input"].([]any)
	if !ok || len(input) == 0 {
		return false
	}
	hasPreviousResponseID := strings.TrimSpace(firstNonEmptyString(reqBody["previous_response_id"])) != ""
	scan := scanOpenAIResponsesToolPairing(input, hasPreviousResponseID)
	if !scan.hasOrphanOutput && !scan.hasMissingCallOutput {
		return false
	}

	reqBody["input"] = rebuildOpenAIResponsesInputToolPairing(input, scan, hasPreviousResponseID)
	return true
}

func scanOpenAIResponsesToolPairing(input []any, hasPreviousResponseID bool) openAIResponsesToolPairingScan {
	scan := openAIResponsesToolPairingScan{
		toolCallIDs:  make(map[string]struct{}, len(input)),
		referenceIDs: make(map[string]struct{}, len(input)),
		outputIDs:    make(map[string]struct{}, len(input)),
	}
	for _, rawItem := range input {
		item, ok := rawItem.(map[string]any)
		if !ok {
			continue
		}
		itemType := strings.TrimSpace(firstNonEmptyString(item["type"]))
		if itemType == "item_reference" {
			if id := strings.TrimSpace(firstNonEmptyString(item["id"])); id != "" {
				scan.referenceIDs[id] = struct{}{}
			}
			continue
		}
		switch {
		case isCodexToolCallContextItemType(itemType):
			if id := strings.TrimSpace(firstNonEmptyString(item["call_id"], item["id"])); id != "" {
				scan.toolCallIDs[id] = struct{}{}
			}
		case isCodexToolCallOutputItemType(itemType):
			if id := strings.TrimSpace(firstNonEmptyString(item["call_id"])); id != "" {
				scan.outputIDs[id] = struct{}{}
			}
		}
	}

	for _, rawItem := range input {
		item, ok := rawItem.(map[string]any)
		if !ok {
			continue
		}
		if !scan.hasMissingCallOutput && openAIResponsesToolCallNeedsOutputPlaceholder(item, scan.outputIDs) {
			scan.hasMissingCallOutput = true
		}
		if !scan.hasOrphanOutput && openAIResponsesToolOutputNeedsOrphanRewrite(item, scan, hasPreviousResponseID) {
			scan.hasOrphanOutput = true
		}
		if scan.hasMissingCallOutput && scan.hasOrphanOutput {
			break
		}
	}
	return scan
}

func rebuildOpenAIResponsesInputToolPairing(input []any, scan openAIResponsesToolPairingScan, hasPreviousResponseID bool) []any {
	normalized := make([]any, 0, len(input)+2)
	missingOutputCallIDs := make(map[string]struct{})
	for _, rawItem := range input {
		item, ok := rawItem.(map[string]any)
		if !ok {
			normalized = append(normalized, rawItem)
			continue
		}

		itemType := strings.TrimSpace(firstNonEmptyString(item["type"]))
		callID := strings.TrimSpace(firstNonEmptyString(item["call_id"]))
		switch {
		case isCodexToolCallOutputItemType(itemType):
			if !openAIResponsesToolOutputNeedsOrphanRewrite(item, scan, hasPreviousResponseID) {
				normalized = append(normalized, rawItem)
				continue
			}
			normalized = append(normalized, orphanOpenAIResponsesToolOutputAsMessage(callID, item["output"]))

		case isCodexToolCallContextItemType(itemType):
			normalized = append(normalized, rawItem)
			if _, hasOutput := scan.outputIDs[callID]; hasOutput {
				continue
			}
			if _, queued := missingOutputCallIDs[callID]; queued {
				continue
			}
			placeholder, ok := newOpenAIResponsesMissingToolOutput(itemType, callID)
			if !ok {
				continue
			}
			// Responses-compatible upstreams validate each call as it is read.
			// Keep its placeholder output adjacent so a later call cannot make
			// the preceding call appear to have no output.
			normalized = append(normalized, placeholder)
			missingOutputCallIDs[callID] = struct{}{}

		default:
			normalized = append(normalized, rawItem)
		}
	}
	return normalized
}

func openAIResponsesToolOutputNeedsOrphanRewrite(item map[string]any, scan openAIResponsesToolPairingScan, hasPreviousResponseID bool) bool {
	itemType := strings.TrimSpace(firstNonEmptyString(item["type"]))
	if !isCodexToolCallOutputItemType(itemType) {
		return false
	}
	callID := strings.TrimSpace(firstNonEmptyString(item["call_id"]))
	if callID == "" {
		// Missing call IDs are invalid HTTP inputs unless they are standalone
		// named delegation outputs. Leave them for the handler validator so the
		// client receives the protocol-specific error.
		return false
	}
	return !hasPreviousResponseID && !hasOpenAIResponsesToolCallContext(scan.toolCallIDs, scan.referenceIDs, callID)
}

func openAIResponsesToolCallNeedsOutputPlaceholder(item map[string]any, outputIDs map[string]struct{}) bool {
	itemType := strings.TrimSpace(firstNonEmptyString(item["type"]))
	if openAIResponsesToolCallOutputTypeForCall(itemType) == "" {
		return false
	}
	callID := strings.TrimSpace(firstNonEmptyString(item["call_id"]))
	if callID == "" {
		return false
	}
	_, hasOutput := outputIDs[callID]
	return !hasOutput
}

func newOpenAIResponsesMissingToolOutput(callType, callID string) (map[string]any, bool) {
	if strings.TrimSpace(callID) == "" {
		return nil, false
	}
	outputType := openAIResponsesToolCallOutputTypeForCall(callType)
	if outputType == "" {
		return nil, false
	}
	return map[string]any{
		"type":    outputType,
		"call_id": callID,
		"output":  openAIResponsesMissingToolOutputPlaceholder,
	}, true
}

// RepairOpenAIResponsesInputToolPairingBytes applies the Responses tool-pairing
// compatibility repair without requiring callers to decode the request body.
func RepairOpenAIResponsesInputToolPairingBytes(body []byte) ([]byte, bool) {
	if len(body) == 0 {
		return body, false
	}
	var reqBody map[string]any
	if err := decodeOpenAIJSONUseNumber(body, &reqBody); err != nil {
		return body, false
	}
	if !repairOpenAIResponsesInputToolPairing(reqBody) {
		return body, false
	}
	repaired, err := marshalOpenAIUpstreamJSON(reqBody)
	if err != nil {
		return body, false
	}
	return repaired, true
}

func hasOpenAIResponsesToolCallContext(toolCallIDs, referenceIDs map[string]struct{}, callID string) bool {
	if _, ok := toolCallIDs[callID]; ok {
		return true
	}
	_, ok := referenceIDs[callID]
	return ok
}

func orphanOpenAIResponsesToolOutputAsMessage(callID string, output any) map[string]any {
	label := "[Tool output from an earlier turn]"
	if callID != "" {
		label = "[Tool output from an earlier turn, call_id " + callID + "]"
	}
	return map[string]any{
		"type": "message",
		"role": "user",
		"content": []any{
			map[string]any{
				"type": "input_text",
				"text": label + "\n" + flattenOpenAIResponsesToolOutput(output),
			},
		},
	}
}

func flattenOpenAIResponsesToolOutput(output any) string {
	switch value := output.(type) {
	case nil:
		return ""
	case string:
		return value
	case []any:
		textParts := make([]string, 0, len(value))
		for _, rawPart := range value {
			part, ok := rawPart.(map[string]any)
			if !ok {
				continue
			}
			text, ok := part["text"].(string)
			if !ok || text == "" {
				continue
			}
			textParts = append(textParts, text)
		}
		if len(textParts) > 0 {
			return strings.Join(textParts, "\n")
		}
	}
	encoded, err := marshalOpenAIUpstreamJSON(output)
	if err != nil {
		return ""
	}
	return string(encoded)
}

func openAIResponsesToolCallOutputTypeForCall(callType string) string {
	switch strings.TrimSpace(callType) {
	case "function_call":
		return "function_call_output"
	case "custom_tool_call":
		return "custom_tool_call_output"
	default:
		return ""
	}
}

func truncateOpenAIResponsesInputText(_ map[string]any) bool {
	// Do not silently rewrite client or tool output. If an upstream enforces a
	// text limit, forwarding the original value preserves its explicit error for
	// the client and the normal Ops error pipeline. This compatibility shim is
	// retained until the two callers can remove the old mutation hook together.
	return false
}

func openAIResponsesInputMayNeedTruncation(_ []byte) bool {
	// See truncateOpenAIResponsesInputText. Returning false also avoids decoding
	// very large bodies solely for a mutation that must not happen.
	return false
}
