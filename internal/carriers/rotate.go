// Package carriers manages stealth carrier credential rotation with grace periods.
package carriers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"time"

	"github.com/NetSepio/erebrus/internal/stealth"
	"github.com/NetSepio/erebrus/internal/store"
)

// Rotator rotates node-wide carrier secrets.
type Rotator struct {
	St      *store.Store
	Stealth *stealth.Manager
}

// Options configures a rotation run.
type Options struct {
	GracePeriod time.Duration
	PeerID      string // optional scope label for audit
}

// Rotate generates new carrier credentials, archives hashes of the previous
// secrets with a grace expiry, and restarts stealth listeners.
func (r *Rotator) Rotate(ctx context.Context, opt Options) error {
	if r.St == nil || r.Stealth == nil {
		return fmt.Errorf("rotator not configured")
	}
	if opt.GracePeriod <= 0 {
		opt.GracePeriod = 24 * time.Hour
	}
	now := time.Now().Unix()
	expires := time.Now().Add(opt.GracePeriod).Unix()

	scope := "node"
	peer := opt.PeerID
	if peer != "" {
		scope = "peer"
	}

	// Archive current secret hashes before rotation.
	if err := r.archiveCurrent(ctx, scope, peer, expires); err != nil {
		return err
	}

	if err := r.Stealth.RotateAllSecrets(ctx); err != nil {
		return fmt.Errorf("rotate secrets: %w", err)
	}

	// Record new active credentials (hashes only).
	for _, item := range []struct{ transport, material string }{
		{"vless_reality", r.Stealth.Params().VLESSUUID},
		{"hysteria2", r.Stealth.Params().Hysteria2Password},
		{"reality_short_id", r.Stealth.Params().RealityShortID},
	} {
		if item.material == "" {
			continue
		}
		if err := r.St.InsertCarrierCredential(ctx, store.CarrierCredential{
			Transport:  item.transport,
			SecretHash: hashSecret(item.material),
			CreatedAt:  now,
			Active:     true,
			Scope:      scope,
			PeerID:     peer,
		}); err != nil {
			return err
		}
	}

	n, _ := r.St.DeactivateExpiredCarrierCredentials(ctx, now)
	if n > 0 {
		slog.Info("carrier credentials expired", "count", n)
	}
	slog.Info("carrier rotation complete", "grace_period", opt.GracePeriod.String(), "scope", scope)
	return nil
}

func (r *Rotator) archiveCurrent(ctx context.Context, scope, peerID string, expiresAt int64) error {
	p := r.Stealth.Params()
	if !p.Enabled {
		return nil
	}
	now := time.Now().Unix()
	for _, item := range []struct{ transport, material string }{
		{"vless_reality", p.VLESSUUID},
		{"hysteria2", p.Hysteria2Password},
		{"reality_short_id", p.RealityShortID},
	} {
		if item.material == "" {
			continue
		}
		if err := r.St.InsertCarrierCredential(ctx, store.CarrierCredential{
			Transport:  item.transport,
			SecretHash: hashSecret(item.material),
			CreatedAt:  now,
			ExpiresAt:  expiresAt,
			Active:     true,
			Scope:      scope,
			PeerID:     peerID,
		}); err != nil {
			return err
		}
	}
	return nil
}

func hashSecret(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
