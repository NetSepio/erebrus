// Package config centralizes all environment-derived configuration for the
// Erebrus v2 node. It replaces the scattered os.Getenv calls of v1.
package config

import (
	"fmt"
	"net"
	"os"
	"regexp"
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
	Zone     string // optional placement hint, e.g. east, west, us-east
	Version  string

	// runtime model (v2.1+)
	Mode                 ModeSettings
	UnsafePublicAPI      bool
	PublicDomain         string
	WildcardDomain       string
	PublicGatewayEnabled bool
	PublicHTTPPort       string
	PublicHTTPSPort      string
	AutoTLS              bool

	// identity
	Mnemonic string

	// gateway
	GatewayURL            string
	GatewayPeerMultiaddr  string
	P2PListenPort         string
	NodeID                string // canonical peer_id; persisted in SQLite when registered
	NodeToken             string // gateway-issued PASETO for WS control plane
	WalletChain           string // SOLANA | ETHEREUM (aliases sol/evm accepted) — gateway enrollment
	NodeRegistrationToken string // EREBRUS_NODE_REGISTRATION_TOKEN — scoped org registration token
	APIPublicURL          string // URL gateway uses for peer provisioning (api_base_url)
	GatewayAutoRegister   bool
	GatewayPublicKey      string // gateway Ed25519 public key (hex) for verifying API calls

	// NodeKey is the per-node bearer (NODE_KEY). NODE_API_TOKEN is a legacy alias.
	NodeKey      string
	NodeAPIToken string // deprecated alias for NodeKey

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

	// Drop storage
	DropEnabled             bool
	DropStorageMax          string
	DropStorageMaxBytes     int64
	DropSwarmPort           string
	DropWebUIEnabled        bool
	DropPublicGatewayDomain string

	// registrar
	ChainRegistration string // off | solana

	// private DNS (Phase 2)
	PrivateDNSEnabled bool
	PrivateDNSDomain  string
	PrivateDNSAddr    string
	UpstreamDNS       string
	DNSQueryLogs      bool

	// deployment profile (standard | shield | sentinel)
	ErebrusProfile      string
	FirewallProvider    string
	FirewallDNSAddr     string
	ShieldAdminURL      string
	ShieldAdminUser     string
	ShieldAdminPassword string
	// ShieldUpstreamDNS is a comma-separated list of AdGuard upstream resolvers
	// (UDP). Defaults to fast public DNS — avoid DoH here; VPN clients time out on
	// slow cache-miss chains through the tunnel DNS forwarder.
	ShieldUpstreamDNS string
	SentinelAPIURL    string
	SentinelImage     string

	// SentinelLicensed is set at runtime after license check (not from env).
	SentinelLicensed bool
}

