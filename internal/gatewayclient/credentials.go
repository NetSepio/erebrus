package gatewayclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// PostFirewallCredentials reports a Shield node's AdGuard admin credential to the
// gateway (node PASETO). The gateway encrypts it at rest and reveals it to org
// paid seats. No-op when unset.
func PostFirewallCredentials(ctx context.Context, gatewayURL, peerID, nodeToken, adminUser, adminPassword, adminURL string) error {
	base := strings.TrimRight(strings.TrimSpace(gatewayURL), "/")
	if base == "" || peerID == "" || strings.TrimSpace(nodeToken) == "" || adminPassword == "" {
		return nil
	}
	body, _ := json.Marshal(map[string]string{
		"admin_user":     adminUser,
		"admin_password": adminPassword,
		"admin_url":      adminURL,
	})
	url := base + "/api/v2/nodes/" + peerID + "/firewall/credentials"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+nodeToken)
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("report firewall credentials: %d %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return nil
}
