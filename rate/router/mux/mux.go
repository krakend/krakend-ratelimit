package mux

import (
	"net/http"
	"strings"

	krakendrate "github.com/krakendio/krakend-ratelimit/v2"
	"github.com/krakendio/krakend-ratelimit/v2/rate"
	"github.com/krakendio/krakend-ratelimit/v2/rate/router"
	"github.com/luraproject/lura/v2/config"
	"github.com/luraproject/lura/v2/logging"
	"github.com/luraproject/lura/v2/proxy"
	luramux "github.com/luraproject/lura/v2/router/mux"
)

// HandlerFactory is the out-of-the-box basic ratelimit handler factory using the default krakend endpoint
// handler for the mux router
var HandlerFactory = NewRateLimiterMw(logging.NoOp, luramux.EndpointHandler)

// NewRateLimiterMw builds a rate limiting wrapper over the received handler factory.
func NewRateLimiterMw(logger logging.Logger, next luramux.HandlerFactory) luramux.HandlerFactory {
	return func(remote *config.EndpointConfig, p proxy.Proxy) http.HandlerFunc {
		handlerFunc := next(remote, p)

		cfg := router.ConfigGetter(remote.ExtraConfig).(router.Config)
		if cfg == router.ZeroCfg || (cfg.MaxRate <= 0 && cfg.ClientMaxRate <= 0) {
			return handlerFunc
		}

		if cfg.MaxRate > 0 {
			handlerFunc = NewEndpointRateLimiterMw(rate.NewLimiter(float64(cfg.MaxRate), cfg.MaxRate), logger)(handlerFunc)
		}
		if cfg.ClientMaxRate > 0 {
			switch strings.ToLower(cfg.Strategy) {
			case "ip":
				handlerFunc = NewIpLimiterMw(float64(cfg.ClientMaxRate), cfg.ClientMaxRate, logger)(handlerFunc)
			case "header":
				handlerFunc = NewHeaderLimiterMw(cfg.Key, float64(cfg.ClientMaxRate), cfg.ClientMaxRate, logger)(handlerFunc)
			}
		}
		return handlerFunc
	}
}

// EndpointMw is a function that decorates the received handlerFunc with some rateliming logic
type EndpointMw func(http.HandlerFunc) http.HandlerFunc

// NewEndpointRateLimiterMw creates a simple ratelimiter for a given handlerFunc
func NewEndpointRateLimiterMw(tb rate.Limiter, l logging.Logger) EndpointMw {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if !tb.Allow() {
				l.Error(krakendrate.ErrLimited)
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			next(w, r)
		}
	}
}

// NewHeaderLimiterMw creates a token ratelimiter using the IP of the request as a token
func NewHeaderLimiterMw(header string, maxRate float64, capacity int, l logging.Logger) EndpointMw {
	return NewTokenLimiterMw(HeaderTokenExtractor(header), rate.NewMemoryStore(maxRate, capacity), l)
}

// NewIpLimiterMw creates a token ratelimiter using the IP of the request as a token
func NewIpLimiterMw(maxRate float64, capacity int, l logging.Logger) EndpointMw {
	return NewTokenLimiterMw(IPTokenExtractor, rate.NewMemoryStore(maxRate, capacity), l)
}

// TokenExtractor defines the interface of the functions to use in order to extract a token for each request
type TokenExtractor func(*http.Request) string

// IPTokenExtractor extracts the IP of the request
func IPTokenExtractor(r *http.Request) string {
	ip := r.Header.Get("X-Forwarded-For")
	if len(ip) > 0 {
		return ip
	}
	ip = r.Header.Get("X-Real-Ip")
	if len(ip) > 0 {
		return ip
	}
	return r.RemoteAddr
}

// HeaderTokenExtractor returns a TokenExtractor that looks for the value of the designed header
func HeaderTokenExtractor(header string) TokenExtractor {
	return func(r *http.Request) string { return r.Header.Get(header) }
}

// NewTokenLimiterMw returns a token based ratelimiting endpoint middleware with the received TokenExtractor and LimiterStore
func NewTokenLimiterMw(tokenExtractor TokenExtractor, limiterStore krakendrate.LimiterStore, l logging.Logger) EndpointMw {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			tokenKey := tokenExtractor(r)
			if tokenKey == "" {
				l.Error(krakendrate.ErrLimited)
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			if !limiterStore(tokenKey).Allow() {
				l.Error(krakendrate.ErrLimited)
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		}
	}
}
