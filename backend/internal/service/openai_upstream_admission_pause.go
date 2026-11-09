package service

import (
	"net/http"
	"strings"

	"github.com/tidwall/gjson"
)

// A relay's user/group admission pause is not an unhealthy model account or
// transient pool capacity. Replaying the same request cannot lift that pause.
const OpenAIUpstreamAdmissionPausedReason = GatewayFailureReason("openai_upstream_admission_paused")

const OpenAIUpstreamAdmissionPausedMessage = "Upstream request admission is temporarily paused; respect Retry-After before retrying"

func isOpenAIUpstreamAdmissionPaused(status int, body []byte) bool {
	if status != http.StatusTooManyRequests || !gjson.ValidBytes(body) {
		return false
	}
	// Require the authoritative structured code. A message can echo user input;
	// a different primary code must not be overridden by a secondary envelope.
	for _, path := range []string{"error.code", "response.error.code", "code"} {
		code := gjson.GetBytes(body, path)
		if code.Type == gjson.String && strings.TrimSpace(code.String()) != "" {
			return strings.EqualFold(strings.TrimSpace(code.String()), "group_anti_distillation_paused")
		}
	}
	return false
}
