package krakendrate

import (
	"context"
)

// NewMemoryStore returns a LimiterStore using the memory backend
//
// Deprecated: Use NewLimiterStore instead
func NewMemoryStore(maxRate float64, capacity int) LimiterStore {
	return NewLimiterStore(maxRate, capacity, DefaultShardedMemoryBackend(context.Background()))
}

// NewLimiterStore returns a LimiterStore using the received backend for persistence
func NewLimiterStore(maxRate float64, capacity int, backend Backend) LimiterStore {
	f := NewTokenBucketBuilder(maxRate, uint64(capacity), uint64(capacity), nil)
	return func(t string) Limiter {
		return backend.Load(t, f).(Limiter)
	}
}

// LimiterBuilderFn defines the function that will be called when there
// is no entry in the backend for a given token.
type LimiterBuilderFn func() interface{}

// NewLimiterFromBackendAndBuilder creates a LimiterStore that uses limiterBuilder to
// creat new token buckets.
func NewLimiterFromBackendAndBuilder(backend Backend, limiterBuilder LimiterBuilderFn) LimiterStore {
	if limiterBuilder == nil {
		limiterBuilder = NewTokenBucketBuilder(1, 1, 1, nil)
	}
	return func(strToken string) Limiter {
		return backend.Load(strToken, limiterBuilder).(Limiter)
	}
}
