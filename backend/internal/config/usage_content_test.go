package config

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestUsageContentMemoryDefaultsAndValidation(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		resetViperWithJWTSecret(t)
		cfg, err := Load()
		require.NoError(t, err)
		require.True(t, cfg.Gateway.UsageContent.Enabled)
		require.Equal(t, 100, cfg.Gateway.UsageContent.MaxRecords)
		require.EqualValues(t, 64, cfg.Gateway.UsageContent.MaxMemoryMiB)
	})
	for _, tc := range []struct {
		key, value string
		valid      bool
	}{
		{"GATEWAY_USAGE_CONTENT_MAX_RECORDS", "12", true},
		{"GATEWAY_USAGE_CONTENT_MAX_RECORDS", "1001", false},
		{"GATEWAY_USAGE_CONTENT_MAX_RECORDS", "-1", false},
		{"GATEWAY_USAGE_CONTENT_MAX_MEMORY_MIB", "32", true},
		{"GATEWAY_USAGE_CONTENT_MAX_MEMORY_MIB", "1025", false},
		{"GATEWAY_USAGE_CONTENT_MAX_MEMORY_MIB", "-1", false},
	} {
		t.Run(tc.key+tc.value, func(t *testing.T) {
			resetViperWithJWTSecret(t)
			t.Setenv(tc.key, tc.value)
			_, err := Load()
			if tc.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}
