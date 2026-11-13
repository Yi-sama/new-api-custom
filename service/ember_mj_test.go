package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmberMJChannelProtocol(t *testing.T) {
	for _, tc := range []struct {
		base, name string
		headers    map[string]interface{}
		want       bool
	}{
		{"https://api.uuyue.top", "Midjourney-v8.2", nil, true},
		{"https://api.uuyue.top/v1", "gpt-image-2", nil, false},
		{"https://api.osodm.com", "mj-v8.2", nil, false},
		{"https://other.invalid", "Midjourney-v8.2", map[string]interface{}{"x-ember-image-protocol": "mj-proxy"}, true},
	} {
		assert.Equal(t, tc.want, UsesEmberMJProtocol(tc.base, tc.name, tc.headers))
	}
}

func TestEmberMJStates(t *testing.T) {
	adaptor := &EmberMJAdaptor{}
	for state, status := range map[string]string{
		"submitting": model.TaskStatusSubmitted, "pending": model.TaskStatusSubmitted,
		"uncertain": model.TaskStatusSubmitted, "processing": model.TaskStatusInProgress,
		"failed": model.TaskStatusFailure,
	} {
		info, err := adaptor.ParseTaskResult([]byte(fmt.Sprintf(`{"id":"original","status":%q}`, state)))
		require.NoError(t, err)
		assert.Equal(t, status, info.Status)
	}
	for _, body := range []string{
		`{"status":"unknown"}`, `{"status":"completed","result":{"data":[]}}`,
		`{"status":"completed","result":{"data":[{"index":1},{"index":2},{"index":3},{"index":3}]}}`,
	} {
		_, err := adaptor.ParseTaskResult([]byte(body))
		require.Error(t, err)
	}
}

func TestEmberMJDoesNotFollowRedirect(t *testing.T) {
	var received atomic.Int32
	sink := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
	}))
	defer sink.Close()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/images/generations", r.URL.Path)
		assert.Equal(t, "Bearer secret", r.Header.Get("Authorization"))
		http.Redirect(w, r, sink.URL, http.StatusTemporaryRedirect)
	}))
	defer upstream.Close()
	response, err := EmberMJRequest(context.Background(), http.MethodPost, upstream.URL+"/v1",
		"/images/generations", "secret", "", strings.NewReader("{}"), "application/json", "original")
	require.NoError(t, err)
	defer response.Body.Close()
	assert.Equal(t, http.StatusTemporaryRedirect, response.StatusCode)
	assert.Zero(t, received.Load(), "redirect must not replay a billable request or expose its key")
}
