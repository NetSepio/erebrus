package node

import (
	"testing"
	"time"

	"github.com/NetSepio/erebrus/internal/api"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func TestValidatePeerInput(t *testing.T) {
	key, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	valid := api.PeerRequest{Name: "device", WGPublicKey: key.PublicKey().String()}
	if err := validatePeerInput("peer-1", valid); err != nil {
		t.Fatalf("valid input rejected: %v", err)
	}

	tests := []struct {
		name string
		id   string
		req  api.PeerRequest
	}{
		{name: "config injection", id: "peer-1", req: api.PeerRequest{Name: "device\nPostUp = bad", WGPublicKey: key.PublicKey().String()}},
		{name: "bad public key", id: "peer-1", req: api.PeerRequest{Name: "device", WGPublicKey: "bad"}},
		{name: "expired", id: "peer-1", req: api.PeerRequest{Name: "device", WGPublicKey: key.PublicKey().String(), ExpiresAt: time.Now().Add(-time.Minute).Unix()}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := validatePeerInput(test.id, test.req); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
