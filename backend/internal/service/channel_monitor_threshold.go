package service

import (
	"strconv"
	"strings"
)

const (
	DefaultChannelMonitorDegradedThresholdSeconds = 6
	MinChannelMonitorDegradedThresholdSeconds     = 1
	MaxChannelMonitorDegradedThresholdSeconds     = 90
)

// Invalid or missing stored values retain the original six-second behavior.
func normalizeChannelMonitorDegradedThresholdSeconds(seconds int) int {
	if seconds < MinChannelMonitorDegradedThresholdSeconds || seconds > MaxChannelMonitorDegradedThresholdSeconds {
		return DefaultChannelMonitorDegradedThresholdSeconds
	}
	return seconds
}

func parseChannelMonitorDegradedThresholdSeconds(raw string) int {
	seconds, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return DefaultChannelMonitorDegradedThresholdSeconds
	}
	return normalizeChannelMonitorDegradedThresholdSeconds(seconds)
}
