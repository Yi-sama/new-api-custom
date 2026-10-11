package model

import (
	"errors"
	"fmt"
	"math"
	"sort"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const (
	ProfitabilityEntryAccountPurchase = "account_purchase"
	ProfitabilityEntryManualIncome    = "manual_income"
	ProfitabilityEntryAdminAdjustment = "admin_adjustment"
	ProfitabilityRevenueBasisCalls    = "calls"
	ProfitabilityRevenueBasisCash     = "cash"
)

type ProfitabilityReportOptions struct {
	RevenueBasis            string
	ChannelIDs              []int
	ChannelSelectionPresent bool
	UseSubSummary           bool
}

type ProfitabilityAvailableChannel struct {
	Id     int    `json:"id"`
	Name   string `json:"name"`
	Status int    `json:"status"`
}

type ProfitabilityEntry struct {
	Id             int    `json:"id"`
	EntryType      string `json:"entry_type" gorm:"index;type:varchar(32)"`
	AmountCents    int64  `json:"amount_cents"`
	AmountRmbCents int64  `json:"amount_rmb_cents" gorm:"default:0"`
	AccountCount   int    `json:"account_count" gorm:"default:0"`
	ChannelId      int    `json:"channel_id" gorm:"index;default:0"`
	Quota          int64  `json:"quota" gorm:"default:0"`
	UserId         int    `json:"user_id" gorm:"index;default:0"`
	Username       string `json:"username" gorm:"index;type:varchar(128);default:''"`
	Source         string `json:"source" gorm:"type:varchar(32);default:''"`
	SourceId       string `json:"source_id" gorm:"index;type:varchar(128);default:''"`
	Note           string `json:"note" gorm:"type:text"`
	OccurredAt     int64  `json:"occurred_at" gorm:"index"`
	CreatedBy      int    `json:"created_by" gorm:"index;default:0"`
	CreatedAt      int64  `json:"created_at" gorm:"index"`
}

func (entry *ProfitabilityEntry) Insert() error {
	return DB.Create(entry).Error
}

type ProfitabilityEntryView struct {
	Id           int     `json:"id"`
	EntryType    string  `json:"entry_type"`
	AmountUsd    float64 `json:"amount_usd"`
	AmountRmb    float64 `json:"amount_rmb"`
	AccountCount int     `json:"account_count"`
	ChannelId    int     `json:"channel_id"`
	Quota        int64   `json:"quota"`
	UserId       int     `json:"user_id"`
	Username     string  `json:"username"`
	Source       string  `json:"source"`
	SourceId     string  `json:"source_id"`
	Note         string  `json:"note"`
	OccurredAt   int64   `json:"occurred_at"`
}

type ProfitabilityRevenueItem struct {
	Id         int     `json:"id"`
	Source     string  `json:"source"`
	SourceId   string  `json:"source_id"`
	AmountUsd  float64 `json:"amount_usd"`
	UserId     int     `json:"user_id"`
	Username   string  `json:"username"`
	Note       string  `json:"note"`
	OccurredAt int64   `json:"occurred_at"`
}

type ProfitabilityUsageUser struct {
	UserId       int     `json:"user_id"`
	Username     string  `json:"username"`
	Quota        int64   `json:"quota"`
	AmountUsd    float64 `json:"amount_usd"`
	RequestCount int64   `json:"request_count"`
	TokenCount   int64   `json:"token_count"`
}

type ProfitabilityUsageSummary struct {
	Quota        int64                     `json:"quota"`
	AmountUsd    float64                   `json:"amount_usd"`
	CustomerUsd  float64                   `json:"customer_usd"`
	SelfUsd      float64                   `json:"self_usd"`
	RequestCount int64                     `json:"request_count"`
	TokenCount   int64                     `json:"token_count"`
	Users        []*ProfitabilityUsageUser `json:"users"`
	Groups       []*ProfitabilityGroup     `json:"groups"`
}

type ProfitabilityGroup struct {
	ConfiguredRatio    *float64                     `json:"configured_ratio"`
	AllCalls           ProfitabilityGroupComparison `json:"all_calls"`
	CustomerCalls      ProfitabilityGroupComparison `json:"customer_calls"`
	Name               string                       `json:"name"`
	Ratio              float64                      `json:"ratio"`
	CustomerUsd        float64                      `json:"customer_usd"`
	SelfUsd            float64                      `json:"self_usd"`
	SubUsageUsd        float64                      `json:"sub_usage_usd"`
	AllocatedCostRmb   float64                      `json:"allocated_cost_rmb"`
	CustomerChargeRmb  float64                      `json:"customer_charge_rmb"`
	EstimatedProfitRmb *float64                     `json:"estimated_profit_rmb"`
	BreakEvenRatio     *float64                     `json:"break_even_ratio"`
	Profitable         bool                         `json:"profitable"`
	ProfitStatus       string                       `json:"profit_status"`
}

type ProfitabilityChannel struct {
	SubUsageStatus     string   `json:"sub_usage_status,omitempty"`
	RequestCount       int64    `json:"request_count"`
	Id                 int      `json:"id"`
	Name               string   `json:"name"`
	PurchaseRmb        float64  `json:"purchase_rmb"`
	SubUsageUsd        float64  `json:"sub_usage_usd"`
	CustomerChargeRmb  float64  `json:"customer_charge_rmb"`
	EstimatedProfitRmb *float64 `json:"estimated_profit_rmb"`
	MatchedRequests    int64    `json:"matched_requests"`
	UnmatchedRequests  int64    `json:"unmatched_requests"`
	Status             int      `json:"status"`
	ProfitStatus       string   `json:"profit_status"`
	PurchaseRecorded   bool     `json:"purchase_recorded"`
}

type ProfitabilityModel struct {
	ChannelId          int      `json:"channel_id"`
	Name               string   `json:"name"`
	SubUsageUsd        float64  `json:"sub_usage_usd"`
	CustomerChargeRmb  float64  `json:"customer_charge_rmb"`
	AllocatedCostRmb   float64  `json:"allocated_cost_rmb"`
	EstimatedProfitRmb *float64 `json:"estimated_profit_rmb"`
}

type ProfitabilityAccount struct {
	ChannelId   int     `json:"channel_id"`
	AccountId   int64   `json:"account_id"`
	SubUsageUsd float64 `json:"sub_usage_usd"`
}

type ProfitabilityCallComparison struct {
	ChargeUsd      float64  `json:"charge_usd"`
	SubUsageUsd    *float64 `json:"sub_usage_usd"`
	EffectiveRatio *float64 `json:"effective_ratio"`
	Status         string   `json:"status"`
}

type ProfitabilityPoolSummary struct {
	AllCalls                   ProfitabilityCallComparison `json:"all_calls"`
	CustomerCalls              ProfitabilityCallComparison `json:"customer_calls"`
	AdminChargeUsd             float64                     `json:"admin_charge_usd"`
	MatchedAdminSubUsageUsd    float64                     `json:"matched_admin_sub_usage_usd"`
	OutsideSelectedSubUsageUsd float64                     `json:"outside_selected_sub_usage_usd"`
	ProfitRmb                  *float64                    `json:"profit_rmb"`
	ProfitStatus               string                      `json:"profit_status"`
}

type ProfitabilityReport struct {
	SubUsageSource       string                          `json:"sub_usage_source"`
	Pool                 ProfitabilityPoolSummary        `json:"pool"`
	RevenueUsd           float64                         `json:"revenue_usd"`
	TopupRevenueUsd      float64                         `json:"topup_revenue_usd"`
	ManualIncomeUsd      float64                         `json:"manual_income_usd"`
	AdminAdjustmentUsd   float64                         `json:"admin_adjustment_usd"`
	AccountCostRmb       float64                         `json:"account_cost_rmb"`
	SubUsageUsd          float64                         `json:"sub_usage_usd"`
	CostRateRmbPerUsd    float64                         `json:"cost_rate_rmb_per_usd"`
	AccountCount         int64                           `json:"account_count"`
	AccountPurchaseCount int64                           `json:"account_purchase_count"`
	Usage                ProfitabilityUsageSummary       `json:"usage"`
	Groups               []*ProfitabilityGroup           `json:"groups"`
	RevenueEntries       []*ProfitabilityRevenueItem     `json:"revenue_entries"`
	CostEntries          []*ProfitabilityEntryView       `json:"cost_entries"`
	SubUsageStatus       string                          `json:"sub_usage_status"`
	SubUsageError        string                          `json:"sub_usage_error,omitempty"`
	Channels             []*ProfitabilityChannel         `json:"channels"`
	Models               []*ProfitabilityModel           `json:"models"`
	Accounts             []*ProfitabilityAccount         `json:"accounts"`
	UnmatchedSubRequests int64                           `json:"unmatched_sub_requests"`
	CashReceivedRmb      float64                         `json:"cash_received_rmb"`
	CashRecognizedRmb    float64                         `json:"cash_recognized_rmb"`
	CashProfitRmb        *float64                        `json:"cash_profit_rmb"`
	CallChargeRmb        float64                         `json:"call_charge_rmb"`
	EstimatedProfitRmb   *float64                        `json:"estimated_profit_rmb"`
	AvailableChannels    []ProfitabilityAvailableChannel `json:"available_channels"`
	RevenueBasis         string                          `json:"revenue_basis"`
	Scope                string                          `json:"scope"`
	SelectedChannelIds   []int                           `json:"selected_channel_ids"`
	ProfitStatus         string                          `json:"profit_status"`
	UnmatchedRequests    int64                           `json:"unmatched_requests"`
	UnmatchedSubUsageUsd float64                         `json:"unmatched_sub_usage_usd"`
	MatchedRequests      int64                           `json:"matched_requests"`
}

func ProfitabilityChannelExists(id int) bool {
	var count int64
	return DB.Model(&Channel{}).Where("id = ?", id).Count(&count).Error == nil && count > 0
}

func QuotaToUSDCents(quota int64) int64 {
	if quota <= 0 || common.QuotaPerUnit <= 0 {
		return 0
	}
	return decimal.NewFromInt(quota).
		Div(decimal.NewFromFloat(common.QuotaPerUnit)).
		Mul(decimal.NewFromInt(100)).
		Round(0).
		IntPart()
}

func USDCentsToUSD(cents int64) float64 {
	return float64(cents) / 100
}

func quotaToUsd(quota int64) float64 {
	if quota <= 0 || common.QuotaPerUnit <= 0 {
		return 0
	}
	return float64(quota) / common.QuotaPerUnit
}

func ValidateProfitabilityEntry(entryType string, amountCents int64, accountCount int) error {
	switch entryType {
	case ProfitabilityEntryAccountPurchase:
		if amountCents <= 0 || accountCount <= 0 {
			return errors.New("account purchase amount and count must be positive")
		}
	case ProfitabilityEntryManualIncome:
		if amountCents <= 0 {
			return errors.New("manual income amount must be positive")
		}
	default:
		return errors.New("unsupported profitability entry type")
	}
	return nil
}

func ParseUSDAmountCents(amountUsd float64) (int64, error) {
	if math.IsNaN(amountUsd) || math.IsInf(amountUsd, 0) || amountUsd <= 0 {
		return 0, errors.New("amount_usd must be a positive finite number")
	}
	cents := decimal.NewFromFloat(amountUsd).Mul(decimal.NewFromInt(100)).Round(0).IntPart()
	if cents <= 0 {
		return 0, errors.New("amount_usd is smaller than one cent")
	}
	return cents, nil
}

func ParseRMBAmountCents(amountRmb float64) (int64, error) {
	if math.IsNaN(amountRmb) || math.IsInf(amountRmb, 0) || amountRmb <= 0 {
		return 0, errors.New("amount_rmb must be a positive finite number")
	}
	cents := decimal.NewFromFloat(amountRmb).Mul(decimal.NewFromInt(100)).Round(0).IntPart()
	if cents <= 0 {
		return 0, errors.New("amount_rmb is smaller than one fen")
	}
	return cents, nil
}

func RecordProfitabilityAdminAdjustment(user *User, operatorId int, quota int64, sourceId string) error {
	if user == nil || user.Role >= common.RoleAdminUser || quota <= 0 {
		return nil
	}
	return (&ProfitabilityEntry{
		EntryType:   ProfitabilityEntryAdminAdjustment,
		AmountCents: QuotaToUSDCents(quota),
		Quota:       quota,
		UserId:      user.Id,
		Username:    user.Username,
		Source:      "admin",
		SourceId:    sourceId,
		Note:        "管理员手动加款",
		OccurredAt:  common.GetTimestamp(),
		CreatedBy:   operatorId,
		CreatedAt:   common.GetTimestamp(),
	}).Insert()
}

func GetProfitabilityEntries(startTimestamp int64, endTimestamp int64) ([]*ProfitabilityEntry, error) {
	query := DB.Model(&ProfitabilityEntry{}).Order("occurred_at desc, id desc")
	if startTimestamp > 0 {
		query = query.Where("occurred_at >= ?", startTimestamp)
	}
	if endTimestamp > 0 {
		query = query.Where("occurred_at <= ?", endTimestamp)
	}
	var entries []*ProfitabilityEntry
	if err := query.Find(&entries).Error; err != nil {
		return nil, err
	}
	return entries, nil
}

func getProfitabilityAdminUserIds() ([]int, error) {
	var userIds []int
	err := DB.Unscoped().Model(&User{}).
		Where("role >= ?", common.RoleAdminUser).
		Pluck("id", &userIds).Error
	return userIds, err
}

func topUpAmountUsd(topUp *TopUp) float64 {
	if topUp == nil {
		return 0
	}
	if topUp.Money > 0 {
		return topUp.Money
	}
	if topUp.PaymentProvider == PaymentProviderCreem && common.QuotaPerUnit > 0 {
		return float64(topUp.Amount) / common.QuotaPerUnit
	}
	return float64(topUp.Amount)
}

func getProfitabilityTopupRevenue(startTimestamp int64, endTimestamp int64, excludedUserIds []int) ([]*ProfitabilityRevenueItem, error) {
	query := DB.Model(&TopUp{}).
		Where("status = ? AND complete_time > 0", common.TopUpStatusSuccess)
	if startTimestamp > 0 {
		query = query.Where("complete_time >= ?", startTimestamp)
	}
	if endTimestamp > 0 {
		query = query.Where("complete_time <= ?", endTimestamp)
	}
	if len(excludedUserIds) > 0 {
		query = query.Where("user_id NOT IN ?", excludedUserIds)
	}

	var topUps []*TopUp
	if err := query.Order("complete_time desc, id desc").Find(&topUps).Error; err != nil {
		return nil, err
	}

	entries := make([]*ProfitabilityRevenueItem, 0, len(topUps))
	for _, topUp := range topUps {
		username, _ := GetUsernameById(topUp.UserId, false)
		entries = append(entries, &ProfitabilityRevenueItem{
			Id:         topUp.Id,
			Source:     "topup",
			SourceId:   topUp.TradeNo,
			AmountUsd:  topUpAmountUsd(topUp),
			UserId:     topUp.UserId,
			Username:   username,
			Note:       topUp.PaymentProvider,
			OccurredAt: topUp.CompleteTime,
		})
	}
	return entries, nil
}

func applyProfitabilityChannelFilter(query *gorm.DB, channelIDs []int, selectionPresent bool) *gorm.DB {
	if !selectionPresent {
		return query
	}
	if len(channelIDs) == 0 {
		return query.Where("1 = 0")
	}
	return query.Where("channel_id IN ?", channelIDs)
}

func getProfitabilityUsage(startTimestamp int64, endTimestamp int64, adminUserIds []int, costRate float64) (ProfitabilityUsageSummary, error) {
	return getProfitabilityUsageForChannels(startTimestamp, endTimestamp, adminUserIds, costRate, nil, false)
}

func getProfitabilityUsageForChannels(startTimestamp int64, endTimestamp int64, adminUserIds []int, costRate float64, channelIDs []int, selectionPresent bool) (ProfitabilityUsageSummary, error) {
	query := LOG_DB.Table("logs").
		Select("COALESCE(SUM(quota), 0) AS quota, COUNT(*) AS request_count, COALESCE(SUM(prompt_tokens), 0) + COALESCE(SUM(completion_tokens), 0) AS token_count").
		Where("type = ?", LogTypeConsume)
	query = applyProfitabilityChannelFilter(query, channelIDs, selectionPresent)
	if startTimestamp > 0 {
		query = query.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp > 0 {
		query = query.Where("created_at <= ?", endTimestamp)
	}
	var total struct {
		Quota        int64 `gorm:"column:quota"`
		RequestCount int64 `gorm:"column:request_count"`
		TokenCount   int64 `gorm:"column:token_count"`
	}
	if err := query.Scan(&total).Error; err != nil {
		return ProfitabilityUsageSummary{}, err
	}

	amountUsd := quotaToUsd(total.Quota)

	customerQuery := LOG_DB.Table("logs").
		Select("COALESCE(SUM(quota), 0)").
		Where("type = ?", LogTypeConsume)
	customerQuery = applyProfitabilityChannelFilter(customerQuery, channelIDs, selectionPresent)
	if startTimestamp > 0 {
		customerQuery = customerQuery.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp > 0 {
		customerQuery = customerQuery.Where("created_at <= ?", endTimestamp)
	}
	if len(adminUserIds) > 0 {
		customerQuery = customerQuery.Where("user_id NOT IN ?", adminUserIds)
	}
	var customerQuota int64
	if err := customerQuery.Scan(&customerQuota).Error; err != nil {
		return ProfitabilityUsageSummary{}, err
	}

	selfQuery := LOG_DB.Table("logs").
		Select("COALESCE(SUM(quota), 0)").
		Where("type = ?", LogTypeConsume)
	selfQuery = applyProfitabilityChannelFilter(selfQuery, channelIDs, selectionPresent)
	if startTimestamp > 0 {
		selfQuery = selfQuery.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp > 0 {
		selfQuery = selfQuery.Where("created_at <= ?", endTimestamp)
	}
	if len(adminUserIds) > 0 {
		selfQuery = selfQuery.Where("user_id IN ?", adminUserIds)
	} else {
		selfQuery = selfQuery.Where("1 = 0")
	}
	var selfQuota int64
	if err := selfQuery.Scan(&selfQuota).Error; err != nil {
		return ProfitabilityUsageSummary{}, err
	}

	userQuery := LOG_DB.Table("logs").
		Select("user_id, username, COALESCE(SUM(quota), 0) AS quota, COUNT(*) AS request_count, COALESCE(SUM(prompt_tokens), 0) + COALESCE(SUM(completion_tokens), 0) AS token_count").
		Where("type = ?", LogTypeConsume).
		Group("user_id, username").
		Order("quota desc").
		Limit(20)
	userQuery = applyProfitabilityChannelFilter(userQuery, channelIDs, selectionPresent)
	if startTimestamp > 0 {
		userQuery = userQuery.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp > 0 {
		userQuery = userQuery.Where("created_at <= ?", endTimestamp)
	}
	if len(adminUserIds) > 0 {
		userQuery = userQuery.Where("user_id NOT IN ?", adminUserIds)
	}
	users := make([]*ProfitabilityUsageUser, 0)
	if err := userQuery.Scan(&users).Error; err != nil {
		return ProfitabilityUsageSummary{}, err
	}
	for _, user := range users {
		user.AmountUsd = quotaToUsd(user.Quota)
	}

	groupRows := make([]struct {
		Name  string `gorm:"column:name"`
		Quota int64  `gorm:"column:quota"`
	}, 0)
	groupQuery := LOG_DB.Table("logs").
		Select(logGroupCol+" AS name, COALESCE(SUM(quota), 0) AS quota").
		Where("type = ?", LogTypeConsume).
		Group("group").
		Order("quota desc")
	groupQuery = applyProfitabilityChannelFilter(groupQuery, channelIDs, selectionPresent)
	if startTimestamp > 0 {
		groupQuery = groupQuery.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp > 0 {
		groupQuery = groupQuery.Where("created_at <= ?", endTimestamp)
	}
	if len(adminUserIds) > 0 {
		groupQuery = groupQuery.Where("user_id NOT IN ?", adminUserIds)
	}
	if err := groupQuery.Scan(&groupRows).Error; err != nil {
		return ProfitabilityUsageSummary{}, err
	}
	groups := make([]*ProfitabilityGroup, 0, len(groupRows))
	for _, row := range groupRows {
		groupUsd := quotaToUsd(row.Quota)
		ratio := ratio_setting.GetGroupRatio(row.Name) + ratio_setting.GetGroupRatioAddition(row.Name)
		status := "unknown"
		profitable := false
		if costRate > 0 && ratio > 0 {
			profitable = ratio > costRate
			status = "loss"
			if profitable {
				status = "profit"
			}
		}
		groups = append(groups, &ProfitabilityGroup{
			Name:         row.Name,
			Ratio:        ratio,
			CustomerUsd:  groupUsd,
			Profitable:   profitable,
			ProfitStatus: status,
		})
	}
	return ProfitabilityUsageSummary{
		Quota:        total.Quota,
		AmountUsd:    amountUsd,
		CustomerUsd:  quotaToUsd(customerQuota),
		SelfUsd:      quotaToUsd(selfQuota),
		RequestCount: total.RequestCount,
		TokenCount:   total.TokenCount,
		Users:        users,
		Groups:       groups,
	}, nil
}

func profitabilityEntryView(entry *ProfitabilityEntry) *ProfitabilityEntryView {
	return &ProfitabilityEntryView{
		Id:           entry.Id,
		EntryType:    entry.EntryType,
		AmountUsd:    USDCentsToUSD(entry.AmountCents),
		AmountRmb:    USDCentsToUSD(entry.AmountRmbCents),
		AccountCount: entry.AccountCount,
		ChannelId:    entry.ChannelId,
		Quota:        entry.Quota,
		UserId:       entry.UserId,
		Username:     entry.Username,
		Source:       entry.Source,
		SourceId:     entry.SourceId,
		Note:         entry.Note,
		OccurredAt:   entry.OccurredAt,
	}
}

func GetProfitabilityReport(startTimestamp int64, endTimestamp int64) (*ProfitabilityReport, error) {
	return GetProfitabilityReportWithOptions(startTimestamp, endTimestamp, ProfitabilityReportOptions{})
}

func GetProfitabilityReportWithOptions(startTimestamp int64, endTimestamp int64, options ProfitabilityReportOptions) (*ProfitabilityReport, error) {
	if options.RevenueBasis == "" {
		options.RevenueBasis = ProfitabilityRevenueBasisCalls
	}
	if options.RevenueBasis == ProfitabilityRevenueBasisCash {
		options.ChannelSelectionPresent = false
		options.ChannelIDs = nil
	}
	options.ChannelIDs = uniquePositiveInts(options.ChannelIDs)
	selected := make(map[int]bool, len(options.ChannelIDs))
	for _, id := range options.ChannelIDs {
		selected[id] = true
	}
	channelSelected := func(id int) bool {
		return !options.ChannelSelectionPresent || selected[id]
	}

	adminUserIds, err := getProfitabilityAdminUserIds()
	if err != nil {
		return nil, err
	}
	allEntries, err := GetProfitabilityEntries(startTimestamp, endTimestamp)
	if err != nil {
		return nil, err
	}
	topupEntries, err := getProfitabilityTopupRevenue(startTimestamp, endTimestamp, adminUserIds)
	if err != nil {
		return nil, err
	}
	usage, err := getProfitabilityUsageForChannels(startTimestamp, endTimestamp, adminUserIds, 0, options.ChannelIDs, options.ChannelSelectionPresent)
	if err != nil {
		return nil, err
	}

	report := &ProfitabilityReport{
		RevenueBasis:       options.RevenueBasis,
		Scope:              "all",
		SelectedChannelIds: append(make([]int, 0, len(options.ChannelIDs)), options.ChannelIDs...),
		RevenueEntries:     make([]*ProfitabilityRevenueItem, 0, len(allEntries)+len(topupEntries)),
		CostEntries:        make([]*ProfitabilityEntryView, 0, len(allEntries)),
		Usage:              usage,
		Groups:             usage.Groups,
		Channels:           make([]*ProfitabilityChannel, 0),
		Models:             make([]*ProfitabilityModel, 0),
		Accounts:           make([]*ProfitabilityAccount, 0),
	}
	if options.ChannelSelectionPresent {
		report.Scope = "selected"
	}

	entries := make([]*ProfitabilityEntry, 0, len(allEntries))
	for _, entry := range allEntries {
		if entry.EntryType == ProfitabilityEntryAccountPurchase && !channelSelected(entry.ChannelId) {
			continue
		}
		entries = append(entries, entry)
		switch entry.EntryType {
		case ProfitabilityEntryAccountPurchase:
			amount := entry.AmountRmbCents
			if amount == 0 {
				amount = entry.AmountCents
			}
			report.AccountCostRmb += USDCentsToUSD(amount)
			report.AccountPurchaseCount += int64(entry.AccountCount)
			report.AccountCount += int64(entry.AccountCount)
			report.CostEntries = append(report.CostEntries, profitabilityEntryView(entry))
		case ProfitabilityEntryManualIncome:
			amount := USDCentsToUSD(entry.AmountCents)
			report.ManualIncomeUsd += amount
			report.RevenueEntries = append(report.RevenueEntries, profitabilityRevenueItem(entry, amount))
		case ProfitabilityEntryAdminAdjustment:
			amount := USDCentsToUSD(entry.AmountCents)
			report.AdminAdjustmentUsd += amount
			report.RevenueEntries = append(report.RevenueEntries, profitabilityRevenueItem(entry, amount))
		}
	}
	for _, entry := range topupEntries {
		report.TopupRevenueUsd += entry.AmountUsd
		report.RevenueEntries = append(report.RevenueEntries, entry)
	}
	report.CashReceivedRmb = report.TopupRevenueUsd + report.ManualIncomeUsd

	if err := addProfitabilityBreakdown(report, entries, adminUserIds, startTimestamp, endTimestamp, options, channelSelected); err != nil {
		return nil, err
	}
	if options.RevenueBasis == ProfitabilityRevenueBasisCash {
		report.RevenueUsd = report.CashRecognizedRmb
	} else {
		report.RevenueUsd = report.CallChargeRmb
	}
	return report, nil
}

func profitabilityRevenueItem(entry *ProfitabilityEntry, amount float64) *ProfitabilityRevenueItem {
	return &ProfitabilityRevenueItem{
		Id: entry.Id, Source: entry.EntryType, SourceId: entry.SourceId, AmountUsd: amount,
		UserId: entry.UserId, Username: entry.Username, Note: entry.Note, OccurredAt: entry.OccurredAt,
	}
}

func uniquePositiveInts(values []int) []int {
	seen := make(map[int]bool, len(values))
	result := make([]int, 0, len(values))
	for _, value := range values {
		if value > 0 && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	sort.Ints(result)
	return result
}

func monetaryProfitStatus(value *float64) string {
	if value == nil {
		return "unknown"
	}
	if *value > 0 {
		return "profit"
	}
	if *value < 0 {
		return "loss"
	}
	return "break_even"
}

func addProfitabilityBreakdown(report *ProfitabilityReport, entries []*ProfitabilityEntry, adminUserIds []int, startTimestamp, endTimestamp int64, options ProfitabilityReportOptions, channelSelected func(int) bool) error {
	available, channelDetails, err := getProfitabilityAvailableChannels(options.ChannelIDs)
	if err != nil {
		return err
	}
	report.AvailableChannels = available
	channelByID := make(map[int]*ProfitabilityChannel)
	addChannel := func(id int) *ProfitabilityChannel {
		if row := channelByID[id]; row != nil {
			return row
		}
		detail := channelDetails[id]
		name := detail.Name
		if name == "" {
			name = fmt.Sprintf("#%d (deleted)", id)
			if id == 0 {
				name = "Unassigned legacy purchases"
			}
		}
		row := &ProfitabilityChannel{Id: id, Name: name, Status: detail.Status, ProfitStatus: "no_activity"}
		channelByID[id] = row
		report.Channels = append(report.Channels, row)
		return row
	}
	if options.ChannelSelectionPresent {
		for _, id := range options.ChannelIDs {
			addChannel(id)
		}
	} else {
		for _, channel := range available {
			addChannel(channel.Id)
		}
	}
	for _, entry := range entries {
		if entry.EntryType != ProfitabilityEntryAccountPurchase {
			continue
		}
		row := addChannel(entry.ChannelId)
		amount := entry.AmountRmbCents
		if amount == 0 {
			amount = entry.AmountCents
		}
		row.PurchaseRmb += USDCentsToUSD(amount)
		row.PurchaseRecorded = true
	}

	var subLogs []sub2APIUsageLog
	var summary sub2APIUsageSummary
	var status, subError string
	if options.UseSubSummary {
		report.SubUsageSource = "summary"
		var channels []Channel
		if err := DB.Select("id", "key", "base_url").Find(&channels).Error; err != nil {
			return err
		}
		selectedChannels := make([]Channel, 0, len(channels))
		for _, channel := range channels {
			if channelSelected(channel.Id) {
				selectedChannels = append(selectedChannels, channel)
			}
		}
		summary, status, subError = getSub2APIUsageSummary(startTimestamp, endTimestamp, selectedChannels)
		for _, row := range report.Channels {
			row.SubUsageStatus = "unavailable"
		}
		for id, usage := range summary.channelUsage {
			row := addChannel(id)
			row.SubUsageUsd = usage
			row.SubUsageStatus = "complete"
		}
	} else {
		report.SubUsageSource = "logs"
		subLogs, status, subError = GetSub2APIUsageLogs(startTimestamp, endTimestamp)
	}
	subByRequest := make(map[string]sub2APIUsageLog, len(subLogs))
	for _, log := range subLogs {
		if log.RequestId == "" {
			continue
		}
		if _, exists := subByRequest[log.RequestId]; exists {
			status, subError = "error", "duplicate Sub2API request IDs"
		}
		subByRequest[log.RequestId] = log
	}

	query := LOG_DB.Model(&Log{}).Select("id", "user_id", "channel_id", "model_name", "group", "quota", "upstream_request_id", "created_at", "other").
		Where("type = ?", LogTypeConsume).Order("created_at, id")
	if endTimestamp > 0 {
		query = query.Where("created_at <= ?", endTimestamp)
	}
	var calls []Log
	if err := query.Find(&calls).Error; err != nil {
		return err
	}
	requestCounts := make(map[string]int)
	for _, call := range calls {
		if !options.UseSubSummary && (startTimestamp == 0 || call.CreatedAt >= startTimestamp) && call.UpstreamRequestId != "" {
			requestCounts[call.UpstreamRequestId]++
			if requestCounts[call.UpstreamRequestId] > 1 {
				status, subError = "error", "duplicate New API upstream request IDs"
			}
		}
	}
	report.SubUsageStatus, report.SubUsageError = status, subError

	var gifts []*ProfitabilityEntry
	if err := DB.Model(&ProfitabilityEntry{}).Where("entry_type = ?", ProfitabilityEntryAdminAdjustment).
		Order("occurred_at, id").Find(&gifts).Error; err != nil {
		return err
	}
	giftByUser := make(map[int]int64)
	giftIndex := 0
	modelByKey := make(map[string]*ProfitabilityModel)
	groupByName := make(map[string]*ProfitabilityGroup)
	groupTotals := make(map[string]*profitabilityGroupTotals)
	groupCostByChannel := make(map[int]map[string]float64)
	for _, group := range report.Groups {
		groupByName[group.Name] = group
	}
	accountByKey := make(map[[2]int64]*ProfitabilityAccount)
	usedRequests := make(map[string]bool)
	adminUser := make(map[int]bool, len(adminUserIds))
	for _, id := range adminUserIds {
		adminUser[id] = true
	}
	var priorCharge, allCharge float64
	var adminSubUsage, outsideSelectedSubUsage float64
	var outsideSelectedRequests int64
	for _, call := range calls {
		for giftIndex < len(gifts) && gifts[giftIndex].OccurredAt <= call.CreatedAt {
			gift := gifts[giftIndex]
			giftByUser[gift.UserId] += gift.Quota
			giftIndex++
		}
		giftUsed := int64(call.Quota)
		if giftUsed > giftByUser[call.UserId] {
			giftUsed = giftByUser[call.UserId]
		}
		if giftUsed < 0 {
			giftUsed = 0
		}
		giftByUser[call.UserId] -= giftUsed
		charge := quotaToUsd(int64(call.Quota) - giftUsed)
		if adminUser[call.UserId] {
			charge = 0
		} else {
			allCharge += charge
		}
		if startTimestamp > 0 && call.CreatedAt < startTimestamp {
			if !adminUser[call.UserId] {
				priorCharge += charge
			}
			continue
		}

		sub, matched := subByRequest[call.UpstreamRequestId]
		matched = status == "connected" && matched && sub.AccountId > 0 && call.UpstreamRequestId != "" && requestCounts[call.UpstreamRequestId] == 1 && !usedRequests[call.UpstreamRequestId]
		if matched {
			usedRequests[call.UpstreamRequestId] = true
		}
		if !channelSelected(call.ChannelId) {
			if matched {
				outsideSelectedSubUsage += sub.accountCost()
				outsideSelectedRequests++
			}
			continue
		}

		row := addChannel(call.ChannelId)
		row.RequestCount++
		report.CallChargeRmb += charge
		row.CustomerChargeRmb += charge
		key := fmt.Sprintf("%d:%s", call.ChannelId, call.ModelName)
		modelRow := modelByKey[key]
		if modelRow == nil {
			modelRow = &ProfitabilityModel{ChannelId: call.ChannelId, Name: call.ModelName}
			modelByKey[key] = modelRow
			report.Models = append(report.Models, modelRow)
		}
		modelRow.CustomerChargeRmb += charge
		groupRow := groupByName[call.Group]
		if groupRow == nil {
			groupRow = &ProfitabilityGroup{Name: call.Group}
			groupByName[call.Group] = groupRow
			report.Groups = append(report.Groups, groupRow)
		}
		groupRow.CustomerChargeRmb += charge
		totals := groupTotals[call.Group]
		if totals == nil {
			totals = &profitabilityGroupTotals{}
			groupTotals[call.Group] = totals
		}
		var cost float64
		if matched {
			cost = sub.accountCost()
		}
		totals.all.add(&call, quotaToUsd(int64(call.Quota)), cost, matched)
		if adminUser[call.UserId] {
			groupRow.SelfUsd += quotaToUsd(int64(call.Quota))
		} else {
			totals.customers.add(&call, charge, cost, matched)
		}
		if !matched {
			if !options.UseSubSummary {
				row.UnmatchedRequests++
				report.UnmatchedRequests++
			}
			continue
		}

		if adminUser[call.UserId] {
			adminSubUsage += cost
		}
		row.SubUsageUsd += cost
		row.MatchedRequests++
		report.MatchedRequests++
		modelRow.SubUsageUsd += cost
		if groupRow != nil {
			groupRow.SubUsageUsd += cost
			if groupCostByChannel[call.ChannelId] == nil {
				groupCostByChannel[call.ChannelId] = make(map[string]float64)
			}
			groupCostByChannel[call.ChannelId][call.Group] += cost
		}
		accountKey := [2]int64{int64(call.ChannelId), sub.AccountId}
		accountRow := accountByKey[accountKey]
		if accountRow == nil {
			accountRow = &ProfitabilityAccount{ChannelId: call.ChannelId, AccountId: sub.AccountId}
			accountByKey[accountKey] = accountRow
			report.Accounts = append(report.Accounts, accountRow)
		}
		accountRow.SubUsageUsd += cost
	}

	var poolUsage float64
	for _, log := range subLogs {
		poolUsage += log.accountCost()
		if log.RequestId == "" || !usedRequests[log.RequestId] {
			report.UnmatchedSubRequests++
			report.UnmatchedSubUsageUsd += log.accountCost()
		}
	}
	paidBefore := 0.0
	if startTimestamp > 0 {
		priorTopups, err := getProfitabilityTopupRevenue(0, startTimestamp-1, adminUserIds)
		if err != nil {
			return err
		}
		for _, topup := range priorTopups {
			paidBefore += topup.AmountUsd
		}
		priorEntries, err := GetProfitabilityEntries(0, startTimestamp-1)
		if err != nil {
			return err
		}
		for _, entry := range priorEntries {
			if entry.EntryType == ProfitabilityEntryManualIncome {
				paidBefore += USDCentsToUSD(entry.AmountCents)
			}
		}
	}
	paidToEnd := paidBefore + report.CashReceivedRmb
	report.CashRecognizedRmb = math.Min(paidToEnd, allCharge) - math.Min(paidBefore, priorCharge)
	if report.CashRecognizedRmb < 0 {
		report.CashRecognizedRmb = 0
	}

	for _, row := range report.Channels {
		report.SubUsageUsd += row.SubUsageUsd
		row.ProfitStatus = profitabilityChannelStatus(row, status)
		if row.Id == 0 && row.PurchaseRecorded {
			row.ProfitStatus = "unmatched"
		}
		if row.ProfitStatus == "complete" {
			profit := row.CustomerChargeRmb - row.PurchaseRmb
			row.EstimatedProfitRmb = &profit
		}
	}
	if report.SubUsageUsd > 0 {
		report.CostRateRmbPerUsd = report.AccountCostRmb / report.SubUsageUsd
	}
	report.ProfitStatus = profitabilityReportStatus(report, options)
	if report.ProfitStatus == "complete" {
		profit := report.CallChargeRmb - report.AccountCostRmb
		if options.RevenueBasis == ProfitabilityRevenueBasisCash {
			profit = report.CashRecognizedRmb - report.AccountCostRmb
		}
		report.EstimatedProfitRmb = &profit
		cashProfit := report.CashRecognizedRmb - report.AccountCostRmb
		report.CashProfitRmb = &cashProfit
	}
	for _, modelRow := range report.Models {
		channel := channelByID[modelRow.ChannelId]
		if !options.UseSubSummary {
			if channel.SubUsageUsd > 0 {
				modelRow.AllocatedCostRmb = channel.PurchaseRmb * modelRow.SubUsageUsd / channel.SubUsageUsd
			}
			if channel.ProfitStatus == "complete" {
				profit := modelRow.CustomerChargeRmb - modelRow.AllocatedCostRmb
				modelRow.EstimatedProfitRmb = &profit
			}
		}
	}
	allocationStatus := report.ProfitStatus
	for _, channel := range report.Channels {
		if allocationStatus == "complete" && channel.PurchaseRmb > 0 && channel.SubUsageUsd == 0 {
			allocationStatus = "unallocated_purchase"
		}
	}
	configuredRatios := ratio_setting.GetGroupRatioCopy()
	for _, groupRow := range report.Groups {
		if configured, exists := configuredRatios[groupRow.Name]; exists {
			configured += ratio_setting.GetGroupRatioAddition(groupRow.Name)
			groupRow.ConfiguredRatio = &configured
			groupRow.Ratio = configured
		}
		for _, channel := range report.Channels {
			if channel.SubUsageUsd > 0 {
				groupRow.AllocatedCostRmb += channel.PurchaseRmb * groupCostByChannel[channel.Id][groupRow.Name] / channel.SubUsageUsd
			}
		}
		if report.ProfitStatus == "complete" {
			profit := groupRow.CustomerChargeRmb - groupRow.AllocatedCostRmb
			groupRow.EstimatedProfitRmb = &profit
			groupRow.ProfitStatus = monetaryProfitStatus(groupRow.EstimatedProfitRmb)
			groupRow.Profitable = profit > 0
		} else {
			groupRow.ProfitStatus = "unknown"
		}
		totals := groupTotals[groupRow.Name]
		if totals == nil {
			totals = &profitabilityGroupTotals{}
		}
		groupRow.AllCalls = buildProfitabilityGroupComparison(report, totals.all, groupRow.AllocatedCostRmb, allocationStatus, false)
		groupRow.CustomerCalls = buildProfitabilityGroupComparison(report, totals.customers, groupRow.AllocatedCostRmb, allocationStatus, true)
		groupRow.BreakEvenRatio = groupRow.CustomerCalls.BreakEvenRatio
	}
	sort.Slice(report.Groups, func(i, j int) bool { return report.Groups[i].Name < report.Groups[j].Name })
	report.Usage.Groups = report.Groups
	sort.Slice(report.Channels, func(i, j int) bool { return report.Channels[i].Id < report.Channels[j].Id })
	sort.Slice(report.Models, func(i, j int) bool { return report.Models[i].CustomerChargeRmb > report.Models[j].CustomerChargeRmb })
	report.Pool = buildProfitabilityPoolSummary(report, poolUsage, adminSubUsage, outsideSelectedSubUsage, outsideSelectedRequests)
	if options.UseSubSummary {
		applyProfitabilityNativeGroups(report, summary, calls, startTimestamp, channelSelected)
	}
	return nil
}

func buildProfitabilityPoolSummary(report *ProfitabilityReport, poolUsage, adminSubUsage, outsideSubUsage float64, outsideRequests int64) ProfitabilityPoolSummary {
	pool := ProfitabilityPoolSummary{
		AllCalls:                   ProfitabilityCallComparison{ChargeUsd: report.Usage.AmountUsd},
		CustomerCalls:              ProfitabilityCallComparison{ChargeUsd: report.Usage.CustomerUsd},
		AdminChargeUsd:             report.Usage.SelfUsd,
		MatchedAdminSubUsageUsd:    adminSubUsage,
		OutsideSelectedSubUsageUsd: outsideSubUsage,
		ProfitStatus:               "complete",
	}
	switch {
	case report.Scope == "selected" && len(report.SelectedChannelIds) == 0:
		pool.AllCalls.Status = "empty_selection"
	case report.SubUsageStatus == "not_configured":
		pool.AllCalls.Status = "not_configured"
	case report.SubUsageStatus != "connected":
		pool.AllCalls.Status = "sync_error"
	case report.RevenueBasis == ProfitabilityRevenueBasisCash:
		pool.AllCalls.Status = "cash_scope"
	case outsideRequests > 0:
		pool.AllCalls.Status = "scope_mismatch"
	case poolUsage == 0:
		pool.AllCalls.Status = "no_usage"
	case report.UnmatchedSubRequests > 0 || report.UnmatchedRequests > 0:
		pool.AllCalls.Status = "unverified_scope"
	default:
		pool.AllCalls.Status = "complete"
	}
	pool.CustomerCalls.Status = pool.AllCalls.Status
	if report.SubUsageStatus == "connected" {
		pool.AllCalls.SubUsageUsd = &poolUsage
	}
	if pool.AllCalls.Status == "complete" || pool.AllCalls.Status == "unverified_scope" {
		ratio := pool.AllCalls.ChargeUsd / poolUsage
		pool.AllCalls.EffectiveRatio = &ratio
	}
	if pool.AllCalls.Status == "complete" || pool.AllCalls.Status == "no_usage" || pool.AllCalls.Status == "unverified_scope" {
		// Unassigned upstream calls may belong to administrators; never guess their role.
		if report.UnmatchedSubRequests > 0 || report.UnmatchedRequests > 0 {
			pool.CustomerCalls.Status = "unmatched"
		} else {
			customerUsage := math.Max(0, poolUsage-adminSubUsage)
			pool.CustomerCalls.SubUsageUsd = &customerUsage
			pool.CustomerCalls.Status = "no_usage"
			if customerUsage > 0 {
				ratio := pool.CustomerCalls.ChargeUsd / customerUsage
				pool.CustomerCalls.EffectiveRatio = &ratio
				pool.CustomerCalls.Status = "complete"
			}
		}
	}

	// Procurement is paid once. Self-use is not revenue and does not refund purchases.
	switch {
	case report.Scope == "selected" && len(report.SelectedChannelIds) == 0:
		pool.ProfitStatus = "empty_selection"
	case report.AccountPurchaseCount == 0 && report.Usage.RequestCount == 0:
		pool.ProfitStatus = "no_activity"
	default:
		for _, channel := range report.Channels {
			if !channel.PurchaseRecorded && channel.RequestCount+channel.MatchedRequests+channel.UnmatchedRequests > 0 {
				pool.ProfitStatus = "missing_purchase"
				break
			}
		}
	}
	if pool.ProfitStatus == "complete" {
		income := report.CallChargeRmb
		if report.RevenueBasis == ProfitabilityRevenueBasisCash {
			income = report.CashRecognizedRmb
		}
		profit := income - report.AccountCostRmb
		pool.ProfitRmb = &profit
	}
	return pool
}

func profitabilityChannelStatus(row *ProfitabilityChannel, subStatus string) string {
	hasActivity := row.RequestCount+row.MatchedRequests+row.UnmatchedRequests > 0 || row.SubUsageUsd > 0
	if !hasActivity && !row.PurchaseRecorded {
		return "no_activity"
	}
	if hasActivity && subStatus == "not_configured" {
		return "not_configured"
	}
	if hasActivity && subStatus != "connected" {
		return "sync_error"
	}
	if row.UnmatchedRequests > 0 {
		return "unmatched"
	}
	if hasActivity && !row.PurchaseRecorded {
		return "missing_purchase"
	}
	return "complete"
}

func profitabilityReportStatus(report *ProfitabilityReport, options ProfitabilityReportOptions) string {
	if options.ChannelSelectionPresent && len(options.ChannelIDs) == 0 {
		return "empty_selection"
	}
	hasActivity := false
	for _, wanted := range []string{"not_configured", "sync_error", "unmatched", "missing_purchase"} {
		for _, row := range report.Channels {
			if row.ProfitStatus != "no_activity" {
				hasActivity = true
			}
			if row.ProfitStatus == wanted {
				return wanted
			}
		}
	}
	if report.Scope == "all" && report.UnmatchedSubRequests > 0 {
		return "unmatched"
	}
	if !hasActivity {
		return "no_activity"
	}
	return "complete"
}

func getProfitabilityAvailableChannels(selectedIDs []int) ([]ProfitabilityAvailableChannel, map[int]ProfitabilityAvailableChannel, error) {
	var channels []Channel
	if err := DB.Select("id", "name", "status").Order("id").Find(&channels).Error; err != nil {
		return nil, nil, err
	}
	byID := make(map[int]ProfitabilityAvailableChannel, len(channels))
	for _, channel := range channels {
		byID[channel.Id] = ProfitabilityAvailableChannel{Id: channel.Id, Name: channel.Name, Status: channel.Status}
	}
	var entryIDs, logIDs []int
	if err := DB.Model(&ProfitabilityEntry{}).Distinct("channel_id").Where("channel_id > 0").Pluck("channel_id", &entryIDs).Error; err != nil {
		return nil, nil, err
	}
	if err := LOG_DB.Model(&Log{}).Distinct("channel_id").Where("channel_id > 0").Pluck("channel_id", &logIDs).Error; err != nil {
		return nil, nil, err
	}
	for _, id := range append(append(entryIDs, logIDs...), selectedIDs...) {
		if _, exists := byID[id]; !exists && id > 0 {
			byID[id] = ProfitabilityAvailableChannel{Id: id, Name: fmt.Sprintf("#%d (deleted)", id), Status: common.ChannelStatusUnknown}
		}
	}
	available := make([]ProfitabilityAvailableChannel, 0, len(byID))
	for _, channel := range byID {
		available = append(available, channel)
	}
	sort.Slice(available, func(i, j int) bool { return available[i].Id < available[j].Id })
	return available, byID, nil
}
