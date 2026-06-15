// Package config centralizes all environment-derived configuration for the
// Erebrus v2 node. It replaces the scattered os.Getenv calls of v1.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds the full node configuration.
type Config struct {
	// app
	RunType  string // debug | release
	BindAddr string // SERVER / API_BIND_ADDR
	HTTPPort string
	NodeName string
	Region   string
	Version  string

	// runtime model (v2.1+)
	Mode            ModeSettings
	UnsafePublicAPI bool
	PublicDomain    string
	WildcardDomain  string

	// identity
	Mnemonic string

	// gateway
	GatewayURL           string
	GatewayPeerMultiaddr string
	P2PListenPort        string
	NodeID               string // gateway-assigned; persisted in SQLite when registered
	NodeToken            string // gateway-issued PASETO for WS control plane
	WalletChain          string // sol | evm — signs gateway registration challenge
	AuthEULA             string // must match gateway AUTH_EULA for registration
	APIPublicURL         string // URL gateway uses for peer provisioning (api_base_url)
	GatewayAutoRegister  bool

	// auth (Phase 1: static bearer token for node API; Phase 2 swaps to
	// gateway-issued PASETO verification)
	NodeAPIToken string

	// wireguard
	WGConfDir      string
	WGInterface    string // e.g. "wg0"
	WGEndpointHost string
	WGEndpointPort string // WG_PORT alias
	StealthTCPPort string // STEALTH_TCP_PORT — VLESS+REALITY
	StealthUDPPort string // STEALTH_UDP_PORT — Hysteria2/QUIC
	WGIPv4Subnet   string // e.g. "10.0.0.1/16"
	WGDNS          string
	WGPostUp       string
	WGPostDown     string
	WGPreUp        string
	WGPreDown      string

	// stealth protocols — sing-box carriers for when WireGuard's UDP is
	// throttled or DPI-blocked. VLESS+REALITY presents as ordinary TLS to a
	// borrowed SNI; Hysteria2 presents as QUIC/HTTP3. Both wrap the same
	// WireGuard tunnel (WG stays the endpoint).
	EnableStealth          bool
	VLESSPort              string
	Hysteria2Port          string
	RealityServerNames     []string // SNIs the REALITY handshake borrows; first is the dial target
	RealityHandshakeServer string   // host:port the node proxies the real TLS handshake to
	Hysteria2ObfsPassword  string
	EnableTUIC             bool

	// node-local state
	StateDir string

	// app hosting (Phase 5)
	EnableAppHosting  bool
	AppWildcardDomain string

	// registrar
	ChainRegistration string // off | solana

	// private DNS (Phase 2)
	PrivateDNSEnabled bool
	PrivateDNSDomain  string
	PrivateDNSAddr    string
	UpstreamDNS       string
	DNSQueryLogs      bool
}

// Load reads configuration from the environment, applying sane defaults.
func Load() *Config {
	bindAddr := env("API_BIND_ADDR", "")
	if bindAddr == "" {
		bindAddr = env("SERVER", "0.0.0.0")
	}
	c := &Config{
		RunType:                env("RUNTYPE", "release"),
		BindAddr:               bindAddr,
		HTTPPort:               env("HTTP_PORT", "9080"),
		UnsafePublicAPI:        boolEnv("UNSAFE_PUBLIC_API", false),
		PublicDomain:           os.Getenv("PUBLIC_DOMAIN"),
		WildcardDomain:         env("WILDCARD_DOMAIN", os.Getenv("APP_WILDCARD_DOMAIN")),
		NodeName:               env("NODE_NAME", hostnameOr("erebrus-node")),
		Region:                 env("REGION", "unknown"),
		Version:                Version,
		Mnemonic:               os.Getenv("MNEMONIC"),
		GatewayURL:             env("GATEWAY_URL", ""),
		GatewayPeerMultiaddr:   env("GATEWAY_PEER_MULTIADDR", ""),
		P2PListenPort:          env("P2P_LISTEN_PORT", "9002"),
		NodeID:                 os.Getenv("NODE_ID"),
		NodeToken:              os.Getenv("NODE_TOKEN"),
		WalletChain:            env("WALLET_CHAIN", "sol"),
		AuthEULA:               env("AUTH_EULA", "I accept the Erebrus Terms of Service https://erebrus.network/terms."),
		APIPublicURL:           os.Getenv("API_PUBLIC_URL"),
		GatewayAutoRegister:    boolEnv("GATEWAY_AUTO_REGISTER", true),
		NodeAPIToken:           os.Getenv("NODE_API_TOKEN"),
		WGConfDir:              env("WG_CONF_DIR", "/etc/wireguard"),
		WGInterface:            normalizeInterface(env("WG_INTERFACE_NAME", "wg0")),
		WGEndpointHost:         os.Getenv("WG_ENDPOINT_HOST"),
		WGEndpointPort:         firstEnv("WG_PORT", "WG_ENDPOINT_PORT", "51820"),
		StealthTCPPort:         firstEnv("STEALTH_TCP_PORT", "VLESS_PORT", "8443"),
		StealthUDPPort:         firstEnv("STEALTH_UDP_PORT", "HYSTERIA2_PORT", "4443"),
		WGIPv4Subnet:           env("WG_IPv4_SUBNET", "10.0.0.1/16"),
		WGDNS:                  env("WG_DNS", "1.1.1.1"),
		WGPostUp:               os.Getenv("WG_POST_UP"),
		WGPostDown:             os.Getenv("WG_POST_DOWN"),
		WGPreUp:                os.Getenv("WG_PRE_UP"),
		WGPreDown:              os.Getenv("WG_PRE_DOWN"),
		EnableStealth:          boolEnv("ENABLE_STEALTH", true),
		VLESSPort:              "", // synced from StealthTCPPort below
		Hysteria2Port:          "", // synced from StealthUDPPort below
		RealityServerNames:     splitCSV(env("REALITY_SERVER_NAMES", "www.microsoft.com")),
		RealityHandshakeServer: env("REALITY_HANDSHAKE_SERVER", ""),
		Hysteria2ObfsPassword:  os.Getenv("HYSTERIA2_OBFS_PASSWORD"),
		EnableTUIC:             boolEnv("ENABLE_TUIC", false),
		StateDir:               env("STATE_DIR", "/var/lib/erebrus"),
		EnableAppHosting:       boolEnv("ENABLE_APP_HOSTING", false),
		AppWildcardDomain:      os.Getenv("APP_WILDCARD_DOMAIN"),
		ChainRegistration:      env("CHAIN_REGISTRATION", "off"),
		PrivateDNSEnabled:      boolEnv("PRIVATE_DNS_ENABLED", false),
		PrivateDNSDomain:       env("PRIVATE_DNS_DOMAIN", "ere"),
		PrivateDNSAddr:         os.Getenv("PRIVATE_DNS_ADDR"),
		UpstreamDNS:            env("UPSTREAM_DNS", "1.1.1.1"),
		DNSQueryLogs:           boolEnv("DNS_QUERY_LOGS", false),
	}
	if mode, err := ParseModeSettings(os.Getenv("EREBRUS_MODE"), os.Getenv("EREBRUS_NETWORK_PROFILE")); err == nil {
		c.Mode = mode
	}
	c.VLESSPort = c.StealthTCPPort
	c.Hysteria2Port = c.StealthUDPPort
	if c.BindAddr == "0.0.0.0" && c.UnsafePublicAPI {
		c.Mode.Warnings = append(c.Mode.Warnings,
			"WARNING: Erebrus management API is publicly exposed. Use this only behind TLS, firewall, or a trusted gateway.")
	}
	return c
}

