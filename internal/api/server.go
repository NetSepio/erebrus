// Package api serves the node's HTTP REST surface (Gin) under /api/v2. It
// replaces the v1 api/v1 tree and the deleted gRPC server. Provisioning logic
// lives in the Provisioner so Phase 2 can extend it with sing-box credentials
// without touching the handlers.
package api

import (
	"context"
	_ "embed"
	"net/http"
	"strconv"

	"github.com/NetSepio/erebrus/internal/config"
	"github.com/NetSepio/erebrus/internal/drop"
	"github.com/NetSepio/erebrus/internal/readiness"
	"github.com/NetSepio/erebrus/internal/wallet"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// indexHTML is the local dashboard served at "/".
//
//go:embed web/index.html
var indexHTML []byte

// Provisioner turns a peer request into a stored peer and a credential bundle.
// Implemented by node.Service; abstracted so handlers stay transport-only.
type Provisioner interface {
	UpsertPeer(ctx context.Context, id string, req PeerRequest) (*CredentialBundle, error)
	DeletePeer(ctx context.Context, id string) error
	Credentials(ctx context.Context, id string) (*CredentialBundle, error)
	ListPeers(ctx context.Context) ([]PeerInfo, error)
	Stats(ctx context.Context) (*NodeStats, error)
}

// Identity supplies the node's stable identifiers for the status endpoint.
type Identity struct {
	PeerID string
	DID    string
}

// Server wires the Gin engine.
type Server struct {
	cfg  *config.Config
	prov Provisioner
	id   Identity
	// status reflects drain state ("online" | "draining"); Phase 2 toggles it.
	status             string
	readinessFn        func() readiness.Input
	wireGuardPublicKey func() string
	serviceSnapshotFn  func() map[string]string
	drop               *drop.Service
}

// NewServer builds the API server.
func NewServer(cfg *config.Config, prov Provisioner, id Identity) *Server {
	return &Server{cfg: cfg, prov: prov, id: id, status: "online"}
}

// SetReadinessProvider supplies live signals for readiness evaluation.
func (s *Server) SetReadinessProvider(fn func() readiness.Input) {
	s.readinessFn = fn
}

// SetWireGuardPublicKeyProvider supplies the node's WireGuard server public key.
func (s *Server) SetWireGuardPublicKeyProvider(fn func() string) {
	s.wireGuardPublicKey = fn
}

// SetServiceSnapshot supplies attached service health for /api/v2/status.
func (s *Server) SetServiceSnapshot(fn func() map[string]string) {
	s.serviceSnapshotFn = fn
}

// SetDropService enables the optional exact-purpose Drop API.
func (s *Server) SetDropService(service *drop.Service) {
	s.drop = service
}

// SetStatus updates the public status field (online | draining).
func (s *Server) SetStatus(status string) {
	if status == "" {
		status = "online"
	}
	s.status = status
}

// Router returns the configured Gin engine.
func (s *Server) Router() *gin.Engine {
	if s.cfg.RunType == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Recovery())

	// Local dashboard (intro, docs, live stats).
	r.GET("/", func(c *gin.Context) { c.Data(http.StatusOK, "text/html; charset=utf-8", indexHTML) })

	r.GET("/metrics", gin.WrapH(promhttp.Handler()))
	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	v2 := r.Group("/api/v2")
	v2.GET("/status", s.handleStatus)
	v2.GET("/stats", s.handleStats) // coarse public aggregates for the dashboard

	authed := v2.Group("")
	authed.Use(s.gatewayAuth())
	{
		authed.GET("/peers", s.handleListPeers)
		authed.PUT("/peers/:id", s.handlePutPeer)
		authed.DELETE("/peers/:id", s.handleDeletePeer)
		authed.GET("/peers/:id/credentials", s.handleCredentials)
	}
	if s.drop != nil {
		dropAPI := v2.Group("/drop")
		dropAPI.GET("/status", s.gatewayAuthForPurpose("drop_status"), s.handleDropStatus)
		dropAPI.PUT("/uploads/:upload_id", s.gatewayAuthForPurpose("drop_upload"), s.handleDropUpload)
		dropAPI.GET("/objects/:cid", s.gatewayAuthForPurpose("drop_read"), s.handleDropRead)
		dropAPI.GET("/pins/:cid", s.gatewayAuthForPurpose("drop_pin_check"), s.handleDropPinStatus)
		dropAPI.DELETE("/pins/:cid", s.gatewayAuthForPurpose("drop_unpin"), s.handleDropUnpin)
		dropAPI.Any("/webui", s.gatewayAuthForPurpose("drop_webui"), s.handleDropWebUI)
		dropAPI.Any("/webui/*path", s.gatewayAuthForPurpose("drop_webui"), s.handleDropWebUI)
	}
	return r
}

