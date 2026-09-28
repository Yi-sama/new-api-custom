package service

import (
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

type ChannelRatioRange struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
}

// Group ranges describe enabled channels; they never replace billing configuration.
func GetChannelRatioRanges(userGroup string) (map[string]ChannelRatioRange, error) {
	channels, err := model.GetEnabledChannelsForRatio()
	if err != nil {
		return nil, err
	}
	return aggregateChannelRatioRanges(channels, userGroup), nil
}

func aggregateChannelRatioRanges(channels []model.Channel, userGroup string) map[string]ChannelRatioRange {
	ranges := make(map[string]ChannelRatioRange)
	for _, channel := range channels {
		for _, group := range channel.GetGroups() {
			var ratio float64
			if channel.ChannelRatio != nil {
				ratio = *channel.ChannelRatio
			} else {
				ratio = ratio_setting.GetGroupRatio(group)
			}
			if special, ok := ratio_setting.GetGroupGroupRatio(userGroup, group); ok {
				ratio = special
			}
			ratio += ratio_setting.GetGroupRatioAddition(group)
			r, exists := ranges[group]
			if !exists {
				r = ChannelRatioRange{Min: ratio, Max: ratio}
			} else {
				r.Min = min(r.Min, ratio)
				r.Max = max(r.Max, ratio)
			}
			ranges[group] = r
		}
	}
	return ranges
}
