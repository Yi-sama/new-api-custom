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

type Sub2APIUsageSnapshot struct {
	TotalCostUsd       float64
	TotalActualCostUsd float64
	Status             string
	Error              string
}

type sub2APIUsageStatsResponse struct {
	Data struct {
		TotalCost       float64 `json:"total_cost"`
		TotalActualCost float64 `json:"total_actual_cost"`
	} `json:"data"`
}

type sub2APIUsageLog struct {
	RequestId             string    `json:"request_id"`
	AccountId             int64     `json:"account_id"`
	CreatedAt             time.Time `json:"created_at"`
	TotalCost             float64   `json:"total_cost"`
	AccountStatsCost      *float64  `json:"account_stats_cost"`
	AccountRateMultiplier *float64  `json:"account_rate_multiplier"`
}

func (log sub2APIUsageLog) accountCost() float64 {
	cost := log.TotalCost
	if log.AccountStatsCost != nil {
		cost = *log.AccountStatsCost
	}
	if log.AccountRateMultiplier != nil {
		cost *= *log.AccountRateMultiplier
	}
	return cost
}

func GetSub2APIUsageLogs(startTimestamp, endTimestamp int64) ([]sub2APIUsageLog, string, string) {
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("SUB2API_BASE_URL")), "/")
	adminKey := strings.TrimSpace(os.Getenv("SUB2API_ADMIN_API_KEY"))
	if baseURL == "" || adminKey == "" {
		return nil, "not_configured", "Sub2API connection is not configured"
	}
	endpoint, err := url.Parse(baseURL + "/api/v1/admin/usage")
	if err != nil {
		return nil, "error", "invalid Sub2API base URL"
	}
	query := endpoint.Query()
	query.Set("timezone", "UTC")
	query.Set("page_size", "1000")
	// New usage is appended by ID; descending offset pages shift during live traffic.
	query.Set("sort_by", "id")
	query.Set("sort_order", "asc")
	if startTimestamp > 0 {
		query.Set("start_date", time.Unix(startTimestamp, 0).UTC().Format("2006-01-02"))
	}
	if endTimestamp > 0 {
		query.Set("end_date", time.Unix(endTimestamp, 0).UTC().Format("2006-01-02"))
	}
	client := &http.Client{Timeout: 8 * time.Second}
	logs := make([]sub2APIUsageLog, 0)
	for page := 1; page <= 1000; page++ {
		query.Set("page", fmt.Sprint(page))
		endpoint.RawQuery = query.Encode()
		request, err := http.NewRequest(http.MethodGet, endpoint.String(), nil)
		if err != nil {
			return nil, "error", "failed to create Sub2API request"
		}
		request.Header.Set("x-api-key", adminKey)
		response, err := client.Do(request)
		if err != nil {
			return nil, "error", "Sub2API request failed"
		}
		if response.StatusCode != http.StatusOK {
			response.Body.Close()
			return nil, "error", fmt.Sprintf("Sub2API returned HTTP %d", response.StatusCode)
		}
		var payload struct {
			Code int `json:"code"`
			Data struct {
				Items []sub2APIUsageLog `json:"items"`
				Pages int               `json:"pages"`
			} `json:"data"`
		}
		err = common.DecodeJson(response.Body, &payload)
		response.Body.Close()
		if err != nil || payload.Code != 0 || payload.Data.Pages < 1 {
			return nil, "error", "invalid Sub2API usage response"
		}
		for _, log := range payload.Data.Items {
			cost := log.accountCost()
			if log.AccountId < 0 || math.IsNaN(cost) || math.IsInf(cost, 0) || cost < 0 {
				return nil, "error", "invalid Sub2API account usage"
			}
			if startTimestamp > 0 && !log.CreatedAt.IsZero() && log.CreatedAt.Unix() < startTimestamp {
				continue
			}
			if endTimestamp > 0 && !log.CreatedAt.IsZero() && log.CreatedAt.Unix() > endTimestamp {
				continue
			}
			logs = append(logs, log)
		}
		if page >= payload.Data.Pages {
			return logs, "connected", ""
		}
	}
	return nil, "error", "Sub2API usage exceeds 1,000,000 records; select a shorter period"
}

func GetSub2APIUsage(startTimestamp int64, endTimestamp int64) Sub2APIUsageSnapshot {
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("SUB2API_BASE_URL")), "/")
	adminKey := strings.TrimSpace(os.Getenv("SUB2API_ADMIN_API_KEY"))
	if baseURL == "" || adminKey == "" {
		return Sub2APIUsageSnapshot{Status: "not_configured", Error: "Sub2API connection is not configured"}
	}

	endpoint, err := url.Parse(baseURL + "/api/v1/admin/usage/stats")
	if err != nil {
		return Sub2APIUsageSnapshot{Status: "error", Error: "invalid Sub2API base URL"}
	}
	query := endpoint.Query()
	query.Set("timezone", "UTC")
	if startTimestamp > 0 {
		query.Set("start_date", time.Unix(startTimestamp, 0).UTC().Format("2006-01-02"))
	}
	if endTimestamp > 0 {
		query.Set("end_date", time.Unix(endTimestamp, 0).UTC().Format("2006-01-02"))
	}
	endpoint.RawQuery = query.Encode()

	request, err := http.NewRequest(http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return Sub2APIUsageSnapshot{Status: "error", Error: "failed to create Sub2API request"}
	}
	request.Header.Set("x-api-key", adminKey)

	client := &http.Client{Timeout: 8 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return Sub2APIUsageSnapshot{Status: "error", Error: "Sub2API request failed"}
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return Sub2APIUsageSnapshot{
			Status: "error",
			Error:  fmt.Sprintf("Sub2API returned HTTP %d", response.StatusCode),
		}
	}

	var payload sub2APIUsageStatsResponse
	if err := common.DecodeJson(response.Body, &payload); err != nil {
		return Sub2APIUsageSnapshot{Status: "error", Error: "invalid Sub2API response"}
	}
	return Sub2APIUsageSnapshot{
		TotalCostUsd:       payload.Data.TotalCost,
		TotalActualCostUsd: payload.Data.TotalActualCost,
		Status:             "connected",
	}
}
