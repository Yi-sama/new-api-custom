package controller

import (
	"net/url"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseProfitabilityReportOptions(t *testing.T) {
	tests := []struct {
		name      string
		values    url.Values
		want      model.ProfitabilityReportOptions
		wantError bool
	}{
		{
			name:   "absent channels keeps legacy all scope",
			values: url.Values{},
			want:   model.ProfitabilityReportOptions{RevenueBasis: model.ProfitabilityRevenueBasisCalls},
		},
		{
			name:   "explicit empty selects no channels",
			values: url.Values{"channel_ids": {""}},
			want:   model.ProfitabilityReportOptions{RevenueBasis: model.ProfitabilityRevenueBasisCalls, ChannelSelectionPresent: true},
		},
		{
			name:   "positive channel ids are deduplicated",
			values: url.Values{"channel_ids": {"4, 4,32,34"}},
			want: model.ProfitabilityReportOptions{
				RevenueBasis: model.ProfitabilityRevenueBasisCalls, ChannelSelectionPresent: true, ChannelIDs: []int{4, 32, 34},
			},
		},
		{
			name:   "cash ignores channel selection",
			values: url.Values{"revenue_basis": {"cash"}, "channel_ids": {"4,32"}},
			want:   model.ProfitabilityReportOptions{RevenueBasis: model.ProfitabilityRevenueBasisCash},
		},
		{name: "zero id is invalid", values: url.Values{"channel_ids": {"0"}}, wantError: true},
		{name: "negative id is invalid", values: url.Values{"channel_ids": {"-1"}}, wantError: true},
		{name: "non numeric id is invalid", values: url.Values{"channel_ids": {"4,x"}}, wantError: true},
		{name: "empty csv item is invalid", values: url.Values{"channel_ids": {"4,,32"}}, wantError: true},
		{name: "unknown basis is invalid", values: url.Values{"revenue_basis": {"quota"}}, wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseProfitabilityReportOptions(tt.values)
			if tt.wantError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
