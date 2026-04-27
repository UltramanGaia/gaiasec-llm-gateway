package handlers

import (
	"net/http/httptest"
	"testing"
)

func TestLogQueryValueSupportsSnakeCase(t *testing.T) {
	request := httptest.NewRequest("GET", "/api/request-logs?backend_config_id=7", nil)

	value := queryValue(request, "backend_config_id")

	if value != "7" {
		t.Fatalf("expected snake_case value, got %q", value)
	}
}
