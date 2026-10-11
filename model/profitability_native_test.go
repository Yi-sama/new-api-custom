package model

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProfitabilityNativeSummaryDoesNotReadOrMatchSubLogs(t *testing.T) {
	for _, mixed := range []bool{false, true} {
		t.Run(fmt.Sprintf("mixed=%v", mixed), func(t *testing.T) {
			setupProfitabilityBreakdownTest(t, "")
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "test-admin-key", r.Header.Get("x-api-key"))
				switch r.URL.Path {
				case "/api/v1/admin/groups":
					fmt.Fprint(w, `{"code":0,"data":{"pages":1,"items":[{"id":1}]}}`)
				case "/api/v1/admin/groups/1/api-keys":
					fmt.Fprint(w, `{"code":0,"data":{"pages":1,"items":[{"id":9,"key":"sub-key","group_id":1}]}}`)
				case "/api/v1/admin/usage/stats":
					require.Equal(t, "2026-10-01", r.URL.Query().Get("start_date"))
					require.Equal(t, "2026-10-31", r.URL.Query().Get("end_date"))
					cost := 47
					if r.URL.Query().Get("api_key_id") == "9" {
						cost = 42
					}
					fmt.Fprintf(w, `{"code":0,"data":{"total_account_cost":%d}}`, cost)
				default:
					t.Errorf("Unexpected endpoint: %s", r.URL.Path)
					http.Error(w, "No log enumeration allowed", http.StatusBadRequest)
				}
			}))
			t.Cleanup(server.Close)
			t.Setenv("SUB2API_BASE_URL", server.URL)
			t.Setenv("TZ", "UTC")
			start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC).Unix()
			end := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC).Unix() - 1
			base := server.URL
			require.NoError(t, DB.Create(&Channel{Id: 7, Name: "Sub", Key: "sub-key", BaseURL: &base}).Error)
			require.NoError(t, DB.Create(&Log{
				UserId: 1, Type: LogTypeConsume, ChannelId: 7, Group: "renamed-group",
				Quota: 1_000_000, CreatedAt: start + 100, Other: `{"group_ratio":0.2}`,
			}).Error)
			ids := []int{7}
			if mixed {
				other := "https://another-upstream.example"
				require.NoError(t, DB.Create(&Channel{Id: 8, Name: "CPR", Key: "other-key", BaseURL: &other}).Error)
				require.NoError(t, DB.Create(&Log{
					UserId: 1, Type: LogTypeConsume, ChannelId: 8, Group: "renamed-group",
					Quota: 500_000, CreatedAt: start + 101,
				}).Error)
				ids = append(ids, 8)
			}
			report, err := GetProfitabilityReportWithOptions(start, end, ProfitabilityReportOptions{
				ChannelSelectionPresent: true, ChannelIDs: ids, UseSubSummary: true,
			})
			require.NoError(t, err)
			assert.Equal(t, "summary", report.SubUsageSource)
			assert.Equal(t, "connected", report.SubUsageStatus)
			require.Len(t, report.Groups, 1)
			group := report.Groups[0]
			require.NotNil(t, group.AllCalls.SubUsageUsd)
			assert.Equal(t, 42.0, *group.AllCalls.SubUsageUsd)
			assert.Equal(t, 42.0, report.SubUsageUsd)
			assert.Equal(t, "complete", report.Channels[0].SubUsageStatus)
			assert.Equal(t, int64(1), report.Channels[0].RequestCount)
			assert.Nil(t, report.Models[0].EstimatedProfitRmb)
			assert.Nil(t, group.CustomerCalls.SubUsageUsd)
			if mixed {
				assert.Equal(t, "mixed_upstream", group.AllCalls.Status)
				assert.Nil(t, group.AllCalls.EffectiveRatio)
				assert.Equal(t, "mixed_upstream", report.ProfitStatus)
			} else {
				assert.Equal(t, "complete", group.AllCalls.Status)
				require.NotNil(t, group.AllCalls.EffectiveRatio)
				assert.InDelta(t, 2.0/42.0, *group.AllCalls.EffectiveRatio, 1e-9)
			}
			assert.Nil(t, group.AllCalls.BreakEvenRatio)
			assert.Zero(t, report.MatchedRequests)
			assert.Zero(t, report.UnmatchedRequests)
			require.NotNil(t, report.Pool.AllCalls.SubUsageUsd)
			assert.Equal(t, 47.0, *report.Pool.AllCalls.SubUsageUsd)
			assert.Nil(t, report.Pool.CustomerCalls.SubUsageUsd)
		})
	}
}

func TestSub2APIUsageSummaryDoesNotDuplicateSharedKeys(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/admin/groups":
			fmt.Fprint(w, `{"code":0,"data":{"pages":1,"items":[{"id":1}]}}`)
		case "/api/v1/admin/groups/1/api-keys":
			fmt.Fprint(w, `{"code":0,"data":{"pages":1,"items":[{"id":9,"key":"shared-key","group_id":1}]}}`)
		case "/api/v1/admin/usage/stats":
			fmt.Fprint(w, `{"code":0,"data":{"total_account_cost":10}}`)
		default:
			t.Errorf("Unexpected endpoint: %s", r.URL.Path)
			http.Error(w, "unexpected endpoint", http.StatusBadRequest)
		}
	}))
	t.Cleanup(server.Close)
	t.Setenv("SUB2API_BASE_URL", server.URL)
	t.Setenv("SUB2API_ADMIN_API_KEY", "test-admin-key")
	t.Setenv("TZ", "UTC")
	base := server.URL
	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC).Unix()
	end := time.Date(2026, 10, 31, 23, 59, 59, 0, time.UTC).Unix()

	summary, status, errText := getSub2APIUsageSummary(start, end, []Channel{
		{Id: 7, Key: "shared-key", BaseURL: &base},
		{Id: 8, Key: "shared-key", BaseURL: &base},
	})

	require.Equal(t, "connected", status)
	require.Empty(t, errText)
	assert.Equal(t, 10.0, summary.poolUsage)
	assert.Empty(t, summary.channelUsage)
}
