package helps

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// EnsureDeepSeekClaudeThinkingContent fills missing Claude-style thinking
// content required by DeepSeek's Anthropic-compatible API.
func EnsureDeepSeekClaudeThinkingContent(model string, baseURL string, body []byte) ([]byte, int, error) {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return body, 0, nil
	}
	if !isDeepSeekTarget(model, gjson.GetBytes(body, "model").String(), baseURL) {
		return body, 0, nil
	}
	if isThinkingDisabled(body) {
		return body, 0, nil
	}

	messages := gjson.GetBytes(body, "messages")
	if !messages.Exists() || !messages.IsArray() {
		return body, 0, nil
	}

	out := append([]byte(nil), body...)
	patched := 0
	for msgIdx, msg := range messages.Array() {
		if strings.TrimSpace(msg.Get("role").String()) != "assistant" {
			continue
		}

		content := msg.Get("content")
		if !content.Exists() || !content.IsArray() {
			continue
		}

		for blockIdx, block := range content.Array() {
			if strings.TrimSpace(block.Get("type").String()) != "thinking" {
				continue
			}
			if block.Get("thinking").Exists() {
				continue
			}

			path := fmt.Sprintf("messages.%d.content.%d.thinking", msgIdx, blockIdx)
			next, err := sjson.SetBytes(out, path, "")
			if err != nil {
				return body, patched, fmt.Errorf("deepseek thinking content patch failed: %w", err)
			}
			out = next
			patched++
		}
	}

	if patched == 0 {
		return body, 0, nil
	}
	return out, patched, nil
}

// EnsureDeepSeekClaudeToolResults adds empty tool_result blocks when a
// DeepSeek Anthropic-compatible request contains assistant tool_use blocks
// without matching results in the immediately following user message.
func EnsureDeepSeekClaudeToolResults(model string, baseURL string, body []byte) ([]byte, int, error) {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return body, 0, nil
	}
	if !isDeepSeekTarget(model, gjson.GetBytes(body, "model").String(), baseURL) {
		return body, 0, nil
	}

	messages := gjson.GetBytes(body, "messages")
	if !messages.Exists() || !messages.IsArray() {
		return body, 0, nil
	}

	messageItems := messages.Array()
	overrides := make(map[int][]byte)
	patched := 0
	outMessages := make([][]byte, 0, len(messageItems))
	for idx, msg := range messageItems {
		if override, ok := overrides[idx]; ok {
			outMessages = append(outMessages, override)
		} else {
			outMessages = append(outMessages, []byte(msg.Raw))
		}

		if strings.TrimSpace(msg.Get("role").String()) != "assistant" {
			continue
		}
		toolUseIDs := claudeToolUseIDs(msg)
		if len(toolUseIDs) == 0 {
			continue
		}

		nextIdx := idx + 1
		if nextIdx < len(messageItems) && strings.TrimSpace(messageItems[nextIdx].Get("role").String()) == "user" {
			nextRaw, added, err := ensureUserMessageToolResults(messageItems[nextIdx], toolUseIDs)
			if err != nil {
				return body, patched, err
			}
			if added > 0 {
				overrides[nextIdx] = nextRaw
				patched += added
			}
			continue
		}

		synthetic, err := buildSyntheticToolResultMessage(toolUseIDs)
		if err != nil {
			return body, patched, err
		}
		outMessages = append(outMessages, synthetic)
		patched += len(toolUseIDs)
	}

	if patched == 0 {
		return body, 0, nil
	}

	messagesRaw := joinJSONArray(outMessages)
	out, err := sjson.SetRawBytes(body, "messages", messagesRaw)
	if err != nil {
		return body, patched, fmt.Errorf("deepseek tool_result patch failed: %w", err)
	}
	return out, patched, nil
}

func isDeepSeekTarget(model string, payloadModel string, baseURL string) bool {
	return isDeepSeekName(model) || isDeepSeekName(payloadModel) || isDeepSeekName(baseURL)
}

func isDeepSeekName(value string) bool {
	return strings.Contains(strings.ToLower(strings.TrimSpace(value)), "deepseek")
}

func isThinkingDisabled(body []byte) bool {
	return strings.EqualFold(strings.TrimSpace(gjson.GetBytes(body, "thinking.type").String()), "disabled")
}

