package p2p

import (
	"encoding/base64"
	"testing"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

const identityTestMnemonic = "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"

func TestStableIdentityDerivations(t *testing.T) {
	erebrusPeerID, _, err := PeerIDFromMnemonic(identityTestMnemonic)
	if err != nil {
		t.Fatalf("derive Erebrus identity: %v", err)
	}
	kubo, err := DeriveKuboIdentity(identityTestMnemonic)
	if err != nil {
		t.Fatalf("derive Kubo identity: %v", err)
	}

	const wantErebrusPeerID = "12D3KooWHXVETqmop8y1iD6XHZujC64269bg1qmwkpDaygtYZMH8"
	const wantKuboPeerID = "12D3KooWRsWLnKUXEbV7yqXDUDu9cBdYFGktUrwJXPW1z9T1eSqR"
	if erebrusPeerID != wantErebrusPeerID {
		t.Fatalf("Erebrus PeerID = %q, want %q", erebrusPeerID, wantErebrusPeerID)
	}
	if kubo.PeerID != wantKuboPeerID {
		t.Fatalf("Kubo PeerID = %q, want %q", kubo.PeerID, wantKuboPeerID)
	}
	repeated, err := DeriveKuboIdentity(identityTestMnemonic)
	if err != nil {
		t.Fatalf("repeat Kubo identity derivation: %v", err)
	}
	if repeated.PeerID != kubo.PeerID || repeated.PrivKey != kubo.PrivKey {
		t.Fatal("Kubo identity derivation must be deterministic")
	}
	if erebrusPeerID == kubo.PeerID {
		t.Fatal("Erebrus and Kubo PeerIDs must differ")
	}
}

func TestKuboIdentityUsesLibp2pPrivateKeyEncoding(t *testing.T) {
	kubo, err := DeriveKuboIdentity(identityTestMnemonic)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.StdEncoding.DecodeString(kubo.PrivKey)
	if err != nil {
		t.Fatalf("decode private key: %v", err)
	}
	priv, err := crypto.UnmarshalPrivateKey(raw)
	if err != nil {
		t.Fatalf("unmarshal private key: %v", err)
	}
	id, err := peer.IDFromPrivateKey(priv)
	if err != nil {
		t.Fatalf("derive PeerID: %v", err)
	}
	if id.String() != kubo.PeerID {
		t.Fatalf("serialized key PeerID = %q, want %q", id, kubo.PeerID)
	}
}

func TestDeriveKuboIdentityRejectsEmptyMnemonic(t *testing.T) {
	if _, err := DeriveKuboIdentity(""); err == nil {
		t.Fatal("expected empty mnemonic error")
	}
}
