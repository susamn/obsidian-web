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
	EventFileCreated  EventType = "file_created"
	EventFileModified EventType = "file_modified"
	EventFileDeleted  EventType = "file_deleted"
	EventTreeRefresh  EventType = "tree_refresh"
	EventBulkUpdate   EventType = "bulk_update"
	EventPing         EventType = "ping"
)

// FileEventData contains metadata about a file event for targeted UI updates
type FileEventData struct {
	Name       string `json:"name,omitempty"`        // File/directory name
	IsDir      bool   `json:"is_dir,omitempty"`      // Whether it's a directory
	IsMarkdown bool   `json:"is_markdown,omitempty"` // Whether it's a markdown file
	ParentPath string `json:"parent_path,omitempty"` // Path of parent directory
	Size       int64  `json:"size,omitempty"`        // File size in bytes
	ModTime    int64  `json:"mod_time,omitempty"`    // Last modification time (Unix timestamp)
}

// Event represents an SSE event to be sent to clients
type Event struct {
	Type      EventType              `json:"type"`
	VaultID   string                 `json:"vault_id"`
	Path      string                 `json:"path,omitempty"`    // Relative path only (NEVER absolute!)
	FileID    string                 `json:"file_id,omitempty"` // DB ID for fetching content
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data,omitempty"`      // Legacy: generic data map
	FileData  *FileEventData         `json:"file_data,omitempty"` // Rich file event metadata for UI updates
	Changes   []EventChange          `json:"changes,omitempty"`   // For bulk updates
	Summary   *EventSummary          `json:"summary,omitempty"`   // Summary for bulk updates
}

// EventChange represents a single change in a bulk update
type EventChange struct {
	Type   EventType `json:"type"`
	Path   string    `json:"path"`    // Relative path only
	FileID string    `json:"file_id"` // DB ID
}

// EventSummary summarizes bulk changes
type EventSummary struct {
	Created  int `json:"created"`
	Modified int `json:"modified"`
	Deleted  int `json:"deleted"`
}

// Client represents a connected SSE client
type Client struct {
	ID       string
	VaultID  string
	Messages chan Event
	Ctx      context.Context
	cancel   context.CancelFunc
}

