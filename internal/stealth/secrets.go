package stealth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// settings keys for the node-wide stealth carrier secrets.
const (
	keyRealityPrivate     = "stealth_reality_private_key"
	keyRealityPublic      = "stealth_reality_public_key"
	keyRealityShortID     = "stealth_reality_short_id"
	keyVLESSUUID          = "stealth_vless_uuid"
	keyVLESSBootstrapUUID = "stealth_vless_bootstrap_uuid"
	keyHysteria2Pass      = "stealth_hysteria2_password"

	// A client whose only route to the gateway is the bootstrap carrier cannot
	// fetch a fresh descriptor after its credential stops being accepted. The
	// retired UUID is therefore kept in plaintext, and honoured, for one grace
	// window. The audit archive holds hashes only and cannot serve this.
	keyVLESSBootstrapPrev      = "stealth_vless_bootstrap_uuid_previous"
	keyVLESSBootstrapPrevUntil = "stealth_vless_bootstrap_previous_until"
)

// SettingsStore is the subset of the node store the stealth manager needs.
type SettingsStore interface {
	GetSetting(ctx context.Context, key string) (string, error)
	SetSetting(ctx context.Context, key, value string) error
	SetSettings(ctx context.Context, values map[string]string) error
}

// Secrets are the node-wide credentials shared by every client of the stealth
// carriers. Per-client authentication still happens inside WireGuard, so these
// secrets only gate access to the obfuscated transport, not to the VPN itself.
type Secrets struct {
	RealityPrivateKey  string // base64 RawURL, x25519
	RealityPublicKey   string // base64 RawURL, x25519
	RealityShortID     string // 8 hex chars
	VLESSUUID          string
	VLESSBootstrapUUID string
	Hysteria2Password  string

	// The previously active bootstrap UUID and the unix second it stops being
	// accepted. Empty until the first rotation.
	VLESSBootstrapPrevUUID  string
	VLESSBootstrapPrevUntil int64
}

// BootstrapGrace returns the retired bootstrap credential while it is still
// inside its grace window.
func (s *Secrets) BootstrapGrace(now time.Time) (string, bool) {
	if s == nil || s.VLESSBootstrapPrevUUID == "" || s.VLESSBootstrapPrevUUID == s.VLESSBootstrapUUID {
		return "", false
	}
	if now.Unix() >= s.VLESSBootstrapPrevUntil {
		return "", false
	}
	return s.VLESSBootstrapPrevUUID, true
}

// loadOrCreateSecrets reads the stealth secrets from the store, generating and
// persisting any that are missing.
func loadOrCreateSecrets(ctx context.Context, st SettingsStore) (*Secrets, error) {
	s := &Secrets{}

	priv, err := st.GetSetting(ctx, keyRealityPrivate)
	if err != nil {
		return nil, err
	}
	if priv == "" {
		key, err := wgtypes.GeneratePrivateKey()
		if err != nil {
			return nil, err
		}
		pub := key.PublicKey()
		priv = base64.RawURLEncoding.EncodeToString(key[:])
		pubStr := base64.RawURLEncoding.EncodeToString(pub[:])
		if err := st.SetSetting(ctx, keyRealityPrivate, priv); err != nil {
			return nil, err
		}
		if err := st.SetSetting(ctx, keyRealityPublic, pubStr); err != nil {
			return nil, err
		}
	}
	s.RealityPrivateKey = priv
	if s.RealityPublicKey, err = st.GetSetting(ctx, keyRealityPublic); err != nil {
		return nil, err
	}

	if s.RealityShortID, err = getOrSet(ctx, st, keyRealityShortID, randHex(4)); err != nil {
		return nil, err
	}
	if s.VLESSUUID, err = getOrSet(ctx, st, keyVLESSUUID, uuid.NewString()); err != nil {
		return nil, err
	}

	if s.VLESSBootstrapUUID, err = getOrSet(ctx, st, keyVLESSBootstrapUUID, uuid.NewString()); err != nil {
		return nil, err
	}
	// Absent before the first rotation, and never generated.
	if s.VLESSBootstrapPrevUUID, err = st.GetSetting(ctx, keyVLESSBootstrapPrev); err != nil {
		return nil, err
	}
	until, err := st.GetSetting(ctx, keyVLESSBootstrapPrevUntil)
	if err != nil {
		return nil, err
	}
	if until != "" {
		if s.VLESSBootstrapPrevUntil, err = strconv.ParseInt(until, 10, 64); err != nil {
			return nil, fmt.Errorf("stealth bootstrap grace deadline: %w", err)
		}
	}

	// Hysteria2 auth password is always node-generated and persisted; the
	// optional Salamander obfs password is operator-supplied (see config).
	if s.Hysteria2Password, err = getOrSet(ctx, st, keyHysteria2Pass, randToken(24)); err != nil {
		return nil, err
	}

	return s, nil
}

// getOrSet returns the stored value for key, persisting def first if absent.
func getOrSet(ctx context.Context, st SettingsStore, key, def string) (string, error) {
	v, err := st.GetSetting(ctx, key)
	if err != nil {
		return "", err
	}
	if v != "" {
		return v, nil
	}
	if err := st.SetSetting(ctx, key, def); err != nil {
		return "", err
	}
	return def, nil
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func randToken(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
