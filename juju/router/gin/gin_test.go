package gin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devopsfaith/krakend-ratelimit/juju/router"
	"github.com/devopsfaith/krakend/config"
	"github.com/devopsfaith/krakend/proxy"
	"github.com/gin-gonic/gin"
)

func TestNewRateLimiterMw_CustomHeaderIP(t *testing.T) {
	header := "X-Custom-Forwarded-For"

	cfg := &config.EndpointConfig{
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

func TestNewRateLimiterMw_TierCustomHeader(t *testing.T) {
	headerTier := "X-Tier"
	headerUser := "X-User"

	cfg := &config.EndpointConfig{
		ExtraConfig: map[string]interface{}{
			router.Namespace: map[string]interface{}{
				"tierConfiguration": map[string]interface{}{
					"headerTier": headerTier,
					"strategy":   "header",
					"key":        headerUser,
					"duration":   "1s",
					"tiers": []map[string]interface{}{
						{
							"name":  "tier1",
							"limit": 100,
						},
						{
							"name":  "tier2",
							"limit": 200,
						},
					},
				},
			},
		},
	}

	rd := func(req *http.Request) {
		req.Header.Add(headerTier, "tier1")
		req.Header.Add(headerUser, "1234567890")
	}

	testRateLimiterMw(t, rd, cfg)
}

func TestNewRateLimiterMw_TierDefaultIP(t *testing.T) {
	headerTier := "X-Tier"

	cfg := &config.EndpointConfig{
		ExtraConfig: map[string]interface{}{
			router.Namespace: map[string]interface{}{
				"tierConfiguration": map[string]interface{}{
					"headerTier": headerTier,
					"strategy":   "ip",
					"duration":   "1s",
					"tiers": []map[string]interface{}{
						{
							"name":  "tier1",
							"limit": 100,
						},
						{
							"name":  "tier2",
							"limit": 200,
						},
					},
				},
			},
		},
	}

	rd := func(req *http.Request) {
		req.Header.Add(headerTier, "tier1")
	}

	testRateLimiterMw(t, rd, cfg)
}

func TestNewRateLimiterMw_TierCustomHeaderIP(t *testing.T) {
	headerTier := "X-Tier"
	headerIP := "X-Custom-Forwarded-For"

	cfg := &config.EndpointConfig{
		ExtraConfig: map[string]interface{}{
			router.Namespace: map[string]interface{}{
				"tierConfiguration": map[string]interface{}{
					"headerTier": headerTier,
					"strategy":   "ip",
					"key":        headerIP,
					"duration":   "1s",
					"tiers": []map[string]interface{}{
						{
							"name":  "tier1",
							"limit": 100,
						},
						{
							"name":  "tier2",
							"limit": 200,
						},
					},
				},
			},
		},
	}

	rd := func(req *http.Request) {
		req.Header.Add(headerTier, "tier1")
		req.Header.Add(headerIP, "1.1.1.1,2.2.2.2,3.3.3.3")
	}

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
		req, _ := http.NewRequest("GET", "/", nil)
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
