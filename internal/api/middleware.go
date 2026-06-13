package api

import (
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

var warnOnce sync.Once

// bearerAuth guards the node API. Phase 1 uses a static token configured via
// NODE_API_TOKEN. If unset (local dev), requests are allowed with a one-time
// warning. Phase 2 replaces this with verification of gateway-issued,
// node-scoped PASETO tokens.
func (s *Server) bearerAuth() gin.HandlerFunc {
	token := s.cfg.NodeAPIToken
	return func(c *gin.Context) {
		if token == "" {
			warnOnce.Do(func() {
				slog.Warn("NODE_API_TOKEN not set — node API is UNAUTHENTICATED (dev only)")
			})
			c.Next()
			return
		}
		auth := c.GetHeader("Authorization")
		got := strings.TrimPrefix(auth, "Bearer ")
		if got == "" || got != token {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Next()
	}
}
