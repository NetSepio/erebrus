package api

// CredentialBundle is the unified response for peer provisioning and re-fetch.
// In Phase 1 only the WireGuard section is populated; Phase 2 fills the VLESS,
// Hysteria2 and sing-box profile fields.
type CredentialBundle struct {
	ID             string          `json:"id"`
	WireGuard      WireGuardBundle `json:"wireguard"`
	VLESSURI       string          `json:"vless_uri,omitempty"`
	Hysteria2URI   string          `json:"hysteria2_uri,omitempty"`
	SingboxProfile any             `json:"singbox_profile,omitempty"`
}

// WireGuardBundle holds everything a client needs for the WireGuard fast path.
type WireGuardBundle struct {
	ClientConf      string `json:"client_conf"`
	ServerPublicKey string `json:"server_public_key"`
	Endpoint        string `json:"endpoint"`
	Address         string `json:"address"`
	DNS             string `json:"dns"`
}

// PeerRequest is the body of PUT /api/v2/peers/{id}.
type PeerRequest struct {
	Name           string `json:"name"`
	Wallet         string `json:"wallet"`
	WGPublicKey    string `json:"wg_public_key"`
	WGPresharedKey string `json:"wg_preshared_key"`
	ExpiresAt      int64  `json:"expires_at"`
}

// PeerInfo is the metadata-only listing item (no credentials).
type PeerInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	WGAllowedIP string `json:"wg_allowed_ip"`
	Enabled     bool   `json:"enabled"`
	CreatedAt   int64  `json:"created_at"`
	ExpiresAt   int64  `json:"expires_at"`
}

// StatusResponse is the public node status.
type StatusResponse struct {
	Version      string         `json:"version"`
	Region       string         `json:"region"`
	Status       string         `json:"status"`
	PeerID       string         `json:"peer_id"`
	DID          string         `json:"did"`
	Capabilities map[string]any `json:"capabilities"`
	Protocols    []string       `json:"protocols"`
}
