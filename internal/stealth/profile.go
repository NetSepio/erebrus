package stealth

import (
	"fmt"
	"net/url"
)

// ClientPrivateKeyPlaceholder marks where the client substitutes its own
// WireGuard private key in the generated sing-box profile. The node never sees
// client private keys. It avoids angle brackets so it survives JSON HTML-escaping
// unchanged.
const ClientPrivateKeyPlaceholder = "REPLACE_WITH_CLIENT_PRIVATE_KEY"

// Params are the node-wide carrier parameters a client needs to reach the
// stealth transports. They contain no per-client data.
type Params struct {
	Enabled           bool   `json:"enabled"`
	Host              string `json:"host"`
	VLESSPort         int    `json:"vless_port"`
	Hysteria2Port     int    `json:"hysteria2_port"`
	SNI               string `json:"sni"`
	VLESSUUID         string `json:"vless_uuid"`
	VLESSFlow         string `json:"vless_flow"`
	RealityPublicKey  string `json:"reality_public_key"`
	RealityShortID    string `json:"reality_short_id"`
	Hysteria2Password string `json:"hysteria2_password"`
	Hysteria2Obfs     string `json:"hysteria2_obfs,omitempty"` // salamander password, "" = none
}

// Params returns the carrier parameters. Returns Enabled=false (and no secrets)
// when stealth is off or Init has not run.
func (m *Manager) Params() Params {
	if !m.cfg.EnableStealth || m.secrets == nil {
		return Params{Enabled: false}
	}
	return Params{
		Enabled:           true,
		Host:              m.cfg.WGEndpointHost,
		VLESSPort:         m.cfg.VLESSPortInt(),
		Hysteria2Port:     m.cfg.Hysteria2PortInt(),
		SNI:               m.cfg.RealitySNI(),
		VLESSUUID:         m.secrets.VLESSUUID,
		VLESSFlow:         vlessFlowVision,
		RealityPublicKey:  m.secrets.RealityPublicKey,
		RealityShortID:    m.secrets.RealityShortID,
		Hysteria2Password: m.secrets.Hysteria2Password,
		Hysteria2Obfs:     m.cfg.Hysteria2ObfsPassword,
	}
}

// PeerStealth is the per-client stealth section of a credential bundle.
type PeerStealth struct {
	VLESSURI       string `json:"vless_uri"`
	Hysteria2URI   string `json:"hysteria2_uri"`
	SingboxProfile any    `json:"singbox_profile"`
}

// BuildPeer renders the per-client stealth artifacts: standard vless:// and
// hysteria2:// carrier share links plus a complete sing-box client profile that
// tunnels WireGuard through the VLESS+REALITY carrier (Topology A — WireGuard
// is the endpoint). clientAddrCIDR is the peer's tunnel address (e.g.
// "10.0.0.7/32"); serverWGPub is the node's WireGuard public key (base64); psk
// is the optional WireGuard preshared key.
func (m *Manager) BuildPeer(label, serverWGPub, clientAddrCIDR, psk string) PeerStealth {
	p := m.Params()
	return PeerStealth{
		VLESSURI:       p.vlessURI(label),
		Hysteria2URI:   p.hysteria2URI(label),
		SingboxProfile: m.singboxProfile(p, serverWGPub, clientAddrCIDR, psk),
	}
}

func (p Params) vlessURI(label string) string {
	q := url.Values{}
	q.Set("encryption", "none")
	q.Set("flow", p.VLESSFlow)
	q.Set("security", "reality")
	q.Set("sni", p.SNI)
	q.Set("fp", "chrome")
	q.Set("pbk", p.RealityPublicKey)
	q.Set("sid", p.RealityShortID)
	q.Set("type", "tcp")
	return fmt.Sprintf("vless://%s@%s:%d?%s#%s",
		p.VLESSUUID, p.Host, p.VLESSPort, q.Encode(), url.PathEscape(label))
}

func (p Params) hysteria2URI(label string) string {
	q := url.Values{}
	q.Set("sni", p.SNI)
	q.Set("insecure", "1")
	q.Set("alpn", "h3")
	if p.Hysteria2Obfs != "" {
		q.Set("obfs", "salamander")
		q.Set("obfs-password", p.Hysteria2Obfs)
	}
	return fmt.Sprintf("hysteria2://%s@%s:%d?%s#%s",
		url.QueryEscape(p.Hysteria2Password), p.Host, p.Hysteria2Port, q.Encode(), url.PathEscape(label))
}

// singboxProfile builds a full client config (as a JSON-serializable map) that
// runs WireGuard over the VLESS+REALITY carrier. The Hysteria2 carrier is also
// included as an outbound; a client switches by repointing the WireGuard
// endpoint's "detour" to "carrier-hysteria2". The WG peer endpoint is the node
// loopback because the node's direct outbound delivers carrier traffic straight
// to its local WireGuard listener.
func (m *Manager) singboxProfile(p Params, serverWGPub, clientAddrCIDR, psk string) map[string]any {
	wgPeer := map[string]any{
		"address":                       "127.0.0.1",
		"port":                          m.cfg.WGEndpointPortInt(),
		"public_key":                    serverWGPub,
		"allowed_ips":                   []string{"0.0.0.0/0", "::/0"},
		"persistent_keepalive_interval": 25,
	}
	if psk != "" {
		wgPeer["pre_shared_key"] = psk
	}

	vlessTLS := map[string]any{
		"enabled":     true,
		"server_name": p.SNI,
		"utls":        map[string]any{"enabled": true, "fingerprint": "chrome"},
		"reality": map[string]any{
			"enabled":    true,
			"public_key": p.RealityPublicKey,
			"short_id":   p.RealityShortID,
		},
	}
	hy2TLS := map[string]any{
		"enabled":     true,
		"server_name": p.SNI,
		"insecure":    true,
		"alpn":        []string{"h3"},
	}
	hy2Out := map[string]any{
		"type":        "hysteria2",
		"tag":         "carrier-hysteria2",
		"server":      p.Host,
		"server_port": p.Hysteria2Port,
		"password":    p.Hysteria2Password,
		"tls":         hy2TLS,
	}
	if p.Hysteria2Obfs != "" {
		hy2Out["obfs"] = map[string]any{"type": "salamander", "password": p.Hysteria2Obfs}
	}

	return map[string]any{
		"log": map[string]any{"level": "warn"},
		"endpoints": []map[string]any{{
			"type":        "wireguard",
			"tag":         "wg-out",
			"address":     []string{clientAddrCIDR},
			"private_key": ClientPrivateKeyPlaceholder,
			"peers":       []map[string]any{wgPeer},
			"detour":      "carrier-vless",
		}},
		"outbounds": []map[string]any{
			{
				"type":        "vless",
				"tag":         "carrier-vless",
				"server":      p.Host,
				"server_port": p.VLESSPort,
				"uuid":        p.VLESSUUID,
				"flow":        p.VLESSFlow,
				"tls":         vlessTLS,
			},
			hy2Out,
		},
		"route": map[string]any{"final": "wg-out"},
	}
}
