package ai

import (
	"sync"
	"time"
)

type State int

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

type CircuitBreaker struct {
	mu            sync.Mutex
	state         State
	failures      int
	successes     int
	MaxFailures   int
	timeout       time.Duration
	lastFailure   time.Time
	halfOpenMax   int
	onStateChange func(from, to State)
}

func NewCircuitBreaker(maxFailures int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:       StateClosed,
		MaxFailures: maxFailures,
		timeout:     timeout,
		halfOpenMax: 1,
	}
}

func (cb *CircuitBreaker) SetOnStateChange(fn func(from, to State)) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.onStateChange = fn
}

func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		if time.Since(cb.lastFailure) > cb.timeout {
			prev := cb.state
			cb.state = StateHalfOpen
			cb.successes = 0
			if cb.onStateChange != nil {
				go cb.onStateChange(prev, cb.state)
			}
			return true
		}
		return false
	case StateHalfOpen:
		return cb.successes < cb.halfOpenMax
	default:
		return false
	}
}

func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateHalfOpen:
		cb.successes++
		if cb.successes >= cb.halfOpenMax {
			prev := cb.state
			cb.state = StateClosed
			cb.failures = 0
			if cb.onStateChange != nil {
				go cb.onStateChange(prev, cb.state)
			}
		}
	case StateClosed:
		cb.failures = 0
	}
}

func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.lastFailure = time.Now()

	switch cb.state {
	case StateClosed:
		if cb.failures >= cb.MaxFailures {
			prev := cb.state
			cb.state = StateOpen
			if cb.onStateChange != nil {
				go cb.onStateChange(prev, cb.state)
			}
		}
	case StateHalfOpen:
		prev := cb.state
		cb.state = StateOpen
		if cb.onStateChange != nil {
			go cb.onStateChange(prev, cb.state)
		}
	}
}

func (cb *CircuitBreaker) State() State {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

func (cb *CircuitBreaker) FailureCount() int {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.failures
}

func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = StateClosed
	cb.failures = 0
	cb.successes = 0
}
