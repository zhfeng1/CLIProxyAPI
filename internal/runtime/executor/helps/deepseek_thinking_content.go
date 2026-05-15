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
		if !content.Exists() || !content.IsArray() {
			continue
		}

		hasThinking := false
		hasToolUse := false
		for blockIdx, block := range content.Array() {
			switch strings.TrimSpace(block.Get("type").String()) {
			case "thinking":
				hasThinking = true
			case "tool_use":
				hasToolUse = true
			default:
				continue
			}
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
		if hasThinking || !hasToolUse {
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

	items := messages.Array()
	patched := 0
	outMessages := make([][]byte, 0, len(items))
	for idx := 0; idx < len(items); {
		msg := items[idx]
		if strings.TrimSpace(msg.Get("role").String()) != "assistant" || len(claudeToolUseIDs(msg)) == 0 {
			outMessages = append(outMessages, []byte(msg.Raw))
			idx++
			continue
		}

		assistantGroup := []gjson.Result{msg}
		assistantGroupToolOnly := isToolOnlyAssistantMessage(msg)
		nextIdx := idx + 1
		if assistantGroupToolOnly {
			for nextIdx < len(items) &&
				strings.TrimSpace(items[nextIdx].Get("role").String()) == "assistant" &&
				isToolOnlyAssistantMessage(items[nextIdx]) &&
				len(claudeToolUseIDs(items[nextIdx])) > 0 {
				assistantGroup = append(assistantGroup, items[nextIdx])
				nextIdx++
			}
		}

		assistantRaw, groupedToolUseIDs, changed, err := buildGroupedAssistantToolUseMessage(assistantGroup)
		if err != nil {
			return body, patched, err
		}
		if assistantGroupToolOnly && len(outMessages) > 0 {
			if mergedRaw, ok, errMerge := mergeAssistantToolUseIntoPrevious(outMessages[len(outMessages)-1], assistantRaw); errMerge != nil {
				return body, patched, errMerge
			} else if ok {
				outMessages[len(outMessages)-1] = mergedRaw
				patched++
			} else {
				outMessages = append(outMessages, assistantRaw)
			}
		} else {
			outMessages = append(outMessages, assistantRaw)
		}
		if changed {
			patched += len(assistantGroup) - 1
		}

		userGroup := make([]gjson.Result, 0)
		if nextIdx < len(items) && strings.TrimSpace(items[nextIdx].Get("role").String()) == "user" {
			nextIdx++
			userGroup = append(userGroup, items[nextIdx-1])
			for nextIdx < len(items) &&
				strings.TrimSpace(items[nextIdx].Get("role").String()) == "user" &&
				hasClaudeToolResults(items[nextIdx]) {
				userGroup = append(userGroup, items[nextIdx])
				nextIdx++
			}
		}

		userRaw, added, userChanged, err := buildGroupedToolResultMessage(userGroup, groupedToolUseIDs)
		if err != nil {
			return body, patched, err
		}
		outMessages = append(outMessages, userRaw)
		patched += added
		if userChanged {
			patched += len(userGroup) - 1
		}

		idx = nextIdx
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

func isThinkingModeEnabled(body []byte) bool {
	thinkingType := strings.TrimSpace(gjson.GetBytes(body, "thinking.type").String())
	return thinkingType != "" && !strings.EqualFold(thinkingType, "disabled")
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

func hasClaudeToolResults(message gjson.Result) bool {
	content := message.Get("content")
	if !content.Exists() || !content.IsArray() {
		return false
	}
	found := false
	content.ForEach(func(_, block gjson.Result) bool {
		if strings.TrimSpace(block.Get("type").String()) == "tool_result" {
			found = true
			return false
		}
		return true
	})
	return found
}

func isToolOnlyAssistantMessage(message gjson.Result) bool {
	content := message.Get("content")
	if !content.Exists() || !content.IsArray() {
		return false
	}
	hasToolUse := false
	toolOnly := true
	content.ForEach(func(_, block gjson.Result) bool {
		switch strings.TrimSpace(block.Get("type").String()) {
		case "tool_use":
			hasToolUse = true
		case "thinking":
		default:
			toolOnly = false
			return false
		}
		return true
	})
	return hasToolUse && toolOnly
}

func buildGroupedAssistantToolUseMessage(group []gjson.Result) ([]byte, []string, bool, error) {
	if len(group) == 0 {
		return nil, nil, false, fmt.Errorf("deepseek assistant tool_use group is empty")
	}
	var blocks [][]byte
	var ids []string
	var thinkingBlock []byte
	var toolUseBlocks [][]byte
	for _, message := range group {
		content := message.Get("content")
		if !content.Exists() || !content.IsArray() {
			continue
		}
		content.ForEach(func(_, block gjson.Result) bool {
			switch strings.TrimSpace(block.Get("type").String()) {
			case "thinking":
				if thinkingBlock == nil {
					thinkingBlock = []byte(block.Raw)
				}
			case "tool_use":
				toolUseBlocks = append(toolUseBlocks, []byte(block.Raw))
				if id := strings.TrimSpace(block.Get("id").String()); id != "" {
					ids = append(ids, id)
				}
			}
			return true
		})
	}
	if len(group) == 1 {
		return []byte(group[0].Raw), ids, false, nil
	}
	if thinkingBlock != nil {
		blocks = append(blocks, thinkingBlock)
	}
	blocks = append(blocks, toolUseBlocks...)
	out, err := sjson.SetRawBytes([]byte(group[0].Raw), "content", joinJSONArray(blocks))
	if err != nil {
		return nil, nil, false, fmt.Errorf("deepseek assistant tool_use grouping failed: %w", err)
	}
	return out, ids, true, nil
}

func buildGroupedToolResultMessage(group []gjson.Result, toolUseIDs []string) ([]byte, int, bool, error) {
	required := make(map[string]struct{}, len(toolUseIDs))
	for _, id := range toolUseIDs {
		required[id] = struct{}{}
	}

	existingResults := make(map[string][][]byte, len(toolUseIDs))
	var extraResultBlocks [][]byte
	var otherBlocks [][]byte
	seen := make(map[string]struct{}, len(toolUseIDs))
	for _, message := range group {
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
	}

	if len(group) == 1 {
		content := group[0].Get("content")
		if content.Exists() && content.IsArray() {
			allPresent := true
			for _, id := range toolUseIDs {
				if _, ok := seen[id]; !ok {
					allPresent = false
					break
				}
			}
			if allPresent {
				return []byte(group[0].Raw), 0, false, nil
			}
		}
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
			return nil, 0, false, err
		}
		resultBlocks = append(resultBlocks, block)
	}
	resultBlocks = append(resultBlocks, extraResultBlocks...)

	blocks := make([][]byte, 0, len(resultBlocks)+len(otherBlocks))
	blocks = append(blocks, resultBlocks...)
	blocks = append(blocks, otherBlocks...)
	contentRaw := joinJSONArray(blocks)
	if len(group) == 0 {
		message := []byte(`{"role":"user","content":[]}`)
		out, err := sjson.SetRawBytes(message, "content", contentRaw)
		if err != nil {
			return nil, 0, false, fmt.Errorf("deepseek synthetic tool_result message failed: %w", err)
		}
		return out, len(missing), false, nil
	}
	out, err := sjson.SetRawBytes([]byte(group[0].Raw), "content", contentRaw)
	if err != nil {
		return nil, 0, false, fmt.Errorf("deepseek tool_result content patch failed: %w", err)
	}
	return out, len(missing), len(group) > 1, nil
}

func mergeAssistantToolUseIntoPrevious(previousRaw []byte, toolUseRaw []byte) ([]byte, bool, error) {
	previous := gjson.ParseBytes(previousRaw)
	if strings.TrimSpace(previous.Get("role").String()) != "assistant" {
		return nil, false, nil
	}
	toolUseMessage := gjson.ParseBytes(toolUseRaw)
	if strings.TrimSpace(toolUseMessage.Get("role").String()) != "assistant" {
		return nil, false, nil
	}

	var blocks [][]byte
	hasThinking := false
	prevContent := previous.Get("content")
	switch {
	case prevContent.Exists() && prevContent.IsArray():
		prevContent.ForEach(func(_, block gjson.Result) bool {
			if strings.TrimSpace(block.Get("type").String()) == "thinking" {
				hasThinking = true
			}
			blocks = append(blocks, []byte(block.Raw))
			return true
		})
	case prevContent.Exists() && prevContent.Type == gjson.String && prevContent.String() != "":
		textBlock := []byte(`{"type":"text","text":""}`)
		textBlock, _ = sjson.SetBytes(textBlock, "text", prevContent.String())
		blocks = append(blocks, textBlock)
	}

	var thinkingBlock []byte
	toolContent := toolUseMessage.Get("content")
	if toolContent.Exists() && toolContent.IsArray() {
		toolContent.ForEach(func(_, block gjson.Result) bool {
			switch strings.TrimSpace(block.Get("type").String()) {
			case "thinking":
				if thinkingBlock == nil {
					thinkingBlock = []byte(block.Raw)
				}
			case "tool_use":
				blocks = append(blocks, []byte(block.Raw))
			}
			return true
		})
	}
	if thinkingBlock != nil && !hasThinking {
		blocks = append([][]byte{thinkingBlock}, blocks...)
	}
	if len(blocks) == 0 {
		return nil, false, nil
	}

	out, err := sjson.SetRawBytes(previousRaw, "content", joinJSONArray(blocks))
	if err != nil {
		return nil, false, fmt.Errorf("deepseek assistant tool_use merge failed: %w", err)
	}
	return out, true, nil
}

func prependDeepSeekThinkingBlock(content gjson.Result) ([]byte, error) {
	var blocks [][]byte
	blocks = append(blocks, []byte(`{"type":"thinking","thinking":""}`))
	if content.Exists() && content.IsArray() {
		content.ForEach(func(_, block gjson.Result) bool {
			blocks = append(blocks, []byte(block.Raw))
			return true
		})
	}
	return joinJSONArray(blocks), nil
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
