package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestStudioJobTextParametersRoundTrip(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	previous := DB
	DB = db
	t.Cleanup(func() {
		DB = previous
		connection, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, connection.Close())
	})
	require.NoError(t, db.AutoMigrate(&StudioJob{}))
	parameters := `{"n":1,"size":"1024x1024","watermark":false,"metadata":{"label":"test"}}`
	job := &StudioJob{
		ID: "studio_text_parameters", UserID: 7, RequestID: "request-text",
		Kind: "image", Operation: "images/generations", Model: "test-image",
		Parameters: []byte(parameters), Status: StudioJobQueued,
	}
	stored, created, err := CreateStudioJob(job)
	require.NoError(t, err)
	require.True(t, created)
	assert.Equal(t, job.ID, stored.ID)

	// A string-backed TEXT value reproduces PostgreSQL's driver return type.
	require.NoError(t, db.Exec("UPDATE studio_jobs SET parameters = ? WHERE id = ?", parameters, job.ID).Error)
	loaded, err := FindStudioJob(7, job.ID)
	require.NoError(t, err)
	assert.JSONEq(t, parameters, string(loaded.Parameters))
	encoded, err := common.Marshal(loaded)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, common.Unmarshal(encoded, &payload))
	assert.IsType(t, map[string]any{}, payload["parameters"], "API parameters must remain a JSON object")
	_, err = FindStudioJob(8, job.ID)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)

	same, created, err := CreateStudioJob(job)
	require.NoError(t, err)
	assert.False(t, created)
	assert.Equal(t, job.ID, same.ID)
	found, err := FindStudioJobRequest(7, job.RequestID)
	require.NoError(t, err)
	assert.JSONEq(t, parameters, string(found.Parameters))
	jobs, total, err := ListStudioJobs(7, "image", &common.PageInfo{Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	assert.EqualValues(t, 1, total)
	assert.JSONEq(t, parameters, string(jobs[0].Parameters))

	claimed, err := ClaimStudioJob("text-regression")
	require.NoError(t, err)
	require.NotNil(t, claimed)
	assert.Equal(t, StudioJobRunning, claimed.Status)
	assert.JSONEq(t, parameters, string(claimed.Parameters))
	unfinished, err := UnfinishedStudioJobs()
	require.NoError(t, err)
	require.Len(t, unfinished, 1)
	claimed.Status = StudioJobSucceeded
	require.NoError(t, SaveStudioJobState(claimed))
	loaded, err = FindStudioJob(7, job.ID)
	require.NoError(t, err)
	assert.Equal(t, StudioJobSucceeded, loaded.Status)
	assert.JSONEq(t, parameters, string(loaded.Parameters))
}
