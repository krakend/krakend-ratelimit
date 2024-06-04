// krakendrate contains a collection of curated rate limit adaptors for the KrakenD framework
package krakendrate

import (
	"context"
	"sync"
	"time"
)

func MemoryBackendBuilder(ctx context.Context, ttl time.Duration) Backend {
	return NewMemoryBackend(ctx, ttl)
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
		var keysToDel []string

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
