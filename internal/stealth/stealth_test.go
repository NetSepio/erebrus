package stealth

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"maps"
	"net"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/NetSepio/erebrus/internal/config"
	"github.com/google/uuid"
	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/route/rule"
	M "github.com/sagernet/sing/common/metadata"
)

// memStore is an in-memory SettingsStore for tests.
type memStore struct{ m map[string]string }

func newMemStore() *memStore { return &memStore{m: map[string]string{}} }

func (s *memStore) GetSetting(_ context.Context, k string) (string, error) { return s.m[k], nil }
func (s *memStore) SetSetting(_ context.Context, k, v string) error        { s.m[k] = v; return nil }

func (s *memStore) SetSettings(_ context.Context, values map[string]string) error {
	maps.Copy(s.m, values)
	return nil
}

func testConfig(vlessPort, hy2Port int) *config.Config {
	return &config.Config{
		RunType:            "release",
		NodeName:           "test-node",
		EnableStealth:      true,
		WGEndpointHost:     "127.0.0.1",
		WGEndpointPort:     "51820",
		VLESSPort:          strconv.Itoa(vlessPort),
		Hysteria2Port:      strconv.Itoa(hy2Port),
		RealityServerNames: []string{"www.microsoft.com"},
	}
}

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("free port: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func TestSecretsStableAndValid(t *testing.T) {
	ctx := context.Background()
	st := newMemStore()

	s1, err := loadOrCreateSecrets(ctx, st)
	if err != nil {
		t.Fatalf("first load: %v", err)
	}
	s2, err := loadOrCreateSecrets(ctx, st)
	if err != nil {
		t.Fatalf("second load: %v", err)
	}
	if *s1 != *s2 {
		t.Fatalf("secrets not stable across loads:\n%+v\n%+v", s1, s2)
	}

	// REALITY keys must be 32-byte x25519 values in base64 RawURL.
	for name, key := range map[string]string{"private": s1.RealityPrivateKey, "public": s1.RealityPublicKey} {
		raw, err := base64.RawURLEncoding.DecodeString(key)
		if err != nil {
			t.Fatalf("reality %s key not base64 RawURL: %v", name, err)
		}
		if len(raw) != 32 {
			t.Fatalf("reality %s key wrong length: %d", name, len(raw))
		}
	}
	if len(s1.RealityShortID) != 8 {
		t.Fatalf("short id should be 8 hex chars, got %q", s1.RealityShortID)
	}
	if s1.VLESSUUID == "" || s1.Hysteria2Password == "" {
		t.Fatal("vless uuid / hysteria2 password must be set")
	}
}

func TestCertStableAndUsable(t *testing.T) {
	ctx := context.Background()
	st := newMemStore()

	cert, key, err := loadOrCreateCert(ctx, st, "www.example.com")
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	if _, err := tls.X509KeyPair([]byte(cert), []byte(key)); err != nil {
		t.Fatalf("cert/key not a valid pair: %v", err)
	}
	cert2, key2, err := loadOrCreateCert(ctx, st, "www.example.com")
	if err != nil {
		t.Fatalf("reload cert: %v", err)
	}
	if cert != cert2 || key != key2 {
		t.Fatal("cert not persisted/stable across calls")
	}
}

func TestParamsDisabled(t *testing.T) {
	cfg := testConfig(8443, 4443)
	cfg.EnableStealth = false
	m := New(cfg, newMemStore())
	if err := m.Init(context.Background()); err != nil {
		t.Fatalf("init: %v", err)
	}
	if p := m.Params(); p.Enabled {
		t.Fatal("params should report disabled when stealth is off")
	}
}

func TestStartListensAndClose(t *testing.T) {
	ctx := context.Background()
	vp, hp := freePort(t), freePort(t)
	m := New(testConfig(vp, hp), newMemStore())
	if err := m.Init(ctx); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := m.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer m.Close()

	// The VLESS+REALITY carrier listens on TCP; a bare connect should succeed.
	addr := fmt.Sprintf("127.0.0.1:%d", vp)
	var conn net.Conn
	var err error
	for i := 0; i < 50; i++ {
		conn, err = net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("VLESS carrier not listening on %s: %v", addr, err)
	}
	conn.Close()
}

