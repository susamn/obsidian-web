package web

import (
	"encoding/json"
	"net/http"
)

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// SuccessResponse represents a success response with data
type SuccessResponse struct {
	Data interface{} `json:"data"`
}

// writeJSON writes JSON response
func writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// writeError writes error response
func writeError(w http.ResponseWriter, statusCode int, message string) {
	writeJSON(w, statusCode, ErrorResponse{
		Error:   http.StatusText(statusCode),
		Message: message,
	})
}

// writeSuccess writes success response
func writeSuccess(w http.ResponseWriter, data interface{}) {
	writeJSON(w, http.StatusOK, SuccessResponse{Data: data})
}

func writeMarkdown(w http.ResponseWriter, data []byte) {
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}
