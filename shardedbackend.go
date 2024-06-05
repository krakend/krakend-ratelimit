package krakendrate

import (
	"context"
	"time"
)

// Hasher gets a hash for the received string
type Hasher func(string) uint64

// BackendBuilder is the type for a function that can build a Backend.
// Is is used by the ShardedMemoryBackend to create several backends / shards.
type BackendBuilder func(ctx context.Context, ttl time.Duration) Backend

// ShardedMemoryBackend is a memory backend shardering the data in order to avoid mutex contention
type ShardedMemoryBackend struct {
	shards []Backend
	total  uint64
	hasher Hasher
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
