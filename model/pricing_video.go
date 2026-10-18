package model

import (
	"math"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

type VideoResolutionPrice struct {
	Resolution     string  `json:"resolution"`
	PricePerSecond float64 `json:"price_per_second"`
}

type VideoPricing struct {
	Currency          string                 `json:"currency"`
	Unit              string                 `json:"unit"`
	DurationSeconds   []int                  `json:"duration_seconds"`
	DefaultResolution string                 `json:"default_resolution"`
	Prices            []VideoResolutionPrice `json:"prices"`
}

// Read the same live ModelPrice entries used by the video adaptor, not description text.
func buildVideoPricing(modelName string, meta *Model) *VideoPricing {
	var parameters struct {
		Video struct {
			Currency    string   `json:"currency"`
			BillingUnit string   `json:"billing_unit"`
			Durations   []int    `json:"durations"`
			Resolutions []string `json:"resolutions"`
		} `json:"video"`
	}
	if common.UnmarshalJsonStr(meta.Parameters, &parameters) != nil {
		return nil
	}
	profile := parameters.Video
	if profile.Currency != "CNY" || profile.BillingUnit != "second" || len(profile.Durations) == 0 {
		return nil
	}
	for _, seconds := range profile.Durations {
		if seconds < 1 || seconds > 30 {
			return nil
		}
	}
	resolutions := profile.Resolutions
	if len(resolutions) == 0 {
		resolutions = []string{""}
	}
	pricing := &VideoPricing{
		Currency: profile.Currency, Unit: profile.BillingUnit,
		DurationSeconds: profile.Durations, DefaultResolution: resolutions[0],
	}
	for _, resolution := range resolutions {
		price, found := ratio_setting.GetModelPrice(modelName+"-"+resolution, false)
		if !found && len(resolutions) == 1 {
			price, found = ratio_setting.GetModelPrice(modelName, false)
		}
		if !found || price <= 0 || math.IsNaN(price) || math.IsInf(price, 0) {
			return nil
		}
		pricing.Prices = append(pricing.Prices, VideoResolutionPrice{
			Resolution: resolution, PricePerSecond: price,
		})
	}
	return pricing
}
