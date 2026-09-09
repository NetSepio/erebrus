package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestAllocateIPSkipsReservedAndDotBroadcast(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "alloc.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	p, err := st.UpsertPeer(ctx, &Peer{
		ID: "a", Name: "a", WGPublicKey: "k1", Enabled: true,
	}, "100.64.0.1/24", GeneratedCreds{ProxyUUID: "u1"})
	if err != nil {
		t.Fatal(err)
	}
	if p.WGAllowedIP != "100.64.0.2/32" {
		t.Fatalf("first alloc = %s", p.WGAllowedIP)
	}
}
