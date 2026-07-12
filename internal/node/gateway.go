package node

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"sync"
	"time"

	droppkg "github.com/NetSepio/erebrus/internal/drop"
	"github.com/NetSepio/erebrus/internal/firewall"
	"github.com/NetSepio/erebrus/internal/gatewayclient"
	"github.com/NetSepio/erebrus/internal/registrar"
	"github.com/NetSepio/erebrus/internal/serviceagent"
	"github.com/NetSepio/erebrus/internal/speedtest"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

// GatewayBridge implements gatewayclient.SnapshotProvider and CommandHandler.
type GatewayBridge struct {
	svc       *Service
	peerID    string
	did       string
	nodeID    string
	speedtest *speedtest.Cache
	agent     *serviceagent.Agent
	fw        *firewall.Client
	drop      *droppkg.Service

	mu     sync.RWMutex
	status string

	lastUsage map[string]usageCounters
}

type usageCounters struct {
	rx int64
	tx int64
}

// NewGatewayBridge wires the node service to the gateway control plane.
func NewGatewayBridge(svc *Service, peerID, did, nodeID string, speedtestCache *speedtest.Cache, agent *serviceagent.Agent, fw *firewall.Client, dropService *droppkg.Service) *GatewayBridge {
	return &GatewayBridge{
		svc:       svc,
		peerID:    peerID,
		did:       did,
		nodeID:    nodeID,
		speedtest: speedtestCache,
		agent:     agent,
		fw:        fw,
		drop:      dropService,
		status:    "online",
		lastUsage: map[string]usageCounters{},
	}
}

// Status returns the node's operational status for heartbeats.
func (g *GatewayBridge) Status() string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.status
}

// SetStatus sets online/draining and updates the public API status mirror.
func (g *GatewayBridge) SetStatus(status string) {
	g.mu.Lock()
	g.status = status
	g.mu.Unlock()
	if g.svc.apiStatus != nil {
		g.svc.apiStatus(status)
	}
}

func (g *GatewayBridge) BuildHello(_ string) gatewayclient.Hello {
	cfg := g.svc.cfg
	eps := gatewayclient.Endpoints{
		WireGuard: gatewayclient.WireGuardEndpoint{
			Host:      cfg.WGEndpointHost,
			Port:      cfg.WGEndpointPortInt(),
			PublicKey: g.svc.wg.ServerPublicKey(),
		},
	}
	if g.svc.stealth != nil && g.svc.stealth.Enabled() {
		p := g.svc.stealth.Params()
		obfs := ""
		if cfg.Hysteria2ObfsPassword != "" {
			obfs = "salamander"
		}
		eps.VLESSReality = gatewayclient.VLESSEndpoint{
			Port:      cfg.VLESSPortInt(),
			PublicKey: p.RealityPublicKey,
			ShortIDs:  []string{p.RealityShortID},
			SNI:       p.SNI,
		}
		eps.Hysteria2 = gatewayclient.Hysteria2Endpoint{
			Port: cfg.Hysteria2PortInt(),
			Obfs: obfs,
		}
	}
	return gatewayclient.Hello{
		NodeID:  g.peerID,
		Version: cfg.Version,
		Identity: gatewayclient.Identity{
			PeerID: g.peerID,
			DID:    g.did,
			IPHash: registrar.HashIP(cfg.WGEndpointHost),
		},
		Spec: gatewayclient.Spec{
			CPU:    fmt.Sprintf("%d CPU", runtime.NumCPU()),
			MemMB:  hostMemMB(),
			Region: cfg.Region,
			Zone:   cfg.Zone,
			IP:     cfg.WGEndpointHost,
		},
		Capabilities: gatewayclient.Capabilities{
			AccessMode:     cfg.Mode.GatewayAccessMode(),
			AppHosting:     cfg.EnableAppHosting,
			WildcardDomain: cfg.AppWildcardDomain,
			Drop:           g.dropCapability(),
		},
		Endpoints:         eps,
		DeploymentProfile: cfg.ErebrusProfile,
		Services:          g.serviceSnapshot(),
	}
}

