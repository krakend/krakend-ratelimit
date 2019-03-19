package krakendrate

import (
	"fmt"
	"runtime"
	"testing"
	"time"
)

func TestMemoryBackend(t *testing.T) {
	if v := len(stores); v != 0 {
		t.Errorf("%d stores already initialized", v)
		return
	}
	DataTTL = 100 * time.Millisecond
	mb := NewMemoryBackend()
	if v := len(stores); v != 1 {
		t.Errorf("%d stores initialized", v)
		return
	}
	total := 1000 * runtime.NumCPU()

	for i := 0; i < total; i++ {
		mb.Store(fmt.Sprintf("key-%d", i), i)
	}
	for i := 0; i < total; i++ {
		v, ok := mb.Load(fmt.Sprintf("key-%d", i))
		if !ok {
			t.Errorf("key %d not present", i)
		}
		if res, ok := v.(int); !ok || res != i {
			t.Errorf("unexpected value. want: %d, have: %v", i, v)
		}
	}
	<-time.After(2 * DataTTL)
	for i := 0; i < total; i++ {
		_, ok := mb.Load(fmt.Sprintf("key-%d", i))
		if ok {
			t.Errorf("key %d present after 2 TTL", i)
		}
	}
}
