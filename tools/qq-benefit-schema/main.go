package main

import (
	"fmt"
	"os"

	"github.com/QuantumNous/new-api/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Public slave nodes do not run the application's full schema migrations.
func main() {
	db, err := gorm.Open(postgres.Open(os.Getenv("SQL_DSN")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		panic("cannot connect to configured database")
	}
	if err := db.AutoMigrate(&model.QQBenefitRequest{}, &model.QQBenefitBinding{}); err != nil {
		panic("cannot migrate QQ benefit tables")
	}
	fmt.Println("QQ benefit tables ready; existing account tables unchanged")
}
