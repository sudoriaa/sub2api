package service

import (
	"encoding/json"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

// AntigravityDeepSeekHarnessContextCompatKey is deliberately opt-in per account:
// relocating a client system template changes its instruction priority.
const AntigravityDeepSeekHarnessContextCompatKey = "antigravity_deepseek_harness_system_as_context"

const deepSeekHarnessSystemOpener = "You are an AI agent powered by DeepSeek Harness."

// applyAntigravityHarnessContextCompatibility preserves the client's complete
// template as the first context message at its original position. Only the
// recognized DeepSeek Harness template changes role; unrelated system/developer
// instructions, user content, tools, models and output limits remain unchanged.
func applyAntigravityHarnessContextCompatibility(account *Account, request *apicompat.ResponsesRequest) error {
	if account == nil || request == nil || account.Platform != PlatformAntigravity {
		return nil
	}
	enabled, _ := account.Extra[AntigravityDeepSeekHarnessContextCompatKey].(bool)
	if !enabled || !strings.HasPrefix(strings.TrimSpace(string(request.Input)), "[") {
		return nil
	}
	var items []map[string]json.RawMessage
	if err := json.Unmarshal(request.Input, &items); err != nil {
		// Leave invalid/unsupported input shapes to the normal protocol validator.
		return nil
	}
	moved := 0
	for _, item := range items {
		var role string
		if json.Unmarshal(item["role"], &role) != nil || (role != "system" && role != "developer") {
			continue
		}
		text := firstHarnessSystemText(item["content"])
		if !strings.HasPrefix(strings.TrimLeft(text, " \t\r\n"), deepSeekHarnessSystemOpener) {
			continue
		}
		item["role"] = json.RawMessage(`"user"`)
		moved++
	}
	if moved == 0 {
		return nil
	}
	input, err := json.Marshal(items)
	if err != nil {
		return err
	}
	request.Input = input
	logger.LegacyPrintf("service.antigravity_gateway", "[Antigravity] deepseek_harness_system_context account=%d moved=%d", account.ID, moved)
	return nil
}

func firstHarnessSystemText(content json.RawMessage) string {
	var text string
	if json.Unmarshal(content, &text) == nil {
		return text
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(content, &blocks) == nil {
		for _, block := range blocks {
			if (block.Type == "input_text" || block.Type == "text") && strings.TrimSpace(block.Text) != "" {
				return block.Text
			}
		}
	}
	return ""
}
