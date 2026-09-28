package model

import (
	"sort"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
)

// ChannelRuntimeSnapshot is intentionally runtime-only. It is not persisted in
// the channels table because inflight state becomes invalid when a process dies.
type ChannelRuntimeSnapshot struct {
	ChannelID       int     `json:"channel_id"`
	KeyIndex        int     `json:"key_index"`
	Inflight        int     `json:"inflight"`
	TotalRequests   uint64  `json:"total_requests"`
	SuccessRequests uint64  `json:"success_requests"`
	FailureRequests uint64  `json:"failure_requests"`
	AvgFirstMs      float64 `json:"avg_first_response_ms"`
	AvgTotalMs      float64 `json:"avg_total_ms"`
	Available       bool    `json:"available"`
}

type channelKeyRuntime struct {
	inflight        int
	totalRequests   uint64
	successRequests uint64
	failureRequests uint64
	avgFirstMs      float64
	avgTotalMs      float64
}

var channelRuntime = struct {
	sync.Mutex
	keys map[[2]int]*channelKeyRuntime
}{keys: make(map[[2]int]*channelKeyRuntime)}

func channelKeyRuntimeFor(channelID, keyIndex int) *channelKeyRuntime {
	key := [2]int{channelID, keyIndex}
	state := channelRuntime.keys[key]
	if state == nil {
		state = &channelKeyRuntime{}
		channelRuntime.keys[key] = state
	}
	return state
}

// ChannelKeyLoad returns the current request count and smoothed latency used by
// channel/key selection. It is deliberately cheap and process-local.
func ChannelKeyLoad(channelID, keyIndex int) (inflight int, avgFirstMs, avgTotalMs float64) {
	channelRuntime.Lock()
	defer channelRuntime.Unlock()
	state := channelKeyRuntimeFor(channelID, keyIndex)
	return state.inflight, state.avgFirstMs, state.avgTotalMs
}

// HasAvailableChannelKey reports whether at least one key can accept a request.
// A zero limit means unlimited, preserving existing deployments by default.
func HasAvailableChannelKey(channel *Channel) bool {
	if channel == nil {
		return false
	}
	keys := channel.GetKeys()
	if !channel.ChannelInfo.IsMultiKey {
		keys = []string{channel.Key}
	}
	if len(keys) == 0 {
		return false
	}
	for i := range keys {
		if channelKeyEnabled(channel, i) && channelKeyAvailable(channel.Id, i) {
			return true
		}
	}
	return false
}

func channelKeyEnabled(channel *Channel, index int) bool {
	if channel == nil || index < 0 {
		return false
	}
	if channel.ChannelInfo.MultiKeyStatusList == nil {
		return true
	}
	status, exists := channel.ChannelInfo.MultiKeyStatusList[index]
	return !exists || status == common.ChannelStatusEnabled
}

func channelKeyAvailable(channelID, keyIndex int) bool {
	limit := channelKeyConcurrencyLimit()
	if limit <= 0 {
		return true
	}
	inflight, _, _ := ChannelKeyLoad(channelID, keyIndex)
	return inflight < limit
}

func channelKeyConcurrencyLimit() int {
	return common.RelayChannelKeyMaxConcurrency
}

// ChannelRuntimeScore is a lower-is-better snapshot used as a small correction
// to the existing priority/weight policy. It deliberately favors low inflight
// keys while retaining the configured priority and weight decisions upstream.
func ChannelRuntimeScore(channel *Channel) float64 {
	if channel == nil {
		return 1
	}
	keys := channel.GetKeys()
	if !channel.ChannelInfo.IsMultiKey {
		keys = []string{channel.Key}
	}
	best := 0.0
	for i := range keys {
		if !channelKeyEnabled(channel, i) {
			continue
		}
		inflight, first, total := ChannelKeyLoad(channel.Id, i)
		score := 1 + float64(inflight)
		if first > 0 {
			score += first / 1000
		}
		if total > 0 {
			score += total / 5000
		}
		if best == 0 || score < best {
			best = score
		}
	}
	if best == 0 {
		return 1e9
	}
	return best
}

