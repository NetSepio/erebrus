package p2p

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	log "github.com/sirupsen/logrus"
)

// QUICManager manages QUIC connections and provides connection pooling and metrics
type QUICManager struct {
	host              host.Host
	connections       sync.Map // peer.ID -> *QUICConnection
	connectionMetrics *QUICMetrics
	streamManager     *QUICStreamManager
	mu                sync.RWMutex
}

// QUICConnection represents a managed QUIC connection with additional metadata
type QUICConnection struct {
	conn         network.Conn
	peerID       peer.ID
	remoteAddr   multiaddr.Multiaddr
	established  time.Time
	lastActivity time.Time
	streamCount  int32
	bytesIn      int64
	bytesOut     int64
	isHealthy    bool
	mu           sync.RWMutex
}

// QUICMetrics collects QUIC-specific performance metrics
type QUICMetrics struct {
	TotalConnections   int64
	ActiveConnections  int64
	FailedConnections  int64
	TotalStreams       int64
	ActiveStreams      int64
	BytesTransferred   int64
	AverageLatency     time.Duration
	ConnectionDuration time.Duration
	QUICPacketsLost    int64
	ConnectionUpgrades int64 // TCP to QUIC upgrades
	mu                 sync.RWMutex
}

// QUICStreamManager manages QUIC streams for efficient multiplexing
type QUICStreamManager struct {
	streams       sync.Map // streamID -> *StreamInfo
	activeStreams int32
	maxStreams    int32
	streamPool    sync.Pool
	mu            sync.RWMutex
}

// StreamInfo contains metadata about individual QUIC streams
type StreamInfo struct {
	ID           string
	PeerID       peer.ID
	Protocol     string
	Created      time.Time
	LastActivity time.Time
	BytesIn      int64
	BytesOut     int64
	IsActive     bool
}

// NewQUICManager creates a new QUIC connection manager
func NewQUICManager(h host.Host) *QUICManager {
	qm := &QUICManager{
		host:              h,
		connectionMetrics: &QUICMetrics{},
		streamManager: &QUICStreamManager{
			maxStreams: 1000, // Default max streams per connection
		},
	}

	// Initialize stream pool for efficient stream object reuse
	qm.streamManager.streamPool = sync.Pool{
		New: func() interface{} {
			return &StreamInfo{}
		},
	}

	// Start background maintenance
	go qm.maintenanceLoop()
	go qm.metricsCollectionLoop()

	return qm
}

// GetConnection retrieves or creates a QUIC connection to a peer
func (qm *QUICManager) GetConnection(peerID peer.ID) (*QUICConnection, error) {
	if conn, exists := qm.connections.Load(peerID); exists {
		qConn := conn.(*QUICConnection)
		qConn.updateActivity()
		return qConn, nil
	}

	return qm.createConnection(peerID)
}

// createConnection establishes a new QUIC connection to a peer
func (qm *QUICManager) createConnection(peerID peer.ID) (*QUICConnection, error) {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	// Double-check locking pattern
	if conn, exists := qm.connections.Load(peerID); exists {
		return conn.(*QUICConnection), nil
	}

	// Get peer addresses
	addrs := qm.host.Peerstore().Addrs(peerID)
	if len(addrs) == 0 {
		return nil, fmt.Errorf("no addresses found for peer %s", peerID)
	}

	// Try to connect using QUIC-enabled addresses first
	var conn network.Conn
	var connErr error

	for _, addr := range addrs {
		// Check if this is a QUIC address
		if isQUICAddress(addr) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			conn, connErr = qm.host.Network().DialPeer(ctx, peerID)
			cancel()

			if connErr == nil {
				break
			}

			log.WithFields(log.Fields{
				"peerID": peerID,
				"addr":   addr,
				"error":  connErr,
			}).Debug("Failed to establish QUIC connection, trying next address")
		}
	}

	if conn == nil {
		qm.connectionMetrics.FailedConnections++
		return nil, fmt.Errorf("failed to establish QUIC connection to peer %s: %v", peerID, connErr)
	}

	// Create connection wrapper
	qConn := &QUICConnection{
		conn:         conn,
		peerID:       peerID,
		remoteAddr:   conn.RemoteMultiaddr(),
		established:  time.Now(),
		lastActivity: time.Now(),
		isHealthy:    true,
	}

	// Store connection
	qm.connections.Store(peerID, qConn)
	qm.connectionMetrics.TotalConnections++
	qm.connectionMetrics.ActiveConnections++

	log.WithFields(log.Fields{
		"peerID":     peerID,
		"remoteAddr": qConn.remoteAddr,
		"transport":  getConnectionTransport(conn),
	}).Info("Established new QUIC connection")

	return qConn, nil
}

// CloseConnection closes a connection to a specific peer
func (qm *QUICManager) CloseConnection(peerID peer.ID) error {
	if conn, exists := qm.connections.LoadAndDelete(peerID); exists {
		qConn := conn.(*QUICConnection)
		qm.connectionMetrics.ActiveConnections--
		return qConn.conn.Close()
	}
	return nil
}

// GetMetrics returns current QUIC performance metrics
func (qm *QUICManager) GetMetrics() QUICMetrics {
	qm.connectionMetrics.mu.RLock()
	defer qm.connectionMetrics.mu.RUnlock()
	return *qm.connectionMetrics
}

