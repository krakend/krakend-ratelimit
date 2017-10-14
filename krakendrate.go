// krakendrate contains a collection of curated rate limit adaptors for the KrakenD framework
package krakendrate

import (
	"errors"
	"sync"
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

func NewMemoryBackend() MemoryBackend {
	return MemoryBackend{&sync.Map{}}
}

// MemoryBackend implements the backend interface by wrapping a sync.Map
type MemoryBackend struct {
	m *sync.Map
}

// Load implements the Backend interface
func (m MemoryBackend) Load(key string) (interface{}, bool) {
	return m.m.Load(key)
}

// Store implements the Backend interface
func (m MemoryBackend) Store(key string, v interface{}) error {
	m.m.Store(key, v)
	return nil
}
