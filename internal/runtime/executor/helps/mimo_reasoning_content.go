package helps

import (
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// EnsureMiMoReasoningContent fills missing assistant reasoning_content fields
// on MiMo chat-completions requests that contain tool calls.
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
		toolCalls := msg.Get("tool_calls")
		if !toolCalls.Exists() || !toolCalls.IsArray() || len(toolCalls.Array()) == 0 {
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

func isMiMoModel(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	return strings.HasPrefix(model, "mimo-")
}
