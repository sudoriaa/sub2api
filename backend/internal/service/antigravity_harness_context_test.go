package service

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestAntigravityHarnessContextOptInAndExactTemplateBoundary(t *testing.T) {
	input := json.RawMessage(`[{"role":"system","content":"You are an AI agent powered by DeepSeek Harness.\nKeep all runtime instructions.","extra":{"number":9007199254740993}},{"role":"developer","content":"Keep this separate higher-priority policy."},{"role":"user","content":"You are an AI agent powered by DeepSeek Harness. This is user content."}]`)
	for _, tt := range []struct {
		name     string
		platform string
		flag     any
		wantRole string
	}{
		{"enabled", PlatformAntigravity, true, "user"},
		{"disabled", PlatformAntigravity, false, "system"},
		{"absent", PlatformAntigravity, nil, "system"},
		{"string flag is not consent", PlatformAntigravity, "true", "system"},
		{"unrelated provider", PlatformGemini, true, "system"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			account := &Account{Platform: tt.platform, Extra: map[string]any{AntigravityDeepSeekHarnessContextCompatKey: tt.flag}}
			request := &apicompat.ResponsesRequest{Model: "gemini-4-carbon-high", Instructions: "Keep these instructions too.", Input: append(json.RawMessage(nil), input...)}
			require.NoError(t, applyAntigravityHarnessContextCompatibility(account, request))
			require.Equal(t, tt.wantRole, gjson.GetBytes(request.Input, "0.role").String())
			require.Equal(t, "developer", gjson.GetBytes(request.Input, "1.role").String())
			require.Equal(t, "user", gjson.GetBytes(request.Input, "2.role").String())
			require.Equal(t, "9007199254740993", gjson.GetBytes(request.Input, "0.extra.number").Raw)
			require.Equal(t, gjson.GetBytes(input, "0.content").String(), gjson.GetBytes(request.Input, "0.content").String())
			require.Equal(t, gjson.GetBytes(input, "2.content").String(), gjson.GetBytes(request.Input, "2.content").String())
			require.Equal(t, "Keep these instructions too.", request.Instructions)
			require.Equal(t, "gemini-4-carbon-high", request.Model)
			if tt.wantRole == "system" {
				require.Equal(t, input, request.Input, "disabled paths do not even reserialize the input")
			}
		})
	}
}

func TestAntigravityHarnessContextContentFormsAndNoFalseMatches(t *testing.T) {
	account := &Account{Platform: PlatformAntigravity, Extra: map[string]any{AntigravityDeepSeekHarnessContextCompatKey: true}}
	for _, tt := range []struct {
		name     string
		input    string
		wantRole string
	}{
		{"text block", `[{"role":"system","content":[{"type":"input_text","text":"You are an AI agent powered by DeepSeek Harness.\nKeep me.","extra":true}]}]`, "user"},
		{"developer template", `[{"role":"developer","content":" \nYou are an AI agent powered by DeepSeek Harness.\nKeep me."}]`, "user"},
		{"ordinary mention", `[{"role":"system","content":"Explain how DeepSeek Harness works."}]`, "system"},
		{"quoted template", `[{"role":"system","content":"Example: You are an AI agent powered by DeepSeek Harness."}]`, "system"},
		{"metadata without template", `[{"role":"system","content":"The runtime context names DeepSeek Harness."}]`, "system"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			request := &apicompat.ResponsesRequest{Input: json.RawMessage(tt.input)}
			require.NoError(t, applyAntigravityHarnessContextCompatibility(account, request))
			require.Equal(t, tt.wantRole, gjson.GetBytes(request.Input, "0.role").String())
			before := gjson.Get(tt.input, "0.content")
			require.JSONEq(t, before.Raw, gjson.GetBytes(request.Input, "0.content").Raw)
		})
	}
	for _, input := range []string{`"plain input"`, `null`, `[]`, `[null]`, `["unsupported item"]`} {
		request := &apicompat.ResponsesRequest{Input: json.RawMessage(input)}
		require.NoError(t, applyAntigravityHarnessContextCompatibility(account, request))
		require.Equal(t, input, string(request.Input))
	}
	require.NoError(t, applyAntigravityHarnessContextCompatibility(nil, nil))
}

func TestAntigravityResponsesHarnessContextPreservesModelToolsAndTokenBudget(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "default unchanged", true: "account opted in"}[enabled], func(t *testing.T) {
			upstream := &queuedHTTPUpstreamStub{responses: []*http.Response{antigravityCompatSuccessResponse()}}
			svc := newAntigravityCompatService(config.GatewayConfig{MaxLineSize: defaultMaxLineSize}, upstream)
			account := newAntigravityCompatAccount(AccountTypeOAuth)
			account.Extra = map[string]any{AntigravityDeepSeekHarnessContextCompatKey: enabled}
			body := []byte(`{"model":"gemini-3.1-pro-high","input":[{"role":"system","content":"You are an AI agent powered by DeepSeek Harness.\nKeep runtime context."},{"role":"user","content":[{"type":"input_text","text":"Keep the actual task."}]}],"stream":true,"max_output_tokens":128000,"tools":[{"type":"function","name":"shell","parameters":{"type":"object","properties":{"command":{"type":"string"}}}}]}`)
			c, _ := newAntigravityCompatContext(http.MethodPost, "/v1/responses", body)
			result, err := svc.ForwardAsResponses(context.Background(), c, account, body, nil)
			require.NoError(t, err)
			require.NotNil(t, result)
			require.Len(t, upstream.requestBodies, 1)
			sent := upstream.requestBodies[0]
			require.Equal(t, "gemini-3.1-pro-high", gjson.GetBytes(sent, "model").String())
			require.Equal(t, int64(128000), gjson.GetBytes(sent, "request.generationConfig.maxOutputTokens").Int())
			require.Equal(t, "shell", gjson.GetBytes(sent, "request.tools.0.functionDeclarations.0.name").String())
			system := gjson.GetBytes(sent, "request.systemInstruction").Raw
			contents := gjson.GetBytes(sent, "request.contents").Raw
			if enabled {
				require.NotContains(t, system, deepSeekHarnessSystemOpener)
				require.Contains(t, contents, deepSeekHarnessSystemOpener)
				require.Contains(t, contents, "Keep runtime context.")
			} else {
				require.Contains(t, system, deepSeekHarnessSystemOpener)
				require.NotContains(t, contents, deepSeekHarnessSystemOpener)
			}
			require.Contains(t, contents, "Keep the actual task.")
			require.Contains(t, string(body), `"role":"system"`, "the captured/raw client request is not mutated")
		})
	}
}
