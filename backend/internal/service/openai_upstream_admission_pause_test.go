package service

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenAIUpstreamAdmissionPausedRequiresStructured429(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		want   bool
	}{
		{"relay", 429, `{"error":{"code":"group_anti_distillation_paused"}}`, true},
		{"responses", 429, `{"response":{"error":{"code":"group_anti_distillation_paused"}}}`, true},
		{"root", 429, `{"code":"group_anti_distillation_paused"}`, true},
		{"other_status", 503, `{"error":{"code":"group_anti_distillation_paused"}}`, false},
		{"echoed_message", 429, `{"error":{"message":"group_anti_distillation_paused"}}`, false},
		{"ordinary_capacity", 429, `{"error":{"code":"rate_limit_exceeded"}}`, false},
		{"primary_code_wins", 429, `{"error":{"code":"invalid_api_key"},"code":"group_anti_distillation_paused"}`, false},
		{"not_json", 429, `group_anti_distillation_paused`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, isOpenAIUpstreamAdmissionPaused(tc.status, []byte(tc.body)))
		})
	}
}

func TestOpenAIUpstreamAdmissionPauseStopsPoolAndOAuthRetries(t *testing.T) {
	body := []byte(`{"error":{"code":"group_anti_distillation_paused"}}`)
	headers := http.Header{"Retry-After": {"300"}}
	for _, typ := range []string{AccountTypeAPIKey, AccountTypeOAuth} {
		t.Run(typ, func(t *testing.T) {
			svc := &OpenAIGatewayService{}
			e := svc.newOpenAIAccountFailoverError(&Account{Platform: PlatformOpenAI, Type: typ}, 429, headers, body, "paused", false, true)
			require.False(t, e.RetryableOnSameAccount)
			require.False(t, e.ShouldRetryNextAccount())
			require.False(t, e.ShouldReportAccountScheduleFailure())
			require.Equal(t, OpenAIUpstreamAdmissionPausedReason, e.Reason)
			require.Equal(t, "300", e.ResponseHeaders.Get("Retry-After"))
			headers.Set("Retry-After", "301")
			require.Equal(t, "300", e.ResponseHeaders.Get("Retry-After"))
			headers.Set("Retry-After", "300")
		})
	}
	e := newOpenAIUpstreamFailoverError(429, nil, []byte(`{"error":{"code":"rate_limit_exceeded"}}`), "capacity", true)
	require.True(t, e.RetryableOnSameAccount, "ordinary capacity retry policy is unchanged")
	require.True(t, e.ShouldRetryNextAccount())
}
