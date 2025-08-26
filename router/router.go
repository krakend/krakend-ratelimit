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

	krakendrate "github.com/krakend/krakend-ratelimit/v3"
	"github.com/luraproject/lura/v2/config"
)

// Namespace is the key to use to store and access the custom config data for the router
const Namespace = "qos/ratelimit/router"

// Config is the custom config struct containing the params for the router middlewares
type Config struct {
	MaxRate        float64       `json:"max_rate"`
	Capacity       uint64        `json:"capacity"`
	Strategy       string        `json:"strategy"`
	ClientMaxRate  float64       `json:"client_max_rate"`
	ClientCapacity uint64        `json:"client_capacity"`
	Key            string        `json:"key"`
	TTL            time.Duration `json:"every"`
	NumShards      uint64        `json:"num_shards"`
	CleanUpPeriod  time.Duration `json:"cleanup_period"`
	CleanUpThreads uint64        `json:"cleanup_threads"`
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
		if err != nil || every < time.Second {
			every = time.Second
		}
		factor := float64(time.Second) / float64(every)
		cfg.MaxRate = cfg.MaxRate * factor
		cfg.ClientMaxRate = cfg.ClientMaxRate * factor

		if every > cfg.TTL {
			// we do not need crypto strength random number to generate some
			// jitter in the duration, so we mark it to skipcq the check:
			cfg.TTL = time.Duration(int64((1 + 0.25*rand.Float64()) * float64(every))) // skipcq: GSC-G404
		}
	}
	cfg.NumShards = krakendrate.DefaultShards
	if v, ok := tmp["num_shards"]; ok {
		switch val := v.(type) {
		case int64:
			cfg.NumShards = uint64(val)
		case int:
			cfg.NumShards = uint64(val)
		case float64:
			cfg.NumShards = uint64(val)
		}
	}
	cfg.CleanUpPeriod = time.Minute
	if v, ok := tmp["cleanup_period"]; ok {
		cr, err := time.ParseDuration(fmt.Sprintf("%v", v))
		if err != nil {
			cr = time.Minute
		}
		// we hardcode a minimum time
		if cr < time.Second {
			cr = time.Second
		}
		cfg.CleanUpPeriod = cr
	}
	cfg.CleanUpThreads = 1
	if v, ok := tmp["cleanup_threads"]; ok {
		switch val := v.(type) {
		case int64:
			cfg.CleanUpThreads = uint64(val)
		case int:
			cfg.CleanUpThreads = uint64(val)
		case float64:
			cfg.CleanUpThreads = uint64(val)
		}
	}

	return cfg, nil
}
