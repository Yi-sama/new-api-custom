package controller

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

type ProfitabilityEntryRequest struct {
	EntryType    string  `json:"entry_type"`
	AmountUsd    float64 `json:"amount_usd"`
	AmountRmb    float64 `json:"amount_rmb"`
	AccountCount int     `json:"account_count"`
	ChannelId    int     `json:"channel_id"`
	Note         string  `json:"note"`
	OccurredAt   int64   `json:"occurred_at"`
}

func GetProfitabilitySummary(c *gin.Context) {
	startTimestamp := parseProfitabilityTimestamp(c.Query("start_timestamp"))
	endTimestamp := parseProfitabilityTimestamp(c.Query("end_timestamp"))
	if startTimestamp < 0 || endTimestamp < 0 || (endTimestamp > 0 && startTimestamp > endTimestamp) {
		common.ApiErrorMsg(c, "invalid profitability time range")
		return
	}

	report, err := model.GetProfitabilityReport(startTimestamp, endTimestamp)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, report)
}

func AddProfitabilityEntry(c *gin.Context) {
	var req ProfitabilityEntryRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "invalid profitability entry")
		return
	}
	req.EntryType = strings.TrimSpace(req.EntryType)

	var amountCents int64
	var amountRmbCents int64
	var err error
	if req.EntryType == model.ProfitabilityEntryAccountPurchase {
		amountRmbCents, err = model.ParseRMBAmountCents(req.AmountRmb)
	} else {
		amountCents, err = model.ParseUSDAmountCents(req.AmountUsd)
	}
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	if err := model.ValidateProfitabilityEntry(req.EntryType, amountCents+amountRmbCents, req.AccountCount); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	if req.EntryType == model.ProfitabilityEntryAccountPurchase {
		if req.ChannelId <= 0 || !model.ProfitabilityChannelExists(req.ChannelId) {
			common.ApiErrorMsg(c, "select an existing channel for account purchase")
			return
		}
	}
	if len(req.Note) > 500 {
		common.ApiErrorMsg(c, "note is too long")
		return
	}

	occurredAt := req.OccurredAt
	if occurredAt == 0 {
		occurredAt = common.GetTimestamp()
	}
	if occurredAt < 0 {
		common.ApiErrorMsg(c, "invalid occurred_at")
		return
	}

	entry := &model.ProfitabilityEntry{
		EntryType:      req.EntryType,
		AmountCents:    amountCents,
		AmountRmbCents: amountRmbCents,
		AccountCount:   req.AccountCount,
		ChannelId:      req.ChannelId,
		Source:         "manual",
		Note:           strings.TrimSpace(req.Note),
		OccurredAt:     occurredAt,
		CreatedBy:      c.GetInt("id"),
		CreatedAt:      common.GetTimestamp(),
	}
	if err := entry.Insert(); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    entry,
	})
}

func parseProfitabilityTimestamp(value string) int64 {
	if value == "" {
		return 0
	}
	timestamp, err := strconv.ParseInt(value, 10, 64)
	if err != nil || timestamp < 0 {
		return -1
	}
	return timestamp
}
