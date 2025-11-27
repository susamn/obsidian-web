package sse

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/susamn/obsidian-web/internal/logger"
)

// EventType represents the type of SSE event
type EventType string

const (
	EventBulkProcess EventType = "bulk_process"
	EventPing        EventType = "ping"
	EventRefresh     EventType = "refresh"
	EventError       EventType = "error"
)

// ActionType represents the action performed on a file
type ActionType string

const (
	ActionCreate ActionType = "create"
	ActionDelete ActionType = "delete"
	ActionMove   ActionType = "move"
)

// FileChange represents a single file change
type FileChange struct {
	ID     string     `json:"id"`     // DB ID
	Path   string     `json:"path"`   // Relative path
	Action ActionType `json:"action"` // create, delete, move
}

// Event represents an SSE event to be sent to clients
type Event struct {
	Type         EventType    `json:"type"`                    // bulk_process, ping, refresh, error
	VaultID      string       `json:"vault_id"`                // Vault identifier
	PendingCount int          `json:"pending_count"`           // Current count of pending events
	Changes      []FileChange `json:"changes,omitempty"`       // List of file changes (for bulk_process)
	ErrorMessage string       `json:"error_message,omitempty"` // Error message (for error type)
	Timestamp    time.Time    `json:"timestamp"`               // Event timestamp
}

// Client represents a connected SSE client
type Client struct {
	ID       string
	VaultID  string
	Messages chan Event
	Ctx      context.Context
	cancel   context.CancelFunc
}

// PendingCountGetter is a function that returns the pending events count for a vault
type PendingCountGetter func() int

// Manager manages SSE connections and broadcasts events
type Manager struct {
	clients   map[string]*Client
	clientsMu sync.RWMutex

	// Index by vault ID for efficient broadcasting
	vaultClients   map[string]map[string]*Client
	vaultClientsMu sync.RWMutex

	// Event queue per vault - workers fill this
	eventQueues   map[string][]FileChange
	eventQueuesMu sync.RWMutex

	// Error messages per vault
	errorMessages   map[string]string
	errorMessagesMu sync.RWMutex

	// Pending count getters per vault
	pendingCountGetters   map[string]PendingCountGetter
	pendingCountGettersMu sync.RWMutex

	register   chan *Client
	unregister chan *Client

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewManager creates a new SSE manager
func NewManager(ctx context.Context) *Manager {
	mgrCtx, cancel := context.WithCancel(ctx)

	return &Manager{
		clients:             make(map[string]*Client),
		vaultClients:        make(map[string]map[string]*Client),
		eventQueues:         make(map[string][]FileChange),
		errorMessages:       make(map[string]string),
		pendingCountGetters: make(map[string]PendingCountGetter),
		register:            make(chan *Client, 10),
		unregister:          make(chan *Client, 10),
		ctx:                 mgrCtx,
		cancel:              cancel,
	}
}

// Start starts the SSE manager
func (m *Manager) Start() {
	m.wg.Add(1)
	go m.run()

	// Start flush goroutine to send events every 2 seconds
	m.wg.Add(1)
	go m.flushEvents()

	logger.Info("SSE manager started")
}

// Stop stops the SSE manager
func (m *Manager) Stop() {
	m.cancel()

	// Wait for goroutines to finish
	m.wg.Wait()

	// Now safe to close channels
	close(m.register)
	close(m.unregister)

	// Close all client connections
	m.clientsMu.Lock()
	for _, client := range m.clients {
		client.cancel()
		close(client.Messages)
	}
	m.clientsMu.Unlock()

	logger.Info("SSE manager stopped")
}

// run is the main event loop for client registration/unregistration
func (m *Manager) run() {
	defer m.wg.Done()

	for {
		select {
		case <-m.ctx.Done():
			return

		case client := <-m.register:
			m.registerClient(client)

		case client := <-m.unregister:
			m.unregisterClient(client)
		}
	}
}

// registerClient registers a new client
func (m *Manager) registerClient(client *Client) {
	m.clientsMu.Lock()
	m.clients[client.ID] = client
	m.clientsMu.Unlock()

	// Index by vault ID
	m.vaultClientsMu.Lock()
	if _, exists := m.vaultClients[client.VaultID]; !exists {
		m.vaultClients[client.VaultID] = make(map[string]*Client)
	}
	m.vaultClients[client.VaultID][client.ID] = client
	m.vaultClientsMu.Unlock()

	logger.WithFields(map[string]interface{}{
		"client_id": client.ID,
		"vault_id":  client.VaultID,
	}).Info("SSE client connected")
}

// unregisterClient removes a client
func (m *Manager) unregisterClient(client *Client) {
	m.clientsMu.Lock()
	if _, exists := m.clients[client.ID]; exists {
		delete(m.clients, client.ID)
		client.cancel()
		close(client.Messages)
	}
	m.clientsMu.Unlock()

	// Remove from vault index
	m.vaultClientsMu.Lock()
	if vaultClients, exists := m.vaultClients[client.VaultID]; exists {
		delete(vaultClients, client.ID)
		if len(vaultClients) == 0 {
			delete(m.vaultClients, client.VaultID)
		}
	}
	m.vaultClientsMu.Unlock()

	logger.WithFields(map[string]interface{}{
		"client_id": client.ID,
		"vault_id":  client.VaultID,
	}).Info("SSE client disconnected")
}

// flushEvents sends events to clients every 2 seconds
func (m *Manager) flushEvents() {
	defer m.wg.Done()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return

		case <-ticker.C:
			// Get all vaults that have clients
			m.vaultClientsMu.RLock()
			vaultIDs := make([]string, 0, len(m.vaultClients))
			for vaultID := range m.vaultClients {
				vaultIDs = append(vaultIDs, vaultID)
			}
			m.vaultClientsMu.RUnlock()

			// Process each vault
			for _, vaultID := range vaultIDs {
				m.sendEventForVault(vaultID)
			}
		}
	}
}

