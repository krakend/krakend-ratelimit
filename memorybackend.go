// krakendrate contains a collection of curated rate limit adaptors for the KrakenD framework
package krakendrate

import (
	"context"
	"sync"
	"time"
)

func MemoryBackendBuilder(ctx context.Context, ttl, cleanupRate time.Duration,
	cleanUpThreads, amount uint64,
) []Backend {
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

	if cleanUpThreads <= 1 {
		go manageEvictions(ctx, ttl, cleanupRate, backends)
		return rv
	}

	if cleanUpThreads > amount {
		// Nop, we wont create more clean up threads than the number of shards
		cleanUpThreads = amount
	}

	from := 0
	for i := uint64(1); i <= cleanUpThreads; i++ {
		to := int((i * amount) / cleanUpThreads)
		go manageEvictions(ctx, ttl, cleanupRate, backends[from:to])
		from = to
	}

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
	// to maintain backards compat, we use ttl as the cleanup rate:
	go manageEvictions(ctx, ttl, ttl, backends)

	return &(backends[0])
}

// MemoryBackend implements the backend interface by wrapping a sync.Map
type MemoryBackend struct {
	data       map[string]interface{}
	lastAccess map[string]time.Time
	mu         *sync.RWMutex
}

func manageEvictions(ctx context.Context, ttl, cleanupRate time.Duration, backends []MemoryBackend) {
	t := time.NewTicker(cleanupRate)
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
				backends[idx].mu.Lock()
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
func (m *MemoryBackend) Store(key string, v interface{}) error {
	m.mu.Lock()
	m.lastAccess[key] = now()
	m.data[key] = v
	m.mu.Unlock()
	return nil
}
