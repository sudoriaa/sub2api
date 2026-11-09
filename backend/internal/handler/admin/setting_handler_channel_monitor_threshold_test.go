package admin

import (
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestChannelMonitorThresholdSettingsRoundTripAndOmission(t *testing.T) {
	h, repo := newStepUpSwitchTestHandler(t, map[string]string{})
	require.EqualValues(t, 6, readForecastSettings(t, h).Get("channel_monitor_degraded_threshold_seconds").Int())
	for _, seconds := range []int{1, 15, 90} {
		result := doUpdateSettings(t, h, map[string]any{"channel_monitor_degraded_threshold_seconds": seconds}, nil)
		require.Equal(t, http.StatusOK, result.Code, result.Body.String())
		require.EqualValues(t, seconds, gjson.GetBytes(result.Body.Bytes(), "data.channel_monitor_degraded_threshold_seconds").Int())
	}
	result := doUpdateSettings(t, h, map[string]any{"site_name": "Legacy client"}, nil)
	require.Equal(t, http.StatusOK, result.Code, result.Body.String())
	require.Equal(t, "90", repo.values[service.SettingKeyChannelMonitorDegradedThresholdSeconds])
	h, _ = newStepUpSwitchTestHandler(t, repo.values)
	require.EqualValues(t, 90, readForecastSettings(t, h).Get("channel_monitor_degraded_threshold_seconds").Int())
	changed := diffSettings(&service.SystemSettings{ChannelMonitorDegradedThresholdSeconds: 6}, &service.SystemSettings{ChannelMonitorDegradedThresholdSeconds: 15}, nil, nil, UpdateSettingsRequest{})
	require.Contains(t, changed, service.SettingKeyChannelMonitorDegradedThresholdSeconds)
}

func TestChannelMonitorThresholdSettingsRejectInvalidWithoutWrites(t *testing.T) {
	for _, value := range []any{0, -1, 91, 1.5, "15"} {
		h, repo := newStepUpSwitchTestHandler(t, map[string]string{service.SettingKeyChannelMonitorDegradedThresholdSeconds: "15"})
		result := doUpdateSettings(t, h, map[string]any{"channel_monitor_degraded_threshold_seconds": value, "site_name": "must not persist"}, nil)
		require.Equal(t, http.StatusBadRequest, result.Code, result.Body.String())
		require.Equal(t, "15", repo.values[service.SettingKeyChannelMonitorDegradedThresholdSeconds])
		require.NotEqual(t, "must not persist", repo.values[service.SettingKeySiteName])
	}
}
