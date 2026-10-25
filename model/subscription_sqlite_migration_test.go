package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSubscriptionAccrualSQLiteBootstrapAndLegacyMigration(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		name := "fresh database"
		if legacy {
			name = "existing plan"
		}
		t.Run(name, func(t *testing.T) {
			original := DB
			db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
			require.NoError(t, err)
			DB = db
			t.Cleanup(func() {
				DB = original
				sqlDB, err := db.DB()
				require.NoError(t, err)
				require.NoError(t, sqlDB.Close())
			})
			if legacy {
				require.NoError(t, DB.Exec("CREATE TABLE subscription_plans (id INTEGER PRIMARY KEY, title TEXT NOT NULL, price_amount DECIMAL(10,6) NOT NULL)").Error)
				require.NoError(t, DB.Exec("INSERT INTO subscription_plans (id, title, price_amount) VALUES (1, 'Original plan', 10)").Error)
			}
			require.NoError(t, ensureSubscriptionPlanTableSQLite())
			require.NoError(t, ensureSubscriptionPlanTableSQLite())
			if legacy {
				var plan SubscriptionPlan
				require.NoError(t, DB.First(&plan, 1).Error)
				assert.Equal(t, "Original plan", plan.Title)
				assert.Equal(t, "full", plan.QuotaGrantMode)
				assert.Equal(t, "expire", plan.QuotaExpiryPolicy)
				assert.Zero(t, plan.DailyAmount)
			}
			plan := SubscriptionPlan{
				Title: "Daily migration fixture", PriceAmount: 10,
				DurationUnit: "day", DurationValue: 30, TotalAmount: 60,
				QuotaGrantMode: "daily", DailyAmount: 2, QuotaExpiryPolicy: "retain",
			}
			require.NoError(t, DB.Create(&plan).Error)
			var persisted SubscriptionPlan
			require.NoError(t, DB.First(&persisted, plan.Id).Error)
			assert.Equal(t, "daily", persisted.QuotaGrantMode)
			assert.Equal(t, "retain", persisted.QuotaExpiryPolicy)
			assert.EqualValues(t, 2, persisted.DailyAmount)
		})
	}
}
