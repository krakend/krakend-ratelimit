// krakendrate contains a collection of curated rate limit adaptors for the KrakenD framework
package krakendrate

import (
	"errors"
	"sync"
	"time"
)

// ErrLimited is the error returned when the rate limit has been exceded
var ErrLimited = errors.New("ERROR: rate limit exceded")

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
	go m.autoCleanup()
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
	m.mu.RLock()
	v, ok := m.data[key]
	if ok {
		m.lastAccess[key] = now()
	}
	m.mu.RUnlock()
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

func (m *MemoryBackend) autoCleanup() {
	for {
		<-time.After(DataTTL)
		m.mu.Lock()
		for k, v := range m.lastAccess {
			if time.Since(v) > DataTTL {
				delete(m.data, k)
				delete(m.lastAccess, k)
			}
		}
		m.mu.Unlock()
	}
}

var (
	DataTTL = 10 * time.Minute
	now     = time.Now
)
