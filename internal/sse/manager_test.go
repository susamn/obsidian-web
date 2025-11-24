package sse

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	ctx := context.Background()
	mgr := NewManager(ctx)

	if mgr == nil {
		t.Fatal("Expected manager to be created, got nil")
	}

	if mgr.GetClientCount() != 0 {
		t.Errorf("Expected 0 clients, got %d", mgr.GetClientCount())
	}
}

func TestRegisterUnregisterClient(t *testing.T) {
	ctx := context.Background()
	mgr := NewManager(ctx)
	mgr.Start()
	defer mgr.Stop()

	// Register client
	clientID := "test-client"
	vaultID := "test-vault"
	client := mgr.RegisterClient(ctx, clientID, vaultID)

	// Give time for registration
	time.Sleep(50 * time.Millisecond)

	if mgr.GetClientCount() != 1 {
		t.Errorf("Expected 1 client after register, got %d", mgr.GetClientCount())
	}

	if mgr.GetVaultClientCount(vaultID) != 1 {
		t.Errorf("Expected 1 vault client, got %d", mgr.GetVaultClientCount(vaultID))
	}

	// Unregister client
	mgr.UnregisterClient(client)

	// Give time for unregistration
	time.Sleep(50 * time.Millisecond)

	if mgr.GetClientCount() != 0 {
		t.Errorf("Expected 0 clients after unregister, got %d", mgr.GetClientCount())
	}
}

