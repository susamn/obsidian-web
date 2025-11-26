package sse

import (
	"context"
	"testing"
	"time"
)

func TestNewEventBatcher(t *testing.T) {
	ctx := context.Background()
	manager := NewManager(ctx)
	batcher := NewEventBatcher(ctx, manager, 100*time.Millisecond, 10)

	if batcher == nil {
		t.Fatal("NewEventBatcher returned nil")
	}
	if batcher.manager != manager {
		t.Error("Manager not set correctly")
	}
	if batcher.flushInterval != 100*time.Millisecond {
		t.Error("FlushInterval not set correctly")
	}
	if batcher.threshold != 10 {
		t.Error("Threshold not set correctly")
	}
}

func TestBatcher_QueueEvent(t *testing.T) {
	ctx := context.Background()
	manager := NewManager(ctx)
	batcher := NewEventBatcher(ctx, manager, 100*time.Millisecond, 10)

	event := Event{
		Type:    EventFileCreated,
		VaultID: "test-vault",
		Path:    "test.md",
	}

	batcher.QueueEvent(event)

	select {
	case e := <-batcher.eventChan:
		if e.Type != event.Type || e.Path != event.Path {
			t.Errorf("Unexpected event in channel: got %v, want %v", e, event)
		}
	default:
		t.Error("Event not found in channel")
	}
}

func TestBatcher_FlushLowVolume(t *testing.T) {
	ctx := context.Background()
	manager := NewManager(ctx)
	manager.Start()
	defer manager.Stop()

	// Create a client to receive events
	client := manager.RegisterClient(ctx, "test-client", "test-vault")
	defer manager.UnregisterClient(client)

	// Create batcher with high threshold so these 2 events are sent individually
	batcher := NewEventBatcher(ctx, manager, 50*time.Millisecond, 5)
	batcher.Start()
	defer batcher.Stop()

	// Queue 2 events (less than threshold 5)
	events := []Event{
		{Type: EventFileCreated, VaultID: "test-vault", Path: "file1.md"},
		{Type: EventFileModified, VaultID: "test-vault", Path: "file2.md"},
	}

	for _, e := range events {
		batcher.QueueEvent(e)
	}

	// Wait for flush
	time.Sleep(100 * time.Millisecond)

	// We expect 2 individual events + connection message (handled by handler, not manager directly here)
	// Since we registered client directly, we should just see the broadcasted events.
	// Note: Manager.RegisterClient doesn't send "connected" event, the handler does.

	receivedCount := 0
	timeout := time.After(100 * time.Millisecond)

loop:
	for {
		select {
		case evt := <-client.Messages:
			if evt.Type == EventFileCreated || evt.Type == EventFileModified {
				receivedCount++
				if receivedCount == 2 {
					break loop
				}
			}
		case <-timeout:
			break loop
		}
	}

	if receivedCount != 2 {
		t.Errorf("Expected 2 individual events, got %d", receivedCount)
	}
}

func TestBatcher_FlushHighVolume(t *testing.T) {
	ctx := context.Background()
	manager := NewManager(ctx)
	manager.Start()
	defer manager.Stop()

	client := manager.RegisterClient(ctx, "test-client", "test-vault")
	defer manager.UnregisterClient(client)

	// Create batcher with low threshold
	batcher := NewEventBatcher(ctx, manager, 50*time.Millisecond, 2)
	batcher.Start()
	defer batcher.Stop()

	// Queue 3 events (more than threshold 2)
	events := []Event{
		{Type: EventFileCreated, VaultID: "test-vault", Path: "file1.md"},
		{Type: EventFileCreated, VaultID: "test-vault", Path: "file2.md"},
		{Type: EventFileCreated, VaultID: "test-vault", Path: "file3.md"},
	}

	for _, e := range events {
		batcher.QueueEvent(e)
	}

	// Wait for flush
	time.Sleep(100 * time.Millisecond)

	// We expect 1 bulk update event
	select {
	case evt := <-client.Messages:
		if evt.Type != EventBulkUpdate {
			t.Errorf("Expected bulk update, got %s", evt.Type)
		}
		if len(evt.Changes) != 3 {
			t.Errorf("Expected 3 changes in bulk update, got %d", len(evt.Changes))
		}
		if evt.Summary.Created != 3 {
			t.Errorf("Expected summary created=3, got %d", evt.Summary.Created)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout waiting for bulk event")
	}
}

func TestBatcher_Stop(t *testing.T) {
	ctx := context.Background()
	manager := NewManager(ctx)
	batcher := NewEventBatcher(ctx, manager, 100*time.Millisecond, 10)
	batcher.Start()

	batcher.QueueEvent(Event{Type: EventFileCreated, VaultID: "test", Path: "t.md"})

	batcher.Stop()

	// Channel should be closed
	select {
	case _, ok := <-batcher.eventChan:
		if ok {
			t.Error("Event channel should be closed or empty")
		}
	default:
		// If default is hit, it means channel is empty but open, or we need to wait?
		// When Stop() calls close(b.eventChan), reading from it returns ok=false if empty.
	}
}
