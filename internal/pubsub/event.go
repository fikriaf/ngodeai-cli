package pubsub

// Event wraps a payload with type information
type Event[T any] struct {
	Type string
	Data T
}
