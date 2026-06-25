package edge

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/NetSepio/erebrus/internal/services"
)

// Proxy routes public HTTP requests to registered services by hostname.
type Proxy struct {
	Reg *services.Registry
	St  interface {
		GetServiceByDomain(ctx context.Context, domain string) (string, error)
	}
	WildcardDomain string
}

// Handler returns an http.Handler for the public edge.
func (p *Proxy) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := strings.Split(r.Host, ":")[0]
		svc, err := p.lookup(r.Context(), host)
		if err != nil || svc == nil {
			http.NotFound(w, r)
			return
		}
		target, err := url.Parse("http://" + svc.InternalAddr)
		if err != nil {
			http.Error(w, "bad target", http.StatusBadGateway)
			return
		}
		rp := httputil.NewSingleHostReverseProxy(target)
		rp.ServeHTTP(w, r)
	})
}

func (p *Proxy) lookup(ctx context.Context, host string) (*services.Service, error) {
	items, err := p.Reg.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, s := range items {
		if s.Public && strings.EqualFold(s.PublicHost, host) {
			return &s, nil
		}
	}
	if p.St != nil {
		if id, err := p.St.GetServiceByDomain(ctx, host); err == nil && id != "" {
			return p.Reg.Get(ctx, id)
		}
	}
	if p.WildcardDomain != "" {
		base := strings.TrimPrefix(p.WildcardDomain, "*.")
		if strings.HasSuffix(host, base) {
			prefix := strings.TrimSuffix(host, "."+base)
			name := strings.Split(prefix, ".")[0]
			return p.Reg.FindByName(ctx, name)
		}
	}
	return nil, fmt.Errorf("no service for host %s", host)
}
