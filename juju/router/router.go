/*
Package router provides several rate-limit routers using the github.com/juju/ratelimit lib.

Sample endpoint extra config

	...
	"extra_config": {
		...
		"github.com/devopsfaith/krakend-ratelimit/juju/router": {
			"max_rate": 2000,
			"strategy": "header",
			"client_max_rate": 100,
			"key": "X-Private-Token",
		},
		...
	},
	...

The ratelimit package provides an efficient token bucket implementation. See https://github.com/juju/ratelimit
and http://en.wikipedia.org/wiki/Token_bucket for more details.
*/
package router

import (
	"errors"
	"fmt"

	"github.com/luraproject/lura/v2/config"
)

// Namespace is the key to use to store and access the custom config data for the router
const Namespace = "github.com/devopsfaith/krakend-ratelimit/juju/router"

// Config is the custom config struct containing the params for the router middlewares
type Config struct {
	MaxRate       int64
	Strategy      string
	ClientMaxRate int64
	Key           string
}

// ZeroCfg is the zero value for the Config struct
var ZeroCfg = Config{}

var (
	ErrNoExtraCfg    = errors.New("no extra config")
	ErrWrongExtraCfg = errors.New("wrong extra config")
)

// ConfigGetter parses the extra config for the rate adapter and
// returns a ZeroCfg and an error if something goes wrong.
func ConfigGetter(e config.ExtraConfig) (Config, error) {
	v, ok := e[Namespace]
	if !ok {
		return ZeroCfg, ErrNoExtraCfg
	}
	tmp, ok := v.(map[string]interface{})
	if !ok {
		return ZeroCfg, ErrWrongExtraCfg
	}
	cfg := Config{}
	if v, ok := tmp["max_rate"]; ok {
		switch val := v.(type) {
		case int64:
			cfg.MaxRate = val
		case int:
			cfg.MaxRate = int64(val)
		case float64:
			cfg.MaxRate = int64(val)
		}
	}
	if v, ok := tmp["strategy"]; ok {
		cfg.Strategy = fmt.Sprintf("%v", v)
	}
	if v, ok := tmp["client_max_rate"]; ok {
		switch val := v.(type) {
		case int64:
			cfg.ClientMaxRate = val
		case int:
			cfg.ClientMaxRate = int64(val)
		case float64:
			cfg.ClientMaxRate = int64(val)
		}
	}
	if v, ok := tmp["key"]; ok {
		cfg.Key = fmt.Sprintf("%v", v)
	}
	return cfg, nil
}
