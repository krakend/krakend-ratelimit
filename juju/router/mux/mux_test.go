package mux

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devopsfaith/krakend-ratelimit/juju/router"
	"github.com/luraproject/lura/config"
	"github.com/luraproject/lura/proxy"
)

func TestNewRateLimiterMw_CustomHeaderIP(t *testing.T) {
	header := "X-Custom-Forwarded-For"

	cfg := &config.EndpointConfig{
		Method: "GET",
		ExtraConfig: map[string]interface{}{
			router.Namespace: map[string]interface{}{
				"strategy":      "ip",
				"clientMaxRate": 100,
				"key":           header,
			},
		},
	}

	rd := func(req *http.Request) {
		req.Header.Add(header, "1.1.1.1,2.2.2.2,3.3.3.3")
	}

	testRateLimiterMw(t, rd, cfg)
}

func TestNewRateLimiterMw_CustomHeader(t *testing.T) {
	header := "X-Custom-Forwarded-For"

	cfg := &config.EndpointConfig{
		Method: "GET",
		ExtraConfig: map[string]interface{}{
			router.Namespace: map[string]interface{}{
				"strategy":      "header",
				"clientMaxRate": 100,
				"key":           header,
			},
		},
	}

	rd := func(req *http.Request) {
		req.Header.Add(header, "1.1.1.1,2.2.2.2,3.3.3.3")
	}

	testRateLimiterMw(t, rd, cfg)
}

func TestNewRateLimiterMw_DefaultIP(t *testing.T) {
	cfg := &config.EndpointConfig{
		ExtraConfig: map[string]interface{}{
			router.Namespace: map[string]interface{}{
				"strategy":      "ip",
				"clientMaxRate": 100,
			},
		},
	}

	rd := func(req *http.Request) {}

	testRateLimiterMw(t, rd, cfg)
}

type requestDecorator func(*http.Request)

func testRateLimiterMw(t *testing.T, rd requestDecorator, cfg *config.EndpointConfig) {
	var hits, ok, ko int64
	p := func(ctx context.Context, req *proxy.Request) (*proxy.Response, error) {
		atomic.AddInt64(&hits, 1)
		return &proxy.Response{
			IsComplete: true,
			Data:       map[string]interface{}{"": ""},
		}, nil
	}

	mux := http.NewServeMux()
	mux.Handle("/", HandlerFactory(cfg, p))

	total := 10000
	start := time.Now()
	for i := 0; i < total; i++ {
		req, _ := http.NewRequest("GET", "/", nil)
		rd(req)
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)
		if w.Result().StatusCode == 200 {
			ok++
			continue
		}
		if w.Result().StatusCode == 429 {
			ko++
			continue
		}
		fmt.Println(w.Result().StatusCode)
	}

	if hits != ok {
		t.Errorf("hits do not match the tracked oks: %d/%d", hits, ok)
	}

	if d := time.Since(start); d > time.Second {
		return
	}

	if ok+ko != int64(total) {
		t.Errorf("not all the requests were tracked: %d/%d", ok, ko)
	}

}
