package gin

import (
	"context"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/devopsfaith/krakend/config"
	"github.com/devopsfaith/krakend/proxy"
	krakendgin "github.com/devopsfaith/krakend/router/gin"
	"github.com/gin-gonic/gin"

	krakendrate "github.com/devopsfaith/krakend-ratelimit"
	"github.com/devopsfaith/krakend-ratelimit/juju"
	"github.com/devopsfaith/krakend-ratelimit/juju/router"
)

// HandlerFactory is the out-of-the-box basic ratelimit handler factory using the default krakend endpoint
// handler for the gin router
var HandlerFactory = NewRateLimiterMw(krakendgin.EndpointHandler)

// NewRateLimiterMw builds a rate limiting wrapper over the received handler factory.
func NewRateLimiterMw(next krakendgin.HandlerFactory) krakendgin.HandlerFactory {
	return func(remote *config.EndpointConfig, p proxy.Proxy) gin.HandlerFunc {
		handlerFunc := next(remote, p)

		cfg := router.ConfigGetter(remote.ExtraConfig).(router.Config)
		if cfg == router.ZeroCfg || (cfg.MaxRate <= 0 && cfg.ClientMaxRate <= 0 || cfg.TierConfiguration == nil) {
			return handlerFunc
		}

		if cfg.MaxRate > 0 {
			handlerFunc = NewEndpointRateLimiterMw(juju.NewLimiter(float64(cfg.MaxRate), cfg.MaxRate))(handlerFunc)
		}
		if cfg.ClientMaxRate > 0 {
			switch strings.ToLower(cfg.Strategy) {
			case "ip":
				handlerFunc = NewIpLimiterWithKeyMw(cfg.Key, float64(cfg.ClientMaxRate), cfg.ClientMaxRate)(handlerFunc)
			case "header":
				handlerFunc = NewHeaderLimiterMw(cfg.Key, float64(cfg.ClientMaxRate), cfg.ClientMaxRate)(handlerFunc)
			}
		}
		if cfg.TierConfiguration != nil {
			strategy := strings.ToLower(cfg.TierConfiguration.Strategy)
			duration, err := time.ParseDuration(cfg.TierConfiguration.Duration)
			if err != nil {
				log.Printf("%s => Tier Configuration will be ignored.", err)
			} else if strategy != "ip" && strategy != "header" {
				log.Printf("%s is not a valid strategy => Tier Configuration will be ignored", strategy)
			} else {
				handlerFunc = NewTierLimiterMw(cfg.TierConfiguration, duration)(handlerFunc)
			}
		}
		return handlerFunc
	}
}

// EndpointMw is a function that decorates the received handlerFunc with some rateliming logic
type EndpointMw func(gin.HandlerFunc) gin.HandlerFunc

// NewEndpointRateLimiterMw creates a simple ratelimiter for a given handlerFunc
func NewEndpointRateLimiterMw(tb juju.Limiter) EndpointMw {
	return func(next gin.HandlerFunc) gin.HandlerFunc {
		return func(c *gin.Context) {
			if !tb.Allow() {
				c.AbortWithError(503, krakendrate.ErrLimited)
				return
			}
			next(c)
		}
	}
}

// NewHeaderLimiterMw creates a token ratelimiter using the value of a header as a token
func NewHeaderLimiterMw(header string, maxRate float64, capacity int64) EndpointMw {
	return NewTokenLimiterMw(HeaderTokenExtractor(header), juju.NewMemoryStore(maxRate, capacity))
}

// NewHeaderLimiterMw creates a token ratelimiter using the IP of the request as a token
func NewIpLimiterMw(maxRate float64, capacity int64) EndpointMw {
	return NewTokenLimiterMw(IPTokenExtractor, juju.NewMemoryStore(maxRate, capacity))
}

// NewIpLimiterWithKeyMw creates a token ratelimiter using the IP of the request as a token
func NewIpLimiterWithKeyMw(header string, maxRate float64, capacity int64) EndpointMw {
	if header == "" {
		return NewIpLimiterMw(maxRate, capacity)
	}
	return NewTokenLimiterMw(NewIPTokenExtractor(header), juju.NewMemoryStore(maxRate, capacity))
}

// NewIpLimiterWithKeyMw creates a token ratelimiter using the IP/header of the request and tier name as a token
func NewTierLimiterMw(tierConfiguration *router.TierConfiguration, fillInterval time.Duration) EndpointMw {
	var storesPerTier = krakendrate.NewShardedMemoryBackend(context.Background(), 2, fillInterval, krakendrate.PseudoFNV64a)
	for _, tier := range tierConfiguration.Tiers {
		if tier.Limit > 0 {
			storesPerTier.Store(tier.Name, juju.NewMemoryDurationStore(fillInterval, tier.Limit))
		}
	}
	return NewTokenLimiterPerTierMw(
		NewConcatTokenExtractor(tierConfiguration.HeaderTier, strings.ToLower(tierConfiguration.Strategy), tierConfiguration.Key),
		fillInterval,
		storesPerTier,
	)
}

