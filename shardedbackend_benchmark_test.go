package krakendrate

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func BenchmarkShardedBackend(b *testing.B) {
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
			sb := NewShardedBackend(ctx, 2048, ttl, ttl, 1, PseudoFNV64a, MemoryBackendBuilder)
			for i := 0; i < b.N; i++ {
				numMax := tc.keep
				if tc.evict > tc.keep {
					numMax = tc.evict
				}
				// load all one time
				for k := 0; k < numMax; k++ {
					if k < tc.keep {
						sb.Load(toKeep[k], bcf)
					}
					if k < tc.evict {
						sb.Load(toEvict[k], bcf)
					}
				}
				// now keep loading the to keep 9 times
				// it might or might not evict depending on the time it takes
				for j := 0; j < 9; j++ {
					for k := 0; k < tc.keep; k++ {
						sb.Load(toKeep[k], bcf)
					}
				}
			}
		})
	}
	cancel()
}