func (g *GatewayBridge) BuildHeartbeat(status string) gatewayclient.Heartbeat {
	live := g.svc.wg.Stats()
	peers, _ := g.svc.st.ListPeers(context.Background())
	versions := map[string]string{
		"node":    g.svc.cfg.Version,
		"singbox": "1.11.15",
	}
	var dropStatus *gatewayclient.DropStatus
	if g.drop != nil {
		snapshot := g.drop.Snapshot()
		dropStatus = &gatewayclient.DropStatus{
			State: snapshot.State, KuboVersion: snapshot.KuboVersion,
			RepoSizeBytes: snapshot.RepoSizeBytes, StorageMaxBytes: snapshot.StorageMaxBytes,
			NumObjects: snapshot.NumObjects,
		}
		if snapshot.KuboVersion != "" {
			versions["kubo"] = snapshot.KuboVersion
		}
	}
	return gatewayclient.Heartbeat{
		TS:     time.Now().Unix(),
		Status: status,
		Load: gatewayclient.Load{
			WGPeersRegistered: len(peers),
			WGPeersConnected:  live.Connected,
			ProxySessions:     0,
			CPUPct:            g.cpuUsedPct(),
			MemPct:            memUsedPct(),
			RxBytes:           live.RxBytes,
			TxBytes:           live.TxBytes,
		},
		Speedtest: g.cachedSpeedtest(),
		Versions:  versions,
		Services:  g.serviceSnapshot(),
		Drop:      dropStatus,
	}
}

func (g *GatewayBridge) serviceSnapshot() map[string]string {
	services := map[string]string{"vpn": "active"}
	if g.agent == nil {
		if g.drop != nil {
			services["drop"] = g.drop.Snapshot().State
		}
		return services
	}
	for name, state := range g.agent.Snapshot() {
		services[name] = state
	}
	if g.drop != nil {
		services["drop"] = g.drop.Snapshot().State
	}
	return services
}

func (g *GatewayBridge) dropCapability() *gatewayclient.DropCapability {
	if g.drop == nil {
		return nil
	}
	return &gatewayclient.DropCapability{
		Enabled:              g.drop.Enabled(),
		AcceptsPublicUploads: g.drop.AcceptsPublicUploads(),
		PublicGatewayURL:     g.drop.PublicGatewayURL(),
		WebUIAvailable:       g.drop.WebUIAvailable(),
	}
}

func (g *GatewayBridge) cachedSpeedtest() gatewayclient.Speedtest {
	if g.speedtest == nil {
		return gatewayclient.Speedtest{}
	}
	return g.speedtest.Get()
}

func (g *GatewayBridge) BuildUsageReport() gatewayclient.UsageReport {
	ctx := context.Background()
	peers, err := g.svc.st.ListPeers(ctx)
	if err != nil {
		return gatewayclient.UsageReport{TS: time.Now().Unix()}
	}
	byKey := map[string]string{}
	for _, p := range peers {
		byKey[p.WGPublicKey] = p.ID
	}
	transfers := g.svc.wg.PeerTransfers()
	out := make([]gatewayclient.PeerUsage, 0)
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, tr := range transfers {
		peerID, ok := byKey[tr.WGPublicKey]
		if !ok {
			continue
		}
		prev := g.lastUsage[peerID]
		dRx := tr.RxBytes - prev.rx
		dTx := tr.TxBytes - prev.tx
		if dRx < 0 {
			dRx = tr.RxBytes
		}
		if dTx < 0 {
			dTx = tr.TxBytes
		}
		g.lastUsage[peerID] = usageCounters{rx: tr.RxBytes, tx: tr.TxBytes}
		if dRx == 0 && dTx == 0 {
			continue
		}
		out = append(out, gatewayclient.PeerUsage{
			PeerID:        peerID,
			RxBytesDelta:  dRx,
			TxBytesDelta:  dTx,
			LastHandshake: tr.LastHandshake,
		})
	}
	return gatewayclient.UsageReport{TS: time.Now().Unix(), Peers: out}
}