func TestBuildPeerArtifacts(t *testing.T) {
	ctx := context.Background()
	m := New(testConfig(8443, 4443), newMemStore())
	if err := m.Init(ctx); err != nil {
		t.Fatalf("init: %v", err)
	}

	ps := m.BuildPeer("alice", "c2VydmVycHVibGlja2V5MDAwMDAwMDAwMDAwMDAwMD0=", "10.0.0.7/32", "")

	if !strings.HasPrefix(ps.VLESSURI, "vless://") {
		t.Fatalf("bad vless uri: %s", ps.VLESSURI)
	}
	for _, want := range []string{"security=reality", "flow=xtls-rprx-vision", "sni=www.microsoft.com", ":8443"} {
		if !strings.Contains(ps.VLESSURI, want) {
			t.Fatalf("vless uri missing %q: %s", want, ps.VLESSURI)
		}
	}
	if !strings.HasPrefix(ps.Hysteria2URI, "hysteria2://") || !strings.Contains(ps.Hysteria2URI, ":4443") {
		t.Fatalf("bad hysteria2 uri: %s", ps.Hysteria2URI)
	}

	// The sing-box profile must serialize and carry a WG endpoint detoured
	// through the VLESS carrier, with the WG peer pinned to the node loopback.
	raw, err := json.Marshal(ps.SingboxProfile)
	if err != nil {
		t.Fatalf("marshal profile: %v", err)
	}
	s := string(raw)
	for _, want := range []string{`"type":"wireguard"`, `"detour":"carrier-vless"`, `"carrier-hysteria2"`, `"127.0.0.1"`, ClientPrivateKeyPlaceholder} {
		if !strings.Contains(s, want) {
			t.Fatalf("profile missing %q: %s", want, s)
		}
	}
}

func TestBootstrapRoute(t *testing.T) {
	cfg := testConfig(8443, 4443)
	cfg.GatewayURL = "https://Gateway.Example:443/api"
	cfg.WGEndpointPort = "51973"
	m := New(cfg, newMemStore())
	if err := m.Init(t.Context()); err != nil {
		t.Fatal(err)
	}
	routes := m.serverOptions().Route
	if len(routes.Rules) != 2 || routes.Final != "direct" {
		t.Fatal("expected the bootstrap rule, then the WireGuard override")
	}
	first := routes.Rules[0]
	allow := first.DefaultOptions.RuleAction
	if allow.Action != C.RuleActionTypeRoute || allow.RouteOptions.Outbound != "direct" || allow.RouteOptions.OverrideAddress != "gateway.example" || allow.RouteOptions.OverridePort != 443 {
		t.Fatal("bootstrap route must terminate at the configured gateway")
	}
	// Exercise sing-box's matcher so its rule combination semantics are covered.
	matcher, err := rule.NewRule(t.Context(), nil, first, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		user, inbound, network, destination string
		allowed                             bool
	}{
		{"bootstrap", "vless-reality", "tcp", "gateway.example:443", true},
		{"bootstrap", "vless-reality", "tcp", "sub.gateway.example:443", false},
		{"bootstrap", "vless-reality", "tcp", "gateway.example.invalid:443", false},
		{"bootstrap", "vless-reality", "tcp", "192.0.2.1:443", false},
		{"bootstrap", "vless-reality", "tcp", "gateway.example:8443", false},
		{"bootstrap", "vless-reality", "udp", "gateway.example:443", false},
		{"erebrus", "vless-reality", "tcp", "gateway.example:443", false},
		{"bootstrap", "hysteria2", "tcp", "gateway.example:443", false},
	} {
		metadata := adapter.InboundContext{User: tc.user, Inbound: tc.inbound, Network: tc.network, Destination: M.ParseSocksaddr(tc.destination)}
		if got := matcher.Match(&metadata); got != tc.allowed {
			t.Errorf("%+v matched=%v", tc, got)
		}
	}
	fallback := routes.Rules[1].DefaultOptions
	if !reflect.DeepEqual(fallback.RawDefaultRule, option.RawDefaultRule{}) || fallback.Action != C.RuleActionTypeRouteOptions || fallback.RouteOptionsOptions.OverrideAddress != "127.0.0.1" || fallback.RouteOptionsOptions.OverridePort != 51973 {
		t.Fatal("fallback must pin every remaining request to the configured WireGuard port")
	}
	for _, raw := range []string{"", "gateway.example", "https://", "https://*.example", "http://gateway.example:8080", "https://gateway.example:8443"} {
		cfg.GatewayURL = raw
		standalone := New(cfg, m.st)
		if err := standalone.Init(t.Context()); err != nil {
			t.Fatal(err)
		}
		if got := standalone.serverOptions().Route.Rules; len(got) != 1 || !reflect.DeepEqual(got[0].DefaultOptions, fallback) {
			t.Errorf("unexpected routes for %q", raw)
		}
	}
}

