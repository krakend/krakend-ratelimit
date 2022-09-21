package gin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/krakendio/krakend-ratelimit/v2/juju/router"
	"github.com/luraproject/lura/v2/config"
	"github.com/luraproject/lura/v2/proxy"
)

func TestNewRateLimiterMw_CustomHeaderIP(t *testing.T) {
	header := "X-Custom-Forwarded-For"

	cfg := &config.EndpointConfig{
		ExtraConfig: map[string]interface{}{
			router.Namespace: map[string]interface{}{
				"strategy":        "ip",
				"client_max_rate": 100,
				"client_capacity": 100,
				"key":             header,
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
		ExtraConfig: map[string]interface{}{
			router.Namespace: map[string]interface{}{
				"strategy":        "header",
				"client_max_rate": 100,
				"client_capacity": 100,
				"key":             header,
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
				"strategy":        "ip",
				"client_max_rate": 100,
				"client_capacity": 100,
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
		return &proxy.Response{}, nil
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()

	r.GET("/", HandlerFactory(cfg, p))

	total := 10000
	start := time.Now()
	for i := 0; i < total; i++ {
		req, _ := http.NewRequest("GET", "/", http.NoBody)
		rd(req)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)
		if w.Result().StatusCode == 200 {
			ok++
			continue
		}
		if w.Result().StatusCode == 429 {
			ko++
			continue
		}
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