// sendEventForVault sends an event for a specific vault
func (m *Manager) sendEventForVault(vaultID string) {
	// Get pending count
	pendingCount := m.getPendingCount(vaultID)

	// Check for error message
	m.errorMessagesMu.RLock()
	errorMsg := m.errorMessages[vaultID]
	m.errorMessagesMu.RUnlock()

	// If there's an error, send error event and clear it
	if errorMsg != "" {
		event := Event{
			Type:         EventError,
			VaultID:      vaultID,
			PendingCount: pendingCount,
			ErrorMessage: errorMsg,
			Timestamp:    time.Now(),
		}

		m.errorMessagesMu.Lock()
		delete(m.errorMessages, vaultID)
		m.errorMessagesMu.Unlock()

		m.broadcastToVault(vaultID, event)
		return
	}

	// Get and clear event queue
	m.eventQueuesMu.Lock()
	changes := m.eventQueues[vaultID]
	m.eventQueues[vaultID] = nil
	m.eventQueuesMu.Unlock()

	var event Event

	// If there are changes, send bulk_process
	if len(changes) > 0 {
		event = Event{
			Type:         EventBulkProcess,
			VaultID:      vaultID,
			PendingCount: pendingCount,
			Changes:      changes,
			Timestamp:    time.Now(),
		}
	} else {
		// No changes, send ping
		event = Event{
			Type:         EventPing,
			VaultID:      vaultID,
			PendingCount: pendingCount,
			Timestamp:    time.Now(),
		}
	}

	m.broadcastToVault(vaultID, event)
}

// broadcastToVault sends an event to all clients of a specific vault
func (m *Manager) broadcastToVault(vaultID string, event Event) {
	// Get clients for this vault
	m.vaultClientsMu.RLock()
	vaultClients := m.vaultClients[vaultID]
	m.vaultClientsMu.RUnlock()

	if len(vaultClients) == 0 {
		return
	}

	logger.WithFields(map[string]interface{}{
		"vault_id":     vaultID,
		"event_type":   event.Type,
		"change_count": len(event.Changes),
		"client_count": len(vaultClients),
	}).Debug("Broadcasting SSE event")

	// Send to all clients for this vault
	for _, client := range vaultClients {
		select {
		case <-client.Ctx.Done():
			continue
		default:
		}

		select {
		case client.Messages <- event:
			// Sent successfully
		case <-time.After(100 * time.Millisecond):
			logger.WithField("client_id", client.ID).Warn("SSE client message channel full")
		case <-client.Ctx.Done():
			continue
		}
	}
}

