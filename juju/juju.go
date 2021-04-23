/*
Package juju provides a set of rate-limit proxy and router middlewares using the github.com/juju/ratelimit lib.

The juju package provides an efficient token bucket implementation. See https://github.com/juju/ratelimit
and http://en.wikipedia.org/wiki/Token_bucket for more details.
*/
package juju

import (
	"context"

	"time"

	"github.com/juju/ratelimit"

	krakendrate "github.com/devopsfaith/krakend-ratelimit"
)

// NewLimiter creates a new Limiter
func NewLimiter(maxRate float64, capacity int64) Limiter {
	return Limiter{ratelimit.NewBucketWithRate(maxRate, capacity)}
}

func NewLimiterDuration(fillInterval time.Duration, capacity int64) Limiter {
	return Limiter{ratelimit.NewBucketWithQuantum(fillInterval, capacity, capacity)}
}

// Limiter is a simple wrapper over the ratelimit.Bucket struct
type Limiter struct {
	limiter *ratelimit.Bucket
}

// Allow checks if its possible to extract 1 token from the bucket
func (l Limiter) Allow() bool {
	return l.limiter.TakeAvailable(1) > 0
}

// NewLimiterStore returns a LimiterStore using the received backend for persistence
func NewLimiterStore(maxRate float64, capacity int64, backend krakendrate.Backend) krakendrate.LimiterStore {
	f := func() interface{} { return NewLimiter(maxRate, capacity) }
	return func(t string) krakendrate.Limiter {
		return backend.Load(t, f).(Limiter)
	}
}

func NewLimiterDurationStore(fillInterval time.Duration, capacity int64, backend krakendrate.Backend) krakendrate.LimiterStore {
	f := func() interface{} { return NewLimiterDuration(fillInterval, capacity) }
	return func(t string) krakendrate.Limiter {
		return backend.Load(t, f).(Limiter)
	}
}

// NewMemoryStore returns a LimiterStore using the memory backend
func NewMemoryStore(maxRate float64, capacity int64) krakendrate.LimiterStore {
	return NewLimiterStore(maxRate, capacity, krakendrate.DefaultShardedMemoryBackend(context.Background()))
}

func NewMemoryDurationStore(fillInterval time.Duration, capacity int64) krakendrate.LimiterStore {
	return NewLimiterDurationStore(fillInterval, capacity, krakendrate.DefaultShardedMemoryBackend(context.Background()))
}