// AcquireChannelKey reserves one request slot. The default limit is unlimited;
// deployments can add a limit later without changing the public channel schema.
func AcquireChannelKey(channelID, keyIndex int) bool {
	channelRuntime.Lock()
	defer channelRuntime.Unlock()
	state := channelKeyRuntimeFor(channelID, keyIndex)
	limit := channelKeyConcurrencyLimit()
	if limit > 0 && state.inflight >= limit {
		return false
	}
	state.inflight++
	state.totalRequests++
	return true
}

// AcquireBestChannelKey tries the preferred key first, then chooses the least
// loaded enabled key. Selection and reservation happen under one lock so two
// parallel requests cannot reserve the same observed slot accidentally.
func AcquireBestChannelKey(channel *Channel, preferred int) (int, bool) {
	if channel == nil {
		return -1, false
	}
	keys := channel.GetKeys()
	if !channel.ChannelInfo.IsMultiKey {
		keys = []string{channel.Key}
	}
	if len(keys) == 0 {
		return -1, false
	}
	channelRuntime.Lock()
	defer channelRuntime.Unlock()
	limit := channelKeyConcurrencyLimit()
	best := -1
	bestInflight := int(^uint(0) >> 1)
	for i := range keys {
		if !channelKeyEnabled(channel, i) {
			continue
		}
		state := channelKeyRuntimeFor(channel.Id, i)
		if limit > 0 && state.inflight >= limit {
			continue
		}
		if state.inflight < bestInflight || (state.inflight == bestInflight && i == preferred) {
			best = i
			bestInflight = state.inflight
		}
	}
	if best < 0 {
		return -1, false
	}
	channelKeyRuntimeFor(channel.Id, best).inflight++
	channelKeyRuntimeFor(channel.Id, best).totalRequests++
	return best, true
}

func ReleaseChannelKey(channelID, keyIndex int, success bool, firstResponse, total time.Duration) {
	channelRuntime.Lock()
	defer channelRuntime.Unlock()
	state := channelKeyRuntimeFor(channelID, keyIndex)
	if state.inflight > 0 {
		state.inflight--
	}
	if success {
		state.successRequests++
	} else {
		state.failureRequests++
	}
	state.avgFirstMs = updateAverage(state.avgFirstMs, firstResponse)
	state.avgTotalMs = updateAverage(state.avgTotalMs, total)
}

func updateAverage(previous float64, duration time.Duration) float64 {
	if duration <= 0 {
		return previous
	}
	value := float64(duration.Microseconds()) / 1000
	if previous == 0 {
		return value
	}
	return previous*0.8 + value*0.2
}

// GetChannelRuntimeSnapshots is used by the admin diagnostics endpoint.
func GetChannelRuntimeSnapshots() []ChannelRuntimeSnapshot {
	channelRuntime.Lock()
	defer channelRuntime.Unlock()
	result := make([]ChannelRuntimeSnapshot, 0, len(channelRuntime.keys))
	for key, state := range channelRuntime.keys {
		result = append(result, ChannelRuntimeSnapshot{
			ChannelID:       key[0],
			KeyIndex:        key[1],
			Inflight:        state.inflight,
			TotalRequests:   state.totalRequests,
			SuccessRequests: state.successRequests,
			FailureRequests: state.failureRequests,
			AvgFirstMs:      state.avgFirstMs,
			AvgTotalMs:      state.avgTotalMs,
			Available:       channelKeyConcurrencyLimit() <= 0 || state.inflight < channelKeyConcurrencyLimit(),
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].ChannelID != result[j].ChannelID {
			return result[i].ChannelID < result[j].ChannelID
		}
		return result[i].KeyIndex < result[j].KeyIndex
	})
	return result
}

func chooseLeastLoadedKey(channelID int, enabled []int) int {
	if len(enabled) == 0 {
		return -1
	}
	best := enabled[0]
	bestInflight, bestFirst, bestTotal := ChannelKeyLoad(channelID, best)
	for _, index := range enabled[1:] {
		inflight, first, total := ChannelKeyLoad(channelID, index)
		if inflight < bestInflight || (inflight == bestInflight && (first < bestFirst || (first == bestFirst && total < bestTotal))) {
			best, bestInflight, bestFirst, bestTotal = index, inflight, first, total
		}
	}
	return best
}
