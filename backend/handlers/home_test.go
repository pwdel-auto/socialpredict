package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHomeHandler_ReturnsEnvelope(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v0/home", nil)
	rec := httptest.NewRecorder()

	HomeHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp SuccessEnvelope[HomeResponse]
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.OK {
		t.Fatalf("expected ok=true, got false")
	}
	if resp.Result.Message != "Data From the Backend!" {
		t.Fatalf("expected backend message, got %q", resp.Result.Message)
	}
}

func TestHomeHandler_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v0/home", nil)
	rec := httptest.NewRecorder()

	HomeHandler(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status 405, got %d", rec.Code)
	}

	var resp FailureEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Reason != string(ReasonMethodNotAllowed) {
		t.Fatalf("expected reason %q, got %q", ReasonMethodNotAllowed, resp.Reason)
	}
}
