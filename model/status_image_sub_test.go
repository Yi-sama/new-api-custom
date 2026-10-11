package model

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubStatusSummaryReadsAllPagesAndNeverExportsCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "private-test-key", r.Header.Get("x-api-key"))
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/groups") {
			_, _ = w.Write([]byte(`{"code":0,"data":{"items":[{"id":1,"name":"Pool A"},{"id":2,"name":"Empty pool"}],"pages":1}}`))
			return
		}
		require.Equal(t, "/api/v1/admin/accounts", r.URL.Path)
		require.Equal(t, "id", r.URL.Query().Get("sort_by"))
		require.Equal(t, "asc", r.URL.Query().Get("sort_order"))
		if r.URL.Query().Get("page") == "1" {
			_, _ = w.Write([]byte(`{"code":0,"data":{"items":[{"id":10,"name":"private@example.org","platform":"openai","status":"active","schedulable":true,"group_ids":[1],"credentials":{"access_token":"DO-NOT-EXPORT"}}],"pages":2}}`))
			return
		}
		_, _ = w.Write([]byte(`{"code":0,"data":{"items":[{"id":11,"platform":"anthropic","status":"active","schedulable":true,"group_ids":[],"overload_until":"2099-01-01T00:00:00Z"}],"pages":2}}`))
	}))
	defer server.Close()
	t.Setenv("SUB2API_BASE_URL", server.URL)
	t.Setenv("SUB2API_ADMIN_API_KEY", "private-test-key")

	report := GetSubStatusSummary(context.Background())
	require.Equal(t, "connected", report.Status)
	require.Len(t, report.Accounts, 2)
	require.Len(t, report.Groups, 3)
	assert.Equal(t, "OpenAI #10", report.Accounts[0].Name)
	assert.Equal(t, "normal", report.Accounts[0].Status)
	assert.Equal(t, "cooling", report.Accounts[1].Status)
	assert.Equal(t, 0, report.Groups[0].Total) // Empty pool is retained.
	raw, err := common.Marshal(report)
	require.NoError(t, err)
	for _, secret := range []string{"private@example.org", "DO-NOT-EXPORT", "private-test-key", "credentials"} {
		assert.NotContains(t, string(raw), secret)
	}
}

func TestSubStatusClassificationSeparatesCoolingDisabledAndExpired(t *testing.T) {
	now := time.Unix(100, 0)
	later := now.Add(time.Minute)
	expired := int64(99)
	cases := []struct {
		name string
		item subStatusAccount
		want string
	}{
		{"ready", subStatusAccount{Status: "active", Schedulable: true}, "normal"},
		{"cooling before false schedulable", subStatusAccount{Status: "active", RateLimitResetAt: &later}, "cooling"},
		{"overloaded", subStatusAccount{Status: "active", OverloadUntil: &later}, "cooling"},
		{"temporary hold", subStatusAccount{Status: "active", TempUnschedulableUntil: &later}, "cooling"},
		{"disabled", subStatusAccount{Status: "disabled", Schedulable: true}, "failed"},
		{"paused", subStatusAccount{Status: "active", Schedulable: false}, "failed"},
		{"expired", subStatusAccount{Status: "active", Schedulable: true, ExpiresAt: &expired, AutoPauseOnExpired: true}, "failed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.item.reportStatus(now))
		})
	}
}

func TestSubStatusSummaryRejectsFailedOrIncompleteResponses(t *testing.T) {
	for _, body := range []string{
		`{"code":0,"data":{"items":[],"pages":101}}`,
		`{"code":1,"message":"private upstream diagnostic"}`,
		`<html>login page</html>`,
	} {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(body))
			}))
			defer server.Close()
			t.Setenv("SUB2API_BASE_URL", server.URL)
			t.Setenv("SUB2API_ADMIN_API_KEY", "private-test-key")
			report := GetSubStatusSummary(context.Background())
			assert.Equal(t, "error", report.Status)
			assert.Empty(t, report.Accounts)
			assert.NotContains(t, report.Error, "private")
		})
	}
}
