// Package api serves the node's HTTP REST surface (Gin) under /api/v2. It
// replaces the v1 api/v1 tree and the deleted gRPC server. Provisioning logic
// lives in the Provisioner so Phase 2 can extend it with sing-box credentials
// without touching the handlers.
package api

import (
	"context"
	_ "embed"
	"net/http"

	"github.com/NetSepio/erebrus/internal/config"
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
	status string
}

// NewServer builds the API server.
func NewServer(cfg *config.Config, prov Provisioner, id Identity) *Server {
	return &Server{cfg: cfg, prov: prov, id: id, status: "online"}
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
	authed.Use(s.bearerAuth())
	{
		authed.GET("/peers", s.handleListPeers)
		authed.PUT("/peers/:id", s.handlePutPeer)
		authed.DELETE("/peers/:id", s.handleDeletePeer)
		authed.GET("/peers/:id/credentials", s.handleCredentials)
	}
	return r
}

func (s *Server) handleStatus(c *gin.Context) {
	protocols := []string{"wireguard"}
	if s.cfg.EnableStealth {
		protocols = append(protocols, "vless-reality", "hysteria2")
	}
	c.JSON(http.StatusOK, StatusResponse{
		Version: s.cfg.Version,
		Region:  s.cfg.Region,
		Status:  s.status,
		PeerID:  s.id.PeerID,
		DID:     s.id.DID,
		Capabilities: map[string]any{
			"runtime_mode":    s.cfg.Mode.RuntimeMode,
			"network_profile": s.cfg.Mode.NetworkProfile,
			"app_hosting":     s.cfg.EnableAppHosting,
			"wildcard_domain": s.cfg.AppWildcardDomain,
			"public_domain":   s.cfg.PublicDomain,
			"stealth":         s.cfg.EnableStealth,
		},
		Protocols: protocols,
	})
}

func (s *Server) handleStats(c *gin.Context) {
	stats, err := s.prov.Stats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read stats"})
		return
	}
	c.JSON(http.StatusOK, stats)
}
