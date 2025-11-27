package web

import (
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/susamn/obsidian-web/internal/logger"
	"github.com/susamn/obsidian-web/internal/sse"
)

// handleSSE godoc
// @Summary SSE endpoint for real-time file updates
// @Description Server-Sent Events endpoint for receiving real-time file change notifications
// @Tags sse
// @Produce text/event-stream
// @Param vault path string true "Vault ID"
// @Success 200 {string} string "SSE stream"
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/sse/{vault} [get]
func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Extract vault ID from path
	vaultID := s.extractVaultIDFromPath(r.URL.Path, "/api/v1/sse/")
	if vaultID == "" {
		writeError(w, http.StatusBadRequest, "Vault ID required")
		return
	}

	// Verify vault exists
	_, ok := s.getVault(vaultID)
	if !ok {
		writeError(w, http.StatusNotFound, "Vault not found")
		return
	}

	// Get flusher before setting SSE headers
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "Streaming not supported")
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Generate client ID
	clientID := uuid.New().String()

	// Register client with SSE manager
	client := s.sseManager.RegisterClient(r.Context(), clientID, vaultID)
	defer s.sseManager.UnregisterClient(client)

	logger.WithFields(map[string]interface{}{
		"client_id": clientID,
		"vault_id":  vaultID,
	}).Info("SSE client connected")

	// Send initial connection message
	fmt.Fprintf(w, "event: connected\ndata: {\"client_id\":\"%s\",\"vault_id\":\"%s\"}\n\n", clientID, vaultID)
	flusher.Flush()

	// Stream events
	for {
		select {
		case <-r.Context().Done():
			// Client disconnected
			logger.WithField("client_id", clientID).Info("SSE client disconnected (context)")
			return

		case <-client.Ctx.Done():
			// Client cancelled
			logger.WithField("client_id", clientID).Info("SSE client disconnected (cancelled)")
			return

		case event, ok := <-client.Messages:
			if !ok {
				// Channel closed
				logger.WithField("client_id", clientID).Info("SSE client disconnected (channel closed)")
				return
			}

			// Format and send event
			sseData := sse.FormatSSE(event)
			if sseData != "" {
				fmt.Fprint(w, sseData)
				flusher.Flush()

				logger.WithFields(map[string]interface{}{
					"client_id":     clientID,
					"event_type":    event.Type,
					"change_count":  len(event.Changes),
					"pending_count": event.PendingCount,
				}).Debug("SSE event sent")
			}
		}
	}
}

// handleSSEStats godoc
// @Summary Get SSE connection statistics
// @Description Get statistics about active SSE connections
// @Tags sse
// @Produce json
// @Success 200 {object} object "SSE statistics"
// @Router /api/v1/sse/stats [get]
func (s *Server) handleSSEStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	stats := map[string]interface{}{
		"total_clients": s.sseManager.GetClientCount(),
		"vaults":        make(map[string]int),
	}

	// Get per-vault stats
	vaults := s.listVaults()
	vaultStats := make(map[string]int)
	for _, v := range vaults {
		vaultID := v.VaultID()
		count := s.sseManager.GetVaultClientCount(vaultID)
		if count > 0 {
			vaultStats[vaultID] = count
		}
	}
	stats["vaults"] = vaultStats

	writeSuccess(w, stats)
}
