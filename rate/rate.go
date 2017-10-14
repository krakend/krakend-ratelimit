/*
Package rate provides a set of rate-limit proxy and router middlewares using the golang.org/x/time/rate lib.

The rate package provides an efficient token bucket implementation. See https://golang.org/x/time/rate
and http://en.wikipedia.org/wiki/Token_bucket for more details.
*/
package rate

import (
	"golang.org/x/time/rate"

	"github.com/devopsfaith/krakend-ratelimit"
)

// NewLimiter creates a new Limiter
func NewLimiter(maxRate float64, capacity int) Limiter {
	return Limiter{rate.NewLimiter(rate.Limit(maxRate), capacity)}
}

// Limiter is a simple wrapper over the rate.Limiter struct
type Limiter struct {
	limiter *rate.Limiter
}

// Allow delegates to the internal limiter allow method
func (l Limiter) Allow() bool {
	return l.limiter.Allow()
}

// NewLimiterStore returns a LimiterStore using the received backend for persistence
func NewLimiterStore(maxRate float64, capacity int, backend krakendrate.Backend) krakendrate.LimiterStore {
	return func(t string) krakendrate.Limiter {
		tmp, ok := backend.Load(t)
		if !ok {
			tb := NewLimiter(maxRate, capacity)
			backend.Store(t, tb)
			return tb
		}
		return tmp.(Limiter)
	}
}

// NewMemoryStore returns a LimiterStore using the memory backend
func NewMemoryStore(maxRate float64, capacity int) krakendrate.LimiterStore {
	return NewLimiterStore(maxRate, capacity, krakendrate.NewMemoryBackend())
}
