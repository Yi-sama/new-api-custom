package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChannelStatusFromChecksUsesAvailabilityThresholds(t *testing.T) {
	tests := []struct {
		name     string
		checks   []model.ChannelStatusCheck
		expected string
	}{
		{name: "no checks stays unknown", expected: "unknown"},
		{name: "seventy five percent is normal", checks: statusChecks(3, 1), expected: "normal"},
		{name: "fifty percent is degraded", checks: statusChecks(2, 2), expected: "degraded"},
		{name: "below fifty percent is failed", checks: statusChecks(1, 3), expected: "failed"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			card := channelStatusFromChecks(test.checks)
			assert.Equal(t, test.expected, card.Status)
		})
	}
}

func TestAggregateChannelStatusChecksCountsOneRequestOnce(t *testing.T) {
	checks := []model.ChannelStatusCheck{
		{ID: 1, RequestID: "request-1", Status: "failed", CheckedAt: 10, LatencyMs: 100},
		{ID: 2, RequestID: "request-1", Status: "normal", CheckedAt: 20, LatencyMs: 200},
		{ID: 3, RequestID: "request-2", Status: "failed", CheckedAt: 30, LatencyMs: 300},
	}

	aggregated := aggregateChannelStatusChecks(checks)
	require.Len(t, aggregated, 2)
	assert.Equal(t, "normal", aggregated[0].Status)
	assert.Equal(t, "failed", aggregated[1].Status)
	assert.Equal(t, "request-1", aggregated[0].RequestID)
}

func statusChecks(successes int, failures int) []model.ChannelStatusCheck {
	checks := make([]model.ChannelStatusCheck, 0, successes+failures)
	for i := 0; i < successes; i++ {
		checks = append(checks, model.ChannelStatusCheck{Status: "normal", LatencyMs: 100, CheckedAt: int64(i + 1)})
	}
	for i := 0; i < failures; i++ {
		checks = append(checks, model.ChannelStatusCheck{Status: "failed", CheckedAt: int64(successes + i + 1)})
	}
	return checks
}
