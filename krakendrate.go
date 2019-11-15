// krakendrate contains a collection of curated rate limit adaptors for the KrakenD framework
package krakendrate

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	// ErrLimited is the error returned when the rate limit has been exceded
	ErrLimited = errors.New("ERROR: rate limit exceded")

	// DataTTL is the default eviction time
	DataTTL = 10 * time.Minute

	now           = time.Now
	shards uint64 = 2048
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

// ShardedMemoryBackend is a memory backend shardering the data in order to avoid mutex contention
type ShardedMemoryBackend struct {
	shards []*MemoryBackend
	total  uint64
	hasher Hasher
}

// DefaultShardedMemoryBackend is a 2018 sharded ShardedMemoryBackend
func DefaultShardedMemoryBackend(ctx context.Context) *ShardedMemoryBackend {
	return NewShardedMemoryBackend(ctx, shards, DataTTL, PseudoFNV64a)
}

// NewShardedMemoryBackend returns a ShardedMemoryBackend with 'shards' shards
func NewShardedMemoryBackend(ctx context.Context, shards uint64, ttl time.Duration, h Hasher) *ShardedMemoryBackend {
	b := &ShardedMemoryBackend{
		shards: make([]*MemoryBackend, shards),
		total:  shards,
		hasher: h,
	}
	var i uint64
	for i = 0; i < shards; i++ {
		b.shards[i] = NewMemoryBackend(ctx, ttl)
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

func NewMemoryBackend(ctx context.Context, ttl time.Duration) *MemoryBackend {
	m := &MemoryBackend{
		data:       map[string]interface{}{},
		lastAccess: map[string]time.Time{},
		mu:         new(sync.RWMutex),
	}

	go m.manageEvictions(ctx, ttl)

	return m
}

// MemoryBackend implements the backend interface by wrapping a sync.Map
type MemoryBackend struct {
	data       map[string]interface{}
	lastAccess map[string]time.Time
	mu         *sync.RWMutex
}

func (m *MemoryBackend) manageEvictions(ctx context.Context, ttl time.Duration) {
	t := time.NewTicker(ttl)
	for {
		keysToDel := []string{}

		select {
		case <-ctx.Done():
			t.Stop()
			return
		case now := <-t.C:
			m.mu.RLock()
			for k, v := range m.lastAccess {
				if v.Add(ttl).Before(now) {
					keysToDel = append(keysToDel, k)
				}
			}
			m.mu.RUnlock()
		}

		m.del(keysToDel...)
	}
}

// Load implements the Backend interface
func (m *MemoryBackend) Load(key string, f func() interface{}) interface{} {
	m.mu.RLock()
	v, ok := m.data[key]
	m.mu.RUnlock()

	n := now()

	if ok {
		go func(t time.Time) {
			m.mu.Lock()
			if t0, ok := m.lastAccess[key]; !ok || t.After(t0) {
				m.lastAccess[key] = t
			}
			m.mu.Unlock()
		}(n)

		return v
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	v, ok = m.data[key]
	if ok {
		return v
	}

	v = f()
	m.lastAccess[key] = n
	m.data[key] = v

	return v
}

// Store implements the Backend interface
func (m *MemoryBackend) Store(key string, v interface{}) error {
	m.mu.Lock()
	m.lastAccess[key] = now()
	m.data[key] = v
	m.mu.Unlock()
	return nil
}

func (m *MemoryBackend) del(key ...string) {
	m.mu.Lock()
	for _, k := range key {
		delete(m.data, k)
		delete(m.lastAccess, k)
	}
	m.mu.Unlock()
}
