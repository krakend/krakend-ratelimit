package krakendrate

import (
	"fmt"
	"testing"
	"time"
)

func TestMemoryBackend(t *testing.T) {
	DataTTL = 100 * time.Millisecond
	mb := NewMemoryBackend()
	total := 100
	for i := 0; i < total; i++ {
		mb.Store(fmt.Sprintf("key-%d", i), i)
	}
	for i := 0; i < total; i++ {
		v, ok := mb.Load(fmt.Sprintf("key-%d", i))
		if !ok {
			t.Errorf("key %d not present", i)
		}
		if v.(int) != i {
			t.Errorf("unexpected value. want: %d, have: %d", i, v.(int))
		}
	}
	time.Sleep(2 * DataTTL)
	for i := 0; i < total; i++ {
		_, ok := mb.Load(fmt.Sprintf("key-%d", i))
		if ok {
			t.Errorf("key %d present after 2 TTL", i)
		}
	}
}
