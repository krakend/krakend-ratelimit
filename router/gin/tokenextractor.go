package gin

import (
	"errors"
	"net"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/krakendio/krakend-ratelimit/v3/router"
)

var ErrNotFound = errors.New("not found")

// TokenExtractor defines the interface of the functions to use in order to extract a token for each request
type TokenExtractor func(*gin.Context) string

// IPTokenExtractor extracts the IP of the request
func IPTokenExtractor(c *gin.Context) string { return c.ClientIP() }

// NewIPTokenExtractor generates an IP TokenExtractor checking first for the contents of the passed header.
// If nothing is found there, the regular IPTokenExtractor function is called.
func NewIPTokenExtractor(header string) TokenExtractor {
	if header == "" {
		return IPTokenExtractor
	}
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

// ParamTokenExtractor returns a TokenExtractor that uses a param a token
func ParamTokenExtractor(param string) TokenExtractor {
	return func(c *gin.Context) string { return c.Param(param) }
}

// TokenExtractorFromCfg selects the token extractor to use from the input config
func TokenExtractorFromCfg(cfg router.Config) (TokenExtractor, error) {
	switch strategy := strings.ToLower(cfg.Strategy); strategy {
	case "ip":
		return NewIPTokenExtractor(cfg.Key), nil
	case "header":
		return HeaderTokenExtractor(cfg.Key), nil
	case "param":
		return ParamTokenExtractor(cfg.Key), nil
	default:
		return nil, ErrNotFound
	}

	return nil, ErrNotFound
}
