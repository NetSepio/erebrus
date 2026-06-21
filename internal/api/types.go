package api

const BundleVersion = 2

// TransportEntry describes one carrier in a v2 credential bundle.
type TransportEntry struct {
	Kind string `json:"kind"`
	URI  string `json:"uri,omitempty"`
}

// CredentialBundle is the unified response for peer provisioning and re-fetch.
type CredentialBundle struct {
	BundleVersion    int              `json:"bundle_version"`
	NodeID           string           `json:"node_id,omitempty"`
	ID               string           `json:"id"`
	IssuedAt         int64            `json:"issued_at"`
	ExpiresAt        int64            `json:"expires_at,omitempty"`
	WireGuard        WireGuardBundle  `json:"wireguard"`
	Transports       []TransportEntry `json:"transports,omitempty"`
	VLESSURI         string           `json:"vless_uri,omitempty"`
	Hysteria2URI     string           `json:"hysteria2_uri,omitempty"`
	SingboxProfile   any              `json:"singbox_profile,omitempty"`
	ServiceDiscovery map[string]any   `json:"service_discovery,omitempty"`
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

// NodeStats is the coarse, public operational snapshot powering the local
// dashboard. It deliberately exposes only aggregates — never per-client data.
type NodeStats struct {
	Status         string   `json:"status"`
	Version        string   `json:"version"`
	Region         string   `json:"region"`
	Protocols      []string `json:"protocols"`
	TotalPeers     int      `json:"total_peers"`     // provisioned in the store
	ConnectedPeers int      `json:"connected_peers"` // handshake in the last 3m
	RxBytes        int64    `json:"rx_bytes"`        // cumulative since interface up
	TxBytes        int64    `json:"tx_bytes"`
	UptimeSec      int64    `json:"uptime_sec"`
}

// WireGuardEndpointStatus is the node's WireGuard listen endpoint (server key + port).
type WireGuardEndpointStatus struct {
	Port      int    `json:"port"`
	PublicKey string `json:"public_key"`
	Endpoint  string `json:"endpoint"` // host:port clients dial
}

// EndpointsStatus mirrors the gateway discovery projection for this node.
type EndpointsStatus struct {
	WireGuard WireGuardEndpointStatus `json:"wireguard"`
}

// IdentityStatus summarizes the node's cryptographic identity (never includes secrets).
type IdentityStatus struct {
	Configured    bool   `json:"configured"`
	PeerID        string `json:"peer_id"`
	DID           string `json:"did"`
	WalletChain   string `json:"wallet_chain,omitempty"`
	WalletLabel   string `json:"wallet_chain_label,omitempty"`
	WalletAddress string `json:"wallet_address,omitempty"`
}

// StatusResponse is the public node status.
type StatusResponse struct {
	Version      string         `json:"version"`
	NodeName     string         `json:"node_name"`
	Region       string         `json:"region"`
	Status       string         `json:"status"`
	AccessMode   string         `json:"access_mode"`
	PeerID       string         `json:"peer_id"` // deprecated: use identity.peer_id
	DID          string         `json:"did"`     // deprecated: use identity.did
	Identity     IdentityStatus  `json:"identity"`
	Endpoints    EndpointsStatus `json:"endpoints"`
	Capabilities map[string]any  `json:"capabilities"`
	Protocols    []string       `json:"protocols"`
	Readiness    any            `json:"readiness"`
}
