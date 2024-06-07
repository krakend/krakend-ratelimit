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
//
// Deprecated: Use NewLimiterStore instead
func NewLimiterStore(maxRate float64, capacity int, backend Backend) LimiterStore {
	f := NewTokenBucketBuilder(maxRate, uint64(capacity), uint64(capacity), nil)
	return func(t string) Limiter {
		return backend.Load(t, f).(Limiter)
	}
}

// NewLimiterFromBackendAndBuilder
func NewLimiterFromBackendAndBuilder(backend Backend, limiterBuilder LimiterBuilderFn) LimiterStore {
	if limiterBuilder == nil {
		limiterBuilder = NewTokenBucketBuilder(1, 1, 1, nil)
	}
	return func(strToken string) Limiter {
		return backend.Load(strToken, limiterBuilder).(Limiter)
	}
}
