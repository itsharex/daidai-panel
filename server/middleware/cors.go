package middleware

import (
	"net/url"
	"net"
	"strings"
	"time"

	"daidai-panel/config"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func matchesConfiguredOrigin(origin string, allowedOrigins []string) bool {
	normalizedOrigin := normalizeConfiguredOrigin(origin)
	for _, allowed := range allowedOrigins {
		if normalizeConfiguredOrigin(allowed) == normalizedOrigin {
			return true
		}
	}
	return false
}

func normalizeConfiguredOrigin(origin string) string {
	trimmed := strings.TrimSpace(origin)
	if trimmed == "" {
		return ""
	}

	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return strings.ToLower(trimmed)
	}

	hostname := strings.ToLower(parsed.Hostname())
	if isLoopbackHost(hostname) {
		hostname = "loopback"
	}

	port := parsed.Port()
	if port != "" {
		hostname = net.JoinHostPort(hostname, port)
	}

	return strings.ToLower(parsed.Scheme) + "://" + hostname
}

func isLoopbackHost(hostname string) bool {
	switch hostname {
	case "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}

func extractHost(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	if strings.Contains(value, ",") {
		value = strings.TrimSpace(strings.Split(value, ",")[0])
	}

	if parsed, err := url.Parse(value); err == nil && parsed.Host != "" {
		return strings.ToLower(parsed.Host)
	}

	return strings.ToLower(value)
}

func isSameOriginRequest(c *gin.Context, origin string) bool {
	originHost := extractHost(origin)
	if originHost == "" {
		return false
	}

	candidates := []string{
		c.Request.Host,
		c.GetHeader("X-Forwarded-Host"),
		c.GetHeader("X-Original-Host"),
	}

	for _, candidate := range candidates {
		if extractHost(candidate) == originHost {
			return true
		}
	}

	return false
}

func CORS() gin.HandlerFunc {
	allowedOrigins := []string{
		"http://localhost:5173",
		"http://localhost:5700",
	}
	if config.C != nil && len(config.C.CORS.Origins) > 0 {
		allowedOrigins = config.C.CORS.Origins
	}

	return cors.New(cors.Config{
		AllowOriginWithContextFunc: func(c *gin.Context, origin string) bool {
			if origin == "" {
				return true
			}
			if matchesConfiguredOrigin(origin, allowedOrigins) {
				return true
			}
			return isSameOriginRequest(c, origin)
		},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "Accept", "X-Requested-With"},
		ExposeHeaders:    []string{"Content-Length", "Content-Disposition"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	})
}