func (g *GatewayBridge) HandleCommand(ctx context.Context, cmd gatewayclient.Command) gatewayclient.CommandResult {
	res := gatewayclient.CommandResult{RequestID: cmd.RequestID, OK: true}
	switch cmd.Action {
	case gatewayclient.ActionDrain:
		g.SetStatus("draining")
	case gatewayclient.ActionUndrain:
		g.SetStatus("online")
	case gatewayclient.ActionRotateReality:
		if g.svc.stealth == nil {
			res.OK = false
			res.Error = "stealth not enabled"
			return res
		}
		if _, err := g.svc.stealth.RotateReality(ctx); err != nil {
			res.OK = false
			res.Error = err.Error()
		}
	case gatewayclient.ActionResyncPeers:
		var args struct {
			PeerIDs []string `json:"peer_ids"`
		}
		if err := json.Unmarshal(cmd.Args, &args); err != nil {
			res.OK = false
			res.Error = "invalid args"
			return res
		}
		missing, err := g.svc.ResyncPeers(ctx, args.PeerIDs)
		if err != nil {
			res.OK = false
			res.Error = err.Error()
			return res
		}
		if len(missing) > 0 {
			b, _ := json.Marshal(map[string]any{"missing_on_node": missing})
			res.Error = string(b)
		}
	case gatewayclient.ActionSyncApps:
		// Phase 5 — acknowledge without effect in v2.0.
	case gatewayclient.ActionSyncFirewall:
		if g.fw == nil {
			res.OK = false
			res.Error = "firewall not configured"
			return res
		}
		if err := g.fw.Sync(ctx, cmd.Args); err != nil {
			res.OK = false
			res.Error = err.Error()
		}
	case gatewayclient.ActionRestartFirewall:
		if g.fw == nil {
			res.OK = false
			res.Error = "firewall not configured"
			return res
		}
		if err := g.fw.Restart(ctx); err != nil {
			res.OK = false
			res.Error = err.Error()
		}
	case gatewayclient.ActionResetFirewallCredentials:
		if g.fw == nil {
			res.OK = false
			res.Error = "firewall not configured"
			return res
		}
		if err := g.fw.ResetCredentials(ctx); err != nil {
			res.OK = false
			res.Error = err.Error()
		}
	case gatewayclient.ActionSetFirewallCredentials:
		if g.fw == nil {
			res.OK = false
			res.Error = "firewall not configured"
			return res
		}
		var args struct {
			AdminUser     string `json:"admin_user"`
			AdminPassword string `json:"admin_password"`
		}
		if err := json.Unmarshal(cmd.Args, &args); err != nil || args.AdminPassword == "" {
			res.OK = false
			res.Error = "invalid args"
			return res
		}
		if err := g.fw.SetAdminPassword(ctx, args.AdminUser, args.AdminPassword); err != nil {
			res.OK = false
			res.Error = err.Error()
		}
	default:
		res.OK = false
		res.Error = "unknown action"
	}
	return res
}

func hostMemMB() int {
	vm, err := mem.VirtualMemory()
	if err != nil {
		return 0
	}
	return int(vm.Total / (1024 * 1024))
}

func memUsedPct() float64 {
	vm, err := mem.VirtualMemory()
	if err != nil {
		return 0
	}
	return vm.UsedPercent
}

func (g *GatewayBridge) cpuUsedPct() float64 {
	pct, err := cpu.PercentWithContext(context.Background(), 0, false)
	if err != nil || len(pct) == 0 {
		return 0
	}
	return pct[0]
}