// Validate returns an error if required fields are missing or invalid.
func (c *Config) Validate() error {
	mode, err := ParseModeSettings(os.Getenv("EREBRUS_MODE"), os.Getenv("EREBRUS_NETWORK_PROFILE"))
	if err != nil {
		return err
	}
	c.Mode = mode
	var missing []string
	if c.Mnemonic == "" {
		missing = append(missing, "MNEMONIC")
	}
	if c.WGEndpointHost == "" {
		missing = append(missing, "WG_ENDPOINT_HOST")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required config: %s", strings.Join(missing, ", "))
	}
	if c.Mode.IsGateway() && c.PublicDomain == "" && c.EnableAppHosting {
		c.Mode.Warnings = append(c.Mode.Warnings,
			"WARNING: Gateway Mode with ENABLE_APP_HOSTING but no PUBLIC_DOMAIN set; public edge routing may be incomplete.")
	}
	if c.Mode.IsGateway() && (c.StealthTCPPort != "443" || c.StealthUDPPort != "443") {
		c.Mode.Warnings = append(c.Mode.Warnings,
			"WARNING: Gateway Mode production should expose stealth on 443/tcp and 443/udp (STEALTH_TCP_PORT/STEALTH_UDP_PORT) for best reachability.")
	}
	return nil
}

// DBPath is the SQLite file path.
func (c *Config) DBPath() string { return c.StateDir + "/erebrus.db" }

// PublicAPIBaseURL returns the URL the gateway should use for peer provisioning.
func (c *Config) PublicAPIBaseURL() string {
	if c.APIPublicURL != "" {
		return strings.TrimRight(c.APIPublicURL, "/")
	}
	host := c.WGEndpointHost
	if host == "" {
		host = "127.0.0.1"
	}
	return fmt.Sprintf("http://%s:%s", host, c.HTTPPort)
}

// GatewayEnabled reports whether the node should connect to the gateway control plane.
func (c *Config) GatewayEnabled() bool { return strings.TrimSpace(c.GatewayURL) != "" }

// WGEndpointPortInt parses the endpoint port.
func (c *Config) WGEndpointPortInt() int {
	n, _ := strconv.Atoi(c.WGEndpointPort)
	return n
}

// VLESSPortInt parses the VLESS+REALITY listen port.
func (c *Config) VLESSPortInt() int { n, _ := strconv.Atoi(c.VLESSPort); return n }

// Hysteria2PortInt parses the Hysteria2 listen port.
func (c *Config) Hysteria2PortInt() int { n, _ := strconv.Atoi(c.Hysteria2Port); return n }

// RealitySNI returns the primary SNI the REALITY handshake borrows.
func (c *Config) RealitySNI() string {
	if len(c.RealityServerNames) > 0 {
		return c.RealityServerNames[0]
	}
	return "www.microsoft.com"
}

// RealityHandshakeTarget returns host:port the node proxies the real TLS
// handshake to. Defaults to the primary SNI on :443.
func (c *Config) RealityHandshakeTarget() string {
	if c.RealityHandshakeServer != "" {
		return c.RealityHandshakeServer
	}
	return c.RealitySNI() + ":443"
}

func firstEnv(keys ...string) string {
	if len(keys) == 0 {
		return ""
	}
	def := keys[len(keys)-1]
	keys = keys[:len(keys)-1]
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return def
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func boolEnv(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// normalizeInterface accepts "wg0" or "wg0.conf" and returns "wg0".
func normalizeInterface(s string) string {
	return strings.TrimSuffix(s, ".conf")
}

func hostnameOr(def string) string {
	if h, err := os.Hostname(); err == nil && h != "" {
		return h
	}
	return def
}
