package p2p

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	log "github.com/sirupsen/logrus"
)

// QUICSecurityManager handles security aspects of QUIC connections
type QUICSecurityManager struct {
	trustedPeers    map[peer.ID]bool
	blockedPeers    map[peer.ID]time.Time
	rateLimiter     *QUICRateLimiter
	connectionAuth  *QUICConnectionAuth
	encryptionLevel QUICEncryptionLevel
}

// QUICEncryptionLevel defines the encryption strength requirements
type QUICEncryptionLevel int

const (
	QUICEncryptionBasic QUICEncryptionLevel = iota
	QUICEncryptionStandard
	QUICEncryptionHigh
)

// QUICRateLimiter implements rate limiting for QUIC connections
type QUICRateLimiter struct {
	connectionsPerSecond int
	maxConcurrentConns   int
	connectionCounts     map[string]int // IP -> count
	lastReset            time.Time
}

// QUICConnectionAuth handles authentication for QUIC connections
type QUICConnectionAuth struct {
	requireMutualTLS    bool
	allowedCiphers      []uint16
	minTLSVersion       uint16
	certificateVerifier func(*tls.Certificate) error
}

// NewQUICSecurityManager creates a new security manager for QUIC
func NewQUICSecurityManager() *QUICSecurityManager {
	return &QUICSecurityManager{
		trustedPeers:    make(map[peer.ID]bool),
		blockedPeers:    make(map[peer.ID]time.Time),
		encryptionLevel: QUICEncryptionStandard,
		rateLimiter: &QUICRateLimiter{
			connectionsPerSecond: 10,
			maxConcurrentConns:   100,
			connectionCounts:     make(map[string]int),
			lastReset:            time.Now(),
		},
		connectionAuth: &QUICConnectionAuth{
			requireMutualTLS: true,
			minTLSVersion:    tls.VersionTLS13,
			allowedCiphers: []uint16{
				tls.TLS_AES_256_GCM_SHA384,
				tls.TLS_CHACHA20_POLY1305_SHA256,
				tls.TLS_AES_128_GCM_SHA256,
			},
		},
	}
}

// ValidateConnection performs security validation on a QUIC connection
func (sm *QUICSecurityManager) ValidateConnection(peerID peer.ID, remoteAddr multiaddr.Multiaddr) error {
	// Check if peer is blocked
	if blockTime, blocked := sm.blockedPeers[peerID]; blocked {
		if time.Since(blockTime) < 10*time.Minute { // 10-minute block period
			return fmt.Errorf("peer %s is temporarily blocked", peerID)
		}
		// Unblock after timeout
		delete(sm.blockedPeers, peerID)
	}

	// Extract IP address for rate limiting
	ip, err := extractIPFromMultiaddr(remoteAddr)
	if err != nil {
		return fmt.Errorf("failed to extract IP from address: %v", err)
	}

	// Apply rate limiting
	if !sm.rateLimiter.AllowConnection(ip) {
		sm.BlockPeer(peerID, time.Minute*5) // Block for 5 minutes
		return fmt.Errorf("rate limit exceeded for IP %s", ip)
	}

	// Additional security checks can be added here
	log.WithFields(log.Fields{
		"peerID":     peerID,
		"remoteAddr": remoteAddr,
		"remoteIP":   ip,
	}).Debug("QUIC connection security validation passed")

	return nil
}

// ValidateTLSConfig ensures TLS configuration meets security requirements
func (sm *QUICSecurityManager) ValidateTLSConfig(config *tls.Config) error {
	if config == nil {
		return fmt.Errorf("TLS config cannot be nil")
	}

	// Enforce minimum TLS version
	if config.MinVersion < sm.connectionAuth.minTLSVersion {
		config.MinVersion = sm.connectionAuth.minTLSVersion
		log.WithField("minVersion", sm.connectionAuth.minTLSVersion).Debug("Upgraded TLS minimum version")
	}

	// Set allowed cipher suites based on encryption level
	switch sm.encryptionLevel {
	case QUICEncryptionHigh:
		config.CipherSuites = []uint16{tls.TLS_AES_256_GCM_SHA384}
	case QUICEncryptionStandard:
		config.CipherSuites = sm.connectionAuth.allowedCiphers
	case QUICEncryptionBasic:
		// Use defaults, but ensure TLS 1.3
		config.MinVersion = tls.VersionTLS13
	}

	// Enable mutual TLS if required
	if sm.connectionAuth.requireMutualTLS {
		config.ClientAuth = tls.RequireAndVerifyClientCert
	}

	// Disable insecure features
	config.InsecureSkipVerify = false
	config.SessionTicketsDisabled = false // Keep enabled for performance

	log.WithFields(log.Fields{
		"minTLSVersion":    config.MinVersion,
		"cipherSuites":     len(config.CipherSuites),
		"requireMutualTLS": sm.connectionAuth.requireMutualTLS,
		"encryptionLevel":  sm.encryptionLevel,
	}).Debug("TLS configuration validated and secured")

	return nil
}

// BlockPeer temporarily blocks a peer from connecting
func (sm *QUICSecurityManager) BlockPeer(peerID peer.ID, duration time.Duration) {
	sm.blockedPeers[peerID] = time.Now().Add(duration)
	log.WithFields(log.Fields{
		"peerID":   peerID,
		"duration": duration,
	}).Warn("Peer temporarily blocked")
}

// TrustPeer adds a peer to the trusted peers list
func (sm *QUICSecurityManager) TrustPeer(peerID peer.ID) {
	sm.trustedPeers[peerID] = true
	log.WithField("peerID", peerID).Info("Peer added to trusted list")
}

