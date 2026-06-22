package ai

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestCircuitBreaker_ClosedState(t *testing.T) {
	cb := NewCircuitBreaker(3, 5*time.Second)
	if cb.State() != StateClosed {
		t.Error("initial state should be closed")
	}
	if !cb.Allow() {
		t.Error("should allow in closed state")
	}
}

func TestCircuitBreaker_OpensAfterFailures(t *testing.T) {
	cb := NewCircuitBreaker(3, 50*time.Millisecond)
	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}
	if cb.State() != StateOpen {
		t.Error("should be open after max failures")
	}
	if cb.Allow() {
		t.Error("should not allow in open state")
	}
}

func TestCircuitBreaker_HalfOpenAfterTimeout(t *testing.T) {
	cb := NewCircuitBreaker(2, 50*time.Millisecond)
	cb.RecordFailure()
	cb.RecordFailure()
	if cb.State() != StateOpen {
		t.Error("should be open")
	}
	time.Sleep(60 * time.Millisecond)
	if !cb.Allow() {
		t.Error("should allow after timeout (half-open)")
	}
	if cb.State() != StateHalfOpen {
		t.Error("should be half-open")
	}
}

func TestCircuitBreaker_ClosesAfterSuccess(t *testing.T) {
	cb := NewCircuitBreaker(1, 50*time.Millisecond)
	cb.RecordFailure()
	time.Sleep(60 * time.Millisecond)
	cb.Allow()
	cb.RecordSuccess()
	if cb.State() != StateClosed {
		t.Error("should close after success in half-open")
	}
}

func TestCircuitBreaker_Reset(t *testing.T) {
	cb := NewCircuitBreaker(1, 50*time.Millisecond)
	cb.RecordFailure()
	cb.Reset()
	if cb.State() != StateClosed {
		t.Error("should be closed after reset")
	}
	if cb.FailureCount() != 0 {
		t.Error("failures should be 0 after reset")
	}
}

func TestCircuitBreaker_StateChangeCallback(t *testing.T) {
	cb := NewCircuitBreaker(1, 50*time.Millisecond)
	var called atomic.Int32
	cb.SetOnStateChange(func(from, to State) {
		called.Add(1)
	})
	cb.RecordFailure()
	time.Sleep(10 * time.Millisecond)
	if called.Load() == 0 {
		t.Error("callback should have been called")
	}
}