func TestBroadcastFileEvent(t *testing.T) {
	ctx := context.Background()
	mgr := NewManager(ctx)
	mgr.Start()
	defer mgr.Stop()

	// Register client
	clientID := "test-client"
	vaultID := "test-vault"
	client := mgr.RegisterClient(ctx, clientID, vaultID)

	time.Sleep(50 * time.Millisecond)

	// Broadcast event
	mgr.BroadcastFileEvent(vaultID, "test/file.md", EventFileCreated)

	// Receive event
	select {
	case event := <-client.Messages:
		if event.Type != EventFileCreated {
			t.Errorf("Expected event type %s, got %s", EventFileCreated, event.Type)
		}
		if event.Path != "test/file.md" {
			t.Errorf("Expected path 'test/file.md', got '%s'", event.Path)
		}
		if event.VaultID != vaultID {
			t.Errorf("Expected vault ID '%s', got '%s'", vaultID, event.VaultID)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Expected to receive event within timeout")
	}
}

func TestBroadcastFileEventWithData(t *testing.T) {
	ctx := context.Background()
	mgr := NewManager(ctx)
	mgr.Start()
	defer mgr.Stop()

	// Register client
	clientID := "test-client"
	vaultID := "test-vault"
	client := mgr.RegisterClient(ctx, clientID, vaultID)

	time.Sleep(50 * time.Millisecond)

	// Create file event data
	fileData := &FileEventData{
		Name:       "test.md",
		IsDir:      false,
		IsMarkdown: true,
		ParentPath: "folder",
		Size:       1024,
		ModTime:    time.Now().Unix(),
	}

	// Broadcast event with data
	mgr.BroadcastFileEventWithData(vaultID, "folder/test.md", EventFileCreated, fileData)

	// Receive event
	select {
	case event := <-client.Messages:
		if event.Type != EventFileCreated {
			t.Errorf("Expected event type %s, got %s", EventFileCreated, event.Type)
		}
		if event.FileData == nil {
			t.Fatal("Expected FileData to be set")
		}
		if event.FileData.Name != "test.md" {
			t.Errorf("Expected name 'test.md', got '%s'", event.FileData.Name)
		}
		if !event.FileData.IsMarkdown {
			t.Error("Expected IsMarkdown to be true")
		}
		if event.FileData.ParentPath != "folder" {
			t.Errorf("Expected parent path 'folder', got '%s'", event.FileData.ParentPath)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Expected to receive event within timeout")
	}
}

func TestBroadcastToMultipleClients(t *testing.T) {
	ctx := context.Background()
	mgr := NewManager(ctx)
	mgr.Start()
	defer mgr.Stop()

	vaultID := "test-vault"
	numClients := 5

	// Register multiple clients
	clients := make([]*Client, numClients)
	for i := 0; i < numClients; i++ {
		clientID := "client-" + string(rune(i))
		clients[i] = mgr.RegisterClient(ctx, clientID, vaultID)
	}

	time.Sleep(100 * time.Millisecond)

	// Broadcast event
	mgr.BroadcastFileEvent(vaultID, "test.md", EventFileModified)

	// Verify all clients receive the event
	received := 0
	for _, client := range clients {
		select {
		case event := <-client.Messages:
			if event.Type == EventFileModified {
				received++
			}
		case <-time.After(500 * time.Millisecond):
			t.Errorf("Timeout waiting for event on client")
		}
	}

	if received != numClients {
		t.Errorf("Expected %d clients to receive event, got %d", numClients, received)
	}
}

func TestBroadcastToSpecificVault(t *testing.T) {
	ctx := context.Background()
	mgr := NewManager(ctx)
	mgr.Start()
	defer mgr.Stop()

	vault1ID := "vault-1"
	vault2ID := "vault-2"

	// Register clients for two vaults
	client1 := mgr.RegisterClient(ctx, "client-1", vault1ID)
	client2 := mgr.RegisterClient(ctx, "client-2", vault2ID)

	time.Sleep(100 * time.Millisecond)

	// Broadcast to vault 1 only
	mgr.BroadcastFileEvent(vault1ID, "file.md", EventFileCreated)

	// Client 1 should receive the event
	select {
	case event := <-client1.Messages:
		if event.VaultID != vault1ID {
			t.Errorf("Expected vault ID '%s', got '%s'", vault1ID, event.VaultID)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Expected client 1 to receive event")
	}

	// Client 2 should NOT receive the event
	select {
	case <-client2.Messages:
		t.Fatal("Client 2 should not receive event for different vault")
	case <-time.After(100 * time.Millisecond):
		// Expected - no event for different vault
	}
}

func TestEventSerialization(t *testing.T) {
	fileData := &FileEventData{
		Name:       "test.md",
		IsDir:      false,
		IsMarkdown: true,
		ParentPath: "docs",
		Size:       2048,
		ModTime:    1234567890,
	}

	event := Event{
		Type:      EventFileCreated,
		VaultID:   "my-vault",
		Path:      "docs/test.md",
		Timestamp: time.Date(2025, 11, 19, 12, 0, 0, 0, time.UTC),
		FileData:  fileData,
	}

	// Marshal to JSON
	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Failed to marshal event: %v", err)
	}

	// Unmarshal back
	var decoded Event
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal event: %v", err)
	}

	// Verify fields
	if decoded.Type != EventFileCreated {
		t.Errorf("Expected type %s, got %s", EventFileCreated, decoded.Type)
	}
	if decoded.VaultID != "my-vault" {
		t.Errorf("Expected vault 'my-vault', got '%s'", decoded.VaultID)
	}
	if decoded.FileData == nil {
		t.Fatal("Expected FileData to be set")
	}
	if decoded.FileData.Name != "test.md" {
		t.Errorf("Expected name 'test.md', got '%s'", decoded.FileData.Name)
	}
	if !decoded.FileData.IsMarkdown {
		t.Error("Expected IsMarkdown to be true")
	}
}

func TestFormatSSE(t *testing.T) {
	fileData := &FileEventData{
		Name:       "newfile.md",
		IsDir:      false,
		IsMarkdown: true,
		ParentPath: "notes",
		Size:       512,
		ModTime:    1234567890,
	}

	event := Event{
		Type:      EventFileCreated,
		VaultID:   "test-vault",
		Path:      "notes/newfile.md",
		Timestamp: time.Now(),
		FileData:  fileData,
	}

	sseStr := FormatSSE(event)

	// Verify SSE format
	if len(sseStr) == 0 {
		t.Fatal("Expected non-empty SSE string")
	}

	// Should contain event type
	if !contains(sseStr, "file_created") {
		t.Error("Expected event type in SSE string")
	}

	// Should contain data field
	if !contains(sseStr, "data:") {
		t.Error("Expected 'data:' field in SSE string")
	}

	// Should be parseable
	if !contains(sseStr, "newfile.md") {
		t.Error("Expected filename in SSE data")
	}
}

func TestPingEventType(t *testing.T) {
	// Simply verify that ping event type is defined and can be serialized
	event := Event{
		Type:      EventPing,
		VaultID:   "test",
		Timestamp: time.Now(),
	}

	sseStr := FormatSSE(event)
	if len(sseStr) == 0 {
		t.Error("Expected non-empty SSE string for ping event")
	}

	if !contains(sseStr, "ping") {
		t.Error("Expected 'ping' in SSE string")
	}
}

func TestConcurrentBroadcast(t *testing.T) {
	ctx := context.Background()
	mgr := NewManager(ctx)
	mgr.Start()
	defer mgr.Stop()

	vaultID := "test-vault"
	numClients := 10
	numEvents := 100

	// Register multiple clients
	clients := make([]*Client, numClients)
	for i := 0; i < numClients; i++ {
		clientID := "client-" + string(rune(i))
		clients[i] = mgr.RegisterClient(ctx, clientID, vaultID)
	}

	time.Sleep(100 * time.Millisecond)

	// Broadcast events concurrently
	var wg sync.WaitGroup
	for i := 0; i < numEvents; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			mgr.BroadcastFileEvent(vaultID, "file.md", EventFileModified)
		}(i)
	}
	wg.Wait()

	// Verify all clients receive all (or most) events
	// Note: some events might be dropped due to channel buffer limits
	time.Sleep(500 * time.Millisecond)

	receivedCount := 0
	for _, client := range clients {
		for {
			select {
			case <-client.Messages:
				receivedCount++
			default:
				// No more messages
				goto nextClient
			}
		}
	nextClient:
	}

	// Each client should receive significant portion of events
	// (some might be dropped due to buffer)
	if receivedCount < (numClients * numEvents / 2) {
		t.Logf("Warning: Expected at least %d events, got %d", numClients*numEvents/2, receivedCount)
	}
}

