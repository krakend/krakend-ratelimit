package mux

import (
	"net/http"
	"strings"

	"github.com/luraproject/lura/config"
	"github.com/luraproject/lura/proxy"
	luramux "github.com/luraproject/lura/router/mux"

	krakendrate "github.com/devopsfaith/krakend-ratelimit"
	"github.com/devopsfaith/krakend-ratelimit/rate"
	"github.com/devopsfaith/krakend-ratelimit/rate/router"
)

// HandlerFactory is the out-of-the-box basic ratelimit handler factory using the default krakend endpoint
// handler for the mux router
var HandlerFactory = NewRateLimiterMw(luramux.EndpointHandler)

// NewRateLimiterMw builds a rate limiting wrapper over the received handler factory.
func NewRateLimiterMw(next luramux.HandlerFactory) luramux.HandlerFactory {
	return func(remote *config.EndpointConfig, p proxy.Proxy) http.HandlerFunc {
		handlerFunc := next(remote, p)

		cfg := router.ConfigGetter(remote.ExtraConfig).(router.Config)
		if cfg == router.ZeroCfg || (cfg.MaxRate <= 0 && cfg.ClientMaxRate <= 0) {
			return handlerFunc
		}

		if cfg.MaxRate > 0 {
			handlerFunc = NewEndpointRateLimiterMw(rate.NewLimiter(float64(cfg.MaxRate), cfg.MaxRate))(handlerFunc)
		}
		if cfg.ClientMaxRate > 0 {
			switch strings.ToLower(cfg.Strategy) {
			case "ip":
				handlerFunc = NewIpLimiterMw(float64(cfg.ClientMaxRate), cfg.ClientMaxRate)(handlerFunc)
			case "header":
				handlerFunc = NewHeaderLimiterMw(cfg.Key, float64(cfg.ClientMaxRate), cfg.ClientMaxRate)(handlerFunc)
			}
		}
		return handlerFunc
	}
}

// EndpointMw is a function that decorates the received handlerFunc with some rateliming logic
type EndpointMw func(http.HandlerFunc) http.HandlerFunc

// NewEndpointRateLimiterMw creates a simple ratelimiter for a given handlerFunc
func NewEndpointRateLimiterMw(tb rate.Limiter) EndpointMw {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if !tb.Allow() {
				w.WriteHeader(http.StatusServiceUnavailable)
				w.Write([]byte(krakendrate.ErrLimited.Error()))
				return
			}
			next(w, r)
		}
	}
}

// NewHeaderLimiterMw creates a token ratelimiter using the value of a header as a token
func NewHeaderLimiterMw(header string, maxRate float64, capacity int) EndpointMw {
	return NewTokenLimiterMw(HeaderTokenExtractor(header), rate.NewMemoryStore(maxRate, capacity))
}

// NewIpLimiterMw creates a token ratelimiter using the IP of the request as a token
func NewIpLimiterMw(maxRate float64, capacity int) EndpointMw {
	return NewTokenLimiterMw(IPTokenExtractor, rate.NewMemoryStore(maxRate, capacity))
}

// TokenExtractor defines the interface of the functions to use in order to extract a token for each request
type TokenExtractor func(*http.Request) string

// IPTokenExtractor extracts the IP of the request
func IPTokenExtractor(r *http.Request) string {
	var ip string = r.Header.Get("X-Forwarded-For")
	if len(ip) < 0 {
		return strings.Split(ip, ":")[0]
	}
	ip = r.Header.Get("X-Real-Ip")
	if len(ip) < 0 {
		return strings.Split(ip, ":")[0]
	}
	return r.RemoteAddr
}

// HeaderTokenExtractor returns a TokenExtractor that looks for the value of the designed header
func HeaderTokenExtractor(header string) TokenExtractor {
	return func(r *http.Request) string { return r.Header.Get(header) }
}

// NewTokenLimiterMw returns a token based ratelimiting endpoint middleware with the received TokenExtractor and LimiterStore
func NewTokenLimiterMw(tokenExtractor TokenExtractor, limiterStore krakendrate.LimiterStore) EndpointMw {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenKey := tokenExtractor(r)
			if tokenKey == "" {
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(krakendrate.ErrLimited.Error()))
				return
			}
			if !limiterStore(tokenKey).Allow() {
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(krakendrate.ErrLimited.Error()))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
