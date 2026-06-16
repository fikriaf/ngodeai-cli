package pubsub

import (
	"context"
	"sync"
)

// Event types
const (
	EventCreated = "created"
	EventUpdated = "updated"
	EventDeleted = "deleted"
)

// Broker is a generic pub/sub system for event-driven communication
type Broker[T any] struct {
	mu          sync.RWMutex
	subscribers map[chan T]context.CancelFunc
	bufferSize  int
}

// NewBroker creates a new pub/sub broker
func NewBroker[T any]() *Broker[T] {
	return &Broker[T]{
		subscribers: make(map[chan T]context.CancelFunc),
		bufferSize:  64,
	}
}

// Subscribe creates a new subscription channel
func (b *Broker[T]) Subscribe(ctx context.Context) <-chan T {
	ch := make(chan T, b.bufferSize)
	subCtx, cancel := context.WithCancel(ctx)

	b.mu.Lock()
	b.subscribers[ch] = cancel
	b.mu.Unlock()

	// Auto-cleanup when context is cancelled
	go func() {
		<-subCtx.Done()
		b.Unsubscribe(ch)
	}()

	return ch
}

// Publish sends an event to all subscribers (non-blocking)
func (b *Broker[T]) Publish(event T) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for ch := range b.subscribers {
		select {
		case ch <- event:
		default:
			// Drop if subscriber is slow (buffer full)
		}
	}
}

// Unsubscribe removes a subscription
func (b *Broker[T]) Unsubscribe(ch <-chan T) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Find and remove the channel
	for subCh, cancel := range b.subscribers {
		if ch == subCh {
			cancel()
			close(subCh)
			delete(b.subscribers, subCh)
			return
		}
	}
}
