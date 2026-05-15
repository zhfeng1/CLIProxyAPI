package helps

import (
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// NormalizeClaudeCustomTools converts OpenAI Responses custom tools into
// Claude-compatible custom tools before sending Anthropic-style requests.
func NormalizeClaudeCustomTools(body []byte) ([]byte, int, error) {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return body, 0, nil
	}

	tools := gjson.GetBytes(body, "tools")
	if !tools.Exists() || !tools.IsArray() {
		return body, 0, nil
	}

	patched := 0
	outTools := make([][]byte, 0, len(tools.Array()))
	for _, tool := range tools.Array() {
		if strings.TrimSpace(tool.Get("type").String()) != "custom" {
			outTools = append(outTools, []byte(tool.Raw))
			continue
		}

		converted, ok, err := normalizeClaudeCustomTool(tool)
		if err != nil {
			return body, patched, err
		}
		if !ok {
			outTools = append(outTools, []byte(tool.Raw))
			continue
		}
		outTools = append(outTools, converted)
		patched++
	}

	if patched == 0 {
		return body, 0, nil
	}
	out, err := sjson.SetRawBytes(body, "tools", joinJSONArray(outTools))
	if err != nil {
		return body, patched, fmt.Errorf("claude custom tool normalization failed: %w", err)
	}
	return out, patched, nil
}

func normalizeClaudeCustomTool(tool gjson.Result) ([]byte, bool, error) {
	name := strings.TrimSpace(tool.Get("name").String())
	if name == "" {
		return nil, false, nil
	}

	out := []byte(`{"name":"","description":"","input_schema":{"type":"object","properties":{"input":{"type":"string","description":"Raw input for the custom tool."}},"required":["input"]}}`)
	var err error
	out, err = sjson.SetBytes(out, "name", name)
	if err != nil {
		return nil, false, fmt.Errorf("claude custom tool name normalization failed: %w", err)
	}
	description := strings.TrimSpace(tool.Get("description").String())
	if description != "" {
		description += "\n\nProvide the raw custom tool input in the input field."
	} else {
		description = "Provide the raw custom tool input in the input field."
	}
	out, err = sjson.SetBytes(out, "description", description)
	if err != nil {
		return nil, false, fmt.Errorf("claude custom tool description normalization failed: %w", err)
	}
	return out, true, nil
}
