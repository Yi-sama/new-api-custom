package model

import (
	"errors"
	"fmt"
	"math"
	"sort"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/shopspring/decimal"
)

const (
	ProfitabilityEntryAccountPurchase = "account_purchase"
	ProfitabilityEntryManualIncome    = "manual_income"
	ProfitabilityEntryAdminAdjustment = "admin_adjustment"
)

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
	Name               string   `json:"name"`
	Ratio              float64  `json:"ratio"`
	CustomerUsd        float64  `json:"customer_usd"`
	SelfUsd            float64  `json:"self_usd"`
	SubUsageUsd        float64  `json:"sub_usage_usd"`
	AllocatedCostRmb   float64  `json:"allocated_cost_rmb"`
	CustomerChargeRmb  float64  `json:"customer_charge_rmb"`
	EstimatedProfitRmb *float64 `json:"estimated_profit_rmb"`
	BreakEvenRatio     *float64 `json:"break_even_ratio"`
	Profitable         bool     `json:"profitable"`
	ProfitStatus       string   `json:"profit_status"`
}

type ProfitabilityChannel struct {
	Id                 int      `json:"id"`
	Name               string   `json:"name"`
	PurchaseRmb        float64  `json:"purchase_rmb"`
	SubUsageUsd        float64  `json:"sub_usage_usd"`
	CustomerChargeRmb  float64  `json:"customer_charge_rmb"`
	EstimatedProfitRmb *float64 `json:"estimated_profit_rmb"`
	MatchedRequests    int64    `json:"matched_requests"`
	UnmatchedRequests  int64    `json:"unmatched_requests"`
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

type ProfitabilityReport struct {
	RevenueUsd           float64                     `json:"revenue_usd"`
	TopupRevenueUsd      float64                     `json:"topup_revenue_usd"`
	ManualIncomeUsd      float64                     `json:"manual_income_usd"`
	AdminAdjustmentUsd   float64                     `json:"admin_adjustment_usd"`
	AccountCostRmb       float64                     `json:"account_cost_rmb"`
	SubUsageUsd          float64                     `json:"sub_usage_usd"`
	CostRateRmbPerUsd    float64                     `json:"cost_rate_rmb_per_usd"`
	AccountCount         int64                       `json:"account_count"`
	AccountPurchaseCount int64                       `json:"account_purchase_count"`
	Usage                ProfitabilityUsageSummary   `json:"usage"`
	Groups               []*ProfitabilityGroup       `json:"groups"`
	RevenueEntries       []*ProfitabilityRevenueItem `json:"revenue_entries"`
	CostEntries          []*ProfitabilityEntryView   `json:"cost_entries"`
	SubUsageStatus       string                      `json:"sub_usage_status"`
	SubUsageError        string                      `json:"sub_usage_error,omitempty"`
	Channels             []*ProfitabilityChannel     `json:"channels"`
	Models               []*ProfitabilityModel       `json:"models"`
	Accounts             []*ProfitabilityAccount     `json:"accounts"`
	UnmatchedSubRequests int64                       `json:"unmatched_sub_requests"`
	CashReceivedRmb      float64                     `json:"cash_received_rmb"`
	CashRecognizedRmb    float64                     `json:"cash_recognized_rmb"`
	CashProfitRmb        *float64                    `json:"cash_profit_rmb"`
	CallChargeRmb        float64                     `json:"call_charge_rmb"`
	EstimatedProfitRmb   *float64                    `json:"estimated_profit_rmb"`
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
	err := DB.Model(&User{}).
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

func getProfitabilityUsage(startTimestamp int64, endTimestamp int64, adminUserIds []int, costRate float64) (ProfitabilityUsageSummary, error) {
	query := LOG_DB.Table("logs").
		Select("COALESCE(SUM(quota), 0) AS quota, COUNT(*) AS request_count, COALESCE(SUM(prompt_tokens), 0) + COALESCE(SUM(completion_tokens), 0) AS token_count").
		Where("type = ?", LogTypeConsume)
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
	if startTimestamp > 0 {
		selfQuery = selfQuery.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp > 0 {
		selfQuery = selfQuery.Where("created_at <= ?", endTimestamp)
	}
	if len(adminUserIds) > 0 {
		selfQuery = selfQuery.Where("user_id IN ?", adminUserIds)
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
	adminUserIds, err := getProfitabilityAdminUserIds()
	if err != nil {
		return nil, err
	}
	entries, err := GetProfitabilityEntries(startTimestamp, endTimestamp)
	if err != nil {
		return nil, err
	}
	topupEntries, err := getProfitabilityTopupRevenue(startTimestamp, endTimestamp, adminUserIds)
	if err != nil {
		return nil, err
	}
	accountCostRmb := 0.0
	accountCount := int64(0)
	accountPurchaseCount := int64(0)
	for _, entry := range entries {
		if entry.EntryType == ProfitabilityEntryAccountPurchase {
			amountRmbCents := entry.AmountRmbCents
			if amountRmbCents == 0 {
				amountRmbCents = entry.AmountCents
			}
			accountCostRmb += USDCentsToUSD(amountRmbCents)
			accountPurchaseCount += int64(entry.AccountCount)
			accountCount += int64(entry.AccountCount)
		}
	}
	usage, err := getProfitabilityUsage(startTimestamp, endTimestamp, adminUserIds, 0)
	if err != nil {
		return nil, err
	}

	report := &ProfitabilityReport{
		RevenueEntries:       make([]*ProfitabilityRevenueItem, 0, len(entries)+len(topupEntries)),
		CostEntries:          make([]*ProfitabilityEntryView, 0, len(entries)),
		Usage:                usage,
		Groups:               usage.Groups,
		AccountCostRmb:       accountCostRmb,
		AccountCount:         accountCount,
		AccountPurchaseCount: accountPurchaseCount,
	}
	for _, entry := range entries {
		switch entry.EntryType {
		case ProfitabilityEntryAccountPurchase:
			report.CostEntries = append(report.CostEntries, profitabilityEntryView(entry))
		case ProfitabilityEntryManualIncome:
			amountUsd := USDCentsToUSD(entry.AmountCents)
			report.ManualIncomeUsd += amountUsd
			report.RevenueUsd += amountUsd
			report.RevenueEntries = append(report.RevenueEntries, &ProfitabilityRevenueItem{
				Id:         entry.Id,
				Source:     entry.EntryType,
				SourceId:   entry.SourceId,
				AmountUsd:  amountUsd,
				UserId:     entry.UserId,
				Username:   entry.Username,
				Note:       entry.Note,
				OccurredAt: entry.OccurredAt,
			})
		case ProfitabilityEntryAdminAdjustment:
			amountUsd := USDCentsToUSD(entry.AmountCents)
			report.AdminAdjustmentUsd += amountUsd
			report.RevenueEntries = append(report.RevenueEntries, &ProfitabilityRevenueItem{
				Id:         entry.Id,
				Source:     entry.EntryType,
				SourceId:   entry.SourceId,
				AmountUsd:  amountUsd,
				UserId:     entry.UserId,
				Username:   entry.Username,
				Note:       entry.Note,
				OccurredAt: entry.OccurredAt,
			})
		}
	}
	for _, entry := range topupEntries {
		report.TopupRevenueUsd += entry.AmountUsd
		report.RevenueUsd += entry.AmountUsd
		report.RevenueEntries = append(report.RevenueEntries, entry)
	}
	if err := addProfitabilityBreakdown(report, entries, adminUserIds, startTimestamp, endTimestamp); err != nil {
		return nil, err
	}
	return report, nil
}

func addProfitabilityBreakdown(report *ProfitabilityReport, entries []*ProfitabilityEntry, adminUserIds []int, startTimestamp, endTimestamp int64) error {
	var channels []Channel
	if err := DB.Select("id", "name").Order("id").Find(&channels).Error; err != nil {
		return err
	}
	channelByID := make(map[int]*ProfitabilityChannel, len(channels))
	report.Channels = make([]*ProfitabilityChannel, 0, len(channels))
	report.Models = make([]*ProfitabilityModel, 0)
	report.Accounts = make([]*ProfitabilityAccount, 0)
	for _, channel := range channels {
		row := &ProfitabilityChannel{Id: channel.Id, Name: channel.Name}
		channelByID[row.Id] = row
		report.Channels = append(report.Channels, row)
	}
	for _, entry := range entries {
		if entry.EntryType == ProfitabilityEntryAccountPurchase {
			row := channelByID[entry.ChannelId]
			if row == nil {
				name := fmt.Sprintf("#%d (deleted)", entry.ChannelId)
				if entry.ChannelId == 0 {
					name = "Unassigned legacy purchases"
				}
				row = &ProfitabilityChannel{Id: entry.ChannelId, Name: name}
				channelByID[row.Id] = row
				report.Channels = append(report.Channels, row)
			}
			amount := entry.AmountRmbCents
			if amount == 0 {
				amount = entry.AmountCents
			}
			row.PurchaseRmb += USDCentsToUSD(amount)
		}
	}
	report.CashReceivedRmb = report.TopupRevenueUsd + report.ManualIncomeUsd

	subLogs, status, subError := GetSub2APIUsageLogs(startTimestamp, endTimestamp)
	report.SubUsageStatus, report.SubUsageError = status, subError
	subByRequest := make(map[string]sub2APIUsageLog, len(subLogs))
	for _, log := range subLogs {
		if log.RequestId != "" {
			if _, exists := subByRequest[log.RequestId]; exists {
				status, subError = "error", "duplicate Sub2API request IDs"
			}
			subByRequest[log.RequestId] = log
		}
	}
	report.SubUsageStatus, report.SubUsageError = status, subError

	// Include earlier usage so a gifted balance carries across report periods.
	query := LOG_DB.Model(&Log{}).Select("user_id", "channel_id", "model_name", "group", "quota", "upstream_request_id", "created_at").
		Where("type = ?", LogTypeConsume).Order("created_at, id")
	if endTimestamp > 0 {
		query = query.Where("created_at <= ?", endTimestamp)
	}
	var calls []Log
	if err := query.Find(&calls).Error; err != nil {
		return err
	}
	var gifts []*ProfitabilityEntry
	if err := DB.Model(&ProfitabilityEntry{}).Where("entry_type = ?", ProfitabilityEntryAdminAdjustment).
		Order("occurred_at, id").Find(&gifts).Error; err != nil {
		return err
	}
	giftByUser := make(map[int]int64)
	giftIndex := 0
	modelByKey := make(map[string]*ProfitabilityModel)
	groupByName := make(map[string]*ProfitabilityGroup)
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
		report.CallChargeRmb += charge
		row := channelByID[call.ChannelId]
		if row == nil {
			row = &ProfitabilityChannel{Id: call.ChannelId, Name: fmt.Sprintf("#%d (deleted)", call.ChannelId)}
			channelByID[row.Id] = row
			report.Channels = append(report.Channels, row)
		}
		row.CustomerChargeRmb += charge
		sub, matched := subByRequest[call.UpstreamRequestId]
		if status != "connected" || !matched || sub.AccountId <= 0 || call.UpstreamRequestId == "" || usedRequests[call.UpstreamRequestId] {
			row.UnmatchedRequests++
			continue
		}
		usedRequests[call.UpstreamRequestId] = true
		cost := sub.accountCost()
		row.SubUsageUsd += cost
		row.MatchedRequests++
		key := fmt.Sprintf("%d:%s", call.ChannelId, call.ModelName)
		modelRow := modelByKey[key]
		if modelRow == nil {
			modelRow = &ProfitabilityModel{ChannelId: call.ChannelId, Name: call.ModelName}
			modelByKey[key] = modelRow
			report.Models = append(report.Models, modelRow)
		}
		modelRow.SubUsageUsd += cost
		modelRow.CustomerChargeRmb += charge
		groupRow := groupByName[call.Group]
		if groupRow != nil {
			groupRow.SubUsageUsd += cost
			groupRow.CustomerChargeRmb += charge
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
	report.UnmatchedSubRequests = int64(len(subLogs) - len(usedRequests))
	var totalCost float64
	paidToEnd := report.CashReceivedRmb
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
	paidToEnd += paidBefore
	report.CashRecognizedRmb = math.Min(paidToEnd, allCharge) - math.Min(paidBefore, priorCharge)
	if report.CashRecognizedRmb < 0 {
		report.CashRecognizedRmb = 0
	}
	if report.CashRecognizedRmb > report.CallChargeRmb {
		report.CashRecognizedRmb = report.CallChargeRmb
	}
	var complete = status == "connected" && report.UnmatchedSubRequests == 0
	for _, row := range report.Channels {
		totalCost += row.PurchaseRmb
		report.SubUsageUsd += row.SubUsageUsd
		if row.Id == 0 && row.PurchaseRmb > 0 {
			complete = false
			continue
		}
		if row.UnmatchedRequests > 0 || report.UnmatchedSubRequests > 0 {
			complete = false
			continue
		}
		profit := row.CustomerChargeRmb - row.PurchaseRmb
		row.EstimatedProfitRmb = &profit
	}
	if report.SubUsageUsd > 0 {
		report.CostRateRmbPerUsd = totalCost / report.SubUsageUsd
	}
	if complete {
		profit := report.CallChargeRmb - totalCost
		report.EstimatedProfitRmb = &profit
		cashProfit := report.CashRecognizedRmb - totalCost
		report.CashProfitRmb = &cashProfit
	}
	for _, modelRow := range report.Models {
		channel := channelByID[modelRow.ChannelId]
		if channel.SubUsageUsd > 0 {
			modelRow.AllocatedCostRmb = channel.PurchaseRmb * modelRow.SubUsageUsd / channel.SubUsageUsd
		}
		if channel.EstimatedProfitRmb != nil {
			profit := modelRow.CustomerChargeRmb - modelRow.AllocatedCostRmb
			modelRow.EstimatedProfitRmb = &profit
		}
	}
	for _, groupRow := range report.Groups {
		for _, channel := range report.Channels {
			if channel.SubUsageUsd == 0 {
				continue
			}
			groupRow.AllocatedCostRmb += channel.PurchaseRmb * groupCostByChannel[channel.Id][groupRow.Name] / channel.SubUsageUsd
		}
		if complete {
			profit := groupRow.CustomerChargeRmb - groupRow.AllocatedCostRmb
			groupRow.EstimatedProfitRmb = &profit
			if groupRow.CustomerChargeRmb > 0 && groupRow.Ratio > 0 {
				ratio := groupRow.Ratio * groupRow.AllocatedCostRmb / groupRow.CustomerChargeRmb
				groupRow.BreakEvenRatio = &ratio
			}
		}
	}
	sort.Slice(report.Models, func(i, j int) bool {
		return report.Models[i].CustomerChargeRmb > report.Models[j].CustomerChargeRmb
	})
	return nil
}