// Manager manages SSE connections and broadcasts events
type Manager struct {
	clients   map[string]*Client
	clientsMu sync.RWMutex

	// Index by vault ID for efficient broadcasting
	vaultClients   map[string]map[string]*Client
	vaultClientsMu sync.RWMutex

	register   chan *Client
	unregister chan *Client
	broadcast  chan Event

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewManager creates a new SSE manager
func NewManager(ctx context.Context) *Manager {
	mgrCtx, cancel := context.WithCancel(ctx)

	return &Manager{
		clients:      make(map[string]*Client),
		vaultClients: make(map[string]map[string]*Client),
		register:     make(chan *Client, 10),
		unregister:   make(chan *Client, 10),
		broadcast:    make(chan Event, 100),
		ctx:          mgrCtx,
		cancel:       cancel,
	}
}

// Start starts the SSE manager
func (m *Manager) Start() {
	m.wg.Add(1)
	go m.run()

	// Start ping goroutine to keep connections alive
	m.wg.Add(1)
	go m.pingClients()

	logger.Info("SSE manager started")
}

// Stop stops the SSE manager
func (m *Manager) Stop() {
	m.cancel()

	// Wait for run goroutine to finish before closing channels
	// This ensures all pending messages are processed
	m.wg.Wait()

	// Now safe to close channels
	close(m.register)
	close(m.unregister)
	close(m.broadcast)

	// Close all client connections
	m.clientsMu.Lock()
	for _, client := range m.clients {
		client.cancel()
		close(client.Messages)
	}
	m.clientsMu.Unlock()

	logger.Info("SSE manager stopped")
}

// run is the main event loop
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

		case event := <-m.broadcast:
			m.broadcastEvent(event)
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

// broadcastEvent sends an event to all relevant clients
func (m *Manager) broadcastEvent(event Event) {
	// Get clients for this vault
	m.vaultClientsMu.RLock()
	vaultClients := m.vaultClients[event.VaultID]
	m.vaultClientsMu.RUnlock()

	if len(vaultClients) == 0 {
		return
	}

	logger.WithFields(map[string]interface{}{
		"vault_id":     event.VaultID,
		"event_type":   event.Type,
		"path":         event.Path,
		"client_count": len(vaultClients),
	}).Debug("Broadcasting SSE event")

	// Send to all clients for this vault
	for _, client := range vaultClients {
		// Check if client is still active before sending
		select {
		case <-client.Ctx.Done():
			// Client has been cancelled, skip
			continue
		default:
			// Client is still active, try to send
		}

		select {
		case client.Messages <- event:
			// Sent successfully
		case <-time.After(100 * time.Millisecond):
			// Client channel full or blocked, skip
			logger.WithField("client_id", client.ID).Warn("SSE client message channel full")
		case <-client.Ctx.Done():
			// Client disconnected while waiting
			continue
		}
	}
}

// pingClients sends periodic ping events to keep connections alive
func (m *Manager) pingClients() {
	defer m.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return

		case <-ticker.C:
			m.clientsMu.RLock()
			clients := make([]*Client, 0, len(m.clients))
			for _, client := range m.clients {
				clients = append(clients, client)
			}
			m.clientsMu.RUnlock()

			// Send ping to all clients
			for _, client := range clients {
				select {
				case client.Messages <- Event{
					Type:      EventPing,
					Timestamp: time.Now(),
				}:
				default:
					// Skip if channel is full
				}
			}
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

// BroadcastFileEvent broadcasts a file change event
func (m *Manager) BroadcastFileEvent(vaultID, path string, eventType interface{}) {
	// Convert interface{} to EventType
	var evtType EventType
	switch v := eventType.(type) {
	case EventType:
		evtType = v
	case string:
		evtType = EventType(v)
	default:
		logger.WithFields(map[string]interface{}{
			"vault_id":   vaultID,
			"event_type": eventType,
		}).Warn("Invalid event type for SSE broadcast")
		return
	}

	event := Event{
		Type:      evtType,
		VaultID:   vaultID,
		Path:      path,
		Timestamp: time.Now(),
	}

	select {
	case m.broadcast <- event:
		// Broadcast queued
	case <-m.ctx.Done():
		// Manager stopped
	default:
		// Broadcast channel full, log warning
		logger.WithFields(map[string]interface{}{
			"vault_id":   vaultID,
			"event_type": eventType,
			"path":       path,
		}).Warn("SSE broadcast channel full")
	}
}

// BroadcastFileEventWithData broadcasts a file change event with rich metadata
func (m *Manager) BroadcastFileEventWithData(vaultID, path string, eventType EventType, fileData *FileEventData) {
	event := Event{
		Type:      eventType,
		VaultID:   vaultID,
		Path:      path,
		Timestamp: time.Now(),
		FileData:  fileData,
	}

	select {
	case m.broadcast <- event:
		// Broadcast queued
	case <-m.ctx.Done():
		// Manager stopped
	default:
		// Broadcast channel full, log warning
		logger.WithFields(map[string]interface{}{
			"vault_id":   vaultID,
			"event_type": eventType,
			"path":       path,
		}).Warn("SSE broadcast channel full")
	}
}

// BroadcastBulkUpdate broadcasts a bulk update event with multiple changes
func (m *Manager) BroadcastBulkUpdate(vaultID string, events []Event) {
	if len(events) == 0 {
		return
	}

	// Build summary and changes
	summary := &EventSummary{}
	changes := make([]EventChange, 0, len(events))

	for _, evt := range events {
		changes = append(changes, EventChange{
			Type:   evt.Type,
			Path:   evt.Path,   // Already relative from worker
			FileID: evt.FileID, // DB ID from worker
		})

		switch evt.Type {
		case EventFileCreated:
			summary.Created++
		case EventFileModified:
			summary.Modified++
		case EventFileDeleted:
			summary.Deleted++
		}
	}

	bulkEvent := Event{
		Type:      EventBulkUpdate,
		VaultID:   vaultID,
		Timestamp: time.Now(),
		Changes:   changes,
		Summary:   summary,
	}

	select {
	case m.broadcast <- bulkEvent:
		logger.WithFields(map[string]interface{}{
			"vault_id": vaultID,
			"count":    len(events),
			"created":  summary.Created,
			"modified": summary.Modified,
			"deleted":  summary.Deleted,
		}).Debug("Broadcast bulk update")
	case <-m.ctx.Done():
		// Manager stopped
	default:
		logger.WithFields(map[string]interface{}{
			"vault_id": vaultID,
			"count":    len(events),
		}).Warn("SSE broadcast channel full, bulk update dropped")
	}
}

// BroadcastTreeRefresh broadcasts a tree refresh event
func (m *Manager) BroadcastTreeRefresh(vaultID, path string) {
	m.BroadcastFileEvent(vaultID, path, EventTreeRefresh)
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

// FormatSSE formats an event as SSE protocol
func FormatSSE(event Event) string {
	data, err := json.Marshal(event)
	if err != nil {
		logger.WithError(err).Error("Failed to marshal SSE event")
		return ""
	}

	return fmt.Sprintf("event: %s\ndata: %s\n\n", event.Type, string(data))
}
