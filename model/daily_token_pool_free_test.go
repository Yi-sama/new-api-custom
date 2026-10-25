package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDailyTokenPoolFreeUsageRequiresExplicitScopeAndDefaultsToPaid(t *testing.T) {
	config := DefaultDailyTokenPoolConfig()
	require.NoError(t, common.UnmarshalJsonStr(`{"enabled":true}`, &config))
	assert.False(t, config.FreeUsage)
	config.FreeUsage = true
	require.Error(t, config.Validate())
	config.Groups = []string{"shared"}
	require.Error(t, config.Validate())
	config.ChannelIDs = []int{34}
	require.NoError(t, config.Validate())
}
