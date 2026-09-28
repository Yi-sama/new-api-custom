package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestGetRandomSatisfiedChannelSkipsFullCachedChannel(t *testing.T) {
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	originalLimit := common.RelayChannelKeyMaxConcurrency
	common.MemoryCacheEnabled = true
	common.RelayChannelKeyMaxConcurrency = 1
	t.Cleanup(func() {
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		common.RelayChannelKeyMaxConcurrency = originalLimit
	})

	fullChannel := &Channel{Id: 101, Key: "full-key"}
	availableChannel := &Channel{Id: 102, Key: "available-key"}
	channelRuntime.Lock()
	originalRuntimeKeys := channelRuntime.keys
	channelRuntime.keys = map[[2]int]*channelKeyRuntime{
		{101, 0}: {inflight: 1},
	}
	channelRuntime.Unlock()
	t.Cleanup(func() {
		channelRuntime.Lock()
		channelRuntime.keys = originalRuntimeKeys
		channelRuntime.Unlock()
	})

	channelSyncLock.Lock()
	originalGroupChannels := group2model2channels
	originalChannels := channelsIDM
	group2model2channels = map[string]map[string][]int{
		"default": {"test-model": {101, 102}},
	}
	channelsIDM = map[int]*Channel{101: fullChannel, 102: availableChannel}
	channelSyncLock.Unlock()
	t.Cleanup(func() {
		channelSyncLock.Lock()
		group2model2channels = originalGroupChannels
		channelsIDM = originalChannels
		channelSyncLock.Unlock()
	})

	selected, err := GetRandomSatisfiedChannel("default", "test-model", 0, "")
	require.NoError(t, err)
	require.NotNil(t, selected)
	require.Equal(t, 102, selected.Id)
}

func TestAcquireBestChannelKeySkipsFullKey(t *testing.T) {
	originalLimit := common.RelayChannelKeyMaxConcurrency
	common.RelayChannelKeyMaxConcurrency = 1
	t.Cleanup(func() { common.RelayChannelKeyMaxConcurrency = originalLimit })

	channelRuntime.Lock()
	originalRuntimeKeys := channelRuntime.keys
	channelRuntime.keys = map[[2]int]*channelKeyRuntime{
		{201, 0}: {inflight: 1},
	}
	channelRuntime.Unlock()
	t.Cleanup(func() {
		channelRuntime.Lock()
		channelRuntime.keys = originalRuntimeKeys
		channelRuntime.Unlock()
	})

	channel := &Channel{
		Id:          201,
		Key:         "key-0\nkey-1",
		ChannelInfo: ChannelInfo{IsMultiKey: true},
	}
	selected, acquired := AcquireBestChannelKey(channel, 0)
	require.True(t, acquired)
	require.Equal(t, 1, selected)

	ReleaseChannelKey(channel.Id, selected, true, 0, 0)
	inflight, _, _ := ChannelKeyLoad(channel.Id, selected)
	require.Zero(t, inflight)
}
