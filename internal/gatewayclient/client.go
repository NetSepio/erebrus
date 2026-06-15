package gatewayclient

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	defaultHeartbeatSec = 30
	usageInterval       = 60 * time.Second
	writeWait           = 10 * time.Second
	pongWait            = 95 * time.Second
	pingPeriod          = 27 * time.Second
)

// SnapshotProvider supplies protocol payloads the gateway expects.
type SnapshotProvider interface {
	BuildHello(nodeID string) Hello
	BuildHeartbeat(status string) Heartbeat
	BuildUsageReport() UsageReport
}

// CommandHandler executes gateway control commands.
type CommandHandler interface {
	HandleCommand(ctx context.Context, cmd Command) CommandResult
}

// Client maintains the node→gateway WebSocket control plane.
type Client struct {
	gatewayURL string
	nodeToken  string
	nodeID     string
	log        *slog.Logger

	snap   SnapshotProvider
	cmds   CommandHandler
	status func() string

	mu           sync.Mutex
	heartbeatSec int
	lastUsage    map[string]peerCounters
	onReconnect  func()
}

type peerCounters struct {
	rx int64
	tx int64
}

// New constructs a gateway WebSocket client.
func New(gatewayURL, nodeID, nodeToken string, snap SnapshotProvider, cmds CommandHandler, status func() string) *Client {
	return &Client{
		gatewayURL:   strings.TrimRight(gatewayURL, "/"),
		nodeID:       nodeID,
		nodeToken:    nodeToken,
		snap:         snap,
		cmds:         cmds,
		status:       status,
		heartbeatSec: defaultHeartbeatSec,
		lastUsage:    map[string]peerCounters{},
		log:          slog.Default(),
	}
}

// SetLogger overrides the default logger.
func (c *Client) SetLogger(log *slog.Logger) {
	if log != nil {
		c.log = log
	}
}

// SetOnReconnect is called after each successful reconnect (e.g. to re-send hello).
func (c *Client) SetOnReconnect(fn func()) { c.onReconnect = fn }

// Run dials the gateway and maintains the connection until ctx is cancelled.
func (c *Client) Run(ctx context.Context) {
	backoff := time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		err := c.session(ctx)
		if ctx.Err() != nil {
			return
		}
		c.log.Warn("gateway disconnected", "err", err)
		jitter := time.Duration(rand.Int63n(int64(backoff / 5)))
		sleep := backoff + jitter - backoff/10
		select {
		case <-ctx.Done():
			return
		case <-time.After(sleep):
		}
		if backoff < 60*time.Second {
			backoff *= 2
		}
	}
}

func (c *Client) session(ctx context.Context) error {
	wsURL := strings.Replace(c.gatewayURL, "https://", "wss://", 1)
	wsURL = strings.Replace(wsURL, "http://", "ws://", 1)
	wsURL += "/api/v2/nodes/ws"

	header := http.Header{}
	header.Set("Authorization", "Bearer "+c.nodeToken)
	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	ws, _, err := dialer.DialContext(ctx, wsURL, header)
	if err != nil {
		return fmt.Errorf("ws dial: %w", err)
	}
	defer ws.Close()

	c.log.Info("gateway connected", "url", wsURL)

	if err := c.sendHello(ws); err != nil {
		return err
	}

	errCh := make(chan error, 2)
	go func() { errCh <- c.readPump(ctx, ws) }()
	go func() { errCh <- c.writePump(ctx, ws) }()

	select {
	case <-ctx.Done():
		_ = ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

func (c *Client) sendHello(ws *websocket.Conn) error {
	hello := c.snap.BuildHello(c.nodeID)
	frame, err := wrap(TypeHello, hello)
	if err != nil {
		return err
	}
	_ = ws.SetWriteDeadline(time.Now().Add(writeWait))
	return ws.WriteMessage(websocket.TextMessage, frame)
}

func (c *Client) readPump(ctx context.Context, ws *websocket.Conn) error {
	ws.SetReadLimit(1 << 20)
	_ = ws.SetReadDeadline(time.Now().Add(pongWait))
	ws.SetPongHandler(func(string) error {
		return ws.SetReadDeadline(time.Now().Add(pongWait))
	})
	for {
		_, raw, err := ws.ReadMessage()
		if err != nil {
			return err
		}
		_ = ws.SetReadDeadline(time.Now().Add(pongWait))
		if err := c.handleFrame(ctx, ws, raw); err != nil {
			c.log.Warn("handle gateway frame", "err", err)
		}
	}
}

func (c *Client) handleFrame(ctx context.Context, ws *websocket.Conn, raw []byte) error {
	var env Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return err
	}
	switch env.Type {
	case TypeHelloAck:
		var ack HelloAck
		if err := json.Unmarshal(env.Data, &ack); err != nil {
			return err
		}
		if ack.HeartbeatIntervalSec > 0 {
			c.mu.Lock()
			c.heartbeatSec = ack.HeartbeatIntervalSec
			c.mu.Unlock()
		}
		if c.onReconnect != nil {
			c.onReconnect()
		}
	case TypeCommand:
		var cmd Command
		if err := json.Unmarshal(env.Data, &cmd); err != nil {
			return err
		}
		res := c.cmds.HandleCommand(ctx, cmd)
		frame, err := wrap(TypeCommandResult, res)
		if err != nil {
			return err
		}
		_ = ws.SetWriteDeadline(time.Now().Add(writeWait))
		return ws.WriteMessage(websocket.TextMessage, frame)
	default:
		c.log.Debug("ignore gateway message", "type", env.Type)
	}
	return nil
}

func (c *Client) writePump(ctx context.Context, ws *websocket.Conn) error {
	c.mu.Lock()
	hbSec := c.heartbeatSec
	c.mu.Unlock()
	if hbSec <= 0 {
		hbSec = defaultHeartbeatSec
	}
	hbTicker := time.NewTicker(time.Duration(hbSec) * time.Second)
	usageTicker := time.NewTicker(usageInterval)
	pingTicker := time.NewTicker(pingPeriod)
	defer hbTicker.Stop()
	defer usageTicker.Stop()
	defer pingTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-hbTicker.C:
			status := "online"
			if c.status != nil {
				status = c.status()
			}
			hb := c.snap.BuildHeartbeat(status)
			frame, err := wrap(TypeHeartbeat, hb)
			if err != nil {
				return err
			}
			_ = ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := ws.WriteMessage(websocket.TextMessage, frame); err != nil {
				return err
			}
		case <-usageTicker.C:
			ur := c.snap.BuildUsageReport()
			frame, err := wrap(TypeUsageReport, ur)
			if err != nil {
				return err
			}
			_ = ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := ws.WriteMessage(websocket.TextMessage, frame); err != nil {
				return err
			}
		case <-pingTicker.C:
			_ = ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				return err
			}
		}
	}
}
