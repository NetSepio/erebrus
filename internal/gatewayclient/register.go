package gatewayclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/NetSepio/erebrus/internal/wallet"
)

const (
	settingNodeID           = "gateway_node_id"
	settingNodeToken        = "gateway_node_token"
	settingNodeKey          = "gateway_node_key"
	settingGatewayPublicKey = "gateway_public_key"
)

// SettingsStore persists gateway registration credentials.
type SettingsStore interface {
	GetSetting(ctx context.Context, key string) (string, error)
	SetSetting(ctx context.Context, key, value string) error
}

// RegistrationInput is the node identity payload sent to the gateway.
type RegistrationInput struct {
	GatewayURL         string
	OrgEnrollmentSecret string
	WalletChain        string
	Mnemonic           string
	PeerID             string
	DID                string
	Name               string
	Region             string
	Zone               string
	APIBaseURL         string
	NodeKey            string // optional; gateway mints if empty
	AccessMode         string // public | private
}

// RegistrationResult holds the gateway-issued node credentials.
type RegistrationResult struct {
	NodeID            string
	NodeToken         string
	NodeKey           string
	GatewayPublicKey  string
}

// Credentials is the persisted gateway registration state.
type Credentials struct {
	NodeID           string
	NodeToken        string
	NodeKey          string
	GatewayPublicKey string
}

// LoadCredentials reads persisted gateway credentials from the store.
func LoadCredentials(ctx context.Context, st SettingsStore) (*Credentials, error) {
	nodeID, err := st.GetSetting(ctx, settingNodeID)
	if err != nil {
		return nil, err
	}
	nodeToken, err := st.GetSetting(ctx, settingNodeToken)
	if err != nil {
		return nil, err
	}
	nodeKey, _ := st.GetSetting(ctx, settingNodeKey)
	gwPub, _ := st.GetSetting(ctx, settingGatewayPublicKey)
	return &Credentials{
		NodeID: nodeID, NodeToken: nodeToken, NodeKey: nodeKey, GatewayPublicKey: gwPub,
	}, nil
}

// SaveCredentials persists gateway credentials.
func SaveCredentials(ctx context.Context, st SettingsStore, cred *Credentials) error {
	if cred == nil {
		return fmt.Errorf("nil credentials")
	}
	if err := st.SetSetting(ctx, settingNodeID, cred.NodeID); err != nil {
		return err
	}
	if err := st.SetSetting(ctx, settingNodeToken, cred.NodeToken); err != nil {
		return err
	}
	if cred.NodeKey != "" {
		if err := st.SetSetting(ctx, settingNodeKey, cred.NodeKey); err != nil {
			return err
		}
	}
	if cred.GatewayPublicKey != "" {
		if err := st.SetSetting(ctx, settingGatewayPublicKey, cred.GatewayPublicKey); err != nil {
			return err
		}
	}
	return nil
}

// Register performs the two-step org enrollment flow and returns node credentials.
func Register(ctx context.Context, in RegistrationInput) (*RegistrationResult, error) {
	base := strings.TrimRight(strings.TrimSpace(in.GatewayURL), "/")
	secret := strings.TrimSpace(in.OrgEnrollmentSecret)
	if base == "" {
		return nil, fmt.Errorf("gateway URL is empty")
	}
	if secret == "" {
		return nil, fmt.Errorf("org enrollment secret is empty")
	}
	if in.PeerID == "" {
		return nil, fmt.Errorf("peer_id is empty")
	}

	walletAddr, err := wallet.AddressFromMnemonic(in.Mnemonic, in.WalletChain)
	if err != nil {
		return nil, fmt.Errorf("wallet address: %w", err)
	}
	pubKey, err := wallet.PublicKeyFromMnemonic(in.Mnemonic, in.WalletChain)
	if err != nil {
		return nil, fmt.Errorf("wallet public key: %w", err)
	}

	client := &http.Client{Timeout: 15 * time.Second}

	// Step 1: machine challenge (gated by enrollment_secret).
	step1, _ := json.Marshal(map[string]string{
		"enrollment_secret": secret,
		"peer_id":           in.PeerID,
	})
	raw, status, err := postJSON(ctx, client, base+"/api/v2/nodes/register", step1)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("register step1: %d: %s", status, truncate(raw))
	}
	var challenge struct {
		FlowID           string `json:"flow_id"`
		Message          string `json:"message"`
		GatewayPublicKey string `json:"gateway_public_key"`
	}
	if err := json.Unmarshal(raw, &challenge); err != nil {
		return nil, fmt.Errorf("parse challenge: %w", err)
	}
	if challenge.FlowID == "" || challenge.Message == "" {
		return nil, fmt.Errorf("gateway returned empty challenge")
	}

	_, _, signature, err := wallet.SignChallengeWithMnemonic(in.Mnemonic, in.WalletChain, challenge.Message)
	if err != nil {
		return nil, fmt.Errorf("sign challenge: %w", err)
	}

	access := in.AccessMode
	if access != "private" {
		access = "public"
	}

	// Step 2: signed machine registration.
	step2, _ := json.Marshal(map[string]string{
		"flow_id":            challenge.FlowID,
		"enrollment_secret":  secret,
		"signature":          signature,
		"public_key":         pubKey,
		"wallet_address":     walletAddr,
		"chain":              wallet.CanonicalChain(in.WalletChain),
		"peer_id":            in.PeerID,
		"did":                in.DID,
		"name":               in.Name,
		"region":             in.Region,
		"zone":               in.Zone,
		"api_base_url":       in.APIBaseURL,
		"node_key":           in.NodeKey,
		"access_mode":        access,
	})
	raw, status, err = postJSON(ctx, client, base+"/api/v2/nodes/register", step2)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("register step2: %d: %s", status, truncate(raw))
	}
	var out struct {
		NodeID           string `json:"node_id"`
		NodeToken        string `json:"node_token"`
		NodeKey          string `json:"node_key"`
		GatewayPublicKey string `json:"gateway_public_key"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("parse registration response: %w", err)
	}
	if out.NodeID == "" || out.NodeToken == "" || out.NodeKey == "" {
		return nil, fmt.Errorf("gateway returned incomplete registration response")
	}
	gwPub := out.GatewayPublicKey
	if gwPub == "" {
		gwPub = challenge.GatewayPublicKey
	}
	return &RegistrationResult{
		NodeID: out.NodeID, NodeToken: out.NodeToken, NodeKey: out.NodeKey, GatewayPublicKey: gwPub,
	}, nil
}

func postJSON(ctx context.Context, client *http.Client, url string, body []byte) (json.RawMessage, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return raw, resp.StatusCode, nil
}

func truncate(b []byte) string {
	if len(b) > 200 {
		return string(b[:200])
	}
	return string(b)
}