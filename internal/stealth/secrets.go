package stealth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"

	"github.com/google/uuid"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// settings keys for the node-wide stealth carrier secrets.
const (
	keyRealityPrivate = "stealth_reality_private_key"
	keyRealityPublic  = "stealth_reality_public_key"
	keyRealityShortID = "stealth_reality_short_id"
	keyVLESSUUID      = "stealth_vless_uuid"
	keyHysteria2Pass  = "stealth_hysteria2_password"
)

// SettingsStore is the subset of the node store the stealth manager needs.
type SettingsStore interface {
	GetSetting(ctx context.Context, key string) (string, error)
	SetSetting(ctx context.Context, key, value string) error
}

// Secrets are the node-wide credentials shared by every client of the stealth
// carriers. Per-client authentication still happens inside WireGuard, so these
// secrets only gate access to the obfuscated transport, not to the VPN itself.
type Secrets struct {
	RealityPrivateKey string // base64 RawURL, x25519
	RealityPublicKey  string // base64 RawURL, x25519
	RealityShortID    string // 8 hex chars
	VLESSUUID         string
	Hysteria2Password string
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
