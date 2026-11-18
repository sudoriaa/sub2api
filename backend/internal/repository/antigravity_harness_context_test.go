//go:build unit

package repository

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAntigravityHarnessContextConsentSurvivesSchedulerProjection(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		account := service.Account{ID: 8, Platform: service.PlatformAntigravity, Type: service.AccountTypeOAuth,
			Extra: map[string]any{service.AntigravityDeepSeekHarnessContextCompatKey: enabled, "secret": "do-not-project"}}
		metadata := buildSchedulerMetadataAccount(account)
		require.Equal(t, enabled, metadata.Extra[service.AntigravityDeepSeekHarnessContextCompatKey])
		require.NotContains(t, metadata.Extra, "secret")
		raw, err := json.Marshal(metadata)
		require.NoError(t, err)
		var restored service.Account
		require.NoError(t, json.Unmarshal(raw, &restored))
		require.Equal(t, enabled, restored.Extra[service.AntigravityDeepSeekHarnessContextCompatKey])
	}
	require.True(t, shouldEnqueueSchedulerOutboxForExtraUpdates(map[string]any{
		service.AntigravityDeepSeekHarnessContextCompatKey: true,
	}))
}
