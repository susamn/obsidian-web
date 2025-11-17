package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteJSON(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		data       interface{}
	}{
		{
			name:       "Success response",
			statusCode: http.StatusOK,
			data:       map[string]string{"message": "success"},
		},
		{
			name:       "Created response",
			statusCode: http.StatusCreated,
			data:       map[string]int{"id": 123},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			writeJSON(w, tt.statusCode, tt.data)

			if w.Code != tt.statusCode {
				t.Errorf("Expected status %d, got %d", tt.statusCode, w.Code)
			}

			contentType := w.Header().Get("Content-Type")
			if contentType != "application/json" {
				t.Errorf("Expected Content-Type application/json, got %s", contentType)
			}

			// Verify JSON is valid
			var result map[string]interface{}
			if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
				t.Errorf("Failed to decode response: %v", err)
			}
		})
	}
}

func TestWriteError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		message    string
	}{
		{
			name:       "Not found",
			statusCode: http.StatusNotFound,
			message:    "Resource not found",
		},
		{
			name:       "Bad request",
			statusCode: http.StatusBadRequest,
			message:    "Invalid input",
		},
		{
			name:       "Internal error",
			statusCode: http.StatusInternalServerError,
			message:    "Something went wrong",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			writeError(w, tt.statusCode, tt.message)

			if w.Code != tt.statusCode {
				t.Errorf("Expected status %d, got %d", tt.statusCode, w.Code)
			}

			var errResp ErrorResponse
			if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
				t.Fatalf("Failed to decode error response: %v", err)
			}

			if errResp.Message != tt.message {
				t.Errorf("Expected message '%s', got '%s'", tt.message, errResp.Message)
			}

			if errResp.Error != http.StatusText(tt.statusCode) {
				t.Errorf("Expected error '%s', got '%s'", http.StatusText(tt.statusCode), errResp.Error)
			}
		})
	}
}

func TestWriteSuccess(t *testing.T) {
	w := httptest.NewRecorder()
	data := map[string]string{"key": "value"}

	writeSuccess(w, data)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp SuccessResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify data field exists
	if resp.Data == nil {
		t.Error("Expected data field in response")
	}
}