func TestBootstrapIPGatewayMatchesOneAddress(t *testing.T) {
	for _, tc := range []struct{ configured, allowed, adjacent string }{
		{"https://192.0.2.7", "192.0.2.7:443", "192.0.2.8:443"},
		{"https://[2001:db8::7]:443", "[2001:db8::7]:443", "[2001:db8::8]:443"},
	} {
		r, err := rule.NewRule(t.Context(), nil, bootstrapRule(bootstrapHost(tc.configured)), true)
		if err != nil {
			t.Fatal(err)
		}
		for _, dst := range []string{tc.allowed, tc.adjacent} {
			m := adapter.InboundContext{Inbound: "vless-reality", User: "bootstrap", Network: "tcp", Destination: M.ParseSocksaddr(dst)}
			if r.Match(&m) != (dst == tc.allowed) {
				t.Errorf("wrong match for %s on %s", dst, tc.configured)
			}
		}
	}
}

func TestBootstrapCredentialLifecycle(t *testing.T) {
	ctx := t.Context()
	cfg := testConfig(freePort(t), freePort(t))
	cfg.GatewayURL = "https://gateway.example"
	st := newMemStore()
	legacy, err := loadOrCreateSecrets(ctx, st)
	if err != nil {
		t.Fatal(err)
	}
	delete(st.m, keyVLESSBootstrapUUID) // Upgrade a pre-bootstrap settings store.
	m := New(cfg, st)
	if err := m.Init(ctx); err != nil {
		t.Fatal(err)
	}
	before := m.Params()
	if _, err := uuid.Parse(before.VLESSBootstrapUUID); err != nil || before.VLESSBootstrapUUID == before.VLESSUUID || before.VLESSUUID != legacy.VLESSUUID {
		t.Fatal("upgrade must preserve the VPN UUID and provision a distinct bootstrap UUID")
	}
	if m.ActiveParams().Enabled {
		t.Fatal("advertised a stopped listener")
	}
	if err := m.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if got := m.ActiveParams().VLESSBootstrapUUID; got != before.VLESSBootstrapUUID {
		t.Fatal("running listener did not advertise its credential")
	}

	if _, err := m.RotateReality(ctx); err != nil {
		t.Fatal(err)
	}
	if got := m.ActiveParams(); got.VLESSBootstrapUUID != before.VLESSBootstrapUUID || got.RealityShortID == before.RealityShortID {
		t.Fatal("short-ID rotation changed the bootstrap UUID or failed to change the short ID")
	}
	if err := m.RotateAllSecrets(ctx); err != nil {
		t.Fatal(err)
	}
	active := m.ActiveParams()
	reloaded, err := loadOrCreateSecrets(ctx, st)
	if err != nil {
		t.Fatal(err)
	}
	if active.VLESSBootstrapUUID == "" || active.VLESSBootstrapUUID == before.VLESSBootstrapUUID || active.VLESSBootstrapUUID != reloaded.VLESSBootstrapUUID || active.VLESSUUID == before.VLESSUUID || active.VLESSUUID == active.VLESSBootstrapUUID {
		t.Fatal("full rotation did not persist and activate distinct fresh credentials")
	}
	for _, obj := range []any{active, m.BuildPeer("alice", "key", "10.0.0.2/32", "")} {
		data, err := json.Marshal(obj)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(data, []byte(active.VLESSBootstrapUUID)) {
			t.Fatal("bootstrap UUID leaked into peer credentials")
		}
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	if got := m.ActiveParams().VLESSBootstrapUUID; got != "" {
		t.Fatal("advertised a credential after close")
	}
}