// Load reads configuration from the environment, applying sane defaults.
func Load() *Config {
	bindAddr := env("API_BIND_ADDR", "")
	if bindAddr == "" {
		bindAddr = env("SERVER", "0.0.0.0")
	}
	c := &Config{
		RunType:                 env("RUNTYPE", "release"),
		BindAddr:                bindAddr,
		HTTPPort:                env("HTTP_PORT", "9080"),
		UnsafePublicAPI:         boolEnv("UNSAFE_PUBLIC_API", false),
		PublicDomain:            firstEnv("EREBRUS_DOMAIN", "PUBLIC_DOMAIN", ""),
		WildcardDomain:          env("WILDCARD_DOMAIN", os.Getenv("EREBRUS_WILDCARD_DOMAIN")),
		PublicGatewayEnabled:    boolEnv("PUBLIC_GATEWAY_ENABLED", boolEnv("EREBRUS_PUBLIC_GATEWAY", false)),
		PublicHTTPPort:          env("PUBLIC_HTTP_PORT", "80"),
		PublicHTTPSPort:         env("PUBLIC_HTTPS_PORT", "443"),
		AutoTLS:                 boolEnv("AUTO_TLS", true),
		NodeName:                env("NODE_NAME", hostnameOr("erebrus-node")),
		Region:                  env("REGION", "unknown"),
		Zone:                    env("ZONE", ""),
		Version:                 Version,
		Mnemonic:                os.Getenv("MNEMONIC"),
		GatewayURL:              env("GATEWAY_URL", ""),
		GatewayPeerMultiaddr:    env("GATEWAY_PEER_MULTIADDR", ""),
		P2PListenPort:           env("P2P_LISTEN_PORT", "9002"),
		NodeID:                  os.Getenv("NODE_ID"),
		NodeToken:               os.Getenv("NODE_TOKEN"),
		WalletChain:             env("WALLET_CHAIN", "SOLANA"),
		NodeRegistrationToken:   firstEnv("EREBRUS_NODE_REGISTRATION_TOKEN", "EREBRUS_ORG_ENROLLMENT_SECRET", "ORG_ENROLLMENT_SECRET", ""),
		APIPublicURL:            os.Getenv("API_PUBLIC_URL"),
		GatewayAutoRegister:     boolEnv("GATEWAY_AUTO_REGISTER", true),
		GatewayPublicKey:        os.Getenv("GATEWAY_PUBLIC_KEY"),
		NodeKey:                 firstEnv("NODE_KEY", "NODE_API_TOKEN", ""),
		NodeAPIToken:            firstEnv("NODE_KEY", "NODE_API_TOKEN", ""),
		WGConfDir:               env("WG_CONF_DIR", "/etc/wireguard"),
		WGInterface:             normalizeInterface(env("WG_INTERFACE_NAME", "wg0")),
		WGEndpointHost:          os.Getenv("WG_ENDPOINT_HOST"),
		WGEndpointPort:          firstEnv("WG_PORT", "WG_ENDPOINT_PORT", "51820"),
		StealthTCPPort:          firstEnv("STEALTH_TCP_PORT", "VLESS_PORT", "8443"),
		StealthUDPPort:          firstEnv("STEALTH_UDP_PORT", "HYSTERIA2_PORT", "4443"),
		WGIPv4Subnet:            env("WG_IPv4_SUBNET", "10.0.0.1/16"),
		WGDNS:                   env("WG_DNS", "1.1.1.1"),
		WGPostUp:                os.Getenv("WG_POST_UP"),
		WGPostDown:              os.Getenv("WG_POST_DOWN"),
		WGPreUp:                 os.Getenv("WG_PRE_UP"),
		WGPreDown:               os.Getenv("WG_PRE_DOWN"),
		EnableStealth:           boolEnv("ENABLE_STEALTH", true),
		VLESSPort:               "", // synced from StealthTCPPort below
		Hysteria2Port:           "", // synced from StealthUDPPort below
		RealityServerNames:      splitCSV(env("REALITY_SERVER_NAMES", "www.microsoft.com")),
		RealityHandshakeServer:  env("REALITY_HANDSHAKE_SERVER", ""),
		Hysteria2ObfsPassword:   os.Getenv("HYSTERIA2_OBFS_PASSWORD"),
		EnableTUIC:              boolEnv("ENABLE_TUIC", false),
		StateDir:                env("STATE_DIR", "/var/lib/erebrus"),
		EnableAppHosting:        boolEnv("ENABLE_APP_HOSTING", false),
		AppWildcardDomain:       os.Getenv("APP_WILDCARD_DOMAIN"),
		DropEnabled:             boolEnv("DROP_ENABLED", false),
		DropStorageMax:          env("DROP_STORAGE_MAX", "10GB"),
		DropSwarmPort:           env("DROP_SWARM_PORT", "4001"),
		DropWebUIEnabled:        boolEnv("DROP_WEBUI_ENABLED", false),
		DropPublicGatewayDomain: env("DROP_PUBLIC_GATEWAY_DOMAIN", ""),
		ChainRegistration:       env("CHAIN_REGISTRATION", "off"),
		PrivateDNSEnabled:       boolEnv("PRIVATE_DNS_ENABLED", false),
		PrivateDNSDomain:        env("PRIVATE_DNS_DOMAIN", "ere"),
		PrivateDNSAddr:          os.Getenv("PRIVATE_DNS_ADDR"),
		UpstreamDNS:             env("UPSTREAM_DNS", "1.1.1.1"),
		DNSQueryLogs:            boolEnv("DNS_QUERY_LOGS", false),
		ErebrusProfile:          env("EREBRUS_PROFILE", ProfileStandard),
		FirewallProvider:        env("FIREWALL_PROVIDER", ""),
		FirewallDNSAddr:         os.Getenv("FIREWALL_DNS_ADDR"),
		ShieldAdminURL:          os.Getenv("SHIELD_ADMIN_URL"),
		ShieldAdminUser:         env("SHIELD_ADMIN_USER", "admin"),
		ShieldAdminPassword:     os.Getenv("SHIELD_ADMIN_PASSWORD"),
		ShieldUpstreamDNS:       env("SHIELD_UPSTREAM_DNS", "1.1.1.1,1.0.0.1"),
		SentinelAPIURL:          os.Getenv("SENTINEL_API_URL"),
		SentinelImage:           env("SENTINEL_IMAGE", "ghcr.io/netsepio/erebrus-sentinel:latest"),
	}
	c.ApplyProfileDefaults()
	if mode, err := ParseModeSettingsFromEnv(); err == nil {
		c.Mode = mode
	}
	c.VLESSPort = c.StealthTCPPort
	c.Hysteria2Port = c.StealthUDPPort
	c.DropStorageMaxBytes, _ = parseByteSize(c.DropStorageMax)
	// The management peer API shares the HTTP listener. When it is bound to a
	// non-loopback address it is reachable off-host (token-gated, fail-closed),
	// so always surface that as a conscious decision — not just under the
	// UNSAFE_PUBLIC_API flag.
	if !isLoopbackAddr(c.BindAddr) {
		c.Mode.Warnings = append(c.Mode.Warnings, fmt.Sprintf(
			"WARNING: management API bound to %s:%s — the token-gated peer API is reachable off-host. "+
				"Firewall this port to the gateway/trusted sources, or set API_BIND_ADDR=127.0.0.1.",
			c.BindAddr, c.HTTPPort))
	}
	return c
}

