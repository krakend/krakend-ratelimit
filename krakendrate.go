// krakendrate contains a collection of curated rate limit adaptors for the KrakenD framework
package krakendrate

import (
	"errors"
	"runtime"
	"sync"
	"time"
)

var (
	// ErrLimited is the error returned when the rate limit has been exceded
	ErrLimited = errors.New("ERROR: rate limit exceded")

	DataTTL = 10 * time.Minute

	now    = time.Now
	stores = []*MemoryBackend{}
	mu     = new(sync.RWMutex)
	once   = new(sync.Once)
)

// Limiter defines a simple interface for a rate limiter
type Limiter interface {
	Allow() bool
}

// LimiterStore defines the interface for a limiter lookup function
type LimiterStore func(string) Limiter

// Backend is the interface of the persistence layer
type Backend interface {
	Load(string) (interface{}, bool)
	Store(string, interface{}) error
}

func NewMemoryBackend() *MemoryBackend {
	m := &MemoryBackend{
		data:       map[string]interface{}{},
		lastAccess: map[string]time.Time{},
		mu:         new(sync.RWMutex),
	}

	mu.Lock()
	stores = append(stores, m)
	mu.Unlock()

	once.Do(func() { go autoCleanup(DataTTL) })

	return m
}

// MemoryBackend implements the backend interface by wrapping a sync.Map
type MemoryBackend struct {
	data       map[string]interface{}
	lastAccess map[string]time.Time
	mu         *sync.RWMutex
}

// Load implements the Backend interface
func (m *MemoryBackend) Load(key string) (interface{}, bool) {
	m.mu.Lock()
	v, ok := m.data[key]
	if ok {
		m.lastAccess[key] = now()
	}
	m.mu.Unlock()
	return v, ok
}

// Store implements the Backend interface
func (m *MemoryBackend) Store(key string, v interface{}) error {
	m.mu.Lock()
	m.lastAccess[key] = now()
	m.data[key] = v
	m.mu.Unlock()
	return nil
}

func (m *MemoryBackend) del(key string) {
	delete(m.data, key)
	delete(m.lastAccess, key)
}

func autoCleanup(ttl time.Duration) {
	for {
		<-time.After(ttl)
		mu.RLock()
		if len(stores) < runtime.NumCPU() {
			for _, store := range stores {
				store.mu.Lock()
				for k, v := range store.lastAccess {
					if time.Since(v) > ttl {
						store.del(k)
					}
				}
				store.mu.Unlock()
			}
			mu.RUnlock()
			continue
		}

		block := len(stores) / runtime.NumCPU()
		for i := 0; i < runtime.NumCPU(); i++ {
			go func(stores []*MemoryBackend) {
				for _, store := range stores {
					store.mu.Lock()
					for k, v := range store.lastAccess {
						if time.Since(v) > ttl {
							store.del(k)
						}
					}
					store.mu.Unlock()
				}
			}(stores[i*block : (i+1)*block])
		}
		mu.RUnlock()
	}
}
