package helps

import (
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// EnsureMiMoReasoningContent fills missing assistant reasoning_content fields
// on MiMo chat-completions requests.
// If the client already provided reasoning_content, it is preserved as-is.
func EnsureMiMoReasoningContent(model string, body []byte) ([]byte, int, error) {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return body, 0, nil
	}
	if !isMiMoModel(model) && !isMiMoModel(gjson.GetBytes(body, "model").String()) {
		return body, 0, nil
	}

	messages := gjson.GetBytes(body, "messages")
	if !messages.Exists() || !messages.IsArray() {
		return body, 0, nil
	}

	out := append([]byte(nil), body...)
	patched := 0
	for idx, msg := range messages.Array() {
		if strings.TrimSpace(msg.Get("role").String()) != "assistant" {
			continue
		}
		reasoning := msg.Get("reasoning_content")
		if reasoning.Exists() {
			continue
		}

		path := fmt.Sprintf("messages.%d.reasoning_content", idx)
		next, err := sjson.SetBytes(out, path, "")
		if err != nil {
			return body, patched, fmt.Errorf("mimo reasoning_content patch failed: %w", err)
		}
		out = next
		patched++
	}

	if patched == 0 {
		return body, 0, nil
	}
	return out, patched, nil
}

// EnsureMiMoClaudeToolResults repairs tool_use/tool_result adjacency on
// MiMo Anthropic-compatible requests translated from Responses history.
func EnsureMiMoClaudeToolResults(model string, baseURL string, body []byte) ([]byte, int, error) {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return body, 0, nil
	}
	if !isMiMoTarget(model, gjson.GetBytes(body, "model").String(), baseURL) {
		return body, 0, nil
	}
	return ensureClaudeToolResults(body, "mimo")
}

// EnsureMiMoClaudeThinkingContent fills missing assistant thinking blocks on
// MiMo Anthropic-compatible requests when thinking mode is enabled.
func EnsureMiMoClaudeThinkingContent(model string, baseURL string, body []byte) ([]byte, int, error) {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return body, 0, nil
	}
	if !isMiMoTarget(model, gjson.GetBytes(body, "model").String(), baseURL) {
		return body, 0, nil
	}
	if !isThinkingModeEnabled(body) {
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
		if !content.Exists() {
			continue
		}
		if content.IsArray() {
			hasThinking := false
			for blockIdx, block := range content.Array() {
				if strings.TrimSpace(block.Get("type").String()) != "thinking" {
					continue
				}
				hasThinking = true
				if block.Get("thinking").Exists() {
					continue
				}

				path := fmt.Sprintf("messages.%d.content.%d.thinking", msgIdx, blockIdx)
				next, err := sjson.SetBytes(out, path, "")
				if err != nil {
					return body, patched, fmt.Errorf("mimo claude thinking content patch failed: %w", err)
				}
				out = next
				patched++
			}
			if hasThinking {
				continue
			}
			nextContent, err := prependMiMoThinkingBlock(content.Raw)
			if err != nil {
				return body, patched, err
			}
			path := fmt.Sprintf("messages.%d.content", msgIdx)
			next, err := sjson.SetRawBytes(out, path, nextContent)
			if err != nil {
				return body, patched, fmt.Errorf("mimo claude thinking block injection failed: %w", err)
			}
			out = next
			patched++
			continue
		}
		if content.Type == gjson.String {
			contentValue := content.String()
			nextContent := []byte(`[{"type":"thinking","thinking":""},{"type":"text","text":""}]`)
			nextContent, _ = sjson.SetBytes(nextContent, "1.text", contentValue)
			path := fmt.Sprintf("messages.%d.content", msgIdx)
			next, err := sjson.SetRawBytes(out, path, nextContent)
			if err != nil {
				return body, patched, fmt.Errorf("mimo claude string content conversion failed: %w", err)
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

func prependMiMoThinkingBlock(rawContent string) ([]byte, error) {
	wrapper := []byte(`{"content":[{"type":"thinking","thinking":""}]}`)
	items := gjson.Parse(rawContent)
	if !items.IsArray() {
		return nil, fmt.Errorf("mimo claude content is not an array")
	}
	for _, item := range items.Array() {
		var err error
		wrapper, err = sjson.SetRawBytes(wrapper, "content.-1", []byte(item.Raw))
		if err != nil {
			return nil, fmt.Errorf("mimo claude content prepend failed: %w", err)
		}
	}
	return []byte(gjson.GetBytes(wrapper, "content").Raw), nil
}

func isMiMoTarget(model string, payloadModel string, baseURL string) bool {
	return isMiMoModel(model) || isMiMoModel(payloadModel) || isMiMoEndpoint(baseURL)
}

func isMiMoModel(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	return strings.HasPrefix(model, "mimo-") || strings.Contains(model, "/mimo-")
}

func isMiMoEndpoint(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return strings.Contains(value, "xiaomimimo.com") || isMiMoModel(value)
}
