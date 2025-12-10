package communication

import (
	"context"
	"sync"
	"time"

	"github.com/delivery-station/ds/pkg/log"
	"github.com/hashicorp/go-hclog"
)

// EventType represents the type of event
type EventType string

const (
	EventPluginStarted  EventType = "plugin.started"
	EventPluginFinished EventType = "plugin.finished"
	EventPluginFailed   EventType = "plugin.failed"
	EventStateChanged   EventType = "state.changed"
	EventCustom         EventType = "custom"
)

// Event represents an event that can be published to the event bus
type Event struct {
	Type      EventType              `json:"type"`
	PluginID  string                 `json:"plugin_id"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

// EventHandler is a function that handles events
type EventHandler func(ctx context.Context, event *Event) error

// EventBus manages event publishing and subscription
type EventBus struct {
	mu          sync.RWMutex
	subscribers map[EventType][]EventHandler
	logger      hclog.Logger
	bufferSize  int
	eventChan   chan *Event
	done        chan struct{}
	wg          sync.WaitGroup
}

// NewEventBus creates a new event bus
func NewEventBus(logger hclog.Logger, bufferSize int) *EventBus {
	if logger == nil {
		logger = log.Named("eventbus")
	}
	if bufferSize <= 0 {
		bufferSize = 100
	}

	bus := &EventBus{
		subscribers: make(map[EventType][]EventHandler),
		logger:      logger,
		bufferSize:  bufferSize,
		eventChan:   make(chan *Event, bufferSize),
		done:        make(chan struct{}),
	}

	// Start event processor
	bus.wg.Add(1)
	go bus.processEvents()

	return bus
}

// Subscribe registers a handler for specific event type
func (b *EventBus) Subscribe(eventType EventType, handler EventHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.subscribers[eventType] = append(b.subscribers[eventType], handler)
	b.logger.Debug("Subscribed handler", "event_type", eventType)
}

// SubscribeAll registers a handler for all event types
func (b *EventBus) SubscribeAll(handler EventHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Subscribe to all known event types
	allTypes := []EventType{
		EventPluginStarted,
		EventPluginFinished,
		EventPluginFailed,
		EventStateChanged,
		EventCustom,
	}

	for _, eventType := range allTypes {
		b.subscribers[eventType] = append(b.subscribers[eventType], handler)
	}

	b.logger.Debug("Subscribed handler for all event types")
}

// Publish publishes an event to all subscribers
func (b *EventBus) Publish(ctx context.Context, event *Event) error {
	event.Timestamp = time.Now()

	select {
	case b.eventChan <- event:
		b.logger.Debug("Published event", "type", event.Type, "plugin_id", event.PluginID)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(5 * time.Second):
		b.logger.Warn("Event publish timeout", "type", event.Type)
		return context.DeadlineExceeded
	}
}

// processEvents processes events from the channel
func (b *EventBus) processEvents() {
	defer b.wg.Done()

	for {
		select {
		case event := <-b.eventChan:
			b.handleEvent(event)
		case <-b.done:
			// Drain remaining events
			for len(b.eventChan) > 0 {
				event := <-b.eventChan
				b.handleEvent(event)
			}
			return
		}
	}
}

// handleEvent dispatches event to all registered handlers
func (b *EventBus) handleEvent(event *Event) {
	b.mu.RLock()
	handlers := b.subscribers[event.Type]
	b.mu.RUnlock()

	if len(handlers) == 0 {
		b.logger.Debug("No subscribers for event type", "type", event.Type)
		return
	}

	ctx := context.Background()
	for _, handler := range handlers {
		// Run handler with timeout
		done := make(chan error, 1)
		go func(h EventHandler) {
			done <- h(ctx, event)
		}(handler)

		select {
		case err := <-done:
			if err != nil {
				b.logger.Error("Event handler failed", "type", event.Type, "error", err)
			}
		case <-time.After(10 * time.Second):
			b.logger.Warn("Event handler timeout", "type", event.Type)
		}
	}
}

// Close shuts down the event bus
func (b *EventBus) Close() {
	close(b.done)
	b.wg.Wait()
	close(b.eventChan)
	b.logger.Debug("Event bus closed")
}
