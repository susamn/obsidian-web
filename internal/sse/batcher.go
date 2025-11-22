package sse

import (
	"context"
	"sync"
	"time"

	"github.com/susamn/obsidian-web/internal/logger"
)

// EventBatcher batches SSE events to prevent UI spam during bulk operations
type EventBatcher struct {
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	manager       *Manager
	eventChan     chan Event
	flushInterval time.Duration
	threshold     int // If events < threshold, send individually
}

// NewEventBatcher creates a new SSE event batcher
func NewEventBatcher(ctx context.Context, manager *Manager, flushInterval time.Duration, threshold int) *EventBatcher {
	batcherCtx, cancel := context.WithCancel(ctx)

	return &EventBatcher{
		ctx:           batcherCtx,
		cancel:        cancel,
		manager:       manager,
		eventChan:     make(chan Event, 1000),
		flushInterval: flushInterval,
		threshold:     threshold,
	}
}

// Start begins the batching process
func (b *EventBatcher) Start() {
	b.wg.Add(1)
	go b.run()

	logger.WithFields(map[string]interface{}{
		"flush_interval": b.flushInterval,
		"threshold":      b.threshold,
	}).Info("SSE event batcher started")
}

// Stop stops the batcher
func (b *EventBatcher) Stop() {
	b.cancel()
	close(b.eventChan)
	b.wg.Wait()
	logger.Info("SSE event batcher stopped")
}

// QueueEvent queues an event for batching
func (b *EventBatcher) QueueEvent(event Event) {
	select {
	case b.eventChan <- event:
		// Event queued
	case <-b.ctx.Done():
		// Batcher stopped
	default:
		// Channel full, drop event
		logger.WithFields(map[string]interface{}{
			"vault_id": event.VaultID,
			"path":     event.Path,
			"type":     event.Type,
		}).Warn("SSE batcher channel full, dropping event")
	}
}

// GetChannel returns the event channel for workers to send events
func (b *EventBatcher) GetChannel() chan Event {
	return b.eventChan
}

// run is the main batching loop
func (b *EventBatcher) run() {
	defer b.wg.Done()

	ticker := time.NewTicker(b.flushInterval)
	defer ticker.Stop()

	// Collect events by vault ID
	eventsByVault := make(map[string][]Event)

	for {
		select {
		case <-b.ctx.Done():
			// Flush remaining events before stopping
			b.flush(eventsByVault)
			return

		case event, ok := <-b.eventChan:
			if !ok {
				// Channel closed, flush and exit
				b.flush(eventsByVault)
				return
			}

			// Add event to vault's batch
			eventsByVault[event.VaultID] = append(eventsByVault[event.VaultID], event)

			// If batch gets large, flush immediately
			if len(eventsByVault[event.VaultID]) >= 100 {
				b.flushVault(event.VaultID, eventsByVault[event.VaultID])
				delete(eventsByVault, event.VaultID)
			}

		case <-ticker.C:
			// Periodic flush
			b.flush(eventsByVault)
			// Clear the map
			eventsByVault = make(map[string][]Event)
		}
	}
}

// flush sends all batched events
func (b *EventBatcher) flush(eventsByVault map[string][]Event) {
	if len(eventsByVault) == 0 {
		return
	}

	for vaultID, events := range eventsByVault {
		b.flushVault(vaultID, events)
	}
}

// flushVault flushes events for a specific vault
func (b *EventBatcher) flushVault(vaultID string, events []Event) {
	if len(events) == 0 {
		return
	}

	// If low volume, send events individually for better real-time experience
	if len(events) < b.threshold {
		for _, event := range events {
			b.manager.BroadcastFileEvent(vaultID, event.Path, event.Type)
		}
		logger.WithFields(map[string]interface{}{
			"vault_id": vaultID,
			"count":    len(events),
		}).Debug("Sent individual SSE events (low volume)")
		return
	}

	// High volume - send as bulk update
	b.manager.BroadcastBulkUpdate(vaultID, events)

	logger.WithFields(map[string]interface{}{
		"vault_id": vaultID,
		"count":    len(events),
	}).Debug("Sent bulk SSE update")
}
