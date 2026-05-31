package main

import (
	"net/http"
	"testing"

	"llm-gateway/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type flushProbeWriter struct {
	header  http.Header
	flushed bool
}

func (w *flushProbeWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *flushProbeWriter) Write(b []byte) (int, error) {
	return len(b), nil
}

func (w *flushProbeWriter) WriteHeader(statusCode int) {}

func (w *flushProbeWriter) Flush() {
	w.flushed = true
}

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

func TestResponseWriterWrapperPreservesFlush(t *testing.T) {
	probe := &flushProbeWriter{}
	wrapper := &responseWriterWrapper{ResponseWriter: probe}

	wrapper.Flush()

	if !probe.flushed {
		t.Fatal("expected wrapper to delegate Flush to underlying writer")
	}
}
