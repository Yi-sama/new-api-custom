package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOCMJProtocolAndTaskResult(t *testing.T) {
	assert.True(t, UsesOCMJProtocol("https://api.osodm.com/v1", "Midjourney v8.2-test", nil))
	assert.True(t, UsesOCMJProtocol("https://other.invalid", "mj-v8.2", map[string]interface{}{OCMJProtocolHeader: "mj-async"}))
	assert.False(t, UsesOCMJProtocol("https://api.uuyue.top", "mj-v8.2", nil))
	assert.False(t, UsesOCMJProtocol("https://api.osodm.com", "gpt-image-2", nil))
	adaptor := &OCMJAdaptor{}
	for _, state := range []string{model.TaskStatusSubmitted, model.TaskStatusQueued, model.TaskStatusInProgress, model.TaskStatusFailure} {
		info, err := adaptor.ParseTaskResult([]byte(fmt.Sprintf(`{"task_id":"mj-public-id","status":%q}`, state)))
		require.NoError(t, err)
		assert.Equal(t, state, info.Status)
	}
	for _, body := range []string{
		`{"task_id":"../wrong","status":"FAILURE"}`,
		`{"status":"SUCCESS","result":{"data":[{},{},{},{}]}}`,
		`{"task_id":"mj-id","status":"unknown"}`,
		`{"task_id":"mj-id","status":"SUCCESS","result":{"data":[{"url":"a"}]}}`,
		`{"task_id":"mj-id","status":"SUCCESS","result":{"data":[{},{},{},{}]}}`,
	} {
		_, err := adaptor.ParseTaskResult([]byte(body))
		require.Error(t, err)
	}
	info, err := adaptor.ParseTaskResult([]byte(`{"task_id":"mj-id","status":"SUCCESS","result":{"data":[{"url":"a"},{"b64_json":"b"},{"url":"c"},{"url":"d"}],"grid_url":"preview-only"}}`))
	require.NoError(t, err)
	assert.Equal(t, model.TaskStatusSuccess, info.Status)

	var calls int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		assert.Equal(t, "/v1/images/midjourney/tasks/mj-public-id", r.URL.Path)
		assert.Equal(t, "Bearer secret", r.Header.Get("Authorization"))
		assert.Empty(t, r.Header.Get(EmberMJProtocolHeader))
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"error":{"code":"task_not_found"}}`)
	}))
	defer upstream.Close()
	_, err = adaptor.FetchTask(upstream.URL+"/v1", "secret", map[string]any{"task_id": "mj-public-id", "context": context.Background()}, "")
	require.ErrorContains(t, err, "HTTP 404")
	assert.Equal(t, 1, calls)
}
