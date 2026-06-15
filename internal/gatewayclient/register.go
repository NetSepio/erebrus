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
	settingNodeID    = "gateway_node_id"
	settingNodeToken = "gateway_node_token"
)

// SettingsStore persists gateway registration credentials.
type SettingsStore interface {
	GetSetting(ctx context.Context, key string) (string, error)
	SetSetting(ctx context.Context, key, value string) error
}

// RegistrationInput is the node identity payload sent to the gateway.
type RegistrationInput struct {
	GatewayURL  string
	AuthEULA    string
	WalletChain string
	Mnemonic    string
	PeerID      string
	DID         string
	Name        string
	Region      string
	APIBaseURL  string
	APIToken    string
}

// RegistrationResult holds the gateway-issued node credentials.
type RegistrationResult struct {
	NodeID    string
	NodeToken string
}

// LoadCredentials reads persisted gateway credentials from the store.
func LoadCredentials(ctx context.Context, st SettingsStore) (nodeID, nodeToken string, err error) {
	nodeID, err = st.GetSetting(ctx, settingNodeID)
	if err != nil {
		return "", "", err
	}
	nodeToken, err = st.GetSetting(ctx, settingNodeToken)
	if err != nil {
		return "", "", err
	}
	return nodeID, nodeToken, nil
}

// SaveCredentials persists gateway credentials.
func SaveCredentials(ctx context.Context, st SettingsStore, nodeID, nodeToken string) error {
	if err := st.SetSetting(ctx, settingNodeID, nodeID); err != nil {
		return err
	}
	return st.SetSetting(ctx, settingNodeToken, nodeToken)
}

// Register performs the two-step gateway node registration flow and returns a
// node-scoped PASETO for the WebSocket control plane.
func Register(ctx context.Context, in RegistrationInput) (*RegistrationResult, error) {
	base := strings.TrimRight(strings.TrimSpace(in.GatewayURL), "/")
	if base == "" {
		return nil, fmt.Errorf("gateway URL is empty")
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

	// Step 1: challenge
	step1, _ := json.Marshal(map[string]string{
		"wallet_address": walletAddr,
		"chain":          normalizeChain(in.WalletChain),
	})
	raw, status, err := postJSON(ctx, client, base+"/api/v2/nodes/register", step1)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("register step1: %d: %s", status, truncate(raw))
	}
	var challenge struct {
		FlowID  string `json:"flow_id"`
		Message string `json:"message"`
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

	// Step 2: signed registration
	step2, _ := json.Marshal(map[string]string{
		"flow_id":        challenge.FlowID,
		"signature":      signature,
		"public_key":     pubKey,
		"peer_id":        in.PeerID,
		"did":            in.DID,
		"name":           in.Name,
		"region":         in.Region,
		"api_base_url":   in.APIBaseURL,
		"api_token":    in.APIToken,
	})
	raw, status, err = postJSON(ctx, client, base+"/api/v2/nodes/register", step2)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("register step2: %d: %s", status, truncate(raw))
	}
	var out struct {
		NodeID    string `json:"node_id"`
		NodeToken string `json:"node_token"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("parse registration response: %w", err)
	}
	if out.NodeID == "" || out.NodeToken == "" {
		return nil, fmt.Errorf("gateway returned incomplete registration response")
	}
	return &RegistrationResult{NodeID: out.NodeID, NodeToken: out.NodeToken}, nil
}

func normalizeChain(chain string) string {
	chain = strings.ToLower(strings.TrimSpace(chain))
	if chain == "" {
		return wallet.ChainSOL
	}
	return chain
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