func claudeToolUseIDs(message gjson.Result) []string {
	content := message.Get("content")
	if !content.Exists() || !content.IsArray() {
		return nil
	}
	ids := make([]string, 0)
	content.ForEach(func(_, block gjson.Result) bool {
		if strings.TrimSpace(block.Get("type").String()) != "tool_use" {
			return true
		}
		id := strings.TrimSpace(block.Get("id").String())
		if id != "" {
			ids = append(ids, id)
		}
		return true
	})
	return ids
}

func ensureUserMessageToolResults(message gjson.Result, toolUseIDs []string) ([]byte, int, error) {
	required := make(map[string]struct{}, len(toolUseIDs))
	for _, id := range toolUseIDs {
		required[id] = struct{}{}
	}

	existingResults := make(map[string][][]byte, len(toolUseIDs))
	var extraResultBlocks [][]byte
	var otherBlocks [][]byte
	seen := make(map[string]struct{}, len(toolUseIDs))
	content := message.Get("content")
	switch {
	case content.Exists() && content.IsArray():
		content.ForEach(func(_, block gjson.Result) bool {
			if strings.TrimSpace(block.Get("type").String()) == "tool_result" {
				id := strings.TrimSpace(block.Get("tool_use_id").String())
				if _, ok := required[id]; ok {
					seen[id] = struct{}{}
					existingResults[id] = append(existingResults[id], []byte(block.Raw))
				} else {
					extraResultBlocks = append(extraResultBlocks, []byte(block.Raw))
				}
				return true
			}
			otherBlocks = append(otherBlocks, []byte(block.Raw))
			return true
		})
	case content.Exists() && content.Type == gjson.String:
		textBlock := []byte(`{"type":"text","text":""}`)
		textBlock, _ = sjson.SetBytes(textBlock, "text", content.String())
		otherBlocks = append(otherBlocks, textBlock)
	}

	missing := make([]string, 0)
	resultBlocks := make([][]byte, 0, len(toolUseIDs)+len(extraResultBlocks))
	for _, id := range toolUseIDs {
		if blocks := existingResults[id]; len(blocks) > 0 {
			resultBlocks = append(resultBlocks, blocks...)
			continue
		}
		missing = append(missing, id)
		block, err := buildSyntheticToolResultBlock(id)
		if err != nil {
			return nil, 0, err
		}
		resultBlocks = append(resultBlocks, block)
	}
	resultBlocks = append(resultBlocks, extraResultBlocks...)
	if len(missing) == 0 {
		return nil, 0, nil
	}

	blocks := make([][]byte, 0, len(resultBlocks)+len(otherBlocks))
	blocks = append(blocks, resultBlocks...)
	blocks = append(blocks, otherBlocks...)
	contentRaw := joinJSONArray(blocks)
	out, err := sjson.SetRawBytes([]byte(message.Raw), "content", contentRaw)
	if err != nil {
		return nil, 0, fmt.Errorf("deepseek tool_result content patch failed: %w", err)
	}
	return out, len(missing), nil
}

func buildSyntheticToolResultMessage(toolUseIDs []string) ([]byte, error) {
	blocks := make([][]byte, 0, len(toolUseIDs))
	for _, id := range toolUseIDs {
		block, err := buildSyntheticToolResultBlock(id)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, block)
	}
	message := []byte(`{"role":"user","content":[]}`)
	message, err := sjson.SetRawBytes(message, "content", joinJSONArray(blocks))
	if err != nil {
		return nil, fmt.Errorf("deepseek synthetic tool_result message failed: %w", err)
	}
	return message, nil
}

func buildSyntheticToolResultBlock(toolUseID string) ([]byte, error) {
	block := []byte(`{"type":"tool_result","tool_use_id":"","content":""}`)
	out, err := sjson.SetBytes(block, "tool_use_id", toolUseID)
	if err != nil {
		return nil, fmt.Errorf("deepseek synthetic tool_result block failed: %w", err)
	}
	return out, nil
}

func joinJSONArray(items [][]byte) []byte {
	var buf bytes.Buffer
	buf.WriteByte('[')
	for idx, item := range items {
		if idx > 0 {
			buf.WriteByte(',')
		}
		buf.Write(item)
	}
	buf.WriteByte(']')
	return buf.Bytes()
}
