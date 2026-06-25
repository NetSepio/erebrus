// Package gatewayauth verifies gateway-issued PASETO tokens on node private APIs.
package gatewayauth

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/vk-rv/pvx"
)

const roleGatewayCall = "gateway_call"

// Claims is the gateway call token payload.
type Claims struct {
	Role    string `json:"role,omitempty"`
	NodeID  string `json:"node_id,omitempty"`
	PeerID  string `json:"peer_id,omitempty"`
	Purpose string `json:"purpose,omitempty"`
	pvx.RegisteredClaims
}

// VerifyGatewayCall parses a gateway PASETO and checks it is a valid short-lived
// gateway→node call for this node.
func VerifyGatewayCall(token, gatewayPublicKeyHex, expectNodeID string) (*Claims, error) {
	raw, err := hex.DecodeString(strings.TrimPrefix(gatewayPublicKeyHex, "0x"))
	if err != nil {
		return nil, fmt.Errorf("decode gateway public key: %w", err)
	}
	if len(raw) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("gateway public key must be %d bytes", ed25519.PublicKeySize)
	}
	pk := pvx.NewAsymmetricPublicKey(ed25519.PublicKey(raw), pvx.Version4)
	pv4 := pvx.NewPV4Public()

	var c Claims
	if err := pv4.Verify(token, pk).ScanClaims(&c); err != nil {
		return nil, err
	}
	if c.Role != roleGatewayCall {
		return nil, fmt.Errorf("unexpected role %q", c.Role)
	}
	if expectNodeID != "" && c.NodeID != "" && c.NodeID != expectNodeID {
		return nil, fmt.Errorf("node_id mismatch")
	}
	return &c, nil
}