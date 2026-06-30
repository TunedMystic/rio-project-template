package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleKit_OK(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/kit", nil)
	rec := httptest.NewRecorder()
	HandleKit().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Foundations") {
		t.Error("response missing kit content")
	}
}
