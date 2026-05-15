package helps

import (
	"testing"

	"github.com/tidwall/gjson"
)

func TestEnsureMiMoReasoningContent_FillsMissingAssistantReasoningContent(t *testing.T) {
	body := []byte(`{
		"model":"mimo-v2.5-pro",
		"messages":[
			{"role":"assistant","tool_calls":[{"id":"call_1","type":"function","function":{"name":"list_directory","arguments":"{}"}}]},
			{"role":"user","content":"next"}
		]
	}`)

	out, patched, err := EnsureMiMoReasoningContent("mimo-v2.5-pro", body)
	if err != nil {
		t.Fatalf("EnsureMiMoReasoningContent() error = %v", err)
	}
	if patched != 1 {
		t.Fatalf("patched = %d, want %d", patched, 1)
	}
	got := gjson.GetBytes(out, "messages.0.reasoning_content").String()
	if got != "" {
		t.Fatalf("messages.0.reasoning_content = %q, want empty string", got)
	}
}

func TestEnsureMiMoReasoningContent_FillsTextAssistantReasoningContent(t *testing.T) {
	body := []byte(`{
		"model":"xiaomi/mimo-v2.5-pro",
		"messages":[
			{"role":"assistant","content":[{"type":"text","text":"previous answer"}]},
			{"role":"user","content":"next"}
		]
	}`)

	out, patched, err := EnsureMiMoReasoningContent("xiaomi/mimo-v2.5-pro", body)
	if err != nil {
		t.Fatalf("EnsureMiMoReasoningContent() error = %v", err)
	}
	if patched != 1 {
		t.Fatalf("patched = %d, want %d", patched, 1)
	}
	got := gjson.GetBytes(out, "messages.0.reasoning_content")
	if !got.Exists() || got.String() != "" {
		t.Fatalf("messages.0.reasoning_content = %q, exists=%v; want empty string", got.String(), got.Exists())
	}
}

func TestEnsureMiMoReasoningContent_PreservesExistingReasoningContent(t *testing.T) {
	body := []byte(`{
		"model":"mimo-v2.5-pro",
		"messages":[
			{"role":"assistant","tool_calls":[{"id":"call_1","type":"function","function":{"name":"list_directory","arguments":"{}"}}],"reasoning_content":"keep me"}
		]
	}`)

	out, patched, err := EnsureMiMoReasoningContent("mimo-v2.5-pro", body)
	if err != nil {
		t.Fatalf("EnsureMiMoReasoningContent() error = %v", err)
	}
	if patched != 0 {
		t.Fatalf("patched = %d, want %d", patched, 0)
	}
	got := gjson.GetBytes(out, "messages.0.reasoning_content").String()
	if got != "keep me" {
		t.Fatalf("messages.0.reasoning_content = %q, want %q", got, "keep me")
	}
}

func TestEnsureMiMoReasoningContent_SkipsNonMiMoModel(t *testing.T) {
	body := []byte(`{
		"model":"gpt-4.1",
		"messages":[
			{"role":"assistant","tool_calls":[{"id":"call_1","type":"function","function":{"name":"list_directory","arguments":"{}"}}]}
		]
	}`)

	out, patched, err := EnsureMiMoReasoningContent("gpt-4.1", body)
	if err != nil {
		t.Fatalf("EnsureMiMoReasoningContent() error = %v", err)
	}
	if patched != 0 {
		t.Fatalf("patched = %d, want %d", patched, 0)
	}
	if gjson.GetBytes(out, "messages.0.reasoning_content").Exists() {
		t.Fatalf("messages.0.reasoning_content should be absent for non-MiMo model")
	}
}

func TestEnsureMiMoReasoningContent_UsesPayloadModelFallback(t *testing.T) {
	body := []byte(`{
		"model":"mimo-v2-pro",
		"messages":[
			{"role":"assistant","tool_calls":[{"id":"call_1","type":"function","function":{"name":"list_directory","arguments":"{}"}}]}
		]
	}`)

	out, patched, err := EnsureMiMoReasoningContent("local-alias", body)
	if err != nil {
		t.Fatalf("EnsureMiMoReasoningContent() error = %v", err)
	}
	if patched != 1 {
		t.Fatalf("patched = %d, want %d", patched, 1)
	}
	if !gjson.GetBytes(out, "messages.0.reasoning_content").Exists() {
		t.Fatalf("messages.0.reasoning_content should exist")
	}
}