// TokenExtractor defines the interface of the functions to use in order to extract a token for each request
type TokenExtractor func(*gin.Context) string

// IPTokenExtractor extracts the IP of the request
func IPTokenExtractor(c *gin.Context) string { return c.ClientIP() }

// NewIPTokenExtractor generates an IP TokenExtractor checking first for the contents of the passed header.
// If nothing is found there, the regular IPTokenExtractor function is called.
func NewIPTokenExtractor(header string) TokenExtractor {
	return func(c *gin.Context) string {
		if clientIP := strings.TrimSpace(strings.Split(c.Request.Header.Get(header), ",")[0]); clientIP != "" {
			ip := strings.Split(clientIP, ":")[0]
			if parsedIP := net.ParseIP(ip); parsedIP != nil {
				return ip
			}
		}
		return IPTokenExtractor(c)
	}
}

// HeaderTokenExtractor returns a TokenExtractor that looks for the value of the designed header
func HeaderTokenExtractor(header string) TokenExtractor {
	return func(c *gin.Context) string { return c.Request.Header.Get(header) }
}

// ConcatTokenExtractor returns a TokenExtractor that concatenates all passed token extractors
func ConcatTokenExtractor(tokenExtractors []TokenExtractor) TokenExtractor {
	return func(c *gin.Context) string {
		var tokenValues = make([]string, len(tokenExtractors))
		for i, tokenExtractor := range tokenExtractors {
			tokenValues[i] = tokenExtractor(c)
		}
		return strings.Join(tokenValues, "-")
	}
}

// NewConcatTokenExtractor generates a ConcatTokenExtractor using ip or header extractors depending on the strategy
func NewConcatTokenExtractor(headerTier string, strategy string, key string) TokenExtractor {
	var tierTokenExtractor = HeaderTokenExtractor(headerTier)
	var clientIdentifierTokenExtractor TokenExtractor
	if strategy == "ip" {
		if key == "" {
			clientIdentifierTokenExtractor = IPTokenExtractor
		} else {
			clientIdentifierTokenExtractor = NewIPTokenExtractor(key)
		}
	} else if strategy == "header" {
		clientIdentifierTokenExtractor = HeaderTokenExtractor(key)
	}
	return ConcatTokenExtractor([]TokenExtractor{tierTokenExtractor, clientIdentifierTokenExtractor})
}

// NewTokenLimiterMw returns a token based ratelimiting endpoint middleware with the received TokenExtractor and LimiterStore
func NewTokenLimiterMw(tokenExtractor TokenExtractor, limiterStore krakendrate.LimiterStore) EndpointMw {
	return func(next gin.HandlerFunc) gin.HandlerFunc {
		return func(c *gin.Context) {
			tokenKey := tokenExtractor(c)
			if tokenKey == "" {
				c.AbortWithError(http.StatusTooManyRequests, krakendrate.ErrLimited)
				return
			}
			if !limiterStore(tokenKey).Allow() {
				c.AbortWithError(http.StatusTooManyRequests, krakendrate.ErrLimited)
				return
			}
			next(c)
		}
	}
}

// NewTokenLimiterPerTierMw returns a token based ratelimiting endpoint middleware with the received TokenExtractor and different LimiterStores per tier
func NewTokenLimiterPerTierMw(tokenExtractor TokenExtractor, fillInterval time.Duration, storesPerTier *krakendrate.ShardedMemoryBackend) EndpointMw {
	var noResult = func() interface{} { return nil }
	return func(next gin.HandlerFunc) gin.HandlerFunc {
		return func(c *gin.Context) {
			tokenKey := tokenExtractor(c)
			if tokenKey == "" {
				c.AbortWithError(http.StatusTooManyRequests, krakendrate.ErrLimited)
				return
			}
			tokenKeyParts := strings.Split(tokenKey, "-")
			tierName, clientIdentifier := tokenKeyParts[0], tokenKeyParts[1]
			tierLimiter := storesPerTier.Load(tierName, noResult)
			if tierLimiter != nil {
				if !tierLimiter.(krakendrate.LimiterStore)(clientIdentifier).Allow() {
					c.AbortWithError(http.StatusTooManyRequests, krakendrate.ErrLimited)
					return
				}
			} else {
				log.Printf("Tier %s does not exist.", tierName)
			}
			next(c)
		}
	}
}
