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

func TestEnsureDeepSeekClaudeThinkingContent_DoesNotInsertThinkingBlockForToolUse(t *testing.T) {
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
	if patched != 0 {
		t.Fatalf("patched = %d, want %d", patched, 0)
	}
	if got := gjson.GetBytes(out, "messages.0.content.0.type").String(); got != "text" {
		t.Fatalf("messages.0.content.0.type = %q, want %q", got, "text")
	}
	if got := gjson.GetBytes(out, "messages.0.content.1.type").String(); got != "tool_use" {
		t.Fatalf("messages.0.content.1.type = %q, want %q", got, "tool_use")
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
				{"type":"thinking"},
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
	if !gjson.GetBytes(out, "messages.0.content.0.thinking").Exists() {
		t.Fatalf("messages.0.content.0.thinking should exist")
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

func TestEnsureDeepSeekClaudeToolResults_AddsMissingResultsToNextUserMessage(t *testing.T) {
	body := []byte(`{
		"model":"deepseek-chat",
		"messages":[
			{"role":"assistant","content":[
				{"type":"tool_use","id":"call_1","name":"Read","input":{"file":"a.go"}},
				{"type":"tool_use","id":"call_2","name":"Read","input":{"file":"b.go"}}
			]},
			{"role":"user","content":[
				{"type":"tool_result","tool_use_id":"call_1","content":"ok"}
			]}
		]
	}`)

	out, patched, err := EnsureDeepSeekClaudeToolResults("deepseek-chat", "", body)
	if err != nil {
		t.Fatalf("EnsureDeepSeekClaudeToolResults() error = %v", err)
	}
	if patched != 1 {
		t.Fatalf("patched = %d, want %d", patched, 1)
	}
	if got := gjson.GetBytes(out, "messages.1.content.0.tool_use_id").String(); got != "call_1" {
		t.Fatalf("messages.1.content.0.tool_use_id = %q, want %q", got, "call_1")
	}
	if got := gjson.GetBytes(out, "messages.1.content.1.tool_use_id").String(); got != "call_2" {
		t.Fatalf("messages.1.content.1.tool_use_id = %q, want %q", got, "call_2")
	}
	if got := gjson.GetBytes(out, "messages.1.content.1.content").String(); got != "" {
		t.Fatalf("messages.1.content.1.content = %q, want empty string", got)
	}
}

func TestEnsureDeepSeekClaudeToolResults_PrependsMissingResultBeforeUserText(t *testing.T) {
	body := []byte(`{
		"model":"deepseek-chat",
		"messages":[
			{"role":"assistant","content":[
				{"type":"tool_use","id":"call_1","name":"Read","input":{"file":"a.go"}}
			]},
			{"role":"user","content":"continue"}
		]
	}`)

	out, patched, err := EnsureDeepSeekClaudeToolResults("deepseek-chat", "", body)
	if err != nil {
		t.Fatalf("EnsureDeepSeekClaudeToolResults() error = %v", err)
	}
	if patched != 1 {
		t.Fatalf("patched = %d, want %d", patched, 1)
	}
	if got := gjson.GetBytes(out, "messages.1.content.0.type").String(); got != "tool_result" {
		t.Fatalf("messages.1.content.0.type = %q, want %q", got, "tool_result")
	}
	if got := gjson.GetBytes(out, "messages.1.content.1.text").String(); got != "continue" {
		t.Fatalf("messages.1.content.1.text = %q, want %q", got, "continue")
	}
}

func TestEnsureDeepSeekClaudeToolResults_InsertsSyntheticUserMessage(t *testing.T) {
	body := []byte(`{
		"model":"deepseek-chat",
		"messages":[
			{"role":"assistant","content":[
				{"type":"tool_use","id":"call_1","name":"Read","input":{"file":"a.go"}}
			]},
			{"role":"assistant","content":[{"type":"text","text":"next"}]}
		]
	}`)

	out, patched, err := EnsureDeepSeekClaudeToolResults("deepseek-chat", "", body)
	if err != nil {
		t.Fatalf("EnsureDeepSeekClaudeToolResults() error = %v", err)
	}
	if patched != 1 {
		t.Fatalf("patched = %d, want %d", patched, 1)
	}
	if got := gjson.GetBytes(out, "messages.1.role").String(); got != "user" {
		t.Fatalf("messages.1.role = %q, want %q", got, "user")
	}
	if got := gjson.GetBytes(out, "messages.1.content.0.tool_use_id").String(); got != "call_1" {
		t.Fatalf("messages.1.content.0.tool_use_id = %q, want %q", got, "call_1")
	}
	if got := gjson.GetBytes(out, "messages.2.role").String(); got != "assistant" {
		t.Fatalf("messages.2.role = %q, want %q", got, "assistant")
	}
}

func TestEnsureDeepSeekClaudeToolResults_SkipsNonDeepSeekTarget(t *testing.T) {
	body := []byte(`{
		"model":"claude-sonnet-4-5-20250929",
		"messages":[
			{"role":"assistant","content":[
				{"type":"tool_use","id":"call_1","name":"Read","input":{"file":"a.go"}}
			]}
		]
	}`)

	out, patched, err := EnsureDeepSeekClaudeToolResults("claude-sonnet-4-5-20250929", "https://api.anthropic.com", body)
	if err != nil {
		t.Fatalf("EnsureDeepSeekClaudeToolResults() error = %v", err)
	}
	if patched != 0 {
		t.Fatalf("patched = %d, want %d", patched, 0)
	}
	if got := len(gjson.GetBytes(out, "messages").Array()); got != 1 {
		t.Fatalf("messages length = %d, want %d", got, 1)
	}
}
