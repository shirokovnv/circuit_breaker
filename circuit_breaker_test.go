package circuit_breaker

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func succeed(cb *CircuitBreaker) error {
	_, err := cb.Execute(func() (interface{}, error) { return nil, nil })
	return err
}

var errServiceError = errors.New("service error")

func fail(cb *CircuitBreaker) error {
	_, err := cb.Execute(func() (interface{}, error) { return nil, errServiceError })
	return err
}

func pseudoSleep(cb *CircuitBreaker, period time.Duration) {
	if !cb.expiredAt.IsZero() {
		cb.expiredAt = cb.expiredAt.Add(-period)
	}
}

func TestCircuitBreaker(t *testing.T) {
	cb := NewCircuitBreaker(Config{
		Name:             "test circuit breaker",
		RequestThreshold: 2,
		ReadyToTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures > defaultConsecutiveFailures
		},
	})

	for i := 0; i < 5; i++ {
		assert.Equal(t, errServiceError, fail(cb))
	}

	assert.Equal(t, StateClosed, cb.state)
	assert.Equal(t, Counts{5, 0, 5, 0, 5}, cb.counts)

	assert.Nil(t, succeed(cb))
	assert.Equal(t, StateClosed, cb.state)
	assert.Equal(t, Counts{6, 1, 5, 1, 0}, cb.counts)

	assert.Equal(t, errServiceError, fail(cb))
	assert.Equal(t, StateClosed, cb.state)
	assert.Equal(t, Counts{7, 1, 6, 0, 1}, cb.counts)

	// StateClosed -> StateOpen
	for i := 0; i < 5; i++ {
		assert.Equal(t, errServiceError, fail(cb)) // 6 consecutive failures
	}

	assert.Equal(t, StateOpen, cb.state)
	assert.Equal(t, Counts{0, 0, 0, 0, 0}, cb.counts)
	assert.False(t, cb.expiredAt.IsZero())

	assert.Error(t, succeed(cb))
	assert.Error(t, fail(cb))
	assert.Equal(t, Counts{0, 0, 0, 0, 0}, cb.counts)

	pseudoSleep(cb, time.Duration(59)*time.Second)
	assert.Equal(t, StateOpen, cb.state)

	// StateOpen -> StateHalfOpen
	pseudoSleep(cb, time.Duration(1)*time.Second) // over Timeout
	assert.Nil(t, succeed(cb))
	assert.Equal(t, StateHalfOpen, cb.state)
	assert.True(t, cb.expiredAt.IsZero())
	assert.Equal(t, Counts{1, 1, 0, 1, 0}, cb.counts)

	// StateHalfOpen -> StateOpen
	assert.Equal(t, errServiceError, fail(cb))
	assert.Equal(t, StateOpen, cb.state)
	assert.False(t, cb.expiredAt.IsZero())
	assert.Equal(t, Counts{0, 0, 0, 0, 0}, cb.counts)

	// StateOpen -> StateHalfOpen
	pseudoSleep(cb, time.Duration(60)*time.Second) // over Timeout
	assert.Nil(t, succeed(cb))
	assert.Equal(t, StateHalfOpen, cb.state)
	assert.True(t, cb.expiredAt.IsZero())
	assert.Equal(t, Counts{1, 1, 0, 1, 0}, cb.counts)

	// StateHalfOpen -> StateClosed
	assert.Nil(t, succeed(cb)) // ConsecutiveSuccesses(2) >= RequestThreshold(2)
	assert.Equal(t, StateClosed, cb.state)
	assert.Equal(t, Counts{0, 0, 0, 0, 0}, cb.counts)
	assert.True(t, cb.expiredAt.IsZero())
}