// RegisterClient registers a new SSE client
func (m *Manager) RegisterClient(ctx context.Context, clientID, vaultID string) *Client {
	clientCtx, cancel := context.WithCancel(ctx)

	client := &Client{
		ID:       clientID,
		VaultID:  vaultID,
		Messages: make(chan Event, 10),
		Ctx:      clientCtx,
		cancel:   cancel,
	}

	m.register <- client
	return client
}

// UnregisterClient removes a client
func (m *Manager) UnregisterClient(client *Client) {
	m.unregister <- client
}

// QueueFileChange adds a file change to the event queue for a vault
func (m *Manager) QueueFileChange(vaultID, fileID, path string, action ActionType) {
	m.eventQueuesMu.Lock()
	defer m.eventQueuesMu.Unlock()

	if m.eventQueues[vaultID] == nil {
		m.eventQueues[vaultID] = make([]FileChange, 0, 100)
	}

	m.eventQueues[vaultID] = append(m.eventQueues[vaultID], FileChange{
		ID:     fileID,
		Path:   path,
		Action: action,
	})
}

// SetError sets an error message for a vault
func (m *Manager) SetError(vaultID, errorMsg string) {
	m.errorMessagesMu.Lock()
	defer m.errorMessagesMu.Unlock()
	m.errorMessages[vaultID] = errorMsg
}

// TriggerRefresh sends a refresh event immediately
func (m *Manager) TriggerRefresh(vaultID string) {
	pendingCount := m.getPendingCount(vaultID)
	event := Event{
		Type:         EventRefresh,
		VaultID:      vaultID,
		PendingCount: pendingCount,
		Timestamp:    time.Now(),
	}
	m.broadcastToVault(vaultID, event)
}

// GetClientCount returns the number of connected clients
func (m *Manager) GetClientCount() int {
	m.clientsMu.RLock()
	defer m.clientsMu.RUnlock()
	return len(m.clients)
}

// GetVaultClientCount returns the number of clients for a specific vault
func (m *Manager) GetVaultClientCount(vaultID string) int {
	m.vaultClientsMu.RLock()
	defer m.vaultClientsMu.RUnlock()
	if clients, exists := m.vaultClients[vaultID]; exists {
		return len(clients)
	}
	return 0
}

// RegisterPendingCountGetter registers a function to get pending events count for a vault
func (m *Manager) RegisterPendingCountGetter(vaultID string, getter PendingCountGetter) {
	m.pendingCountGettersMu.Lock()
	defer m.pendingCountGettersMu.Unlock()
	m.pendingCountGetters[vaultID] = getter
}

// UnregisterPendingCountGetter removes the pending count getter for a vault
func (m *Manager) UnregisterPendingCountGetter(vaultID string) {
	m.pendingCountGettersMu.Lock()
	defer m.pendingCountGettersMu.Unlock()
	delete(m.pendingCountGetters, vaultID)
}

// getPendingCount gets the pending events count for a vault
func (m *Manager) getPendingCount(vaultID string) int {
	m.pendingCountGettersMu.RLock()
	defer m.pendingCountGettersMu.RUnlock()
	if getter, exists := m.pendingCountGetters[vaultID]; exists {
		return getter()
	}
	return 0
}

// FormatSSE formats an event as SSE protocol
func FormatSSE(event Event) string {
	data, err := json.Marshal(event)
	if err != nil {
		logger.WithError(err).Error("Failed to marshal SSE event")
		return ""
	}

	return fmt.Sprintf("event: %s\ndata: %s\n\n", event.Type, string(data))
}
