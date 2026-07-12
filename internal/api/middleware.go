package api

import (
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/NetSepio/erebrus/internal/gatewayauth"
	"github.com/gin-gonic/gin"
)

var warnOnce sync.Once

// gatewayAuth guards peer-management APIs. Production requires a gateway-issued
// short-lived PASETO (Authorization) plus the per-node key (X-Erebrus-Node-Key).
// Debug mode still accepts the legacy bearer node key in Authorization.
func (s *Server) gatewayAuth() gin.HandlerFunc {
	return s.gatewayAuthForPurpose("")
}

func (s *Server) gatewayAuthForPurpose(purpose string) gin.HandlerFunc {
	nodeKey := s.cfg.EffectiveNodeKey()
	gwPub := s.cfg.GatewayPublicKey
	debug := s.cfg.RunType == "debug"
	return func(c *gin.Context) {
		if nodeKey == "" {
			if debug && purpose == "" {
				warnOnce.Do(func() {
					slog.Warn("NODE_KEY not set — peer API is UNAUTHENTICATED (debug only)")
				})
				c.Next()
				return
			}
			warnOnce.Do(func() {
				slog.Error("NODE_KEY not set in release mode — peer API is DISABLED until configured")
			})
			c.AbortWithStatusJSON(http.StatusServiceUnavailable,
				gin.H{"error": "node API disabled: NODE_KEY not configured"})
			return
		}

		bearer := strings.TrimSpace(strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer "))
		headerKey := strings.TrimSpace(c.GetHeader("X-Erebrus-Node-Key"))

		if gwPub != "" && bearer != "" && headerKey != "" {
			var err error
			if purpose == "" {
				_, err = gatewayauth.VerifyGatewayCall(bearer, gwPub, s.cfg.NodeID)
			} else {
				_, err = gatewayauth.VerifyGatewayCallPurpose(bearer, gwPub, s.cfg.NodeID, purpose)
			}
			if err == nil {
				if subtle.ConstantTimeCompare([]byte(headerKey), []byte(nodeKey)) == 1 {
					c.Next()
					return
				}
			}
		}

		// Debug fallback: legacy single bearer (NODE_API_TOKEN style).
		if purpose == "" && debug && bearer != "" && subtle.ConstantTimeCompare([]byte(bearer), []byte(nodeKey)) == 1 {
			c.Next()
			return
		}

		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
	}
}
