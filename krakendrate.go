// krakendrate contains a collection of curated rate limit adaptors for the KrakenD framework
package krakendrate

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrLimited is the error returned when the rate limit has been exceded
	ErrLimited = errors.New("rate limit exceded")

	// DataTTL is the default eviction time
	DataTTL = 10 * time.Minute

	// DefaultShards are the number of shards to create by default
	DefaultShards uint64 = 2048

	now = time.Now
)

// Token conaints the key to use to find the Limiter. The extra
// Data field can be anything (an http.Request, a contex.Context,
// a map[string]interface{}...) that can be useful to tweak the search 
// or creation of the Limiter, and that is 
type Token struct {
    Key     string
    Data    interface{}     // Extra data that can be used 
}

// Limiter defines a simple interface for a rate limiter
type Limiter interface {
	Allow() bool
}

// LimiterStore defines the interface for a limiter lookup function
type LimiterStore func(string) Limiter

// LimiterBuilderFn defines the function that will be called when there
// is no entry in the backend for a given token.
// The ctx is passed so build
type LimiterBuilderFn func(ctx contex.Context) interface{}

// Backend is the interface of the persistence layer
type Backend interface {
	Load(token *Token, limiterBuilder LimiterBuilderFn) Limiter
	Store(token *Token, interface{}) error
}

// DefaultShardedMemoryBackend is a 2018 sharded ShardedMemoryBackend
func DefaultShardedMemoryBackend(ctx context.Context) *ShardedMemoryBackend {
	return NewShardedMemoryBackend(ctx, DefaultShards, DataTTL, PseudoFNV64a)
}