// HealthCheck performs health checks on active connections
func (qm *QUICManager) HealthCheck() {
	qm.connections.Range(func(key, value interface{}) bool {
		qConn := value.(*QUICConnection)

		// Check connection health
		if time.Since(qConn.lastActivity) > 5*time.Minute {
			log.WithField("peerID", qConn.peerID).Debug("Connection inactive, performing health check")

			// Simple ping to check if connection is alive
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// Try to open a test stream
			if stream, err := qConn.conn.NewStream(ctx); err != nil {
				log.WithFields(log.Fields{
					"peerID": qConn.peerID,
					"error":  err,
				}).Warn("Connection health check failed, marking as unhealthy")
				qConn.isHealthy = false
			} else {
				stream.Close()
				qConn.isHealthy = true
			}
		}

		return true
	})
}

// maintenanceLoop performs periodic maintenance tasks
func (qm *QUICManager) maintenanceLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		// Perform health checks
		qm.HealthCheck()

		// Clean up inactive connections
		qm.cleanupInactiveConnections()

		// Log connection statistics
		qm.logConnectionStats()
	}
}

// cleanupInactiveConnections removes connections that have been inactive too long
func (qm *QUICManager) cleanupInactiveConnections() {
	now := time.Now()
	inactiveThreshold := 10 * time.Minute

	qm.connections.Range(func(key, value interface{}) bool {
		peerID := key.(peer.ID)
		qConn := value.(*QUICConnection)

		if now.Sub(qConn.lastActivity) > inactiveThreshold || !qConn.isHealthy {
			log.WithFields(log.Fields{
				"peerID":       peerID,
				"lastActivity": qConn.lastActivity,
				"isHealthy":    qConn.isHealthy,
			}).Debug("Cleaning up inactive QUIC connection")

			qm.CloseConnection(peerID)
		}

		return true
	})
}

// metricsCollectionLoop collects and updates metrics periodically
func (qm *QUICManager) metricsCollectionLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		qm.updateMetrics()
	}
}

// updateMetrics calculates and updates performance metrics
func (qm *QUICManager) updateMetrics() {
	qm.connectionMetrics.mu.Lock()
	defer qm.connectionMetrics.mu.Unlock()

	var totalDuration time.Duration
	var activeCount int64
	var totalStreams int64
	var totalBytes int64

	qm.connections.Range(func(key, value interface{}) bool {
		qConn := value.(*QUICConnection)
		qConn.mu.RLock()

		if qConn.isHealthy {
			activeCount++
			totalDuration += time.Since(qConn.established)
			totalStreams += int64(qConn.streamCount)
			totalBytes += qConn.bytesIn + qConn.bytesOut
		}

		qConn.mu.RUnlock()
		return true
	})

	qm.connectionMetrics.ActiveConnections = activeCount
	qm.connectionMetrics.TotalStreams = totalStreams
	qm.connectionMetrics.BytesTransferred = totalBytes

	if activeCount > 0 {
		qm.connectionMetrics.ConnectionDuration = totalDuration / time.Duration(activeCount)
	}
}

// logConnectionStats logs current connection statistics
func (qm *QUICManager) logConnectionStats() {
	metrics := qm.GetMetrics()

	log.WithFields(log.Fields{
		"totalConnections":      metrics.TotalConnections,
		"activeConnections":     metrics.ActiveConnections,
		"failedConnections":     metrics.FailedConnections,
		"totalStreams":          metrics.TotalStreams,
		"bytesTransferred":      metrics.BytesTransferred,
		"avgConnectionDuration": metrics.ConnectionDuration,
	}).Info("QUIC connection statistics")
}

// Helper methods for QUICConnection

func (qc *QUICConnection) updateActivity() {
	qc.mu.Lock()
	defer qc.mu.Unlock()
	qc.lastActivity = time.Now()
}

func (qc *QUICConnection) addBytes(in, out int64) {
	qc.mu.Lock()
	defer qc.mu.Unlock()
	qc.bytesIn += in
	qc.bytesOut += out
	qc.lastActivity = time.Now()
}

// Helper functions

// isQUICAddress checks if a multiaddr represents a QUIC connection
func isQUICAddress(addr multiaddr.Multiaddr) bool {
	protocols := addr.Protocols()
	for _, p := range protocols {
		if p.Code == multiaddr.P_QUIC || p.Code == multiaddr.P_QUIC_V1 {
			return true
		}
	}
	return false
}

// getConnectionTransport determines the transport type of a connection
func getConnectionTransport(conn network.Conn) string {
	addr := conn.RemoteMultiaddr()
	if isQUICAddress(addr) {
		if _, err := addr.ValueForProtocol(multiaddr.P_QUIC_V1); err == nil {
			return "QUIC-v1"
		}
		return "QUIC"
	}
	return "TCP"
}

// GetQUICManager returns the global QUIC manager instance
func GetQUICManager() *QUICManager {
	if Host != nil {
		// Create or return existing manager
		// This is a simplified approach - in production you might want
		// to store this as a global variable or in a context
		return NewQUICManager(Host)
	}
	return nil
}
