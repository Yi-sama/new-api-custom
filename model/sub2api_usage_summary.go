package model

import (
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

type sub2APIUsageSummary struct {
	poolUsage    float64
	channelUsage map[int]float64
	channelKeys  map[int][]int64
	keyUsage     map[int64]float64
	keyChannels  map[int64]map[int]bool
	subChannels  map[int]bool
}

func readSub2APIAdmin(client *http.Client, base, key, path string, query url.Values, target any) error {
	request, err := http.NewRequest(http.MethodGet, base+path+"?"+query.Encode(), nil)
	if err != nil {
		return fmt.Errorf("invalid Sub2API request")
	}
	request.Header.Set("x-api-key", key)
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("Sub2API summary request failed")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("Sub2API returned HTTP %d", response.StatusCode)
	}
	if err := common.DecodeJson(response.Body, target); err != nil {
		return fmt.Errorf("invalid Sub2API summary response")
	}
	return nil
}

func readSub2APIAdminItems[T any](client *http.Client, base, key, path string) ([]T, error) {
	var result []T
	for page := 1; page <= 100; page++ {
		var payload struct {
			Code *int `json:"code"`
			Data *struct {
				Items []T `json:"items"`
				Pages int `json:"pages"`
			} `json:"data"`
		}
		query := url.Values{"page": {fmt.Sprint(page)}, "page_size": {"1000"}}
		if err := readSub2APIAdmin(client, base, key, path, query, &payload); err != nil {
			return nil, err
		}
		if payload.Code == nil || *payload.Code != 0 || payload.Data == nil {
			return nil, fmt.Errorf("invalid Sub2API metadata response")
		}
		result = append(result, payload.Data.Items...)
		if page >= payload.Data.Pages {
			return result, nil
		}
	}
	return nil, fmt.Errorf("Sub2API metadata exceeds the pagination limit")
}

func getSub2APIUsageSummary(start, end int64, channels []Channel) (sub2APIUsageSummary, string, string) {
	result := sub2APIUsageSummary{
		channelUsage: make(map[int]float64), channelKeys: make(map[int][]int64),
		keyUsage: make(map[int64]float64), keyChannels: make(map[int64]map[int]bool),
		subChannels: make(map[int]bool),
	}
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("SUB2API_BASE_URL")), "/")
	key := strings.TrimSpace(os.Getenv("SUB2API_ADMIN_API_KEY"))
	if base == "" || key == "" {
		return result, "not_configured", "Sub2API connection is not configured"
	}
	zone := strings.TrimSpace(os.Getenv("TZ"))
	if zone == "" {
		zone = "UTC"
	}
	location, err := time.LoadLocation(zone)
	if err != nil {
		return result, "error", "invalid Sub2API summary timezone"
	}
	from := time.Unix(start, 0).In(location)
	until := time.Now().In(location)
	if end > 0 {
		until = time.Unix(end, 0).In(location)
	}
	// Sub's native summaries accept calendar dates, not arbitrary timestamps.
	if start > 0 && (from.Hour() != 0 || from.Minute() != 0 || from.Second() != 0) {
		return result, "error", "Sub2API summaries require whole-day ranges"
	}
	if end > 0 && (until.Hour() != 23 || until.Minute() != 59 || until.Second() != 59) {
		return result, "error", "Sub2API summaries require whole-day ranges"
	}
	query := url.Values{
		"timezone": {zone}, "start_date": {from.Format("2006-01-02")},
		"end_date": {until.Format("2006-01-02")}, "nocache": {"true"},
	}
	client := &http.Client{Timeout: 8 * time.Second}
	readCost := func(apiKeyID int64) (float64, error) {
		if apiKeyID > 0 {
			query.Set("api_key_id", fmt.Sprint(apiKeyID))
		} else {
			query.Del("api_key_id")
		}
		var payload struct {
			Code *int `json:"code"`
			Data *struct {
				AccountCost *float64 `json:"total_account_cost"`
			} `json:"data"`
		}
		if err := readSub2APIAdmin(client, base, key, "/api/v1/admin/usage/stats", query, &payload); err != nil {
			return 0, err
		}
		if payload.Code == nil || *payload.Code != 0 || payload.Data == nil || payload.Data.AccountCost == nil {
			return 0, fmt.Errorf("Sub2API account usage is unavailable")
		}
		cost := *payload.Data.AccountCost
		if cost < 0 || math.IsNaN(cost) || math.IsInf(cost, 0) {
			return 0, fmt.Errorf("invalid Sub2API account usage")
		}
		return cost, nil
	}
	result.poolUsage, err = readCost(0)
	if err != nil {
		return result, "error", err.Error()
	}
	wanted := make(map[string]bool)
	for _, channel := range channels {
		if channel.BaseURL == nil || strings.TrimRight(strings.TrimSpace(*channel.BaseURL), "/") != base {
			continue
		}
		result.subChannels[channel.Id] = true
		for _, value := range strings.Split(channel.Key, "\n") {
			if value = strings.TrimSpace(value); value != "" {
				wanted[value] = true
			}
		}
	}
	if len(wanted) == 0 {
		return result, "connected", ""
	}
	groups, err := readSub2APIAdminItems[struct {
		ID int64 `json:"id"`
	}](client, base, key, "/api/v1/admin/groups")
	if err != nil {
		return result, "error", err.Error()
	}
	keyIDs := make(map[string]int64)
	for _, group := range groups {
		keys, err := readSub2APIAdminItems[struct {
			ID  int64  `json:"id"`
			Key string `json:"key"`
		}](client, base, key, fmt.Sprintf("/api/v1/admin/groups/%d/api-keys", group.ID))
		if err != nil {
			return result, "error", err.Error()
		}
		for _, item := range keys {
			if !wanted[item.Key] {
				continue
			}
			keyIDs[item.Key] = item.ID
			if _, exists := result.keyUsage[item.ID]; !exists {
				cost, err := readCost(item.ID)
				if err != nil {
					return result, "error", err.Error()
				}
				result.keyUsage[item.ID] = cost
			}
		}
	}
	for _, channel := range channels {
		if !result.subChannels[channel.Id] {
			continue
		}
		seen := make(map[int64]bool)
		complete := true
		for _, value := range strings.Split(channel.Key, "\n") {
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			id, exists := keyIDs[value]
			if !exists {
				complete = false
				break
			}
			if !seen[id] {
				seen[id] = true
				result.channelKeys[channel.Id] = append(result.channelKeys[channel.Id], id)
				if result.keyChannels[id] == nil {
					result.keyChannels[id] = make(map[int]bool)
				}
				result.keyChannels[id][channel.Id] = true
			}
		}
		if !complete || len(seen) == 0 {
			delete(result.channelKeys, channel.Id)
			continue
		}
		for id := range seen {
			if len(result.keyChannels[id]) != 1 {
				complete = false
				break
			}
		}
		if !complete {
			delete(result.channelKeys, channel.Id)
			continue
		}
		for id := range seen {
			result.channelUsage[channel.Id] += result.keyUsage[id]
		}
	}
	for id, channelIDs := range result.keyChannels {
		if len(channelIDs) <= 1 {
			continue
		}
		for channelID := range channelIDs {
			delete(result.channelKeys, channelID)
			delete(result.channelUsage, channelID)
		}
		delete(result.keyChannels, id)
	}
	return result, "connected", ""
}

