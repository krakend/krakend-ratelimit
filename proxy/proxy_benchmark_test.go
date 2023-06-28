package proxy

import (
	"context"
	"testing"

	"github.com/luraproject/lura/v2/config"
	"github.com/luraproject/lura/v2/logging"
	"github.com/luraproject/lura/v2/proxy"
)

func BenchmarkNewMiddleware_ok(b *testing.B) {
	p := NewMiddleware(logging.NoOp, &config.Backend{
		ExtraConfig: map[string]interface{}{
			Namespace: map[string]interface{}{
				"max_rate": 10000000000000.0,
				"capacity": 100000000000.0,
			},
		},
	})(dummyProxy(&proxy.Response{}, nil))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p(context.Background(), &proxy.Request{
			Path: "/tupu",
		})
	}
}

func BenchmarkNewMiddleware_ko(b *testing.B) {
	p := NewMiddleware(logging.NoOp, &config.Backend{
		ExtraConfig: map[string]interface{}{
			Namespace: map[string]interface{}{
				"max_rate": 1.0,
				"capacity": 1.0,
			},
		},
	})(dummyProxy(&proxy.Response{}, nil))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p(context.Background(), &proxy.Request{
			Path: "/tupu",
		})
	}
}
