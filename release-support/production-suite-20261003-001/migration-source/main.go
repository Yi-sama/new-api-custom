package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/QuantumNous/new-api/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func verifySuiteSchema(db *gorm.DB) error {
	if !db.Migrator().HasColumn(&model.Model{}, "Parameters") {
		return fmt.Errorf("models.parameters is missing")
	}
	for _, table := range []any{&model.StudioJob{}, &model.TavernCredit{}} {
		if !db.Migrator().HasTable(table) {
			return fmt.Errorf("required suite table %T is missing", table)
		}
		statement := &gorm.Statement{DB: db}
		if err := statement.Parse(table); err != nil {
			return err
		}
		for _, field := range statement.Schema.Fields {
			if field.DBName != "" && !db.Migrator().HasColumn(table, field.DBName) {
				return fmt.Errorf("%s.%s is missing", statement.Schema.Table, field.DBName)
			}
		}
	}
	if !db.Migrator().HasIndex(&model.StudioJob{}, "idx_studio_request") {
		return fmt.Errorf("studio request idempotency index is missing")
	}
	return nil
}

func migrateSuiteSchema(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		// Only add the known delta; never AutoMigrate the live baseline tables.
		if !tx.Migrator().HasTable(&model.Model{}) || !tx.Migrator().HasTable(&model.ImageStudioCredit{}) {
			return fmt.Errorf("expected production baseline tables are missing")
		}
		if tx.Dialector.Name() == "postgres" {
			if err := tx.Exec("SET LOCAL lock_timeout = '5s'").Error; err != nil {
				return err
			}
		}
		for _, table := range []any{&model.StudioJob{}, &model.TavernCredit{}} {
			if !tx.Migrator().HasTable(table) {
				if err := tx.Migrator().CreateTable(table); err != nil {
					return err
				}
			}
		}
		if !tx.Migrator().HasColumn(&model.Model{}, "Parameters") {
			if err := tx.Migrator().AddColumn(&model.Model{}, "Parameters"); err != nil {
				return err
			}
		}
		return verifySuiteSchema(tx)
	})
}

func main() {
	apply := flag.Bool("apply", false, "apply the additive production schema delta")
	flag.Parse()
	dsn := os.Getenv("SQL_DSN")
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "SQL_DSN is required")
		os.Exit(1)
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to open production database")
		os.Exit(1)
	}
	sqlDB, err := db.DB()
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to access production connection")
		os.Exit(1)
	}
	defer sqlDB.Close()
	sqlDB.SetMaxOpenConns(1)
	if *apply {
		err = migrateSuiteSchema(db)
	} else {
		err = verifySuiteSchema(db)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("SUITE_SCHEMA_VERIFIED")
}
