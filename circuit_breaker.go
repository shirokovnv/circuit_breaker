package circuit_breaker

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

type State uint32

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

const (
	defaultConsecutiveFailures = 5
	defaultTimeout             = 60 * time.Second
)

var (
	// ErrTooManyRequests is returned when the CB state is half open and the requests count is over the cb requestThreshold
	ErrTooManyRequests = errors.New("too many requests")
	// ErrOpenState is returned when the CB state is open
	ErrOpenState = errors.New("circuit breaker is open")
)

func (state State) String() string {
	switch state {
	case StateClosed:
		return "closed"
	case StateHalfOpen:
		return "half-open"
	case StateOpen:
		return "open"
	default:
		return fmt.Sprintf("undefined state: %d", state)
	}
}

type Counts struct {
	Requests             uint32
	TotalSuccesses       uint32
	TotalFailures        uint32
	ConsecutiveSuccesses uint32
	ConsecutiveFailures  uint32
}

func (c *Counts) onRequest() {
	c.Requests++
}

func (c *Counts) onSuccess() {
	c.TotalSuccesses++
	c.ConsecutiveSuccesses++
	c.ConsecutiveFailures = 0
}

func (c *Counts) onFailure() {
	c.TotalFailures++
	c.ConsecutiveFailures++
	c.ConsecutiveSuccesses = 0
}

func (c *Counts) reset() {
	c.Requests = 0
	c.TotalSuccesses = 0
	c.TotalFailures = 0
	c.ConsecutiveSuccesses = 0
	c.ConsecutiveFailures = 0
}

// RequestThreshold is the maximum number of requests allowed to pass through
// when the CircuitBreaker is half-opened
//
// Timeout is the period of the open state,
// after which the state of the CircuitBreaker becomes half-open.
//
// ReadyToTrip is called with a copy of Counts whenever a request fails in the closed state.
// If ReadyToTrip returns true, the CircuitBreaker will be placed into the open state.
// If ReadyToTrip is nil, default ReadyToTrip is used.
// Default ReadyToTrip returns true when the number of consecutive failures is more than 5.

type CircuitBreaker struct {
	mu               sync.Mutex
	name             string
	requestThreshold uint32
	timeout          time.Duration
	readyToTrip      func(counts Counts) bool
	onStateChange    func(name string, from State, to State)

	state     State
	counts    Counts
	expiredAt time.Time
}

type Config struct {
	Name             string
	RequestThreshold uint32
	Timeout          time.Duration

	ReadyToTrip   func(counts Counts) bool
	OnStateChange func(name string, from State, to State)
}

func NewCircuitBreaker(cfg Config) *CircuitBreaker {
	cb := CircuitBreaker{
		name:             cfg.Name,
		requestThreshold: cfg.RequestThreshold,
		timeout:          cfg.Timeout,
		readyToTrip:      cfg.ReadyToTrip,
		onStateChange:    cfg.OnStateChange,
		state:            StateClosed,
		counts:           Counts{},
	}

	if cb.readyToTrip == nil {
		cb.readyToTrip = defaultReadyToTrip
	}
	if cb.timeout == 0 {
		cb.timeout = defaultTimeout
	}

	return &cb
}

func (cb *CircuitBreaker) Execute(req func() (interface{}, error)) (interface{}, error) {

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == StateOpen && cb.expiredAt.Before(time.Now()) {
		cb.expiredAt = time.Time{}
		cb.setState(StateHalfOpen)
	}

	if cb.state == StateOpen {
		return nil, ErrOpenState
	} else if cb.state == StateHalfOpen && cb.counts.Requests >= cb.requestThreshold {
		return nil, ErrTooManyRequests
	}
	cb.counts.onRequest()

	result, err := req()

	if err != nil {
		cb.onFailure(cb.state)
	} else {
		cb.onSuccess(cb.state)
	}

	return result, err
}

func defaultReadyToTrip(counts Counts) bool {
	return counts.ConsecutiveFailures > defaultConsecutiveFailures
}

func (cb *CircuitBreaker) onSuccess(state State) {
	switch state {
	case StateClosed:
		cb.counts.onSuccess()
	case StateHalfOpen:
		cb.counts.onSuccess()
		if cb.counts.ConsecutiveSuccesses >= cb.requestThreshold {
			cb.setState(StateClosed)
		}
	}
}

func (cb *CircuitBreaker) onFailure(state State) {
	switch state {
	case StateClosed:
		cb.counts.onFailure()
		if cb.readyToTrip(cb.counts) {
			cb.expiredAt = time.Now().Add(cb.timeout)
			cb.setState(StateOpen)
		}
	case StateHalfOpen:
		cb.expiredAt = time.Now().Add(cb.timeout)
		cb.setState(StateOpen)
	}
}

func (cb *CircuitBreaker) setState(state State) {
	if cb.state == state {
		return
	}

	prev := cb.state
	cb.state = state

	if cb.onStateChange != nil {
		cb.onStateChange(cb.name, prev, state)
	}

	cb.counts.reset()
}
