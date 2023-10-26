/*
Package router provides several rate-limit routers.

The ratelimit package provides an efficient token bucket implementation. See http://en.wikipedia.org/wiki/Token_bucket for more details.
*/
package router

import (
	"errors"
	"fmt"
	"math/rand"
	"time"

	krakendrate "github.com/krakendio/krakend-ratelimit/v3"
	"github.com/luraproject/lura/v2/config"
)

// Namespace is the key to use to store and access the custom config data for the router
const Namespace = "qos/ratelimit/router"

// Config is the custom config struct containing the params for the router middlewares
type Config struct {
	MaxRate        float64
	Capacity       uint64
	Strategy       string
	ClientMaxRate  float64
	ClientCapacity uint64
	Key            string
	TTL            time.Duration
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
			cfg.MaxRate = float64(val)
		case int:
			cfg.MaxRate = float64(val)
		case float64:
			cfg.MaxRate = val
		}
	}
	if v, ok := tmp["capacity"]; ok {
		switch val := v.(type) {
		case int64:
			cfg.Capacity = uint64(val)
		case int:
			cfg.Capacity = uint64(val)
		case float64:
			cfg.Capacity = uint64(val)
		}
	}
	if v, ok := tmp["strategy"]; ok {
		cfg.Strategy = fmt.Sprintf("%v", v)
	}
	if v, ok := tmp["client_max_rate"]; ok {
		switch val := v.(type) {
		case int64:
			cfg.ClientMaxRate = float64(val)
		case int:
			cfg.ClientMaxRate = float64(val)
		case float64:
			cfg.ClientMaxRate = val
		}
	}
	if v, ok := tmp["client_capacity"]; ok {
		switch val := v.(type) {
		case int64:
			cfg.ClientCapacity = uint64(val)
		case int:
			cfg.ClientCapacity = uint64(val)
		case float64:
			cfg.ClientCapacity = uint64(val)
		}
	}
	if v, ok := tmp["key"]; ok {
		cfg.Key = fmt.Sprintf("%v", v)
	}

	cfg.TTL = krakendrate.DataTTL
	if v, ok := tmp["every"]; ok {
		every, err := time.ParseDuration(fmt.Sprintf("%v", v))
		if err != nil {
			every = time.Second
		}
		factor := float64(time.Second) / float64(every)
		cfg.MaxRate = cfg.MaxRate * factor
		cfg.ClientMaxRate = cfg.ClientMaxRate * factor

		if every > cfg.TTL {
			cfg.TTL = time.Duration(int64((1 + 0.25*rand.Float64()) * float64(every)))
		}
	}

	return cfg, nil
}
