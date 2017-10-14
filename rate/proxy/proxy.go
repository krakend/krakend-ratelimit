/*
Package proxy provides a rate-limit proxy middleware using the golang.org/x/time/rate lib.

Sample backend extra config

	...
	"extra_config": {
		...
		"github.com/devopsfaith/krakend-ratelimit/rate/proxy": {
			"maxRate": 100,
			"capacity": 100
		},
		...
	},
	...

Adding the middleware to your proxy stack

	import rate "github.com/devopsfaith/krakend-ratelimit/rate/proxy"

	...

	var p proxy.Proxy
	var backend *config.Backend

	...

	p = rate.NewMiddleware(backend)(p)

	...

The ratelimit package provides an efficient token bucket implementation. See https://golang.org/x/time/rate
and http://en.wikipedia.org/wiki/Token_bucket for more details.
*/
package proxy

import (
	"context"

	"github.com/devopsfaith/krakend/config"
	"github.com/devopsfaith/krakend/proxy"

	"github.com/devopsfaith/krakend-ratelimit"
	"github.com/devopsfaith/krakend-ratelimit/rate"
)

// Namespace is the key to use to store and access the custom config data for the proxy
const Namespace = "github.com/devopsfaith/krakend-ratelimit/rate/proxy"

// Config is the custom config struct containing the params for the limiter
type Config struct {
	MaxRate  float64
	Capacity int
}

// BackendFactory adds a ratelimiting middleware wrapping the internal factory
func BackendFactory(next proxy.BackendFactory) proxy.BackendFactory {
	return func(cfg *config.Backend) proxy.Proxy {
		return NewMiddleware(cfg)(next(cfg))
	}
}

// NewMiddleware builds a middleware based on the extra config params or fallbacks to the next proxy
func NewMiddleware(remote *config.Backend) proxy.Middleware {
	cfg := ConfigGetter(remote.ExtraConfig).(Config)
	if cfg == ZeroCfg || cfg.MaxRate <= 0 {
		return proxy.EmptyMiddleware
	}
	tb := rate.NewLimiter(cfg.MaxRate, cfg.Capacity)
	return func(next ...proxy.Proxy) proxy.Proxy {
		if len(next) > 1 {
			panic(proxy.ErrTooManyProxies)
		}
		return func(ctx context.Context, request *proxy.Request) (*proxy.Response, error) {
			if !tb.Allow() {
				return nil, krakendrate.ErrLimited
			}
			return next[0](ctx, request)
		}
	}
}

// ZeroCfg is the zero value for the Config struct
var ZeroCfg = Config{}

// ConfigGetter implements the config.ConfigGetter interface. It parses the extra config for the
// rate adapter and returns a ZeroCfg if something goes wrong.
func ConfigGetter(e config.ExtraConfig) interface{} {
	v, ok := e[Namespace]
	if !ok {
		return ZeroCfg
	}
	tmp, ok := v.(map[string]interface{})
	if !ok {
		return ZeroCfg
	}
	cfg := Config{}
	if v, ok := tmp["maxRate"]; ok {
		cfg.MaxRate = v.(float64)
	}
	if v, ok := tmp["capacity"]; ok {
		cfg.Capacity = int(v.(float64))
	}
	return cfg
}
