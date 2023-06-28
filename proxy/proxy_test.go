package proxy

import (
	"context"
	"sync/atomic"
	"testing"

	krakendrate "github.com/krakendio/krakend-ratelimit/v3"
	"github.com/luraproject/lura/v2/config"
	"github.com/luraproject/lura/v2/logging"
	"github.com/luraproject/lura/v2/proxy"
)

func TestNewMiddleware_multipleNext(t *testing.T) {
	defer func() {
		if r := recover(); r != proxy.ErrTooManyProxies {
			t.Errorf("The code did not panic\n")
		}
	}()
	NewMiddleware(logging.NoOp, &config.Backend{})(proxy.NoopProxy, proxy.NoopProxy)
}

func TestNewMiddleware_zeroConfig(t *testing.T) {
	for _, cfg := range []*config.Backend{
		{},
		{ExtraConfig: map[string]interface{}{Namespace: 42}},
	} {
		resp := proxy.Response{}
		mdw := NewMiddleware(logging.NoOp, cfg)
		p := mdw(dummyProxy(&resp, nil))

		request := proxy.Request{
			Path: "/tupu",
		}

		for i := 0; i < 100; i++ {
			r, err := p(context.Background(), &request)
			if err != nil {
				t.Error(err.Error())
				return
			}
			if &resp != r {
				t.Fail()
			}
		}
	}
}

func TestNewMiddleware_ok(t *testing.T) {
	resp := proxy.Response{}
	mdw := NewMiddleware(logging.NoOp, &config.Backend{
		ExtraConfig: map[string]interface{}{Namespace: map[string]interface{}{"max_rate": 10000.0, "capacity": 10000}},
	})
	p := mdw(dummyProxy(&resp, nil))

	request := proxy.Request{
		Path: "/tupu",
	}

	for i := 0; i < 1000; i++ {
		r, err := p(context.Background(), &request)
		if err != nil {
			t.Error(err.Error())
			return
		}
		if &resp != r {
			t.Fail()
		}
	}
}

func TestNewMiddleware_capacity(t *testing.T) {
	resp := proxy.Response{}
	mdw := NewMiddleware(logging.NoOp, &config.Backend{
		ExtraConfig: map[string]interface{}{Namespace: map[string]interface{}{"max_rate": 100000.0, "every": "10s"}},
	})
	p := mdw(dummyProxy(&resp, nil))

	request := proxy.Request{
		Path: "/tupu",
	}

	for i := 0; i < 10000; i++ {
		r, err := p(context.Background(), &request)
		if err != nil {
			t.Error(err.Error())
			return
		}
		if &resp != r {
			t.Fail()
		}
	}
}

func TestNewMiddleware_ko(t *testing.T) {
	expected := proxy.Response{}
	calls := uint64(0)
	mdw := NewMiddleware(logging.NoOp, &config.Backend{
		ExtraConfig: map[string]interface{}{Namespace: map[string]interface{}{"max_rate": 1.0, "capacity": 1.0}},
	})
	p := mdw(func(_ context.Context, _ *proxy.Request) (*proxy.Response, error) {
		total := atomic.AddUint64(&calls, 1)
		if total > 2 {
			t.Error("This proxy shouldn't been executed!")
		}
		return &expected, nil
	})

	request := proxy.Request{
		Path: "/tupu",
	}

	for i := 0; i < 100; i++ {
		p(context.Background(), &request)
	}

	r, err := p(context.Background(), &request)
	if err != krakendrate.ErrLimited {
		t.Errorf("error expected")
	}
	if nil != r {
		t.Error("unexpected response")
	}
	if calls != 1 {
		t.Error("unexpected number of calls to the proxy")
	}
}

func dummyProxy(r *proxy.Response, err error) proxy.Proxy {
	return func(_ context.Context, _ *proxy.Request) (*proxy.Response, error) {
		return r, err
	}
}
