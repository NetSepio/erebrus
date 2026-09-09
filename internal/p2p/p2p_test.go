package p2p

import (
	"strings"
	"testing"
)

func TestAdvertisedAddress(t *testing.T) {
	tests := map[string]string{
		"203.0.113.10": "/ip4/203.0.113.10/tcp/9002",
		"2001:db8::1":  "/ip6/2001:db8::1/tcp/9002",
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

func TestAdvertisedAddressesHostnameIncludesDNS6(t *testing.T) {
	got, err := advertisedAddresses("node.example", "9002")
	if err != nil {
		t.Fatal(err)
	}
	joined := ""
	for _, a := range got {
		joined += a.String() + " "
	}
	if !strings.Contains(joined, "/dns4/node.example/tcp/9002") || !strings.Contains(joined, "/dns6/node.example/tcp/9002") {
		t.Fatalf("got %s", joined)
	}
}
