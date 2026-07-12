package drop

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/NetSepio/erebrus/internal/p2p"
)

func TestPrepareKuboIdentityFirstRunAndRecovery(t *testing.T) {
	repo := t.TempDir()
	mnemonic := "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"
	if err := PrepareKuboIdentity(repo, mnemonic); err != nil {
		t.Fatal(err)
	}
	identity, err := p2p.DeriveKuboIdentity(mnemonic)
	if err != nil {
		t.Fatal(err)
	}
	peerID, err := os.ReadFile(filepath.Join(repo, identityPeerIDFile))
	if err != nil {
		t.Fatal(err)
	}
	if string(peerID) != identity.PeerID+"\n" {
		t.Fatalf("peer ID handoff = %q", peerID)
	}
	info, err := os.Stat(filepath.Join(repo, identityPrivateKeyFile))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("private key mode = %o", info.Mode().Perm())
	}
	if _, err := os.Stat(filepath.Join(repo, identityReadyFile)); err != nil {
		t.Fatalf("identity ready handoff: %v", err)
	}

	config := map[string]any{"Identity": map[string]string{
		"PeerID": identity.PeerID, "PrivKey": identity.PrivKey,
	}}
	data, _ := json.Marshal(config)
	if err := os.WriteFile(filepath.Join(repo, "config"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := PrepareKuboIdentity(repo, mnemonic); err != nil {
		t.Fatalf("matching persisted identity: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, identityPrivateKeyFile)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("private key handoff should be removed after recovery, stat err=%v", err)
	}
}

func TestPrepareKuboIdentityRejectsConflict(t *testing.T) {
	repo := t.TempDir()
	config := []byte(`{"Identity":{"PeerID":"12D3KooWConflict"}}`)
	if err := os.WriteFile(filepath.Join(repo, "config"), config, 0o600); err != nil {
		t.Fatal(err)
	}
	err := PrepareKuboIdentity(repo, "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about")
	if !errors.Is(err, ErrIdentityConflict) {
		t.Fatalf("error = %v, want identity conflict", err)
	}
	if _, err := os.Stat(filepath.Join(repo, identityReadyFile)); err != nil {
		t.Fatalf("conflict ready handoff: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, identityPrivateKeyFile)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("conflicting repo must not receive private key handoff, stat err=%v", err)
	}
}
