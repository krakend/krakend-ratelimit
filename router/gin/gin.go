package gin

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/luraproject/lura/v2/config"
	"github.com/luraproject/lura/v2/logging"
	"github.com/luraproject/lura/v2/proxy"
	krakendgin "github.com/luraproject/lura/v2/router/gin"

	krakendrate "github.com/krakendio/krakend-ratelimit/v3"
	"github.com/krakendio/krakend-ratelimit/v3/router"
)

// HandlerFactory is the out-of-the-box basic ratelimit handler factory using the default krakend endpoint
// handler for the gin router
var HandlerFactory = NewRateLimiterMw(logging.NoOp, krakendgin.EndpointHandler)

// NewRateLimiterMw builds a rate limiting wrapper over the received handler factory.
func NewRateLimiterMw(logger logging.Logger, next krakendgin.HandlerFactory) krakendgin.HandlerFactory {
	return func(remote *config.EndpointConfig, p proxy.Proxy) gin.HandlerFunc {

		logPrefix := "[ENDPOINT: " + remote.Endpoint + "][Ratelimit]"
		handlerFunc := next(remote, p)

		cfg, err := router.ConfigGetter(remote.ExtraConfig)
		if err != nil {
			if err != router.ErrNoExtraCfg {
				logger.Error(logPrefix, err)
			}
			return handlerFunc
		}

		return RateLimiterWrapperFromCfg(logger, logPrefix, cfg, handlerFunc)
	}
}

func RateLimiterWrapperFromCfg(logger logging.Logger, logPrefix string, cfg router.Config,
	handler gin.HandlerFunc) gin.HandlerFunc {

	if cfg.MaxRate <= 0 && cfg.ClientMaxRate <= 0 {
		return handler
	}

	if cfg.MaxRate > 0 {
		if cfg.Capacity == 0 {
			if cfg.MaxRate < 1 {
				cfg.Capacity = 1
			} else {
				cfg.Capacity = uint64(cfg.MaxRate)
			}
		}
		logger.Debug(logPrefix, fmt.Sprintf("Rate limit enabled. MaxRate: %f, Capacity: %d", cfg.MaxRate, cfg.Capacity))
		handler = NewEndpointRateLimiterMw(krakendrate.NewTokenBucket(cfg.MaxRate, cfg.Capacity))(handler)
	}

	if cfg.ClientMaxRate > 0 {
		if cfg.ClientCapacity == 0 {
			if cfg.MaxRate < 1 {
				cfg.ClientCapacity = 1
			} else {
				cfg.ClientCapacity = uint64(cfg.ClientMaxRate)
			}
		}
		switch strategy := strings.ToLower(cfg.Strategy); strategy {
		case "ip":
			logger.Debug(logPrefix, fmt.Sprintf("IP-based rate limit enabled. MaxRate: %f, Capacity: %d", cfg.ClientMaxRate, cfg.ClientCapacity))
			handler = NewIpLimiterWithKeyMwFromCfg(cfg)(handler)
		case "header":
			logger.Debug(logPrefix, fmt.Sprintf("Header-based rate limit enabled. MaxRate: %f, Capacity: %d", cfg.ClientMaxRate, cfg.ClientCapacity))
			handler = NewHeaderLimiterMwFromCfg(cfg)(handler)
		default:
			logger.Warning(logPrefix, "Unknown strategy", strategy)
		}
	}
	return handler
}

// EndpointMw is a function that decorates the received handlerFunc with some rateliming logic
type EndpointMw func(gin.HandlerFunc) gin.HandlerFunc

// NewEndpointRateLimiterMw creates a simple ratelimiter for a given handlerFunc
func NewEndpointRateLimiterMw(tb *krakendrate.TokenBucket) EndpointMw {
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
//
// Deprecated: Use NewHeaderLimiterMwFromCfg instead
func NewHeaderLimiterMw(header string, maxRate float64, capacity uint64) EndpointMw {
	return NewTokenLimiterMw(HeaderTokenExtractor(header), krakendrate.NewMemoryStore(maxRate, int(capacity)))
}

// NewHeaderLimiterMwFromCfg creates a token ratelimiter using the value of a header as a token
func NewHeaderLimiterMwFromCfg(cfg router.Config) EndpointMw {
	store := StoreFromCfg(cfg)
	tokenExtractor := HeaderTokenExtractor(cfg.Key)
	return NewTokenLimiterMw(tokenExtractor, store)
}

// NewIpLimiterMw creates a token ratelimiter using the IP of the request as a token
func NewIpLimiterMw(maxRate float64, capacity uint64) EndpointMw {
	return NewTokenLimiterMw(IPTokenExtractor, krakendrate.NewMemoryStore(maxRate, int(capacity)))
}

// NewIpLimiterWithKeyMw creates a token ratelimiter using the IP of the request as a token
//
// Deprecated: Use NewIpLimiterWithKeyMwFromCfg instead
func NewIpLimiterWithKeyMw(header string, maxRate float64, capacity uint64) EndpointMw {
	tokenExtractor := NewIPTokenExtractor(header)
	return NewTokenLimiterMw(tokenExtractor, krakendrate.NewMemoryStore(maxRate, int(capacity)))
}

// NewIpLimiterWithKeyMwFromCfg creates a token ratelimiter using the IP of the request as a token
func NewIpLimiterWithKeyMwFromCfg(cfg router.Config) EndpointMw {
	store := StoreFromCfg(cfg)
	tokenExtractor := NewIPTokenExtractor(cfg.Key)
	return NewTokenLimiterMw(tokenExtractor, store)
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
