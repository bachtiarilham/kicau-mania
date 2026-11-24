package resilience

import (
	"errors"
	"time"

	"github.com/sony/gobreaker"
)

var (
	ErrCircuitBreakerOpen = errors.New("circuit breaker is open: external service unavailable")
)

type CircuitBreaker struct {
	cb *gobreaker.CircuitBreaker
}

// NewCircuitBreaker creates a circuit breaker wrapper around gobreaker.
// maxRequests: max requests allowed in half-open state (e.g. 5)
// interval: cyclic period of closed state to reset counts (e.g. 10s)
// timeout: duration of open state before half-open probe (e.g. 30s)
func NewCircuitBreaker(name string, maxRequests uint32, interval, timeout time.Duration) *CircuitBreaker {
	settings := gobreaker.Settings{
		Name:        name,
		MaxRequests: maxRequests,
		Interval:    interval,
		Timeout:     timeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 3 && failureRatio >= 0.6
		},
	}
	return &CircuitBreaker{
		cb: gobreaker.NewCircuitBreaker(settings),
	}
}

func (c *CircuitBreaker) Execute(fn func() (any, error)) (any, error) {
	res, err := c.cb.Execute(func() (interface{}, error) {
		return fn()
	})
	if err != nil {
		if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests) {
			return nil, ErrCircuitBreakerOpen
		}
		return nil, err
	}
	return res, nil
}
