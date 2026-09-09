package p2p

import "testing"

func TestAdvertisedAddress(t *testing.T) {
	tests := map[string]string{
		"203.0.113.10": "/ip4/203.0.113.10/tcp/9002",
		"node.example": "/dns4/node.example/tcp/9002",
	}
	for host, want := range tests {
		got, err := advertisedAddress(host, "9002")
		if err != nil {
			t.Fatalf("%s: %v", host, err)
		}
		if got.String() != want {
			t.Fatalf("%s: got %s, want %s", host, got, want)
		}
	}
}
