package stealth

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/NetSepio/erebrus/internal/config"
)

// memStore is an in-memory SettingsStore for tests.
type memStore struct{ m map[string]string }

func newMemStore() *memStore { return &memStore{m: map[string]string{}} }

func (s *memStore) GetSetting(_ context.Context, k string) (string, error) { return s.m[k], nil }
func (s *memStore) SetSetting(_ context.Context, k, v string) error        { s.m[k] = v; return nil }

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
