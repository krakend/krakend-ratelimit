package router

import (
	"context"

	krakendrate "github.com/krakendio/krakend-ratelimit/v3"
)

func StoreFromCfg(cfg Config) krakendrate.LimiterStore {
	ctx := context.Background()
	var storeBackend krakendrate.Backend
	if cfg.NumShards > 1 {
		storeBackend = krakendrate.NewShardedBackend(
			ctx,
			cfg.NumShards,
			cfg.TTL,
			cfg.CleanUpRate,
			krakendrate.PseudoFNV64a,
			krakendrate.MemoryBackendBuilder,
		)
	} else {
		storeBackend = krakendrate.MemoryBackendBuilder(ctx, cfg.TTL, cfg.CleanUpRate, 1)[0]
	}

	return krakendrate.NewLimiterStore(cfg.ClientMaxRate, int(cfg.ClientCapacity),
		storeBackend)
}
