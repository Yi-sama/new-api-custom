package service

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestValidateVideoStudioParametersUsesConfiguredGroupAllowlist(t *testing.T) {
	originalDB := model.DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	t.Cleanup(func() {
		model.DB = originalDB
	})
	require.NoError(t, db.AutoMigrate(&model.Model{}))
	require.NoError(t, db.Create(&model.Model{
		ModelName:  "video-model",
		Parameters: `{"video":{"durations":[4],"resolutions":["720p"]},"groups":{"premium":{"video":{"durations":[8]}}}}`,
	}).Error)

	duration := 8
	require.NoError(t, ValidateVideoStudioParameters("premium", "video-model", "video", VideoStudioParameterValues{
		Duration:   &duration,
		Resolution: "720p",
	}))

	duration = 4
	err = ValidateVideoStudioParameters("premium", "video-model", "video", VideoStudioParameterValues{Duration: &duration})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duration")
}

func TestValidateVideoStudioParametersDoesNotCreateAnUnconfiguredBound(t *testing.T) {
	originalDB := model.DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	t.Cleanup(func() {
		model.DB = originalDB
	})
	require.NoError(t, db.AutoMigrate(&model.Model{}))
	require.NoError(t, db.Create(&model.Model{ModelName: "video-model", Parameters: `{"video":{"modes":["frames"]}}`}).Error)

	duration := 120
	require.NoError(t, ValidateVideoStudioParameters("default", "video-model", "video", VideoStudioParameterValues{Duration: &duration}))
}
