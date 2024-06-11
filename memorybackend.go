// krakendrate contains a collection of curated rate limit adaptors for the KrakenD framework
package krakendrate

import (
	"context"
	"sync"
	"time"
)

func MemoryBackendBuilder(ctx context.Context, ttl time.Duration, amount uint64) []Backend {
	if amount == 0 {
		return []Backend{}
	}
	backends := make([]MemoryBackend, amount)
	for idx := range backends {
		backends[idx].data = map[string]interface{}{}
		backends[idx].lastAccess = map[string]time.Time{}
		backends[idx].mu = new(sync.RWMutex)
	}

	rv := make([]Backend, amount)
	for idx := range backends {
		rv[idx] = &(backends[idx])
	}
	go manageEvictions(ctx, ttl, backends)
	return rv
}

func NewMemoryBackend(ctx context.Context, ttl time.Duration) *MemoryBackend {
	backends := []MemoryBackend{
		{
			data:       map[string]interface{}{},
			lastAccess: map[string]time.Time{},
			mu:         new(sync.RWMutex),
		},
	}
	go manageEvictions(ctx, ttl, backends)

	return &(backends[0])
}

// MemoryBackend implements the backend interface by wrapping a sync.Map
type MemoryBackend struct {
	data       map[string]interface{}
	lastAccess map[string]time.Time
	mu         *sync.RWMutex
}

func manageEvictions(ctx context.Context, ttl time.Duration, backends []MemoryBackend) {
	t := time.NewTicker(ttl)
	for {
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case now := <-t.C:
			for idx := range backends {
				// We need to do a write lock, because between collecting the keys
				// to delete, and the actual deletion, another thread could have
				// hit one of the keys to delete.
				//
				// TODO: review this :
				// A different optimization would be not be "data agnostic", and
				// define an interface that allows to reause already allocated
				// data (like a Pool of token buckets).
				backends[idx].mu.Lock()
				// TODO: we could make this an array ? and the map only for the index ?
				for k, v := range backends[idx].lastAccess {
					if v.Add(ttl).Before(now) {
						delete(backends[idx].data, k)
						delete(backends[idx].lastAccess, k)
					}
				}
				backends[idx].mu.Unlock()
			}
		}
	}
}

// Load implements the Backend interface.
// The f function should always return a non nil value, or that nil value
// will be assigned and returned on load.
func (m *MemoryBackend) Load(key string, f func() interface{}) interface{} {
	var lastAccess time.Time
	lastAccessOk := true

	m.mu.RLock()
	v, ok := m.data[key]
	if ok {
		lastAccess, lastAccessOk = m.lastAccess[key]
	}
	m.mu.RUnlock()

	n := now()
	if ok {
		if !lastAccessOk || n.After(lastAccess) {
			m.mu.Lock()
			m.lastAccess[key] = n
			m.mu.Unlock()
		}
		return v
	}

	// we create the new associated data outside the loop (we will
	// discard it if it is already set in parallel by another thread)
	newData := f()
	m.mu.Lock()
	v, ok = m.data[key]
	if ok { // some other thread has just created the value
		m.mu.Unlock()
		return v
	}
	m.lastAccess[key] = n
	m.data[key] = newData
	m.mu.Unlock()
	return newData
}

// Store implements the Backend interface
// TODO: we might want to remove this function if we do not expect
// any other external code to store a value
func (m *MemoryBackend) Store(key string, v interface{}) error {
	m.mu.Lock()
	m.lastAccess[key] = now()
	m.data[key] = v
	m.mu.Unlock()
	return nil
}