func TestEnsureMiMoClaudeThinkingContent_PrependsThinkingForAssistantText(t *testing.T) {
	body := []byte(`{
		"model":"xiaomi/mimo-v2.5-pro",
		"thinking":{"type":"enabled","budget_tokens":2048},
		"messages":[
			{"role":"assistant","content":"previous answer"},
			{"role":"user","content":"next"}
		]
	}`)

	out, patched, err := EnsureMiMoClaudeThinkingContent("xiaomi/mimo-v2.5-pro", "", body)
	if err != nil {
		t.Fatalf("EnsureMiMoClaudeThinkingContent() error = %v", err)
	}
	if patched != 1 {
		t.Fatalf("patched = %d, want %d", patched, 1)
	}
	if got := gjson.GetBytes(out, "messages.0.content.0.type").String(); got != "thinking" {
		t.Fatalf("messages.0.content.0.type = %q, want thinking", got)
	}
	if got := gjson.GetBytes(out, "messages.0.content.0.thinking"); !got.Exists() || got.String() != "" {
		t.Fatalf("messages.0.content.0.thinking = %q, exists=%v; want empty string", got.String(), got.Exists())
	}
	if got := gjson.GetBytes(out, "messages.0.content.1.text").String(); got != "previous answer" {
		t.Fatalf("messages.0.content.1.text = %q, want previous answer", got)
	}
}

func TestEnsureMiMoClaudeThinkingContent_FillsExistingThinkingBlock(t *testing.T) {
	body := []byte(`{
		"model":"mimo-v2.5-pro",
		"thinking":{"type":"enabled","budget_tokens":2048},
		"messages":[
			{"role":"assistant","content":[{"type":"thinking"},{"type":"text","text":"previous answer"}]}
		]
	}`)

	out, patched, err := EnsureMiMoClaudeThinkingContent("mimo-v2.5-pro", "", body)
	if err != nil {
		t.Fatalf("EnsureMiMoClaudeThinkingContent() error = %v", err)
	}
	if patched != 1 {
		t.Fatalf("patched = %d, want %d", patched, 1)
	}
	if got := gjson.GetBytes(out, "messages.0.content.0.thinking"); !got.Exists() || got.String() != "" {
		t.Fatalf("messages.0.content.0.thinking = %q, exists=%v; want empty string", got.String(), got.Exists())
	}
}

func TestEnsureMiMoClaudeThinkingContent_SkipsDisabledThinking(t *testing.T) {
	body := []byte(`{
		"model":"mimo-v2.5-pro",
		"thinking":{"type":"disabled"},
		"messages":[
			{"role":"assistant","content":"previous answer"}
		]
	}`)

	out, patched, err := EnsureMiMoClaudeThinkingContent("mimo-v2.5-pro", "", body)
	if err != nil {
		t.Fatalf("EnsureMiMoClaudeThinkingContent() error = %v", err)
	}
	if patched != 0 {
		t.Fatalf("patched = %d, want %d", patched, 0)
	}
	if gjson.GetBytes(out, "messages.0.content.0.thinking").Exists() {
		t.Fatalf("thinking should be absent when thinking is disabled")
	}
}

func TestEnsureMiMoClaudeToolResults_GroupsResponsesToolHistory(t *testing.T) {
	body := []byte(`{
		"model":"xiaomi/mimo-v2.5-pro",
		"messages":[
			{"role":"user","content":"inspect"},
			{"role":"assistant","content":"checking"},
			{"role":"assistant","content":[{"type":"tool_use","id":"fc_call_01","name":"Bash","input":{"command":"pwd"}}]},
			{"role":"assistant","content":[{"type":"tool_use","id":"fc_call_02","name":"Bash","input":{"command":"ls"}}]},
			{"role":"user","content":[{"type":"tool_result","tool_use_id":"fc_call_02","content":"ls output"}]},
			{"role":"user","content":[{"type":"tool_result","tool_use_id":"fc_call_01","content":"pwd output"}]}
		]
	}`)

	out, patched, err := EnsureMiMoClaudeToolResults("xiaomi/mimo-v2.5-pro", "", body)
	if err != nil {
		t.Fatalf("EnsureMiMoClaudeToolResults() error = %v", err)
	}
	if patched == 0 {
		t.Fatal("patched = 0, want tool history repair")
	}
	if got := len(gjson.GetBytes(out, "messages").Array()); got != 3 {
		t.Fatalf("messages length = %d, want %d", got, 3)
	}
	if got := gjson.GetBytes(out, "messages.1.content.0.text").String(); got != "checking" {
		t.Fatalf("messages.1.content.0.text = %q, want checking", got)
	}
	if got := gjson.GetBytes(out, "messages.1.content.1.id").String(); got != "fc_call_01" {
		t.Fatalf("messages.1.content.1.id = %q, want fc_call_01", got)
	}
	if got := gjson.GetBytes(out, "messages.1.content.2.id").String(); got != "fc_call_02" {
		t.Fatalf("messages.1.content.2.id = %q, want fc_call_02", got)
	}
	if got := gjson.GetBytes(out, "messages.2.content.0.tool_use_id").String(); got != "fc_call_01" {
		t.Fatalf("messages.2.content.0.tool_use_id = %q, want fc_call_01", got)
	}
	if got := gjson.GetBytes(out, "messages.2.content.1.tool_use_id").String(); got != "fc_call_02" {
		t.Fatalf("messages.2.content.1.tool_use_id = %q, want fc_call_02", got)
	}
}
