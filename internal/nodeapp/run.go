package nodeapp

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/NetSepio/erebrus/internal/api"
	"github.com/NetSepio/erebrus/internal/config"
	dnspkg "github.com/NetSepio/erebrus/internal/dns"
	"github.com/NetSepio/erebrus/internal/edge"
	"github.com/NetSepio/erebrus/internal/gatewayclient"
	"github.com/NetSepio/erebrus/internal/node"
	"github.com/NetSepio/erebrus/internal/p2p"
	"github.com/NetSepio/erebrus/internal/readiness"
	"github.com/NetSepio/erebrus/internal/registrar"
	"github.com/NetSepio/erebrus/internal/serviceagent"
	"github.com/NetSepio/erebrus/internal/services"
	"github.com/NetSepio/erebrus/internal/speedtest"
	"github.com/NetSepio/erebrus/internal/stealth"
	"github.com/NetSepio/erebrus/internal/store"
	"github.com/NetSepio/erebrus/internal/telemetry"
	"github.com/NetSepio/erebrus/internal/transport/probe"
	"github.com/NetSepio/erebrus/internal/wg"
)

// Run starts the VPN node until SIGINT/SIGTERM.
func Run(cfg *config.Config) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if best, ok := probe.Select(ctx, &probe.LocalProber{
		StealthEnabled: cfg.EnableStealth,
		WGPort:         cfg.WGEndpointPortInt(),
		VLESSPort:      cfg.VLESSPortInt(),
		Hysteria2Port:  cfg.Hysteria2PortInt(),
	}, cfg.EnableStealth); ok {
		slog.Info("transport ladder", "preferred", best.Kind, "score", best.Score)
	}

	if err := os.MkdirAll(cfg.StateDir, 0o700); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}

	peerID, did, err := p2p.PeerIDFromMnemonic(cfg.Mnemonic)
	if err != nil {
		return fmt.Errorf("derive identity: %w", err)
	}
	slog.Info("node identity", "peer_id", peerID, "did", did, "node", cfg.NodeName)

	st, err := store.Open(cfg.DBPath())
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer st.Close()

	metrics := telemetry.NewMetrics()

	wgm := wg.New(cfg, st, wg.NewController())
	wgErr := wgm.Init(ctx)
	wgOK := wgErr == nil
	if !wgOK {
		slog.Warn("wireguard interface init incomplete", "err", wgErr)
	}

	stealthMgr := stealth.New(cfg, st)
	stealthOK := false
	if err := stealthMgr.Init(ctx); err != nil {
		slog.Warn("stealth init failed; carriers unavailable", "err", err)
	} else if err := stealthMgr.Start(ctx); err != nil {
		slog.Warn("stealth carriers failed to start", "err", err)
	} else {
		stealthOK = cfg.EnableStealth
		if cfg.EnableStealth {
			slog.Info("stealth carriers listening", "vless_port", cfg.VLESSPort, "hysteria2_port", cfg.Hysteria2Port)
		}
		defer stealthMgr.Close()
	}

	p2pNode, err := p2p.Start(ctx, cfg.Mnemonic, cfg.P2PListenPort, cfg.GatewayPeerMultiaddr)
	if err != nil {
		slog.Warn("libp2p host failed to start", "err", err)
	} else {
		defer p2pNode.Close()
	}

	reg := registrar.New(cfg.ChainRegistration)
	if err := reg.Register(ctx, registrar.NodeIdentity{
		PeerID: peerID, DID: did, IPHash: registrar.HashIP(cfg.WGEndpointHost),
		Region: cfg.Region, Version: cfg.Version,
	}); err != nil {
		slog.Warn("registrar register failed", "err", err)
	}

	svcReg := &services.Registry{St: st}
	tunnelDNS := dnspkg.DefaultListenAddr(cfg.WGIPv4Subnet, cfg.PrivateDNSAddr)

	if cfg.PrivateDNSEnabled {
		dnsCfg := dnspkg.Config{
			Enabled: true, Domain: cfg.PrivateDNSDomain, ListenAddr: tunnelDNS,
			Upstream: cfg.UpstreamDNS, QueryLogs: cfg.DNSQueryLogs,
		}
		if err := dnsCfg.Validate(); err != nil {
			slog.Warn("private DNS disabled", "err", err)
		} else {
			go func() {
				if err := dnspkg.New(dnsCfg, svcReg).Start(ctx); err != nil {
					slog.Warn("private DNS stopped", "err", err)
				}
			}()
		}
	} else if cfg.HasFirewallService() {
		fwd := dnspkg.ForwarderConfig{ListenAddr: tunnelDNS, Upstream: cfg.FirewallDNSAddr}
		go func() {
			if err := dnspkg.NewForwarder(fwd).Start(ctx); err != nil {
				slog.Warn("firewall DNS forwarder stopped", "err", err)
			}
		}()
		slog.Info("firewall DNS forwarder listening", "addr", tunnelDNS, "upstream", cfg.FirewallDNSAddr)
	}

	agent := serviceagent.New(cfg)
	agent.Start(ctx)

	svc := node.New(cfg, st, wgm, stealthMgr, metrics)
	apiServer := api.NewServer(cfg, svc, api.Identity{PeerID: peerID, DID: did})
	apiServer.SetWireGuardPublicKeyProvider(wgm.ServerPublicKey)
	svc.SetAPIStatusHook(apiServer.SetStatus)

	if cfg.Mode.IsPublic() && cfg.PublicGatewayEnabled {
		edgeProxy := &edge.Proxy{Reg: svcReg, St: st, WildcardDomain: cfg.WildcardDomain}
		edgeSrv := &http.Server{
			Addr: ":9081", Handler: edgeProxy.Handler(), ReadHeaderTimeout: 10 * time.Second,
		}
		go func() {
			slog.Info("public edge proxy listening", "addr", edgeSrv.Addr)
			if err := edgeSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				slog.Warn("edge proxy error", "err", err)
			}
		}()
		defer func() {
			shut, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = edgeSrv.Shutdown(shut)
		}()
	}

	var gwClient *gatewayclient.Client
	if cfg.GatewayEnabled() {
		creds, err := gatewayclient.LoadCredentials(ctx, st)
		if err != nil {
			slog.Warn("load gateway credentials failed", "err", err)
			creds = &gatewayclient.Credentials{}
		}
		nodeID := creds.NodeID
		nodeToken := creds.NodeToken
		if nodeID == "" {
			nodeID = cfg.NodeID
		}
		if nodeToken == "" {
			nodeToken = cfg.NodeToken
		}
		if nodeID != "" && nodeID != peerID {
			slog.Warn("stored gateway node_id is not peer_id; will re-register", "stored", nodeID, "peer_id", peerID)
			nodeID, nodeToken = "", ""
		}
		if creds.NodeKey != "" {
			cfg.NodeKey = creds.NodeKey
			cfg.NodeAPIToken = creds.NodeKey
		}
		if creds.GatewayPublicKey != "" {
			cfg.GatewayPublicKey = creds.GatewayPublicKey
		}
		if (nodeID == "" || nodeToken == "") && cfg.GatewayAutoRegister {
			regOut, err := gatewayclient.Register(ctx, gatewayclient.RegistrationInput{
				GatewayURL: cfg.GatewayURL, RegistrationToken: cfg.EffectiveRegistrationToken(),
				WalletChain: cfg.WalletChain, Mnemonic: cfg.Mnemonic, PeerID: peerID, DID: did,
				Name: cfg.NodeName, Region: cfg.Region, Zone: cfg.Zone,
				APIBaseURL: cfg.PublicAPIBaseURL(), NodeKey: cfg.EffectiveNodeKey(),
				AccessMode: cfg.Mode.GatewayAccessMode(),
			})
			if err != nil {
				slog.Warn("gateway registration failed", "err", err)
			} else {
				nodeID, nodeToken = regOut.NodeID, regOut.NodeToken
				cfg.NodeID = nodeID
				cfg.NodeToken = nodeToken
				cfg.NodeKey = regOut.NodeKey
				cfg.NodeAPIToken = regOut.NodeKey
				if regOut.GatewayPublicKey != "" {
					cfg.GatewayPublicKey = regOut.GatewayPublicKey
				}
				if err := gatewayclient.SaveCredentials(ctx, st, &gatewayclient.Credentials{
					NodeID: nodeID, NodeToken: nodeToken, NodeKey: regOut.NodeKey, GatewayPublicKey: cfg.GatewayPublicKey,
				}); err != nil {
					slog.Warn("persist gateway credentials failed", "err", err)
				} else {
					slog.Info("gateway registered", "node_id", nodeID)
				}
			}
		}
		cfg.NodeID = nodeID
		if nodeID != "" && nodeToken != "" {
			speedtestCache := speedtest.NewCache()
			speedtestCache.Start(ctx)
			bridge := node.NewGatewayBridge(svc, peerID, did, nodeID, speedtestCache)
			gwClient = gatewayclient.New(cfg.GatewayURL, nodeID, nodeToken, bridge, bridge, bridge.Status)
			go gwClient.Run(ctx)
		} else {
			slog.Warn("gateway URL set but node credentials missing — WS control plane disabled")
		}
	}

	apiServer.SetReadinessProvider(func() readiness.Input {
		gwReg, gwConn := false, false
		if cfg.GatewayEnabled() {
			if cred, err := gatewayclient.LoadCredentials(ctx, st); err == nil && cred.NodeID != "" && cred.NodeToken != "" {
				gwReg = true
			}
			if gwClient != nil {
				gwConn = gwClient.Connected()
			}
		}
		return readiness.Input{
			Cfg: cfg, IdentityConfigured: true, GatewayRegistered: gwReg, GatewayConnected: gwConn,
			WireGuardOK: wgOK, StealthListening: stealthOK,
		}
	})

	srv := &http.Server{
		Addr: fmt.Sprintf("%s:%s", cfg.BindAddr, cfg.HTTPPort), Handler: apiServer.Router(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		slog.Info("HTTP API listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("http server error", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")
	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutCtx)
}