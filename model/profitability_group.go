package model

import (
	"math"

	"github.com/QuantumNous/new-api/common"
)

type ProfitabilityGroupComparison struct {
	ProfitabilityCallComparison
	BreakEvenRatio  *float64 `json:"break_even_ratio"`
	BreakEvenStatus string   `json:"break_even_status"`
}

type profitabilityGroupTotals struct {
	all       profitabilityGroupCallTotals
	customers profitabilityGroupCallTotals
}

type profitabilityGroupCallTotals struct {
	chargeUsd   float64
	subUsageUsd float64
	ratioBase   float64
	requests    int64
	unmatched   int64
	ratioStatus string
}

func (totals *profitabilityGroupCallTotals) add(call *Log, income, subCost float64, matched bool) {
	totals.requests++
	totals.chargeUsd += quotaToUsd(int64(call.Quota))
	if matched {
		totals.subUsageUsd += subCost
	} else {
		totals.unmatched++
	}
	base, status := profitabilityHistoricalRatioBase(call, income)
	totals.ratioBase += base
	if status != "complete" {
		totals.ratioStatus = status
	}
}

// Reconstruct a rate-one charge from the applied rate, not today's configuration.
func profitabilityHistoricalRatioBase(call *Log, income float64) (float64, string) {
	var pricing struct {
		GroupRatio     *float64 `json:"group_ratio"`
		EffectiveRatio *float64 `json:"effective_ratio"`
		ChannelRatio   *float64 `json:"channel_ratio"`
		UserGroupRatio *float64 `json:"user_group_ratio"`
	}
	if err := common.UnmarshalJsonStr(call.Other, &pricing); err != nil {
		return 0, "missing_ratio"
	}
	if pricing.ChannelRatio != nil || (pricing.UserGroupRatio != nil && *pricing.UserGroupRatio >= 0) {
		return 0, "ratio_override"
	}
	ratio := pricing.EffectiveRatio
	if ratio == nil {
		ratio = pricing.GroupRatio
	}
	if ratio == nil || *ratio <= 0 || math.IsNaN(*ratio) || math.IsInf(*ratio, 0) || income < 0 {
		return 0, "missing_ratio"
	}
	base := income / *ratio
	if math.IsNaN(base) || math.IsInf(base, 0) {
		return 0, "missing_ratio"
	}
	return base, "complete"
}

func buildProfitabilityGroupComparison(report *ProfitabilityReport, totals profitabilityGroupCallTotals, cost float64, allocationStatus string, customerOnly bool) ProfitabilityGroupComparison {
	result := ProfitabilityGroupComparison{
		ProfitabilityCallComparison: ProfitabilityCallComparison{ChargeUsd: totals.chargeUsd},
	}
	switch {
	case report.RevenueBasis == ProfitabilityRevenueBasisCash:
		result.Status = "cash_scope"
	case report.SubUsageStatus == "not_configured":
		result.Status = "not_configured"
	case report.SubUsageStatus != "connected":
		result.Status = "sync_error"
	case totals.requests == 0:
		result.Status = "no_activity"
	case totals.unmatched > 0:
		result.Status = "unmatched"
	case report.UnmatchedSubRequests > 0:
		result.Status = "unverified_scope"
	default:
		// Known calls in unselected channels do not change this group's denominator.
		result.SubUsageUsd = &totals.subUsageUsd
		result.Status = "no_usage"
		if totals.subUsageUsd > 0 {
			ratio := totals.chargeUsd / totals.subUsageUsd
			result.EffectiveRatio = &ratio
			result.Status = "complete"
		}
	}
	result.BreakEvenStatus = result.Status
	if result.Status != "complete" {
		return result
	}
	if allocationStatus != "complete" {
		result.BreakEvenStatus = allocationStatus
		return result
	}
	if totals.ratioStatus != "" {
		result.BreakEvenStatus = totals.ratioStatus
		return result
	}
	if totals.ratioBase <= 0 || math.IsInf(totals.ratioBase, 0) {
		result.BreakEvenStatus = "no_usage"
		if customerOnly {
			result.BreakEvenStatus = "no_paid_usage"
		}
		return result
	}
	ratio := cost / totals.ratioBase
	if math.IsNaN(ratio) || math.IsInf(ratio, 0) {
		result.BreakEvenStatus = "missing_ratio"
		return result
	}
	result.BreakEvenRatio = &ratio
	return result
}