func applyProfitabilityNativeGroups(report *ProfitabilityReport, summary sub2APIUsageSummary, calls []Log, start int64, channelSelected func(int) bool) {
	report.SubUsageUsd = 0
	report.CostRateRmbPerUsd = 0
	nativeUsageComplete := true
	selectedNativeUsage := 0.0
	mixedUpstream := false
	channelByID := make(map[int]*ProfitabilityChannel, len(report.Channels))
	for _, row := range report.Channels {
		channelByID[row.Id] = row
		row.MatchedRequests = 0
		row.UnmatchedRequests = 0
		row.SubUsageUsd = 0
		row.EstimatedProfitRmb = nil
		if !summary.subChannels[row.Id] {
			row.SubUsageStatus = "unavailable"
			if row.RequestCount > 0 {
				row.ProfitStatus = "mixed_upstream"
				mixedUpstream = true
			} else if row.PurchaseRecorded {
				row.ProfitStatus = "unallocated_purchase"
			} else {
				row.ProfitStatus = "no_activity"
			}
			continue
		}
		usage, exists := summary.channelUsage[row.Id]
		if !exists {
			row.SubUsageStatus = "unavailable"
			if row.RequestCount > 0 || row.PurchaseRecorded {
				nativeUsageComplete = false
				row.ProfitStatus = "unverified_scope"
			} else {
				row.ProfitStatus = "no_activity"
			}
			continue
		}
		row.SubUsageStatus = "complete"
		row.SubUsageUsd = usage
		report.SubUsageUsd += usage
		selectedNativeUsage += usage
		row.ProfitStatus = profitabilityChannelStatus(row, "connected")
		if row.ProfitStatus == "complete" {
			profit := row.CustomerChargeRmb - row.PurchaseRmb
			row.EstimatedProfitRmb = &profit
		}
	}
	if report.SubUsageUsd > 0 {
		report.CostRateRmbPerUsd = report.AccountCostRmb / report.SubUsageUsd
	}

	channelsByGroup := make(map[string]map[int]bool)
	groupsByKey := make(map[int64]map[string]bool)
	for _, call := range calls {
		if call.CreatedAt < start || !channelSelected(call.ChannelId) {
			continue
		}
		if channelsByGroup[call.Group] == nil {
			channelsByGroup[call.Group] = make(map[int]bool)
		}
		channelsByGroup[call.Group][call.ChannelId] = true
		for _, id := range summary.channelKeys[call.ChannelId] {
			if groupsByKey[id] == nil {
				groupsByKey[id] = make(map[string]bool)
			}
			groupsByKey[id][call.Group] = true
		}
	}
	for _, group := range report.Groups {
		comparison := &group.AllCalls
		comparison.SubUsageUsd, comparison.EffectiveRatio, comparison.BreakEvenRatio = nil, nil, nil
		group.AllocatedCostRmb = 0
		group.EstimatedProfitRmb = nil
		group.ProfitStatus = "unknown"
		group.Profitable = false
		if report.SubUsageStatus != "connected" {
			continue
		}
		seen := make(map[int64]bool)
		mixed, incomplete := false, false
		groupUsageByChannel := make(map[int]float64)
		for channelID := range channelsByGroup[group.Name] {
			if !summary.subChannels[channelID] {
				mixed = true
				continue
			}
			if _, exists := summary.channelUsage[channelID]; !exists {
				incomplete = true
			}
			for _, id := range summary.channelKeys[channelID] {
				seen[id] = true
				groupUsageByChannel[channelID] += summary.keyUsage[id]
				if len(groupsByKey[id]) > 1 {
					incomplete = true
				}
			}
		}
		comparison.Status = "unverified_scope"
		if mixed && len(seen) == 0 {
			comparison.Status = "mixed_upstream"
		} else if !incomplete && len(seen) > 0 {
			cost := 0.0
			for id := range seen {
				cost += summary.keyUsage[id]
			}
			comparison.SubUsageUsd = &cost
			comparison.Status = "no_usage"
			if cost > 0 {
				comparison.Status = "complete"
				ratio := comparison.ChargeUsd / cost
				comparison.EffectiveRatio = &ratio
			}
			if mixed {
				comparison.Status = "mixed_upstream"
				comparison.EffectiveRatio = nil
			}
		}
		if comparison.Status == "complete" {
			for channelID, usage := range groupUsageByChannel {
				channel := channelByID[channelID]
				if channel != nil && channel.SubUsageUsd > 0 {
					group.AllocatedCostRmb += channel.PurchaseRmb * usage / channel.SubUsageUsd
				}
			}
		}
		if report.RevenueBasis == ProfitabilityRevenueBasisCash {
			comparison.Status = "cash_scope"
			comparison.EffectiveRatio = nil
		}
		comparison.BreakEvenStatus = comparison.Status
		if comparison.Status == "complete" {
			comparison.BreakEvenStatus = "unallocated_purchase"
			if report.AccountPurchaseCount == 0 {
				comparison.BreakEvenStatus = "missing_purchase"
			}
		}
		group.SubUsageUsd = 0
		if comparison.SubUsageUsd != nil {
			group.SubUsageUsd = *comparison.SubUsageUsd
		}
		group.CustomerCalls.SubUsageUsd, group.CustomerCalls.EffectiveRatio, group.CustomerCalls.BreakEvenRatio = nil, nil, nil
		group.CustomerCalls.Status, group.CustomerCalls.BreakEvenStatus = "unmatched", "unmatched"
		group.BreakEvenRatio = nil
	}
	poolCoverage := math.Abs(selectedNativeUsage-summary.poolUsage) < 0.0000001
	report.Pool.AllCalls.SubUsageUsd = nil
	report.Pool.AllCalls.EffectiveRatio = nil
	report.Pool.CustomerCalls.SubUsageUsd, report.Pool.CustomerCalls.EffectiveRatio = nil, nil
	report.Pool.CustomerCalls.Status = "unmatched"
	if report.SubUsageStatus == "connected" {
		report.Pool.AllCalls.SubUsageUsd = &summary.poolUsage
		switch {
		case report.RevenueBasis == ProfitabilityRevenueBasisCash:
			report.Pool.AllCalls.Status = "cash_scope"
		case summary.poolUsage == 0:
			report.Pool.AllCalls.Status = "no_usage"
		case !nativeUsageComplete || !poolCoverage:
			report.Pool.AllCalls.Status = "scope_mismatch"
		case mixedUpstream:
			report.Pool.AllCalls.Status = "mixed_upstream"
		default:
			report.Pool.AllCalls.Status = "complete"
			ratio := report.Pool.AllCalls.ChargeUsd / summary.poolUsage
			report.Pool.AllCalls.EffectiveRatio = &ratio
		}
	}
	report.ProfitStatus = profitabilityReportStatus(report, ProfitabilityReportOptions{
		RevenueBasis:            report.RevenueBasis,
		ChannelSelectionPresent: report.Scope == "selected",
		ChannelIDs:              report.SelectedChannelIds,
	})
	if mixedUpstream {
		report.ProfitStatus = "mixed_upstream"
	}
	if !nativeUsageComplete && report.ProfitStatus == "complete" {
		report.ProfitStatus = "unverified_scope"
	}
	if report.ProfitStatus == "complete" {
		profit := report.CallChargeRmb - report.AccountCostRmb
		if report.RevenueBasis == ProfitabilityRevenueBasisCash {
			profit = report.CashRecognizedRmb - report.AccountCostRmb
		}
		report.EstimatedProfitRmb = &profit
		cashProfit := report.CashRecognizedRmb - report.AccountCostRmb
		report.CashProfitRmb = &cashProfit
	} else {
		report.EstimatedProfitRmb = nil
		report.CashProfitRmb = nil
	}
}
