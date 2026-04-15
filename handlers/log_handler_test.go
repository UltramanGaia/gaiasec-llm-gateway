package handlers

import (
	"net/http/httptest"
	"testing"
)

func TestLogQueryValueSupportsSnakeCase(t *testing.T) {
	request := httptest.NewRequest("GET", "/api/request-logs?backend_config_id=7&backendConfigId=9", nil)

	value := queryValue(request, "backend_config_id", "backendConfigId")

	if value != "7" {
		t.Fatalf("expected snake_case value to win, got %q", value)
	}
}
