package main

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type baselineModel struct {
	ID        int
	ModelName string
}

func (baselineModel) TableName() string { return "models" }

func TestSuiteMigrationPreservesProductionCreditsAndModelRows(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&baselineModel{}, &model.ImageStudioCredit{}))
	original := model.ImageStudioCredit{UserId: 42, Quota: 123456, GrantedQuota: 500000, CreatedTime: 1000}
	require.NoError(t, db.Create(&original).Error)
	require.NoError(t, db.Create(&baselineModel{ID: 7, ModelName: "production-model"}).Error)
	require.Error(t, verifySuiteSchema(db))
	assert.False(t, db.Migrator().HasTable(&model.StudioJob{}))

	require.NoError(t, migrateSuiteSchema(db))
	require.NoError(t, migrateSuiteSchema(db))
	require.NoError(t, verifySuiteSchema(db))

	var credit model.ImageStudioCredit
	require.NoError(t, db.First(&credit, 42).Error)
	assert.Equal(t, original, credit)
	var existing baselineModel
	require.NoError(t, db.First(&existing, 7).Error)
	assert.Equal(t, "production-model", existing.ModelName)
	for _, table := range []any{&model.StudioJob{}, &model.TavernCredit{}} {
		var count int64
		require.NoError(t, db.Model(table).Count(&count).Error)
		assert.Zero(t, count)
	}
}

func TestSuiteMigrationDoesNotRepairAnUnexpectedExistingTable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&baselineModel{}, &model.ImageStudioCredit{}))
	require.NoError(t, db.Exec("CREATE TABLE studio_jobs (id TEXT PRIMARY KEY)").Error)

	require.ErrorContains(t, migrateSuiteSchema(db), "studio_jobs.")

	assert.False(t, db.Migrator().HasTable(&model.TavernCredit{}))
	assert.False(t, db.Migrator().HasColumn(&model.Model{}, "Parameters"))
}
