package router

import (
	"errors"
	"sync"
	"time"
)

var ErrCircuitBreakerOpen = errors.New("circuit breaker is OPEN: target AI Pod sidecar is degraded. Falling back to cached / safety response")

type CircuitState string

const (
	StateClosed   CircuitState = "CLOSED"   // Normal operation
	StateOpen     CircuitState = "OPEN"     // Failures exceeded limit, rejecting calls immediately
	StateHalfOpen CircuitState = "HALF_OPEN" // Testing recovery
)

// CircuitBreaker protects the Smart Router from cascading failures when external AI Pod sidecars fail
type CircuitBreaker struct {
	mu               sync.Mutex
	state            CircuitState
	failureCount     int
	failureThreshold int
	resetTimeout     time.Duration
	lastStateChange  time.Time
}

func NewCircuitBreaker(failureThreshold int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:            StateClosed,
		failureThreshold: failureThreshold,
		resetTimeout:     resetTimeout,
		lastStateChange:  time.Now(),
	}
}

func (cb *CircuitBreaker) AllowRequest() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == StateOpen {
		if time.Since(cb.lastStateChange) > cb.resetTimeout {
			cb.state = StateHalfOpen
			cb.lastStateChange = time.Now()
			return true
		}
		return false
	}
	return true
}

func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failureCount = 0
	if cb.state == StateHalfOpen {
		cb.state = StateClosed
		cb.lastStateChange = time.Now()
	}
}

func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failureCount++
	if cb.failureCount >= cb.failureThreshold {
		cb.state = StateOpen
		cb.lastStateChange = time.Now()
	}
}

func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}
