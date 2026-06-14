package api

import (
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

var warnOnce sync.Once

// bearerAuth guards the node's peer-management API with the static
// NODE_API_TOKEN. It fails CLOSED: if the token is unset, the API is allowed
// only in debug mode (local dev). In release mode an unset token disables the
// authenticated routes entirely rather than exposing them. Comparison is
// constant-time to avoid leaking the token via response timing.
func (s *Server) bearerAuth() gin.HandlerFunc {
	token := s.cfg.NodeAPIToken
	debug := s.cfg.RunType == "debug"
	return func(c *gin.Context) {
		if token == "" {
			if debug {
				warnOnce.Do(func() {
					slog.Warn("NODE_API_TOKEN not set — peer API is UNAUTHENTICATED (debug only)")
				})
				c.Next()
				return
			}
			warnOnce.Do(func() {
				slog.Error("NODE_API_TOKEN not set in release mode — peer API is DISABLED until configured")
			})
			c.AbortWithStatusJSON(http.StatusServiceUnavailable,
				gin.H{"error": "node API disabled: NODE_API_TOKEN not configured"})
			return
		}
		got := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		if subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Next()
	}
}
