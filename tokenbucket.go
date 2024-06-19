package krakendrate

import (
	"sync"
	"time"
)

// NewTokenBucket returns a token bucket with the given rate and capacity, using the default clock and
// an initial stock of cap
func NewTokenBucket(rate float64, capacity uint64) *TokenBucket {
	return NewTokenBucketWithClock(rate, capacity, nil)
}

// Clock defines the interface for clock sources
type Clock interface {
	Now() time.Time
	Since(time.Time) time.Duration
}

// NewTokenBucketWithClock returns a token bucket with the given rate, capacity, and clock and
// an initial stock of capacity
func NewTokenBucketWithClock(rate float64, capacity uint64, c Clock) *TokenBucket {
	return NewTokenBucketWithInitialStock(rate, capacity, capacity, c)
}

// NewTokenBucketWithInitialStock returns a token bucket with the given rate, capacity, clock
// and initial stock
func NewTokenBucketWithInitialStock(r float64, capacity, i uint64, c Clock) *TokenBucket {
	if c == nil {
		c = defaultClock{}
	}
	if capacity < 1 {
		capacity = 1
	}
	if i > capacity {
		i = capacity
	}
	if r < 1e-9 {
		r = 1e-9
	}

	return &TokenBucket{
		fillInterval: time.Duration(int64(1e9 / r)),
		capacity:     capacity,
		clock:        c,
		tokens:       i,
		lastRefill:   c.Now(),
		mu:           new(sync.Mutex),
	}
}

func NewTokenBucketBuilder(rate float64, capacity, initialStock uint64, clk Clock) LimiterBuilderFn {
	// we do not call NewTokenBucketWithIntialStock inside the returned function to
	// avoid the following block of checks that can be done just once
	if clk == nil {
		clk = defaultClock{}
	}
	if capacity < 1 {
		capacity = 1
	}
	if initialStock > capacity {
		initialStock = capacity
	}
	if rate < 1e-9 {
		rate = 1e-9
	}
	fillInterval := time.Duration(int64(1e9 / rate))

	return func() interface{} {
		return &TokenBucket{
			fillInterval: fillInterval,
			capacity:     capacity,
			clock:        clk,
			tokens:       initialStock,
			lastRefill:   clk.Now(),
			mu:           new(sync.Mutex),
		}
	}
}

// TokenBucket is an implementation of the token bucket pattern
type TokenBucket struct {
	fillInterval time.Duration
	capacity     uint64
	tokens       uint64
	clock        Clock
	lastRefill   time.Time
	mu           *sync.Mutex
}

// Allow flags if the current request can be processed or not. It updates the internal state if
// the request can be processed
func (t *TokenBucket) Allow() bool {
	t.mu.Lock()
	r := t.canConsume()
	t.mu.Unlock()
	return r
}

func (t *TokenBucket) canConsume() bool {
	if t.tokens > 0 {
		// delay the refill until the bucket is empty
		t.tokens--
		return true
	}

	// if there are no more tokens in the bucket, calculate how many tokens should be added
	tokensToAdd := uint64(t.clock.Since(t.lastRefill) / t.fillInterval)

	// if there are no tokens to be added to the empty bucket
	if tokensToAdd == 0 {
		return false
	}

	// update the time of the last refill depending on how many tokens we added
	t.lastRefill = t.lastRefill.Add(time.Duration(tokensToAdd) * t.fillInterval)

	// normalize the amount of tokens to add
	if t.tokens+tokensToAdd > t.capacity {
		t.tokens = t.capacity
		return true
	}

	t.tokens += tokensToAdd - 1
	return true
}

type defaultClock struct{}

func (defaultClock) Now() time.Time {
	return time.Now()
}

func (defaultClock) Since(t time.Time) time.Duration {
	return time.Since(t)
}
