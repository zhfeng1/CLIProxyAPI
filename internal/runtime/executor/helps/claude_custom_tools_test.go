package helps

import (
	"testing"

	"github.com/tidwall/gjson"
)

func TestNormalizeClaudeCustomTools_ConvertsResponsesCustomTool(t *testing.T) {
	body := []byte(`{
		"tools":[
			{"type":"web_search_20250305","name":"web_search"},
			{"type":"custom","name":"apply_patch","description":"Apply a patch.","format":{"type":"grammar","syntax":"lark","definition":"start: /.+/"}}
		]
	}`)

	out, patched, err := NormalizeClaudeCustomTools(body)
	if err != nil {
		t.Fatalf("NormalizeClaudeCustomTools() error = %v", err)
	}
	if patched != 1 {
		t.Fatalf("patched = %d, want %d", patched, 1)
	}
	if got := gjson.GetBytes(out, "tools.0.type").String(); got != "web_search_20250305" {
		t.Fatalf("tools.0.type = %q, want web_search_20250305", got)
	}
	if gjson.GetBytes(out, "tools.1.type").Exists() {
		t.Fatalf("tools.1.type should be absent: %s", string(out))
	}
	if gjson.GetBytes(out, "tools.1.format").Exists() {
		t.Fatalf("tools.1.format should be absent: %s", string(out))
	}
	if got := gjson.GetBytes(out, "tools.1.name").String(); got != "apply_patch" {
		t.Fatalf("tools.1.name = %q, want apply_patch", got)
	}
	if got := gjson.GetBytes(out, "tools.1.input_schema.properties.input.type").String(); got != "string" {
		t.Fatalf("tools.1.input_schema.properties.input.type = %q, want string", got)
	}
}

func TestNormalizeClaudeCustomTools_SkipsFunctionTools(t *testing.T) {
	body := []byte(`{"tools":[{"name":"read","description":"Read","input_schema":{"type":"object","properties":{}}}]}`)

	out, patched, err := NormalizeClaudeCustomTools(body)
	if err != nil {
		t.Fatalf("NormalizeClaudeCustomTools() error = %v", err)
	}
	if patched != 0 {
		t.Fatalf("patched = %d, want %d", patched, 0)
	}
	if string(out) != string(body) {
		t.Fatalf("body changed: %s", string(out))
	}
}
