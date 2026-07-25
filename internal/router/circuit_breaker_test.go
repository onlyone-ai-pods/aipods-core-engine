package router

import (
	"testing"
	"time"
)

func TestCircuitBreakerStateTransitions(t *testing.T) {
	cb := NewCircuitBreaker(2, 100*time.Millisecond)

	// 1. Initial State: CLOSED
	if cb.State() != StateClosed {
		t.Errorf("Expected initial state CLOSED, got %s", cb.State())
	}
	if !cb.AllowRequest() {
		t.Errorf("Expected AllowRequest to be true in CLOSED state")
	}

	// 2. Record 2 failures -> State must trip to OPEN
	cb.RecordFailure()
	cb.RecordFailure()

	if cb.State() != StateOpen {
		t.Errorf("Expected state OPEN after threshold reached, got %s", cb.State())
	}
	if cb.AllowRequest() {
		t.Errorf("Expected AllowRequest to be false in OPEN state")
	}

	// 3. Wait for reset timeout -> State transitions to HALF_OPEN
	time.Sleep(150 * time.Millisecond)

	if !cb.AllowRequest() {
		t.Errorf("Expected AllowRequest to be true after reset timeout")
	}

	// 4. Record success -> State resets to CLOSED
	cb.RecordSuccess()
	if cb.State() != StateClosed {
		t.Errorf("Expected state CLOSED after recovery, got %s", cb.State())
	}
}
