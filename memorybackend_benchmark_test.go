package krakendrate

import (
	"context"
	"crypto/sha256"
	"fmt"
	"math/rand"
	"testing"
	"time"
)

func generateTestKeys(num, length int) []string {
	res := make([]string, 0, num)

	// we do not need crypto strength random number to test
	r := rand.New(rand.NewSource(int64(num))) // skipcq: GSC-G404
	h := sha256.New()

	for i := 0; i < num; i++ {
		fmt.Fprintf(h, "%x", r.Int())
		res = append(res, fmt.Sprintf("%x", h.Sum(nil))[:length])
	}
	return res
}

type buildCounter struct {
	counter int64
}

func (bc *buildCounter) builder() func() interface{} {
	return func() interface{} {
		bc.counter += 1
		return bc.counter
	}
}

func BenchmarkMemoryBackend(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	ttl := time.Millisecond

	maxNumToEvict := 1000000 // 1 million * (8 length + 4 ) ~ 12 megs ?
	maxNumToKeep := 1000000  //
	toEvict := generateTestKeys(maxNumToEvict, 8)
	toKeep := generateTestKeys(maxNumToKeep, 8)
	bc := buildCounter{}
	bcf := bc.builder()

	t := []struct {
		keep  int
		evict int
	}{
		{keep: 100, evict: 100},
		{keep: 100, evict: 10000},
		{keep: 100, evict: 1000000},
		{keep: 10000, evict: 100},
		{keep: 10000, evict: 10000},
		{keep: 10000, evict: 1000000},
	}

	for _, tc := range t {
		b.Run(fmt.Sprintf("loads_%d_keep_%d_evict_%d", tc.keep*10+tc.evict, tc.keep, tc.evict), func(b *testing.B) {
			mb := NewMemoryBackend(ctx, ttl)
			for i := 0; i < b.N; i++ {
				numMax := tc.keep
				if tc.evict > tc.keep {
					numMax = tc.evict
				}
				// load all one time
				for k := 0; k < numMax; k++ {
					if k < tc.keep {
						mb.Load(toKeep[k], bcf)
					}
					if k < tc.evict {
						mb.Load(toEvict[k], bcf)
					}
				}
				// now keep loading the to keep 9 times
				// it might or might not evict depending on the time it takes
				for j := 0; j < 9; j++ {
					for k := 0; k < tc.keep; k++ {
						mb.Load(toKeep[k], bcf)
					}
				}
			}
		})
	}
	cancel()
}
