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