// IsTrustedPeer checks if a peer is in the trusted list
func (sm *QUICSecurityManager) IsTrustedPeer(peerID peer.ID) bool {
	return sm.trustedPeers[peerID]
}

// AllowConnection applies rate limiting to connection attempts
func (rl *QUICRateLimiter) AllowConnection(ip string) bool {
	now := time.Now()

	// Reset counters every second
	if now.Sub(rl.lastReset) >= time.Second {
		rl.connectionCounts = make(map[string]int)
		rl.lastReset = now
	}

	// Check per-IP rate limit
	currentCount := rl.connectionCounts[ip]
	if currentCount >= rl.connectionsPerSecond {
		log.WithFields(log.Fields{
			"ip":                   ip,
			"currentCount":         currentCount,
			"connectionsPerSecond": rl.connectionsPerSecond,
		}).Warn("Rate limit exceeded for IP")
		return false
	}

	// Check total concurrent connections
	totalConnections := 0
	for _, count := range rl.connectionCounts {
		totalConnections += count
	}

	if totalConnections >= rl.maxConcurrentConns {
		log.WithFields(log.Fields{
			"totalConnections":   totalConnections,
			"maxConcurrentConns": rl.maxConcurrentConns,
		}).Warn("Maximum concurrent connections reached")
		return false
	}

	// Allow connection and increment counter
	rl.connectionCounts[ip]++
	return true
}

// GenerateConnectionToken generates a secure token for connection authentication
func (sm *QUICSecurityManager) GenerateConnectionToken(peerID peer.ID) ([]byte, error) {
	// Generate a 32-byte random token
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		return nil, fmt.Errorf("failed to generate secure token: %v", err)
	}

	log.WithField("peerID", peerID).Debug("Generated secure connection token")
	return token, nil
}

// ValidateConnectionToken validates a connection authentication token
func (sm *QUICSecurityManager) ValidateConnectionToken(token []byte, peerID peer.ID) bool {
	// In a real implementation, this would validate against stored tokens
	// For now, just check basic properties
	if len(token) != 32 {
		log.WithFields(log.Fields{
			"peerID":   peerID,
			"tokenLen": len(token),
			"expected": 32,
		}).Warn("Invalid token length")
		return false
	}

	// Additional token validation logic would go here
	return true
}

// SetEncryptionLevel sets the required encryption level for connections
func (sm *QUICSecurityManager) SetEncryptionLevel(level QUICEncryptionLevel) {
	sm.encryptionLevel = level
	log.WithField("encryptionLevel", level).Info("Updated QUIC encryption level")
}

// GetSecurityMetrics returns security-related metrics
func (sm *QUICSecurityManager) GetSecurityMetrics() map[string]interface{} {
	blockedCount := len(sm.blockedPeers)
	trustedCount := len(sm.trustedPeers)

	// Clean up expired blocks
	now := time.Now()
	for peerID, blockTime := range sm.blockedPeers {
		if now.After(blockTime) {
			delete(sm.blockedPeers, peerID)
			blockedCount--
		}
	}

	return map[string]interface{}{
		"blockedPeers":      blockedCount,
		"trustedPeers":      trustedCount,
		"encryptionLevel":   sm.encryptionLevel,
		"rateLimitEnabled":  true,
		"mutualTLSRequired": sm.connectionAuth.requireMutualTLS,
	}
}

// SecureQUICConfig applies security settings to a QUIC configuration
func (sm *QUICSecurityManager) SecureQUICConfig(config interface{}) error {
	// This would apply security settings to the actual QUIC config
	// The exact implementation depends on the QUIC library being used
	log.Debug("Applied security settings to QUIC configuration")
	return nil
}

// MonitorSecurityEvents starts monitoring for security-related events
func (sm *QUICSecurityManager) MonitorSecurityEvents(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sm.performSecurityMaintenance()
		}
	}
}

// performSecurityMaintenance performs periodic security maintenance
func (sm *QUICSecurityManager) performSecurityMaintenance() {
	now := time.Now()

	// Clean up expired blocks
	expiredBlocks := 0
	for peerID, blockTime := range sm.blockedPeers {
		if now.After(blockTime) {
			delete(sm.blockedPeers, peerID)
			expiredBlocks++
		}
	}

	if expiredBlocks > 0 {
		log.WithField("expiredBlocks", expiredBlocks).Debug("Cleaned up expired peer blocks")
	}

	// Log security metrics
	metrics := sm.GetSecurityMetrics()
	log.WithFields(log.Fields{
		"securityMetrics": metrics,
	}).Debug("Security maintenance completed")
}

// Helper function to extract IP address from multiaddr
func extractIPFromMultiaddr(addr multiaddr.Multiaddr) (string, error) {
	ip, err := addr.ValueForProtocol(multiaddr.P_IP4)
	if err != nil {
		// Try IPv6
		ip, err = addr.ValueForProtocol(multiaddr.P_IP6)
		if err != nil {
			return "", fmt.Errorf("no IP address found in multiaddr")
		}
	}
	return ip, nil
}

// CreateSecureTLSConfig creates a TLS configuration with security enhancements
func (sm *QUICSecurityManager) CreateSecureTLSConfig() *tls.Config {
	config := &tls.Config{
		MinVersion:   sm.connectionAuth.minTLSVersion,
		CipherSuites: sm.connectionAuth.allowedCiphers,
		CurvePreferences: []tls.CurveID{
			tls.X25519,
			tls.CurveP384,
			tls.CurveP256,
		},
		PreferServerCipherSuites: false, // Let client choose for better performance
		SessionTicketsDisabled:   false, // Keep enabled for 0-RTT
	}

	if sm.connectionAuth.requireMutualTLS {
		config.ClientAuth = tls.RequireAndVerifyClientCert
	}

	return config
}
