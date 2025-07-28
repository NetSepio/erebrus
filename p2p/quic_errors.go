package p2p

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	log "github.com/sirupsen/logrus"
)

// QUIC-specific error types for better error handling and debugging

// QUICConnectionError represents errors specific to QUIC connections
type QUICConnectionError struct {
	PeerID    peer.ID
	Cause     error
	ErrorType QUICErrorType
	Timestamp time.Time
	Retryable bool
}

// QUICErrorType represents different types of QUIC errors
type QUICErrorType int

const (
	QUICErrorUnknown QUICErrorType = iota
	QUICErrorHandshakeFailed
	QUICErrorConnectionTimeout
	QUICErrorStreamLimitExceeded
	QUICErrorTransportError
	QUICErrorVersionNegotiation
	QUICErrorConnectionRefused
	QUICErrorCertificateError
	QUICErrorNetworkUnreachable
	QUICErrorDatagramTooLarge
	QUICErrorIdleTimeout
	QUICErrorKeyUpdateFailed
	QUICErrorCongestionControl
)

// String returns a human-readable description of the error type
func (e QUICErrorType) String() string {
	switch e {
	case QUICErrorHandshakeFailed:
		return "QUIC handshake failed"
	case QUICErrorConnectionTimeout:
		return "QUIC connection timeout"
	case QUICErrorStreamLimitExceeded:
		return "QUIC stream limit exceeded"
	case QUICErrorTransportError:
		return "QUIC transport error"
	case QUICErrorVersionNegotiation:
		return "QUIC version negotiation failed"
	case QUICErrorConnectionRefused:
		return "QUIC connection refused"
	case QUICErrorCertificateError:
		return "QUIC certificate error"
	case QUICErrorNetworkUnreachable:
		return "Network unreachable"
	case QUICErrorDatagramTooLarge:
		return "QUIC datagram too large"
	case QUICErrorIdleTimeout:
		return "QUIC idle timeout"
	case QUICErrorKeyUpdateFailed:
		return "QUIC key update failed"
	case QUICErrorCongestionControl:
		return "QUIC congestion control error"
	default:
		return "Unknown QUIC error"
	}
}

// Error implements the error interface
func (e *QUICConnectionError) Error() string {
	return fmt.Sprintf("QUIC error for peer %s: %s - %v",
		e.PeerID, e.ErrorType.String(), e.Cause)
}

// IsRetryable returns whether this error indicates a retryable condition
func (e *QUICConnectionError) IsRetryable() bool {
	return e.Retryable
}

// IsTemporary indicates if this is a temporary error
func (e *QUICConnectionError) IsTemporary() bool {
	switch e.ErrorType {
	case QUICErrorConnectionTimeout, QUICErrorNetworkUnreachable,
		QUICErrorCongestionControl, QUICErrorIdleTimeout:
		return true
	default:
		return false
	}
}

// QUICErrorHandler handles QUIC-specific errors with appropriate recovery strategies
type QUICErrorHandler struct {
	maxRetries   int
	retryBackoff time.Duration
	errorCounts  map[peer.ID]int
	fallbackFunc func(peer.ID) error // Fallback to TCP if QUIC fails
}

// NewQUICErrorHandler creates a new error handler for QUIC connections
func NewQUICErrorHandler() *QUICErrorHandler {
	return &QUICErrorHandler{
		maxRetries:   3,
		retryBackoff: 1 * time.Second,
		errorCounts:  make(map[peer.ID]int),
		fallbackFunc: nil, // Can be set later
	}
}

// HandleError processes a QUIC error and determines the appropriate response
func (h *QUICErrorHandler) HandleError(err error, peerID peer.ID) (*QUICConnectionError, bool) {
	quicErr := h.classifyError(err, peerID)

	log.WithFields(log.Fields{
		"peerID":    peerID,
		"errorType": quicErr.ErrorType.String(),
		"retryable": quicErr.Retryable,
		"cause":     quicErr.Cause,
	}).Warn("QUIC connection error occurred")

	shouldRetry := h.shouldRetry(quicErr, peerID)

	if shouldRetry {
		h.errorCounts[peerID]++
		log.WithFields(log.Fields{
			"peerID":     peerID,
			"retryCount": h.errorCounts[peerID],
			"maxRetries": h.maxRetries,
		}).Debug("Will retry QUIC connection")
	} else if quicErr.ErrorType == QUICErrorVersionNegotiation ||
		quicErr.ErrorType == QUICErrorHandshakeFailed {
		// Try TCP fallback for compatibility issues
		if h.fallbackFunc != nil {
			log.WithField("peerID", peerID).Info("Attempting TCP fallback for QUIC compatibility issue")
			if err := h.fallbackFunc(peerID); err == nil {
				log.WithField("peerID", peerID).Info("Successfully fell back to TCP connection")
				return quicErr, false // Don't retry QUIC, TCP worked
			}
		}
	}

	return quicErr, shouldRetry
}

