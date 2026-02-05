package httputil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteJSONSetsContentTypeHeader(t *testing.T) {
	recorder := httptest.NewRecorder()

	WriteJSON(recorder, http.StatusOK, map[string]string{"key": "value"})

	contentType := recorder.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", contentType)
	}
}

func TestWriteJSONSetsStatusCode(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"OK", http.StatusOK},
		{"Created", http.StatusCreated},
		{"BadRequest", http.StatusBadRequest},
		{"InternalServerError", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()

			WriteJSON(recorder, tt.statusCode, map[string]string{"key": "value"})

			if recorder.Code != tt.statusCode {
				t.Errorf("expected status %d, got %d", tt.statusCode, recorder.Code)
			}
		})
	}
}

func TestWriteJSONEncodesBody(t *testing.T) {
	recorder := httptest.NewRecorder()
	payload := map[string]string{"name": "sendrec", "type": "platform"}

	WriteJSON(recorder, http.StatusOK, payload)

	var decoded map[string]string
	err := json.NewDecoder(recorder.Body).Decode(&decoded)
	if err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if decoded["name"] != "sendrec" {
		t.Errorf("expected name=sendrec, got %s", decoded["name"])
	}
	if decoded["type"] != "platform" {
		t.Errorf("expected type=platform, got %s", decoded["type"])
	}
}

func TestWriteJSONEncodesStructBody(t *testing.T) {
	type item struct {
		ID    int    `json:"id"`
		Title string `json:"title"`
	}

	recorder := httptest.NewRecorder()
	payload := item{ID: 42, Title: "test item"}

	WriteJSON(recorder, http.StatusCreated, payload)

	var decoded item
	err := json.NewDecoder(recorder.Body).Decode(&decoded)
	if err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if decoded.ID != 42 {
		t.Errorf("expected id=42, got %d", decoded.ID)
	}
	if decoded.Title != "test item" {
		t.Errorf("expected title=test item, got %s", decoded.Title)
	}
}

func TestWriteJSONEncodesSliceBody(t *testing.T) {
	recorder := httptest.NewRecorder()
	payload := []string{"alpha", "beta", "gamma"}

	WriteJSON(recorder, http.StatusOK, payload)

	var decoded []string
	err := json.NewDecoder(recorder.Body).Decode(&decoded)
	if err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if len(decoded) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(decoded))
	}
	if decoded[0] != "alpha" || decoded[1] != "beta" || decoded[2] != "gamma" {
		t.Errorf("unexpected slice contents: %v", decoded)
	}
}

func TestWriteErrorProducesCorrectJSON(t *testing.T) {
	recorder := httptest.NewRecorder()

	WriteError(recorder, http.StatusBadRequest, "invalid input")

	var decoded ErrorBody
	err := json.NewDecoder(recorder.Body).Decode(&decoded)
	if err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if decoded.Error != "invalid input" {
		t.Errorf("expected error=invalid input, got %s", decoded.Error)
	}
}

func TestWriteErrorSetsStatusCode(t *testing.T) {
	recorder := httptest.NewRecorder()

	WriteError(recorder, http.StatusForbidden, "forbidden")

	if recorder.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, recorder.Code)
	}
}

func TestWriteErrorSetsContentType(t *testing.T) {
	recorder := httptest.NewRecorder()

	WriteError(recorder, http.StatusInternalServerError, "something broke")

	contentType := recorder.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", contentType)
	}
}

func TestWriteErrorWithVariousMessages(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		message    string
	}{
		{"NotFound", http.StatusNotFound, "resource not found"},
		{"Unauthorized", http.StatusUnauthorized, "authentication required"},
		{"Conflict", http.StatusConflict, "duplicate entry"},
		{"InternalError", http.StatusInternalServerError, "unexpected server error"},
		{"EmptyMessage", http.StatusBadRequest, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()

			WriteError(recorder, tt.statusCode, tt.message)

			if recorder.Code != tt.statusCode {
				t.Errorf("expected status %d, got %d", tt.statusCode, recorder.Code)
			}

			var decoded ErrorBody
			err := json.NewDecoder(recorder.Body).Decode(&decoded)
			if err != nil {
				t.Fatalf("failed to decode response body: %v", err)
			}
			if decoded.Error != tt.message {
				t.Errorf("expected error=%q, got %q", tt.message, decoded.Error)
			}
		})
	}
}