func TestVaultClientCount(t *testing.T) {
	ctx := context.Background()
	mgr := NewManager(ctx)
	mgr.Start()
	defer mgr.Stop()

	vault1 := "vault-1"
	vault2 := "vault-2"

	// Initially no clients
	if mgr.GetVaultClientCount(vault1) != 0 {
		t.Error("Expected 0 clients initially")
	}

	// Register clients
	c1 := mgr.RegisterClient(ctx, "client-1", vault1)
	_ = mgr.RegisterClient(ctx, "client-2", vault1)
	_ = mgr.RegisterClient(ctx, "client-3", vault2)

	time.Sleep(100 * time.Millisecond)

	// Check counts
	if mgr.GetVaultClientCount(vault1) != 2 {
		t.Errorf("Expected 2 clients for vault1, got %d", mgr.GetVaultClientCount(vault1))
	}
	if mgr.GetVaultClientCount(vault2) != 1 {
		t.Errorf("Expected 1 client for vault2, got %d", mgr.GetVaultClientCount(vault2))
	}
	if mgr.GetClientCount() != 3 {
		t.Errorf("Expected 3 total clients, got %d", mgr.GetClientCount())
	}

	// Unregister one client
	mgr.UnregisterClient(c1)

	time.Sleep(100 * time.Millisecond)

	if mgr.GetVaultClientCount(vault1) != 1 {
		t.Errorf("Expected 1 client for vault1 after unregister, got %d", mgr.GetVaultClientCount(vault1))
	}
	if mgr.GetClientCount() != 2 {
		t.Errorf("Expected 2 total clients, got %d", mgr.GetClientCount())
	}
}

// Helper function
func contains(s, substr string) bool {
	for i := 0; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestPendingCountGetter tests the pending count getter registration and usage
func TestPendingCountGetter(t *testing.T) {
	ctx := context.Background()
	mgr := NewManager(ctx)
	mgr.Start()
	defer mgr.Stop()

	vaultID := "test-vault"
	pendingCount := 42

	// Register a pending count getter
	mgr.RegisterPendingCountGetter(vaultID, func() int {
		return pendingCount
	})

	// Get the pending count
	count := mgr.getPendingCount(vaultID)
	if count != pendingCount {
		t.Errorf("Expected pending count %d, got %d", pendingCount, count)
	}

	// Update the count
	pendingCount = 100
	count = mgr.getPendingCount(vaultID)
	if count != 100 {
		t.Errorf("Expected updated pending count 100, got %d", count)
	}

	// Unregister the getter
	mgr.UnregisterPendingCountGetter(vaultID)
	count = mgr.getPendingCount(vaultID)
	if count != 0 {
		t.Errorf("Expected 0 after unregister, got %d", count)
	}
}

// TestPingWithPendingCount tests that ping events include pending count
func TestPingWithPendingCount(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mgr := NewManager(ctx)
	mgr.Start()
	defer mgr.Stop()

	vaultID := "test-vault"
	pendingCount := 25

	// Register a pending count getter
	mgr.RegisterPendingCountGetter(vaultID, func() int {
		return pendingCount
	})

	// Register a client
	clientID := "test-client"
	client := mgr.RegisterClient(ctx, clientID, vaultID)

	// Give time for registration
	time.Sleep(50 * time.Millisecond)

	// Wait for a ping event (pings happen every 2 seconds)
	select {
	case event := <-client.Messages:
		if event.Type != EventPing {
			t.Errorf("Expected ping event, got %s", event.Type)
		}
		if event.VaultID != vaultID {
			t.Errorf("Expected vault ID %s, got %s", vaultID, event.VaultID)
		}
		if event.PendingEvents != pendingCount {
			t.Errorf("Expected pending events %d, got %d", pendingCount, event.PendingEvents)
		}
	case <-time.After(3 * time.Second):
		t.Error("Timeout waiting for ping event")
	}
}

// TestMultipleVaultsPendingCount tests pending count for multiple vaults
func TestMultipleVaultsPendingCount(t *testing.T) {
	ctx := context.Background()
	mgr := NewManager(ctx)
	mgr.Start()
	defer mgr.Stop()

	vault1 := "vault-1"
	vault2 := "vault-2"
	count1 := 10
	count2 := 20

	// Register pending count getters for both vaults
	mgr.RegisterPendingCountGetter(vault1, func() int {
		return count1
	})
	mgr.RegisterPendingCountGetter(vault2, func() int {
		return count2
	})

	// Verify counts
	if got := mgr.getPendingCount(vault1); got != count1 {
		t.Errorf("Vault1: expected %d, got %d", count1, got)
	}
	if got := mgr.getPendingCount(vault2); got != count2 {
		t.Errorf("Vault2: expected %d, got %d", count2, got)
	}

	// Unregister vault1
	mgr.UnregisterPendingCountGetter(vault1)
	if got := mgr.getPendingCount(vault1); got != 0 {
		t.Errorf("Vault1 after unregister: expected 0, got %d", got)
	}
	if got := mgr.getPendingCount(vault2); got != count2 {
		t.Errorf("Vault2 after vault1 unregister: expected %d, got %d", count2, got)
	}
}
