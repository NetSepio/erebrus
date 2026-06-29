package dns

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
)

// ForwarderConfig forwards all queries from VPN clients to an upstream resolver.
type ForwarderConfig struct {
	ListenAddr string // e.g. 10.0.0.1:53
	Upstream   string // e.g. adguardhome:53 or erebrus-sentinel:53
}

// Forwarder is a simple DNS proxy for Shield/Sentinel profiles.
type Forwarder struct {
	cfg ForwarderConfig
	srv *dns.Server
}

// NewForwarder constructs a forwarder (call Start to listen).
func NewForwarder(cfg ForwarderConfig) *Forwarder {
	return &Forwarder{cfg: cfg}
}

// Start listens until ctx is cancelled.
func (f *Forwarder) Start(ctx context.Context) error {
	if f.cfg.ListenAddr == "" || strings.TrimSpace(f.cfg.Upstream) == "" {
		return fmt.Errorf("forwarder listen and upstream are required")
	}
	mux := dns.NewServeMux()
	mux.HandleFunc(".", f.handle)
	f.srv = &dns.Server{Addr: f.cfg.ListenAddr, Net: "udp", Handler: mux}
	go func() {
		<-ctx.Done()
		_ = f.srv.Shutdown()
	}()
	return f.srv.ListenAndServe()
}

func (f *Forwarder) handle(w dns.ResponseWriter, r *dns.Msg) {
	up := f.cfg.Upstream
	if !strings.Contains(up, ":") {
		up = net.JoinHostPort(up, "53")
	}
	c := &dns.Client{Timeout: 2 * time.Second}
	msg, _, err := c.Exchange(r, up)
	if err != nil {
		m := new(dns.Msg)
		m.SetRcode(r, dns.RcodeServerFailure)
		_ = w.WriteMsg(m)
		return
	}
	_ = w.WriteMsg(msg)
}