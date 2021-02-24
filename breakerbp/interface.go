package breakerbp

// CircuitBreaker is the interface that baseplate expects a circuit breaker to implement.
type CircuitBreaker interface {
	// Execute should wrap the given function call in circuit breaker logic and return the result.
	Execute(func() (interface{}, error)) (interface{}, error)
}
