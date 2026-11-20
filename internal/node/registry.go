package node

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/NetSepio/erebrus/internal/config"
	"github.com/NetSepio/erebrus/internal/registrar"
	"github.com/NetSepio/erebrus/internal/store"
)

const settingRegistryAttestation = "registry_attestation"

// attestation is the persisted outcome of the last sponsored registration, and
// the nonce of one still in flight. The nonce outlives the connection on
// purpose: the gateway sponsors per nonce, so a node that forgets it and asks
// again is asking for a second paid write of the same facts.
type attestation struct {
	Fingerprint string `json:"fingerprint"`
	State       string `json:"state"`
	TxHash      string `json:"tx_hash,omitempty"`
	Nonce       string `json:"nonce,omitempty"`
}

// Attester adapts the signing primitives to the gateway client, adding the
// memory that keeps a reconnecting node from re-registering identical data.
// The gateway caps sponsorships per operator per day, so repeat registrations
// would spend a budget that exists for genuine identity changes.
type Attester struct {
	inner       *registrar.Attester
	st          *store.Store
	fingerprint string

	mu     sync.Mutex
	loaded bool
	saved  attestation
}

// NewAttester returns nil when chain registration is switched off, which keeps
// the node out of the registry: no frames are sent and the gateway simply never
// receives a registration for this peer.
func NewAttester(cfg *config.Config, st *store.Store, peerID, did string) (*Attester, error) {
	if !cfg.ChainRegistrationEnabled() {
		return nil, nil
	}
	id := registrar.NodeIdentity{
		PeerID:  peerID,
		DID:     did,
		IPHash:  registrar.HashIP(cfg.WGEndpointHost),
		Region:  cfg.Region,
		Wallet:  "",
		Version: cfg.Version,
	}
	inner, err := registrar.NewAttester(id, cfg.WalletChain, cfg.Mnemonic)
	if err != nil {
		return nil, err
	}
	return &Attester{inner: inner, st: st, fingerprint: fingerprint(cfg, id, inner.Operator())}, nil
}

// fingerprint covers every field the registry records. The gateway clears a
// node's badge when any of them changes, so a change here must re-register.
func fingerprint(cfg *config.Config, id registrar.NodeIdentity, operator string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{
		cfg.WalletChain, id.PeerID, id.DID, id.IPHash, strings.ToUpper(id.Region), operator,
	}, "\x00")))
	return hex.EncodeToString(sum[:])
}

// Operator returns the wallet this node attests with.
func (a *Attester) Operator() string { return a.inner.Operator() }

// Pending reports whether a registration is still needed for this identity.
func (a *Attester) Pending() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.load().State != "confirmed"
}

// PendingNonce returns the nonce of a registration the gateway has already been
// asked to sponsor and has not settled, so a reconnecting node can ask after
// that job instead of starting another one. Empty when there is nothing in
// flight for this identity.
func (a *Attester) PendingNonce() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	saved := a.load()
	if saved.State == "confirmed" || saved.State == "failed" {
		return ""
	}
	return saved.Nonce
}

// Queue records the nonce a registration went out under. An empty nonce forgets
// it, which is right when the gateway reports no job for it: there is nothing
// to resume and nothing was spent.
func (a *Attester) Queue(nonce string) {
	a.update(func(s *attestation) {
		s.Nonce = nonce
		if nonce != "" {
			s.State = "queued"
		}
	})
}

// load reads the persisted attestation once. Caller holds the lock.
func (a *Attester) load() attestation {
	if !a.loaded {
		a.saved, a.loaded = a.read(), true
	}
	return a.saved
}

func (a *Attester) read() attestation {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	raw, err := a.st.GetSetting(ctx, settingRegistryAttestation)
	if err != nil || raw == "" {
		return attestation{}
	}
	var saved attestation
	if json.Unmarshal([]byte(raw), &saved) != nil || saved.Fingerprint != a.fingerprint {
		// A different identity was registered; this one still needs to, and
		// the other one's nonce is none of its business.
		return attestation{}
	}
	return saved
}

// update mutates the persisted attestation and writes it back. Persisting
// outside the lock keeps a slow store from blocking Pending on the read pump.
func (a *Attester) update(f func(*attestation)) {
	a.mu.Lock()
	next := a.load()
	f(&next)
	next.Fingerprint = a.fingerprint
	a.saved, a.loaded = next, true
	a.mu.Unlock()
	body, err := json.Marshal(next)
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := a.st.SetSetting(ctx, settingRegistryAttestation, string(body)); err != nil {
		slog.Warn("persist registry attestation failed", "err", err)
	}
}

// SignRegistry verifies the challenge describes this node before signing it.
func (a *Attester) SignRegistry(message string) (string, string, error) {
	return a.inner.SignRegistry(message)
}

// Settled persists a terminal result so the node stops asking, and drops the
// nonce it was waiting on.
func (a *Attester) Settled(state, txHash string) {
	a.update(func(s *attestation) { s.State, s.TxHash, s.Nonce = state, txHash, "" })
}

func (a *Attester) SignUptime(bucket int64) (string, error) { return a.inner.SignUptime(bucket) }

func (a *Attester) UptimeBucket(now time.Time) int64 { return registrar.CurrentUptimeBucket(now) }
