package krakendrate

import (
	"testing"
)

func BenchmarkTokenBucket(b *testing.B) {
	tb := NewTokenBucket(1000, 100)
	for i := 0; i < b.N; i++ {
		tb.Allow()
	}
}