// isLoopbackAddr reports whether the bind address is loopback-only.
func isLoopbackAddr(addr string) bool {
	switch addr {
	case "127.0.0.1", "::1", "localhost":
		return true
	}
	return strings.HasPrefix(addr, "127.")
}

// Validate returns an error if required fields are missing or invalid.
func (c *Config) Validate() error {
	mode, err := ParseModeSettingsFromEnv()
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
	if c.Mode.IsPublic() && c.PublicDomain == "" && c.EnableAppHosting {
		c.Mode.Warnings = append(c.Mode.Warnings,
			"WARNING: Public access mode with ENABLE_APP_HOSTING but no PUBLIC_DOMAIN set; public edge routing may be incomplete.")
	}
	if c.Mode.IsPublic() && (c.StealthTCPPort != "443" || c.StealthUDPPort != "443") {
		c.Mode.Warnings = append(c.Mode.Warnings,
			"WARNING: Public access mode production should expose stealth on 443/tcp and 443/udp (STEALTH_TCP_PORT/STEALTH_UDP_PORT) for best reachability.")
	}
	if c.DropEnabled {
		if c.Mode.Deploy == DeployHost {
			return fmt.Errorf("Drop requires container deployment in v1")
		}
		if c.DropStorageMaxBytes <= 0 {
			return fmt.Errorf("DROP_STORAGE_MAX must be a positive byte size")
		}
		port, err := strconv.Atoi(c.DropSwarmPort)
		if err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("DROP_SWARM_PORT must be a valid port")
		}
		if c.DropWebUIEnabled && c.Mode.IsPublic() {
			return fmt.Errorf("DROP_WEBUI_ENABLED is allowed only for private or shared nodes")
		}
		if c.DropPublicGatewayDomain != "" {
			normalized, err := normalizePublicGatewayDomain(c.DropPublicGatewayDomain)
			if err != nil {
				return fmt.Errorf("invalid DROP_PUBLIC_GATEWAY_DOMAIN: %w", err)
			}
			c.DropPublicGatewayDomain = normalized
		}
	}
	return c.ValidateProfile()
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

// EffectiveRegistrationToken returns the scoped node registration token.
func (c *Config) EffectiveRegistrationToken() string {
	return strings.TrimSpace(c.NodeRegistrationToken)
}

