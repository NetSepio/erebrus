// Package dns provides a private VPN DNS resolver backed by the service registry.
package dns

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/NetSepio/erebrus/internal/services"
	"github.com/miekg/dns"
)

// Config drives the private resolver.
type Config struct {
	Enabled      bool
	Domain       string // e.g. "ere"
	ListenAddr   string // e.g. "10.66.0.1:53"
	Upstream     string // e.g. "1.1.1.1:53"
	QueryLogs    bool
}

// Server resolves <name>.<domain> from the registry and forwards other queries.
type Server struct {
	cfg  Config
	reg  *services.Registry
	srv  *dns.Server
}

// New constructs a DNS server (call Start to listen).
func New(cfg Config, reg *services.Registry) *Server {
	return &Server{cfg: cfg, reg: reg}
}

// Start listens until ctx is cancelled.
func (s *Server) Start(ctx context.Context) error {
	if !s.cfg.Enabled {
		return nil
	}
	mux := dns.NewServeMux()
	mux.HandleFunc(".", s.handle)
	s.srv = &dns.Server{Addr: s.cfg.ListenAddr, Net: "udp", Handler: mux}
	go func() {
		<-ctx.Done()
		_ = s.srv.Shutdown()
	}()
	slog.Info("private DNS listening", "addr", s.cfg.ListenAddr, "domain", s.cfg.Domain)
	return s.srv.ListenAndServe()
}

func (s *Server) handle(w dns.ResponseWriter, r *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative = true

	for _, q := range r.Question {
		if q.Qtype != dns.TypeA {
			continue
		}
		name := strings.TrimSuffix(strings.ToLower(q.Name), ".")
		suffix := "." + strings.ToLower(s.cfg.Domain)
		if !strings.HasSuffix(name, suffix) {
			continue
		}
		svcName := strings.TrimSuffix(name, suffix)
		// Support ollama.local.ere -> ollama
		if i := strings.Index(svcName, "."); i >= 0 {
			svcName = svcName[:i]
		}
		svc, err := s.reg.FindByName(context.Background(), svcName)
		if err != nil {
			m.Rcode = dns.RcodeNameError
			continue
		}
		host, _, err := net.SplitHostPort(svc.InternalAddr)
		if err != nil {
			host = strings.Split(svc.InternalAddr, ":")[0]
		}
		ip := net.ParseIP(host)
		if ip == nil || ip.To4() == nil {
			m.Rcode = dns.RcodeNameError
			continue
		}
		rr := &dns.A{
			Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
			A:   ip.To4(),
		}
		m.Answer = append(m.Answer, rr)
		if s.cfg.QueryLogs {
			slog.Debug("dns query", "name", q.Name, "answer", ip.String())
		}
	}

	if len(m.Answer) == 0 && len(r.Question) > 0 {
		if fwd, err := s.forward(r); err == nil {
			_ = w.WriteMsg(fwd)
			return
		}
	}
	_ = w.WriteMsg(m)
}

func (s *Server) forward(r *dns.Msg) (*dns.Msg, error) {
	up := s.cfg.Upstream
	if !strings.Contains(up, ":") {
		up = net.JoinHostPort(up, "53")
	}
	c := &dns.Client{Timeout: 2 * time.Second}
	msg, _, err := c.Exchange(r, up)
	return msg, err
}

// DefaultListenAddr derives host:53 from a CIDR subnet gateway IP override.
func DefaultListenAddr(subnet, override string) string {
	if override != "" {
		if !strings.Contains(override, ":") {
			return net.JoinHostPort(override, "53")
		}
		return override
	}
	ip, _, err := net.ParseCIDR(subnet)
	if err != nil {
		return "127.0.0.1:53"
	}
	return net.JoinHostPort(ip.String(), "53")
}

// Validate checks resolver config.
func (c Config) Validate() error {
	if !c.Enabled {
		return nil
	}
	if c.Domain == "" {
		return fmt.Errorf("PRIVATE_DNS_DOMAIN is required when private DNS is enabled")
	}
	if c.ListenAddr == "" {
		return fmt.Errorf("PRIVATE_DNS_ADDR is required when private DNS is enabled")
	}
	return nil
}