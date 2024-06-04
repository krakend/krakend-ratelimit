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

// Limiter defines a simple interface for a rate limiter
type Limiter interface {
	Allow() bool
}

// LimiterStore defines the interface for a limiter lookup function
type LimiterStore func(string) Limiter

// Hasher gets a hash for the received string
type Hasher func(string) uint64

// Backend is the interface of the persistence layer
type Backend interface {
	Load(string, func() interface{}) interface{}
	Store(string, interface{}) error
}

// BackendBuilder is the type for a function that can build a Backend.
// Is is used by the ShardedMemoryBackend to create several backends / shards.
type BackendBuilder func(ctx context.Context, ttl time.Duration) Backend

// ShardedMemoryBackend is a memory backend shardering the data in order to avoid mutex contention
type ShardedMemoryBackend struct {
	shards []Backend
	total  uint64
	hasher Hasher
}

// DefaultShardedMemoryBackend is a 2018 sharded ShardedMemoryBackend
func DefaultShardedMemoryBackend(ctx context.Context) *ShardedMemoryBackend {
	return NewShardedMemoryBackend(ctx, DefaultShards, DataTTL, PseudoFNV64a)
}

// NewShardedMemoryBackend returns a ShardedMemoryBackend with 'shards' shards
func NewShardedMemoryBackend(ctx context.Context, shards uint64, ttl time.Duration, h Hasher) *ShardedMemoryBackend {
	return NewShardedBackend(ctx, shards, ttl, h, MemoryBackendBuilder)
}

func NewShardedBackend(ctx context.Context, shards uint64, ttl time.Duration, h Hasher,
	backendBuilder BackendBuilder) *ShardedMemoryBackend {

	b := &ShardedMemoryBackend{
		shards: make([]Backend, shards),
		total:  shards,
		hasher: h,
	}
	var i uint64
	for i = 0; i < shards; i++ {
		b.shards[i] = backendBuilder(ctx, ttl)
	}
	return b
}

func (b *ShardedMemoryBackend) shard(key string) uint64 {
	return b.hasher(key) % b.total
}

// Load implements the Backend interface
func (b *ShardedMemoryBackend) Load(key string, f func() interface{}) interface{} {
	return b.shards[b.shard(key)].Load(key, f)
}

// Store implements the Backend interface
func (b *ShardedMemoryBackend) Store(key string, v interface{}) error {
	return b.shards[b.shard(key)].Store(key, v)
}

/*

This function looks like is not called from anywhere and allows us to
avoid using the specific MemoryBackend type and just use the
BackendInterface.

func (b *ShardedMemoryBackend) del(key ...string) {
	buckets := map[uint64][]string{}
	for _, k := range key {
		h := b.shard(k)
		ks, ok := buckets[h]
		if !ok {
			ks = []string{k}
		} else {
			ks = append(ks, k)
		}
		buckets[h] = ks
	}

	for s, ks := range buckets {
		b.shards[s].del(ks...)
	}
}
*/