// EffectiveNodeKey returns the per-node API bearer (NODE_KEY legacy: NODE_API_TOKEN).
func (c *Config) EffectiveNodeKey() string {
	if k := strings.TrimSpace(c.NodeKey); k != "" {
		return k
	}
	return strings.TrimSpace(c.NodeAPIToken)
}

// WGEndpointPortInt parses the endpoint port.
func (c *Config) WGEndpointPortInt() int {
	n, _ := strconv.Atoi(c.WGEndpointPort)
	return n
}

// VLESSPortInt parses the VLESS+REALITY listen port.
func (c *Config) VLESSPortInt() int { n, _ := strconv.Atoi(c.VLESSPort); return n }

// Hysteria2PortInt parses the Hysteria2 listen port.
func (c *Config) Hysteria2PortInt() int { n, _ := strconv.Atoi(c.Hysteria2Port); return n }

// DropSwarmPortInt parses the Kubo swarm port.
func (c *Config) DropSwarmPortInt() int { n, _ := strconv.Atoi(c.DropSwarmPort); return n }

// DropAcceptsPublicUploads reports whether the gateway may select this node for public Drop storage.
func (c *Config) DropAcceptsPublicUploads() bool {
	return c.DropEnabled && c.Mode.IsPublic()
}

// DropPublicGatewayURL returns the canonical HTTPS URL for the configured public
// CID gateway domain. It returns an empty string when no domain is configured
// or when the domain is invalid.
func (c *Config) DropPublicGatewayURL() string {
	return publicGatewayURL(c.DropPublicGatewayDomain)
}

// DropWebUIAvailable reports whether the private Kubo administration proxy is enabled.
func (c *Config) DropWebUIAvailable() bool {
	return c.DropEnabled && c.DropWebUIEnabled && !c.Mode.IsPublic()
}

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

func parseByteSize(value string) (int64, error) {
	s := strings.ToUpper(strings.TrimSpace(value))
	if s == "" {
		return 0, fmt.Errorf("empty byte size")
	}
	units := []struct {
		suffix string
		scale  int64
	}{
		{"TB", 1_000_000_000_000},
		{"GB", 1_000_000_000},
		{"MB", 1_000_000},
		{"KB", 1_000},
		{"B", 1},
	}
	for _, unit := range units {
		if !strings.HasSuffix(s, unit.suffix) {
			continue
		}
		raw := strings.TrimSpace(strings.TrimSuffix(s, unit.suffix))
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || n <= 0 || n > (1<<63-1)/unit.scale {
			return 0, fmt.Errorf("invalid byte size %q", value)
		}
		return n * unit.scale, nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid byte size %q", value)
	}
	return n, nil
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

// publicGatewayURL constructs the canonical HTTPS base URL for a public CID
// gateway domain. It returns the empty string for empty or invalid domains.
func publicGatewayURL(domain string) string {
	domain, err := normalizePublicGatewayDomain(domain)
	if err != nil || domain == "" {
		return ""
	}
	return "https://" + domain
}

// normalizePublicGatewayDomain validates and normalizes a drop public gateway
// domain. It rejects schemes, ports, paths, query fragments, credentials,
// localhost, and IP literals.
func normalizePublicGatewayDomain(domain string) (string, error) {
	domain = strings.TrimSpace(domain)
	domain = strings.TrimSuffix(domain, ".")
	if domain == "" {
		return "", nil
	}
	if strings.ContainsAny(domain, ":/?#@") || strings.Contains(domain, "..") {
		return "", fmt.Errorf("domain must not include a scheme, port, path, query, credentials, or empty labels")
	}
	if strings.EqualFold(domain, "localhost") {
		return "", fmt.Errorf("domain must not be localhost")
	}
	if net.ParseIP(domain) != nil {
		return "", fmt.Errorf("domain must not be an IP literal")
	}
	if !isValidDNSHost(domain) {
		return "", fmt.Errorf("domain must be a valid DNS hostname")
	}
	return strings.ToLower(domain), nil
}

var dnsHostRe = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`)

func isValidDNSHost(domain string) bool {
	if len(domain) > 253 {
		return false
	}
	return dnsHostRe.MatchString(domain)
}
