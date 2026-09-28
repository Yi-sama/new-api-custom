package types

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestChannelRatioLogContainsConfiguredAndEffectiveValues(t *testing.T) {
	ratio := 0.18
	info := GroupRatioInfo{ChannelRatio: &ratio, GroupRatio: 0.1, HasSpecialRatio: true, GroupSpecialRatio: 0.1}
	other := make(map[string]interface{})
	info.AppendLogFields(other)
	require.Equal(t, 0.18, other["channel_ratio"])
	require.Equal(t, 0.1, other["effective_ratio"])
	require.Equal(t, 0.1, other["user_group_ratio"])
}
