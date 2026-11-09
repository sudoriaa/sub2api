package service

// Early semantic SSE delivery is an opt-in for compatible API-key relays.
// A complete reasoning-start event commits the attempt just like other semantic
// output. Lifecycle-only events and incomplete JSON remain staged, and neither
// visible TTFT nor the inference/completion clocks are changed.
func (a *Account) IsOpenAIEarlySemanticSSEEnabled() bool {
	if a == nil || a.Platform != PlatformOpenAI || a.Type != AccountTypeAPIKey {
		return false
	}
	enabled, _ := a.Extra["openai_early_semantic_sse"].(bool)
	return enabled
}
