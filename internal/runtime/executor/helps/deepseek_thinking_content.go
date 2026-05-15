package helps

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

var emptyDeepSeekThinkingBlock = []byte(`{"type":"thinking","thinking":""}`)

// EnsureDeepSeekClaudeThinkingContent fills missing Claude-style thinking
// content required by DeepSeek's Anthropic-compatible API during tool use.
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

		hasToolUse := false
		hasThinkingBlock := false
		for blockIdx, block := range content.Array() {
			switch strings.TrimSpace(block.Get("type").String()) {
			case "thinking":
				hasThinkingBlock = true
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
			case "tool_use":
				hasToolUse = true
			}
		}

		if !hasToolUse || hasThinkingBlock {
			continue
		}

		contentWithThinking, err := prependDeepSeekThinkingBlock(content)
		if err != nil {
			return body, patched, err
		}
		path := fmt.Sprintf("messages.%d.content", msgIdx)
		next, err := sjson.SetRawBytes(out, path, contentWithThinking)
		if err != nil {
			return body, patched, fmt.Errorf("deepseek thinking content block injection failed: %w", err)
		}
		out = next
		patched++
	}

	if patched == 0 {
		return body, 0, nil
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

func prependDeepSeekThinkingBlock(content gjson.Result) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('[')
	buf.Write(emptyDeepSeekThinkingBlock)
	for _, block := range content.Array() {
		buf.WriteByte(',')
		buf.WriteString(block.Raw)
	}
	buf.WriteByte(']')
	out := buf.Bytes()
	if !gjson.ValidBytes(out) {
		return nil, fmt.Errorf("deepseek thinking content block injection produced invalid JSON")
	}
	return out, nil
}
