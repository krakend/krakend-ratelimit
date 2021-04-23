package gin

import (
	"encoding/base64"
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

	"github.com/tidwall/gjson"
)

// HandlerFactory is the out-of-the-box basic ratelimit handler factory using the default krakend endpoint
// handler for the gin router
var HandlerFactory = NewRateLimiterMw(krakendgin.EndpointHandler)

// NewRateLimiterMw builds a rate limiting wrapper over the received handler factory.
func NewRateLimiterMw(next krakendgin.HandlerFactory) krakendgin.HandlerFactory {
	return func(remote *config.EndpointConfig, p proxy.Proxy) gin.HandlerFunc {
		handlerFunc := next(remote, p)

		cfg := router.ConfigGetter(remote.ExtraConfig).(router.Config)
		if cfg == router.ZeroCfg || (cfg.MaxRate <= 0 && cfg.ClientMaxRate <= 0) {
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
			duration, err := time.ParseDuration(cfg.TierConfiguration.Duration)
			if err != nil {
				log.Printf("%s => Tier Configuration will be ignored.", err)
			} else {
				handlerFunc = NewJwtClaimLimiterMw(cfg.TierConfiguration, duration)(handlerFunc)
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

func NewJwtClaimLimiterMw(tierConfiguration *router.TierConfiguration, fillInterval time.Duration) EndpointMw {
	var stores = map[string]krakendrate.LimiterStore{}
	var capacities = map[string]int64{}
	for _, tier := range tierConfiguration.Tiers {
		if tier.Limit > 0 {
			stores[tier.Name] = nil
			capacities[tier.Name] = tier.Limit
		}
	}
	return NewTokenLimiterPerPlanMw(JwtClaimTokenExtractor(tierConfiguration.JwtClaim), fillInterval, stores, capacities)
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

func NewTokenLimiterPerPlanMw(tokenExtractor TokenExtractor, fillInterval time.Duration, mapLimiterStore map[string]krakendrate.LimiterStore, mapCapacities map[string]int64) EndpointMw {
	return func(next gin.HandlerFunc) gin.HandlerFunc {
		return func(c *gin.Context) {
			tokenKey := tokenExtractor(c)
			if tokenKey == "" {
				c.AbortWithError(http.StatusTooManyRequests, krakendrate.ErrLimited)
				return
			}
			tierName := strings.Split(tokenKey, "-")[0]
			_, tierNameExists := mapLimiterStore[tierName]
			if tierNameExists {
				if mapLimiterStore[tierName] == nil {
					mapLimiterStore[tierName] = juju.NewMemoryDurationStore(fillInterval, mapCapacities[tierName])
				}
				if !mapLimiterStore[tierName](tokenKey).Allow() {
					c.AbortWithError(http.StatusTooManyRequests, krakendrate.ErrLimited)
					return
				}
			}
			next(c)
		}
	}
}

func JwtClaimTokenExtractor(jwtClaimName string) TokenExtractor {
	return func(c *gin.Context) string {
		bearer := c.Request.Header.Get("Authorization")
		if bearer != "" && strings.Count(bearer, ".") == 2 {
			start := strings.Index(bearer, ".")
			end := strings.LastIndex(bearer, ".")
			rawPayload, err := base64.RawStdEncoding.DecodeString(bearer[start+1 : end])
			if err != nil {
				log.Println("Invalid JWT payload (not Base64)")
				return ""
			}
			jsonPayload := string(rawPayload)
			if !gjson.Valid(jsonPayload) {
				log.Println("Invalid JWT payload (not JSON)")
				return ""
			}
			jwtClaim := gjson.Get(jsonPayload, jwtClaimName)
			sub := gjson.Get(jsonPayload, "sub")
			if !jwtClaim.Exists() {
				log.Printf("Claim '%s' not found in payload", jwtClaimName)
				return ""
			}
			if !sub.Exists() {
				log.Println("Claim 'sub' not found in payload")
				return ""
			}
			return jwtClaim.String() + "-" + sub.String()
		}
		return ""
	}
}
