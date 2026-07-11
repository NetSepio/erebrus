package drop

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/NetSepio/erebrus/internal/p2p"
)

const (
	DefaultKuboRepoPath    = "/var/lib/erebrus-kubo"
	identityPeerIDFile     = ".erebrus-peer-id"
	identityPrivateKeyFile = ".erebrus-private-key"
	identityConflictMarker = ".erebrus-identity-conflict"
)

var ErrIdentityConflict = errors.New("existing Kubo identity conflicts with the mnemonic-derived Drop identity")

// PrepareKuboIdentity writes a private initialization handoff for the Kubo
// sidecar and refuses to replace an existing, different identity.
func PrepareKuboIdentity(repoPath, mnemonic string) error {
	identity, err := p2p.DeriveKuboIdentity(mnemonic)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(repoPath, 0o700); err != nil {
		return fmt.Errorf("create Kubo repo path: %w", err)
	}
	if _, err := os.Stat(filepath.Join(repoPath, identityConflictMarker)); err == nil {
		return ErrIdentityConflict
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("check Kubo identity conflict: %w", err)
	}
	if err := atomicWrite(filepath.Join(repoPath, identityPeerIDFile), identity.PeerID+"\n", 0o600); err != nil {
		return err
	}

	configPath := filepath.Join(repoPath, "config")
	data, err := os.ReadFile(configPath)
	if errors.Is(err, os.ErrNotExist) {
		return atomicWrite(filepath.Join(repoPath, identityPrivateKeyFile), identity.PrivKey+"\n", 0o600)
	}
	if err != nil {
		return fmt.Errorf("read Kubo config: %w", err)
	}
	var config struct {
		Identity struct {
			PeerID string `json:"PeerID"`
		} `json:"Identity"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("parse Kubo config: %w", err)
	}
	if config.Identity.PeerID != "" && config.Identity.PeerID != identity.PeerID {
		return ErrIdentityConflict
	}
	if err := os.Remove(filepath.Join(repoPath, identityPrivateKeyFile)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove stale identity handoff: %w", err)
	}
	return nil
}

func atomicWrite(path, value string, mode os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".erebrus-identity-*")
	if err != nil {
		return fmt.Errorf("create identity handoff: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return fmt.Errorf("secure identity handoff: %w", err)
	}
	if _, err := tmp.WriteString(strings.TrimSpace(value) + "\n"); err != nil {
		tmp.Close()
		return fmt.Errorf("write identity handoff: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close identity handoff: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("install identity handoff: %w", err)
	}
	return nil
}
