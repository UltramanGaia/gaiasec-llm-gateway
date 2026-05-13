package main

import (
	"testing"

	"llm-gateway/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestValidateDatabaseSchemaSucceedsForCurrentTables(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.ModelConfig{}, &models.RequestLog{}, &models.Session{}); err != nil {
		t.Fatalf("migrate schema: %v", err)
	}

	if err := validateDatabaseSchema(db); err != nil {
		t.Fatalf("validate schema: %v", err)
	}
}

func TestValidateDatabaseSchemaFailsWhenTableMissing(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.ModelConfig{}, &models.RequestLog{}); err != nil {
		t.Fatalf("migrate partial schema: %v", err)
	}

	if err := validateDatabaseSchema(db); err == nil {
		t.Fatal("expected schema validation to fail when sessions table is missing")
	}
}