func (s *Server) handleStatus(c *gin.Context) {
	protocols := []string{"wireguard"}
	if s.cfg.EnableStealth {
		protocols = append(protocols, "vless-reality", "hysteria2")
	}
	in := readiness.Input{Cfg: s.cfg, IdentityConfigured: s.id.PeerID != ""}
	if s.readinessFn != nil {
		in = s.readinessFn()
		in.Cfg = s.cfg
		if in.IdentityConfigured == false && s.id.PeerID != "" {
			in.IdentityConfigured = true
		}
	}
	rep := readiness.Evaluate(in)
	chain := wallet.CanonicalChain(s.cfg.WalletChain)
	idStatus := IdentityStatus{
		Configured:  in.IdentityConfigured && s.cfg.Mnemonic != "",
		PeerID:      s.id.PeerID,
		DID:         s.id.DID,
		WalletChain: chain,
		WalletLabel: wallet.ChainLabel(chain),
	}
	if s.cfg.Mnemonic != "" {
		if addr, err := wallet.AddressFromMnemonic(s.cfg.Mnemonic, chain); err == nil {
			idStatus.WalletAddress = addr
		}
	}
	wgPort, _ := strconv.Atoi(s.cfg.WGEndpointPort)
	if wgPort == 0 {
		wgPort = 51820
	}
	wgPub := ""
	if s.wireGuardPublicKey != nil {
		wgPub = s.wireGuardPublicKey()
	}
	wgHost := s.cfg.WGEndpointHost
	c.JSON(http.StatusOK, StatusResponse{
		Version:    s.cfg.Version,
		NodeName:   s.cfg.NodeName,
		Region:     s.cfg.Region,
		Zone:       s.cfg.Zone,
		Status:     s.status,
		AccessMode: string(s.cfg.Mode.RuntimeMode),
		PeerID:     s.id.PeerID,
		DID:        s.id.DID,
		Identity:   idStatus,
		Endpoints: EndpointsStatus{
			WireGuard: WireGuardEndpointStatus{
				Port:      wgPort,
				PublicKey: wgPub,
				Endpoint:  wgHost + ":" + strconv.Itoa(wgPort),
			},
		},
		Capabilities: map[string]any{
			"access_mode":        s.cfg.Mode.RuntimeMode,
			"access_label":       readiness.AccessModeLabel(s.cfg.Mode.RuntimeMode),
			"access_hint":        readiness.AccessModeHint(s.cfg.Mode.RuntimeMode),
			"region_label":       readiness.RegionLabel(s.cfg.Region),
			"zone_label":         readiness.ZoneLabel(s.cfg.Zone),
			"network_profile":    s.cfg.Mode.NetworkProfile,
			"deployment_profile": s.cfg.ErebrusProfile,
			"firewall_provider":  s.cfg.FirewallProvider,
			"app_hosting":        s.cfg.EnableAppHosting,
			"wildcard_domain":    s.cfg.AppWildcardDomain,
			"public_domain":      s.cfg.PublicDomain,
			"stealth":            s.cfg.EnableStealth,
			"public_api_url":     readiness.PublicAPIURL(s.cfg),
			"services":           s.servicesSnapshot(),
			"drop":               s.publicDropCapability(),
		},
		Protocols: protocols,
		Readiness: rep,
	})
}

func (s *Server) servicesSnapshot() map[string]string {
	services := map[string]string{"vpn": "active"}
	if s.serviceSnapshotFn != nil {
		services = s.serviceSnapshotFn()
	}
	out := make(map[string]string, len(services)+1)
	for name, state := range services {
		out[name] = state
	}
	if s.drop != nil {
		out["drop"] = s.drop.Snapshot().State
	}
	return out
}

func (s *Server) publicDropCapability() map[string]any {
	if s.drop == nil {
		return map[string]any{
			"enabled": false, "accepts_public_uploads": false,
			"webui_available": false,
		}
	}
	out := map[string]any{
		"enabled":                s.drop.Enabled(),
		"accepts_public_uploads": s.drop.AcceptsPublicUploads(),
		"webui_available":        s.drop.WebUIAvailable(),
	}
	if url := s.drop.PublicGatewayURL(); url != "" {
		out["public_gateway_url"] = url
	}
	return out
}

func (s *Server) handleStats(c *gin.Context) {
	stats, err := s.prov.Stats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read stats"})
		return
	}
	c.JSON(http.StatusOK, stats)
}
