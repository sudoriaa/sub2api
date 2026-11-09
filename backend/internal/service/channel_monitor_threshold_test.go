//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestChannelMonitorThresholdParsingAndPublicSettings(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want int
	}{
		{"", 6}, {"invalid", 6}, {"0", 6}, {"-1", 6}, {"91", 6}, {"1.5", 6},
		{"1", 1}, {" 15 ", 15}, {"90", 90},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			require.Equal(t, tc.want, parseChannelMonitorDegradedThresholdSeconds(tc.raw))
			svc := NewSettingService(&settingPublicRepoStub{values: map[string]string{
				SettingKeyChannelMonitorDegradedThresholdSeconds: tc.raw,
			}}, &config.Config{})
			require.Equal(t, tc.want, svc.GetChannelMonitorRuntime(context.Background()).DegradedThresholdSeconds)
			public, err := svc.GetPublicSettings(context.Background())
			require.NoError(t, err)
			require.Equal(t, tc.want, public.ChannelMonitorDegradedThresholdSeconds)
			injected, err := svc.GetPublicSettingsForInjection(context.Background())
			require.NoError(t, err)
			raw, err := json.Marshal(injected)
			require.NoError(t, err)
			require.EqualValues(t, tc.want, gjson.GetBytes(raw, "channel_monitor_degraded_threshold_seconds").Int())
		})
	}
	var nilService *SettingService
	require.Equal(t, 6, nilService.GetChannelMonitorRuntime(context.Background()).DegradedThresholdSeconds)
	failing := NewSettingService(&settingPublicRepoStub{err: errors.New("unavailable")}, &config.Config{})
	require.Equal(t, 6, failing.GetChannelMonitorRuntime(context.Background()).DegradedThresholdSeconds)
}

func TestChannelMonitorThresholdBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name    string
		opts    *CheckOptions
		latency time.Duration
		want    string
	}{
		{"legacy below", nil, 5999 * time.Millisecond, MonitorStatusOperational},
		{"legacy equal", nil, 6 * time.Second, MonitorStatusDegraded},
		{"custom below", &CheckOptions{DegradedThreshold: 15 * time.Second}, 14999 * time.Millisecond, MonitorStatusOperational},
		{"custom equal", &CheckOptions{DegradedThreshold: 15 * time.Second}, 15 * time.Second, MonitorStatusDegraded},
		{"custom above", &CheckOptions{DegradedThreshold: time.Second}, 2 * time.Second, MonitorStatusDegraded},
		{"zero fallback", &CheckOptions{}, 6 * time.Second, MonitorStatusDegraded},
		{"negative fallback", &CheckOptions{DegradedThreshold: -time.Second}, 6 * time.Second, MonitorStatusDegraded},
		{"out of range fallback", &CheckOptions{DegradedThreshold: 91 * time.Second}, 6 * time.Second, MonitorStatusDegraded},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := finalizeOperationalOrDegraded(&CheckResult{}, tc.latency, int(tc.latency/time.Millisecond), tc.opts)
			require.Equal(t, tc.want, result.Status)
			if tc.want == MonitorStatusDegraded {
				require.Contains(t, result.Message, "slow response:")
			}
		})
	}
}

func TestChannelMonitorThresholdReloadedForNextCheck(t *testing.T) {
	repo := &settingPublicRepoStub{values: map[string]string{SettingKeyChannelMonitorDegradedThresholdSeconds: "6"}}
	svc := NewChannelMonitorService(nil, nil)
	svc.SetRuntimeReader(NewSettingService(repo, &config.Config{}))
	monitor := &ChannelMonitor{APIMode: MonitorAPIModeResponses, BodyOverrideMode: MonitorBodyOverrideModeMerge}
	oldOptions := svc.buildCheckOptions(context.Background(), monitor)
	require.Equal(t, 6*time.Second, oldOptions.DegradedThreshold)
	repo.values[SettingKeyChannelMonitorDegradedThresholdSeconds] = "15"
	newOptions := svc.buildCheckOptions(context.Background(), monitor)
	require.Equal(t, 15*time.Second, newOptions.DegradedThreshold)
	require.Equal(t, 6*time.Second, oldOptions.DegradedThreshold, "running checks keep their settings snapshot")
	require.Equal(t, monitor.APIMode, newOptions.APIMode)
}

func TestChannelMonitorThresholdDoesNotHideFailures(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		want   string
	}{
		{"HTTP error", 503, MonitorStatusError},
		{"wrong answer", 200, MonitorStatusFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			endpoint := setupFakeAnthropic(t, &captureHandler{status: tc.status, respondText: "not an answer"})
			result := runCheckForModel(context.Background(), MonitorProviderAnthropic, endpoint, "test-key", "test-model", &CheckOptions{DegradedThreshold: 90 * time.Second})
			require.Equal(t, tc.want, result.Status)
		})
	}
}
