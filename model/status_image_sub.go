package model

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

type SubStatusGroup struct {
	Name    string `json:"name"`
	Ready   int    `json:"ready"`
	Total   int    `json:"total"`
	Cooling int    `json:"cooling"`
	Error   int    `json:"error"`
}

type SubStatusAccount struct {
	ID     int64    `json:"id"`
	Name   string   `json:"name"`
	Groups []string `json:"groups"`
	Status string   `json:"status"`
}

type SubStatusSummary struct {
	Status    string             `json:"status"`
	Error     string             `json:"error,omitempty"`
	CheckedAt int64              `json:"checked_at"`
	Groups    []SubStatusGroup    `json:"groups"`
	Accounts  []SubStatusAccount  `json:"accounts"`
}

// Decode only status fields. Account credentials, names, notes and errors stay upstream.
type subStatusAccount struct {
	ID                     int64      `json:"id"`
	Platform               string     `json:"platform"`
	Status                 string     `json:"status"`
	Schedulable            bool       `json:"schedulable"`
	GroupIDs               []int64    `json:"group_ids"`
	ExpiresAt              *int64     `json:"expires_at"`
	AutoPauseOnExpired     bool       `json:"auto_pause_on_expired"`
	RateLimitResetAt       *time.Time `json:"rate_limit_reset_at"`
	OverloadUntil          *time.Time `json:"overload_until"`
	TempUnschedulableUntil *time.Time `json:"temp_unschedulable_until"`
}

func (account subStatusAccount) reportStatus(now time.Time) string {
	if account.Status != "active" || (account.AutoPauseOnExpired && account.ExpiresAt != nil && *account.ExpiresAt <= now.Unix()) {
		return "failed"
	}
	for _, until := range []*time.Time{account.RateLimitResetAt, account.OverloadUntil, account.TempUnschedulableUntil} {
		if until != nil && until.After(now) {
			return "cooling"
		}
	}
	if !account.Schedulable {
		return "failed"
	}
	return "normal"
}

func GetSubStatusSummary(parent context.Context) SubStatusSummary {
	now := time.Now()
	report := SubStatusSummary{
		Status: "not_configured", CheckedAt: now.Unix(),
		Groups: []SubStatusGroup{}, Accounts: []SubStatusAccount{},
	}
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("SUB2API_BASE_URL")), "/")
	key := strings.TrimSpace(os.Getenv("SUB2API_ADMIN_API_KEY"))
	if base == "" || key == "" {
		report.Error = "Sub2API connection is not configured"
		return report
	}
	ctx, cancel := context.WithTimeout(parent, 20*time.Second)
	defer cancel()
	client := &http.Client{Timeout: 8 * time.Second}
	groupNames := make(map[int64]string)
	groups := make(map[string]*SubStatusGroup)
	accounts := make([]subStatusAccount, 0)
	for _, resource := range []string{"groups", "accounts"} {
		endpoint, err := url.Parse(base + "/api/v1/admin/" + resource)
		if err != nil || endpoint.Host == "" || (endpoint.Scheme != "http" && endpoint.Scheme != "https") {
			report.Status, report.Error = "error", "Invalid Sub2API connection"
			return report
		}
		for page := 1; page <= 100; page++ {
			query := endpoint.Query()
			query.Set("page", fmt.Sprint(page))
			query.Set("page_size", "100")
			query.Set("sort_by", "id")
			query.Set("sort_order", "asc")
			if resource == "accounts" {
				query.Set("lite", "1")
			}
			endpoint.RawQuery = query.Encode()
			request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
			if err != nil {
				report.Status, report.Error = "error", "Cannot create Sub2API status request"
				return report
			}
			request.Header.Set("x-api-key", key)
			response, err := client.Do(request)
			if err != nil {
				report.Status, report.Error = "error", "Sub2API status request failed"
				return report
			}
			if response.StatusCode != http.StatusOK {
				response.Body.Close()
				report.Status, report.Error = "error", fmt.Sprintf("Sub2API returned HTTP %d", response.StatusCode)
				return report
			}
			var payload struct {
				Code *int `json:"code"`
				Data struct {
					Items []struct {
						subStatusAccount
						Name string `json:"name"`
					} `json:"items"`
					Pages *int `json:"pages"`
				} `json:"data"`
			}
			err = common.DecodeJson(io.LimitReader(response.Body, 4<<20), &payload)
			response.Body.Close()
			if err != nil || payload.Code == nil || *payload.Code != 0 || payload.Data.Pages == nil || *payload.Data.Pages < 0 || *payload.Data.Pages > 100 {
				report.Status, report.Error = "error", "Invalid or incomplete Sub2API status response"
				return report
			}
			for _, item := range payload.Data.Items {
				if resource == "groups" {
					groupNames[item.ID] = item.Name
					groups[item.Name] = &SubStatusGroup{Name: item.Name}
				} else {
					accounts = append(accounts, item.subStatusAccount)
				}
			}
			if page >= *payload.Data.Pages {
				break
			}
		}
	}
	seen := make(map[int64]bool)
	for _, account := range accounts {
		if seen[account.ID] {
			continue
		}
		seen[account.ID] = true
		names := make(map[string]bool)
		for _, id := range account.GroupIDs {
			name := groupNames[id]
			if name == "" {
				name = fmt.Sprintf("Group #%d", id)
			}
			names[name] = true
		}
		if len(names) == 0 {
			names["Ungrouped"] = true
		}
		state := account.reportStatus(now)
		platform := map[string]string{"openai": "OpenAI", "anthropic": "Claude", "gemini": "Gemini", "antigravity": "Antigravity", "grok": "Grok"}[account.Platform]
		if platform == "" {
			platform = "Account"
		}
		entry := SubStatusAccount{
			ID: account.ID, Name: fmt.Sprintf("%s #%d", platform, account.ID),
			Groups: []string{}, Status: state,
		}
		for name := range names {
			group := groups[name]
			if group == nil {
				group = &SubStatusGroup{Name: name}
				groups[name] = group
			}
			group.Total++
			switch state {
			case "normal":
				group.Ready++
			case "cooling":
				group.Cooling++
			default:
				group.Error++
			}
			entry.Groups = append(entry.Groups, name)
		}
		sort.Strings(entry.Groups)
		report.Accounts = append(report.Accounts, entry)
	}
	for _, group := range groups {
		report.Groups = append(report.Groups, *group)
	}
	sort.Slice(report.Groups, func(i, j int) bool { return report.Groups[i].Name < report.Groups[j].Name })
	sort.Slice(report.Accounts, func(i, j int) bool { return report.Accounts[i].ID < report.Accounts[j].ID })
	report.Status = "connected"
	return report
}
