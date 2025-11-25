package communication

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEventBus(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	bus := NewEventBus(logger, 10)
	require.NotNil(t, bus)
	assert.Equal(t, 10, bus.bufferSize)

	bus.Close()
}

func TestEventBusSubscribe(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	bus := NewEventBus(logger, 10)
	defer bus.Close()

	var mu sync.Mutex
	called := false
	handler := func(ctx context.Context, event *Event) error {
		mu.Lock()
		called = true
		mu.Unlock()
		return nil
	}

	bus.Subscribe(EventPluginStarted, handler)

	// Publish event
	ctx := context.Background()
	err := bus.Publish(ctx, &Event{
		Type:     EventPluginStarted,
		PluginID: "test-plugin",
		Data:     map[string]interface{}{"key": "value"},
	})
	require.NoError(t, err)

	// Wait for event processing
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	assert.True(t, called)
	mu.Unlock()
}

func TestEventBusMultipleSubscribers(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	bus := NewEventBus(logger, 10)
	defer bus.Close()

	var mu sync.Mutex
	count := 0

	handler1 := func(ctx context.Context, event *Event) error {
		mu.Lock()
		count++
		mu.Unlock()
		return nil
	}

	handler2 := func(ctx context.Context, event *Event) error {
		mu.Lock()
		count++
		mu.Unlock()
		return nil
	}

	bus.Subscribe(EventPluginStarted, handler1)
	bus.Subscribe(EventPluginStarted, handler2)

	// Publish event
	ctx := context.Background()
	err := bus.Publish(ctx, &Event{
		Type:     EventPluginStarted,
		PluginID: "test-plugin",
		Data:     map[string]interface{}{},
	})
	require.NoError(t, err)

	// Wait for event processing
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, 2, count)
}

func TestEventBusSubscribeAll(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	bus := NewEventBus(logger, 10)
	defer bus.Close()

	var mu sync.Mutex
	receivedTypes := make(map[EventType]bool)

	handler := func(ctx context.Context, event *Event) error {
		mu.Lock()
		receivedTypes[event.Type] = true
		mu.Unlock()
		return nil
	}

	bus.SubscribeAll(handler)

	// Publish multiple event types
	ctx := context.Background()
	eventTypes := []EventType{
		EventPluginStarted,
		EventPluginFinished,
		EventStateChanged,
	}

	for _, eventType := range eventTypes {
		err := bus.Publish(ctx, &Event{
			Type:     eventType,
			PluginID: "test-plugin",
			Data:     map[string]interface{}{},
		})
		require.NoError(t, err)
	}

	// Wait for event processing
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	for _, eventType := range eventTypes {
		assert.True(t, receivedTypes[eventType], "Expected to receive event type: %s", eventType)
	}
}

func TestEventBusEventData(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	bus := NewEventBus(logger, 10)
	defer bus.Close()

	var receivedEvent *Event
	var mu sync.Mutex

	handler := func(ctx context.Context, event *Event) error {
		mu.Lock()
		receivedEvent = event
		mu.Unlock()
		return nil
	}

	bus.Subscribe(EventCustom, handler)

	// Publish event with data
	ctx := context.Background()
	publishedData := map[string]interface{}{
		"foo": "bar",
		"num": 42,
	}

	err := bus.Publish(ctx, &Event{
		Type:     EventCustom,
		PluginID: "test-plugin",
		Data:     publishedData,
	})
	require.NoError(t, err)

	// Wait for event processing
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	require.NotNil(t, receivedEvent)
	assert.Equal(t, EventCustom, receivedEvent.Type)
	assert.Equal(t, "test-plugin", receivedEvent.PluginID)
	assert.Equal(t, "bar", receivedEvent.Data["foo"])
	assert.Equal(t, 42, receivedEvent.Data["num"])
}

func TestEventBusNoSubscribers(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	bus := NewEventBus(logger, 10)
	defer bus.Close()

	// Publish event with no subscribers
	ctx := context.Background()
	err := bus.Publish(ctx, &Event{
		Type:     EventPluginStarted,
		PluginID: "test-plugin",
		Data:     map[string]interface{}{},
	})

	// Should not error even with no subscribers
	require.NoError(t, err)
}

func TestEventBusClose(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	bus := NewEventBus(logger, 10)

	var mu sync.Mutex
	processed := 0

	handler := func(ctx context.Context, event *Event) error {
		mu.Lock()
		processed++
		mu.Unlock()
		return nil
	}

	bus.Subscribe(EventPluginStarted, handler)

	// Publish events
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		err := bus.Publish(ctx, &Event{
			Type:     EventPluginStarted,
			PluginID: "test-plugin",
			Data:     map[string]interface{}{"count": i},
		})
		require.NoError(t, err)
	}

	// Close bus - should drain events
	bus.Close()

	// All events should be processed
	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, 5, processed)
}

func TestEventBusTimestamp(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	bus := NewEventBus(logger, 10)
	defer bus.Close()

	var receivedEvent *Event
	var mu sync.Mutex

	handler := func(ctx context.Context, event *Event) error {
		mu.Lock()
		receivedEvent = event
		mu.Unlock()
		return nil
	}

	bus.Subscribe(EventPluginStarted, handler)

	// Publish event
	ctx := context.Background()
	before := time.Now()
	err := bus.Publish(ctx, &Event{
		Type:     EventPluginStarted,
		PluginID: "test-plugin",
		Data:     map[string]interface{}{},
	})
	require.NoError(t, err)
	after := time.Now()

	// Wait for event processing
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	require.NotNil(t, receivedEvent)
	assert.True(t, receivedEvent.Timestamp.After(before) || receivedEvent.Timestamp.Equal(before))
	assert.True(t, receivedEvent.Timestamp.Before(after) || receivedEvent.Timestamp.Equal(after))
}

func TestEventBusConcurrentPublish(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	bus := NewEventBus(logger, 100)
	defer bus.Close()

	var mu sync.Mutex
	processed := 0

	handler := func(ctx context.Context, event *Event) error {
		mu.Lock()
		processed++
		mu.Unlock()
		return nil
	}

	bus.Subscribe(EventPluginStarted, handler)

	// Publish events concurrently
	ctx := context.Background()
	var wg sync.WaitGroup
	numEvents := 50

	for i := 0; i < numEvents; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			err := bus.Publish(ctx, &Event{
				Type:     EventPluginStarted,
				PluginID: "test-plugin",
				Data:     map[string]interface{}{"idx": idx},
			})
			require.NoError(t, err)
		}(i)
	}

	wg.Wait()
	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, numEvents, processed)
}