// classifyError analyzes an error and determines its QUIC-specific type
func (h *QUICErrorHandler) classifyError(err error, peerID peer.ID) *QUICConnectionError {
	errorType := QUICErrorUnknown
	retryable := false

	errStr := strings.ToLower(err.Error())

	switch {
	case strings.Contains(errStr, "handshake"):
		errorType = QUICErrorHandshakeFailed
		retryable = true
	case strings.Contains(errStr, "timeout"):
		errorType = QUICErrorConnectionTimeout
		retryable = true
	case strings.Contains(errStr, "stream limit"):
		errorType = QUICErrorStreamLimitExceeded
		retryable = false
	case strings.Contains(errStr, "version negotiation"):
		errorType = QUICErrorVersionNegotiation
		retryable = false
	case strings.Contains(errStr, "connection refused"):
		errorType = QUICErrorConnectionRefused
		retryable = true
	case strings.Contains(errStr, "certificate") || strings.Contains(errStr, "tls"):
		errorType = QUICErrorCertificateError
		retryable = false
	case strings.Contains(errStr, "network unreachable") || strings.Contains(errStr, "no route"):
		errorType = QUICErrorNetworkUnreachable
		retryable = true
	case strings.Contains(errStr, "datagram too large"):
		errorType = QUICErrorDatagramTooLarge
		retryable = false
	case strings.Contains(errStr, "idle timeout"):
		errorType = QUICErrorIdleTimeout
		retryable = true
	case strings.Contains(errStr, "key update"):
		errorType = QUICErrorKeyUpdateFailed
		retryable = true
	case strings.Contains(errStr, "congestion"):
		errorType = QUICErrorCongestionControl
		retryable = true
	}

	// Check if it's a network error
	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			errorType = QUICErrorConnectionTimeout
			retryable = true
		}
		if netErr.Temporary() {
			retryable = true
		}
	}

	return &QUICConnectionError{
		PeerID:    peerID,
		Cause:     err,
		ErrorType: errorType,
		Timestamp: time.Now(),
		Retryable: retryable,
	}
}

// shouldRetry determines if a connection attempt should be retried
func (h *QUICErrorHandler) shouldRetry(err *QUICConnectionError, peerID peer.ID) bool {
	if !err.Retryable {
		return false
	}

	errorCount := h.errorCounts[peerID]
	return errorCount < h.maxRetries
}

// ResetErrorCount resets the error count for a peer (call after successful connection)
func (h *QUICErrorHandler) ResetErrorCount(peerID peer.ID) {
	delete(h.errorCounts, peerID)
}

// SetFallbackFunc sets a function to call when QUIC fails and TCP fallback is needed
func (h *QUICErrorHandler) SetFallbackFunc(fallback func(peer.ID) error) {
	h.fallbackFunc = fallback
}

// GetErrorStats returns error statistics for monitoring
func (h *QUICErrorHandler) GetErrorStats() map[QUICErrorType]int {
	// This would be implemented with actual tracking in a production system
	stats := make(map[QUICErrorType]int)
	// Placeholder implementation
	return stats
}

// QUICRetryStrategy handles retry logic with exponential backoff
type QUICRetryStrategy struct {
	baseDelay  time.Duration
	maxDelay   time.Duration
	multiplier float64
	jitter     bool
}

// NewQUICRetryStrategy creates a new retry strategy with exponential backoff
func NewQUICRetryStrategy() *QUICRetryStrategy {
	return &QUICRetryStrategy{
		baseDelay:  100 * time.Millisecond,
		maxDelay:   30 * time.Second,
		multiplier: 2.0,
		jitter:     true,
	}
}

// GetDelay calculates the delay for a given retry attempt
func (rs *QUICRetryStrategy) GetDelay(attempt int) time.Duration {
	delay := rs.baseDelay
	for i := 0; i < attempt; i++ {
		delay = time.Duration(float64(delay) * rs.multiplier)
		if delay > rs.maxDelay {
			delay = rs.maxDelay
			break
		}
	}

	if rs.jitter {
		// Add up to 10% jitter to prevent thundering herd
		jitterRange := float64(delay) * 0.1
		jitter := time.Duration(jitterRange * (0.5 - float64(time.Now().UnixNano()%1000)/1000))
		delay += jitter
	}

	return delay
}

// IsQUICError checks if an error is QUIC-related
func IsQUICError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())
	quicKeywords := []string{
		"quic", "udp", "datagram", "stream limit",
		"version negotiation", "handshake", "0-rtt",
	}

	for _, keyword := range quicKeywords {
		if strings.Contains(errStr, keyword) {
			return true
		}
	}

	return false
}

// RecoverFromQUICError attempts to recover from a QUIC error
func RecoverFromQUICError(err error, peerID peer.ID, manager *QUICManager) error {
	if manager == nil {
		return err
	}

	// Close the problematic connection
	if closeErr := manager.CloseConnection(peerID); closeErr != nil {
		log.WithFields(log.Fields{
			"peerID": peerID,
			"error":  closeErr,
		}).Warn("Failed to close problematic QUIC connection")
	}

	// Wait a bit before attempting recovery
	time.Sleep(1 * time.Second)

	// Attempt to establish a new connection
	_, newErr := manager.GetConnection(peerID)
	if newErr == nil {
		log.WithField("peerID", peerID).Info("Successfully recovered QUIC connection")
		return nil
	}

	log.WithFields(log.Fields{
		"peerID":      peerID,
		"originalErr": err,
		"recoveryErr": newErr,
	}).Error("Failed to recover QUIC connection")

	return fmt.Errorf("failed to recover QUIC connection: %v (original: %v)", newErr, err)
}
