package gatewayclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// PostRESTHeartbeat mirrors the WS heartbeat on the optional REST path
// POST /api/v2/nodes/{peer_id}/heartbeat (node PASETO).
func PostRESTHeartbeat(ctx context.Context, gatewayURL, peerID, nodeToken string, hb Heartbeat) error {
	base := strings.TrimRight(strings.TrimSpace(gatewayURL), "/")
	peerID = strings.TrimSpace(peerID)
	if base == "" || peerID == "" || strings.TrimSpace(nodeToken) == "" {
		return fmt.Errorf("gateway URL, peer_id, and node token are required")
	}
	load, _ := json.Marshal(hb.Load)
	speedtest, _ := json.Marshal(hb.Speedtest)
	version := ""
	if hb.Versions != nil {
		version = hb.Versions["node"]
	}
	body, err := json.Marshal(map[string]any{
		"status":    hb.Status,
		"load":      json.RawMessage(load),
		"speedtest": json.RawMessage(speedtest),
		"rx_bytes":  hb.Load.RxBytes,
		"tx_bytes":  hb.Load.TxBytes,
		"version":   version,
	})
	if err != nil {
		return err
	}
	url := base + "/api/v2/nodes/" + peerID + "/heartbeat"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+nodeToken)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("rest heartbeat: status %d", resp.StatusCode)
	}
	return nil
}
