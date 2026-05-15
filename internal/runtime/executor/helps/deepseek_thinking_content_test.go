package helps

import (
	"testing"

	"github.com/tidwall/gjson"
)

func TestEnsureDeepSeekClaudeThinkingContent_FillsMissingThinkingField(t *testing.T) {
	body := []byte(`{
		"model":"deepseek-chat",
		"thinking":{"type":"enabled","budget_tokens":2048},
		"messages":[
			{"role":"assistant","content":[
				{"type":"thinking"},
				{"type":"tool_use","id":"toolu_1","name":"Bash","input":{"command":"pwd"}}
			]}
		]
	}`)

	out, patched, err := EnsureDeepSeekClaudeThinkingContent("deepseek-chat", "", body)
	if err != nil {
		t.Fatalf("EnsureDeepSeekClaudeThinkingContent() error = %v", err)
	}
	if patched != 1 {
		t.Fatalf("patched = %d, want %d", patched, 1)
	}
	got := gjson.GetBytes(out, "messages.0.content.0.thinking")
	if !got.Exists() || got.String() != "" {
		t.Fatalf("messages.0.content.0.thinking = %q, exists=%v; want empty string", got.String(), got.Exists())
	}
}

func TestEnsureDeepSeekClaudeThinkingContent_InsertsThinkingBlockForToolUse(t *testing.T) {
	body := []byte(`{
		"model":"deepseek-chat",
		"thinking":{"type":"enabled","budget_tokens":2048},
		"messages":[
			{"role":"assistant","content":[
				{"type":"text","text":"I'll check."},
				{"type":"tool_use","id":"toolu_1","name":"Bash","input":{"command":"pwd"}}
			]}
		]
	}`)

	out, patched, err := EnsureDeepSeekClaudeThinkingContent("deepseek-chat", "", body)
	if err != nil {
		t.Fatalf("EnsureDeepSeekClaudeThinkingContent() error = %v", err)
	}
	if patched != 1 {
		t.Fatalf("patched = %d, want %d", patched, 1)
	}
	if got := gjson.GetBytes(out, "messages.0.content.0.type").String(); got != "thinking" {
		t.Fatalf("messages.0.content.0.type = %q, want %q", got, "thinking")
	}
	gotThinking := gjson.GetBytes(out, "messages.0.content.0.thinking")
	if !gotThinking.Exists() || gotThinking.String() != "" {
		t.Fatalf("messages.0.content.0.thinking = %q, exists=%v; want empty string", gotThinking.String(), gotThinking.Exists())
	}
	if got := gjson.GetBytes(out, "messages.0.content.1.type").String(); got != "text" {
		t.Fatalf("messages.0.content.1.type = %q, want %q", got, "text")
	}
}

func TestEnsureDeepSeekClaudeThinkingContent_PreservesExistingThinking(t *testing.T) {
	body := []byte(`{
		"model":"deepseek-chat",
		"thinking":{"type":"enabled","budget_tokens":2048},
		"messages":[
			{"role":"assistant","content":[
				{"type":"thinking","thinking":"keep me"},
				{"type":"tool_use","id":"toolu_1","name":"Bash","input":{"command":"pwd"}}
			]}
		]
	}`)

	out, patched, err := EnsureDeepSeekClaudeThinkingContent("deepseek-chat", "", body)
	if err != nil {
		t.Fatalf("EnsureDeepSeekClaudeThinkingContent() error = %v", err)
	}
	if patched != 0 {
		t.Fatalf("patched = %d, want %d", patched, 0)
	}
	if got := gjson.GetBytes(out, "messages.0.content.0.thinking").String(); got != "keep me" {
		t.Fatalf("messages.0.content.0.thinking = %q, want %q", got, "keep me")
	}
}

func TestEnsureDeepSeekClaudeThinkingContent_SkipsNonDeepSeekTarget(t *testing.T) {
	body := []byte(`{
		"model":"claude-sonnet-4-5-20250929",
		"thinking":{"type":"enabled","budget_tokens":2048},
		"messages":[
			{"role":"assistant","content":[
				{"type":"tool_use","id":"toolu_1","name":"Bash","input":{"command":"pwd"}}
			]}
		]
	}`)

	out, patched, err := EnsureDeepSeekClaudeThinkingContent("claude-sonnet-4-5-20250929", "https://api.anthropic.com", body)
	if err != nil {
		t.Fatalf("EnsureDeepSeekClaudeThinkingContent() error = %v", err)
	}
	if patched != 0 {
		t.Fatalf("patched = %d, want %d", patched, 0)
	}
	if gjson.GetBytes(out, "messages.0.content.0.thinking").Exists() {
		t.Fatalf("thinking field should be absent for non-DeepSeek target")
	}
}

func TestEnsureDeepSeekClaudeThinkingContent_UsesBaseURLFallback(t *testing.T) {
	body := []byte(`{
		"model":"claude-opus-alias",
		"thinking":{"type":"enabled","budget_tokens":2048},
		"messages":[
			{"role":"assistant","content":[
				{"type":"tool_use","id":"toolu_1","name":"Bash","input":{"command":"pwd"}}
			]}
		]
	}`)

	out, patched, err := EnsureDeepSeekClaudeThinkingContent("claude-opus-alias", "https://api.deepseek.com/anthropic", body)
	if err != nil {
		t.Fatalf("EnsureDeepSeekClaudeThinkingContent() error = %v", err)
	}
	if patched != 1 {
		t.Fatalf("patched = %d, want %d", patched, 1)
	}
	if got := gjson.GetBytes(out, "messages.0.content.0.type").String(); got != "thinking" {
		t.Fatalf("messages.0.content.0.type = %q, want %q", got, "thinking")
	}
}

func TestEnsureDeepSeekClaudeThinkingContent_SkipsDisabledThinking(t *testing.T) {
	body := []byte(`{
		"model":"deepseek-chat",
		"thinking":{"type":"disabled"},
		"messages":[
			{"role":"assistant","content":[
				{"type":"tool_use","id":"toolu_1","name":"Bash","input":{"command":"pwd"}}
			]}
		]
	}`)

	out, patched, err := EnsureDeepSeekClaudeThinkingContent("deepseek-chat", "", body)
	if err != nil {
		t.Fatalf("EnsureDeepSeekClaudeThinkingContent() error = %v", err)
	}
	if patched != 0 {
		t.Fatalf("patched = %d, want %d", patched, 0)
	}
	if gjson.GetBytes(out, "messages.0.content.0.thinking").Exists() {
		t.Fatalf("thinking field should be absent when thinking is disabled")
	}
}
