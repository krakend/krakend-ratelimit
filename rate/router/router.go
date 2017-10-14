/*
Package router provides several rate-limit routers using the golang.org/x/time/rate lib.

Sample endpoint extra config

	...
	"extra_config": {
		...
		"github.com/devopsfaith/krakend-ratelimit/rate/router": {
			"maxRate": 2000,
			"strategy": "header",
			"clientMaxRate": 100,
			"key": "X-Private-Token",
		},
		...
	},
	...

The ratelimit package provides an efficient token bucket implementation. See https://golang.org/x/time/rate
and http://en.wikipedia.org/wiki/Token_bucket for more details.
*/
package router

import "github.com/devopsfaith/krakend/config"

// Namespace is the key to use to store and access the custom config data for the router
const Namespace = "github.com/devopsfaith/krakend-ratelimit/rate/router"

// Config is the custom config struct containing the params for the router middlewares
type Config struct {
	MaxRate       int
	Strategy      string
	ClientMaxRate int
	Key           string
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
		cfg.MaxRate = int(v.(float64))
	}
	if v, ok := tmp["strategy"]; ok {
		cfg.Strategy = v.(string)
	}
	if v, ok := tmp["clientMaxRate"]; ok {
		cfg.ClientMaxRate = int(v.(float64))
	}
	if v, ok := tmp["key"]; ok {
		cfg.Key = v.(string)
	}
	return cfg
}
