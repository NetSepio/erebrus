// Package registrar abstracts on-chain node registration. v2.0 ships only a
// no-op implementation; a Solana implementation will register the node's
// PeerID, DID and IP-hash on-chain later. The NodeIdentity shape is frozen so
// the future on-chain payload is known now (see docs/v2/identity.md in the
// gateway repo).
package registrar

import (
	"context"
	"encoding/hex"
	"log/slog"

	"golang.org/x/crypto/sha3"
)

// NodeIdentity is the registration payload.
type NodeIdentity struct {
	PeerID  string
	DID     string
	IPHash  string // sha3-256 hex of the public IPv4
	Region  string
	Spec    string
	Wallet  string
	Version string
}

// Registrar registers nodes and reports status to an external system.
type Registrar interface {
	Register(ctx context.Context, id NodeIdentity) error
	UpdateStatus(ctx context.Context, peerID, status string) error
}

// New returns a Registrar for the given mode. Only "off"/"noop" are supported
// in v2.0; unknown modes fall back to no-op with a warning.
func New(mode string) Registrar {
	switch mode {
	case "", "off", "noop":
		return noop{}
	default:
		slog.Warn("unsupported chain registration mode, using noop", "mode", mode)
		return noop{}
	}
}

// HashIP returns the lowercase hex SHA3-256 of an IP string, used to obfuscate
// node IPs anywhere they leave the operational trust boundary.
func HashIP(ip string) string {
	sum := sha3.Sum256([]byte(ip))
	return hex.EncodeToString(sum[:])
}

type noop struct{}

func (noop) Register(_ context.Context, id NodeIdentity) error {
	slog.Info("registrar noop: skipping on-chain registration",
		"peer_id", id.PeerID, "did", id.DID, "region", id.Region)
	return nil
}

func (noop) UpdateStatus(_ context.Context, peerID, status string) error {
	slog.Debug("registrar noop: skipping status update", "peer_id", peerID, "status", status)
	return nil
}
