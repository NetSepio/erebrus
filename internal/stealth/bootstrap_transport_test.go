package stealth

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/outbound"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/protocol/vless"
	M "github.com/sagernet/sing/common/metadata"
)

// Return the address sing-box actually dials over the authenticated connection.
// This exercises the router and both VLESS identities without contacting a gateway.
type echoDestination struct{ outbound.Adapter }

func (d *echoDestination) DialContext(_ context.Context, _ string, destination M.Socksaddr) (net.Conn, error) {
	client, server := net.Pipe()
	go func() {
		defer server.Close()
		_ = server.SetDeadline(time.Now().Add(5 * time.Second))
		var request [1]byte
		if _, err := io.ReadFull(server, request[:]); err == nil {
			_, _ = io.WriteString(server, destination.String()+"\n")
		}
	}()
	return client, nil
}
func (d *echoDestination) ListenPacket(context.Context, M.Socksaddr) (net.PacketConn, error) {
	return nil, errors.New("unexpected UDP dial")
}

func TestBootstrapRealityTransport(t *testing.T) {
	handshake := httptest.NewUnstartedServer(nil)
	handshake.EnableHTTP2 = true
	handshake.TLS = &tls.Config{MinVersion: tls.VersionTLS13, CurvePreferences: []tls.CurveID{tls.X25519}}
	handshake.StartTLS()
	defer handshake.Close()

	cfg := testConfig(freePort(t), freePort(t))
	cfg.RealityServerNames = []string{"cover.example"}
	cfg.RealityHandshakeServer = handshake.Listener.Addr().String()
	cfg.GatewayURL = "https://gateway.example"
	m := New(cfg, newMemStore())
	if err := m.Init(t.Context()); err != nil {
		t.Fatal(err)
	}
	// Rotate first: a client holding the retired descriptor must still reach the
	// gateway, or rotation locks out exactly the clients the carrier exists for.
	// The descriptor is captured whole, before the rotation — a retired UUID
	// paired with the new short-id would prove nothing, since the short-id is
	// what the handshake checks first.
	retired, retiredShortID := m.secrets.VLESSBootstrapUUID, m.secrets.RealityShortID
	if err := m.RotateAllSecrets(t.Context(), time.Hour); err != nil {
		t.Fatal(err)
	}
	if retiredShortID == m.secrets.RealityShortID {
		t.Fatal("rotation left the short-id unchanged, so this proves nothing")
	}
	opts := m.serverOptions()
	registry := outboundRegistry()
	outbound.Register(registry, C.TypeDirect, func(_ context.Context, _ adapter.Router, _ log.ContextLogger, tag string, _ option.DirectOutboundOptions) (adapter.Outbound, error) {
		return &echoDestination{outbound.NewAdapter(C.TypeDirect, tag, []string{"tcp", "udp"}, nil)}, nil
	})
	vless.RegisterOutbound(registry)
	for _, client := range []struct{ tag, id, shortID string }{
		{"bootstrap-client", m.secrets.VLESSBootstrapUUID, m.secrets.RealityShortID},
		{"retired-client", retired, retiredShortID},
		{"vpn-client", m.secrets.VLESSUUID, m.secrets.RealityShortID},
	} {
		opts.Outbounds = append(opts.Outbounds, option.Outbound{Tag: client.tag, Type: C.TypeVLESS, Options: &option.VLESSOutboundOptions{
			ServerOptions: option.ServerOptions{Server: "127.0.0.1", ServerPort: uint16(cfg.VLESSPortInt())},
			UUID:          client.id, Flow: vlessFlowVision,
			OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{TLS: &option.OutboundTLSOptions{
				Enabled: true, ServerName: cfg.RealitySNI(), UTLS: &option.OutboundUTLSOptions{Enabled: true, Fingerprint: "chrome"},
				Reality: &option.OutboundRealityOptions{Enabled: true, PublicKey: m.secrets.RealityPublicKey, ShortID: client.shortID},
			}},
		}})
	}
	ctx := box.Context(t.Context(), inboundRegistry(), registry, endpointRegistry(), dnsTransportRegistry(), serviceRegistry(), certificateProviderRegistry())
	instance, err := box.New(box.Options{Context: ctx, Options: opts})
	if err != nil {
		t.Fatal(err)
	}
	defer instance.Close()
	if err := instance.Start(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ client, destination, want string }{
		{"bootstrap-client", "gateway.example:443", "gateway.example:443"},
		{"bootstrap-client", "other.example:443", "127.0.0.1:51820"},
		{"bootstrap-client", "gateway.example:80", "127.0.0.1:51820"},
		{"vpn-client", "gateway.example:443", "127.0.0.1:51820"},
		// The retired credential keeps the same narrow reach during its grace window.
		{"retired-client", "gateway.example:443", "gateway.example:443"},
		{"retired-client", "other.example:443", "127.0.0.1:51820"},
	} {
		t.Run(tc.client+"/"+tc.destination, func(t *testing.T) {
			client, ok := instance.Outbound().Outbound(tc.client)
			if !ok {
				t.Fatal("missing test client")
			}
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			conn, err := client.DialContext(ctx, "tcp", M.ParseSocksaddr(tc.destination))
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
			if _, err := conn.Write([]byte{1}); err != nil {
				t.Fatal(err)
			}
			got, err := bufio.NewReader(conn).ReadString('\n')
			if err != nil {
				t.Fatal(err)
			}
			if strings.TrimSpace(got) != tc.want {
				t.Fatalf("dialed %q, want %q", got, tc.want)
			}
		})
	}
}
