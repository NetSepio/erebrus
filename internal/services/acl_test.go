package services

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/NetSepio/erebrus/internal/store"
)

func TestACLVpnPeer(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "acl.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	reg := &Registry{St: st}
	acl := &ACLChecker{St: st}
	ctx := context.Background()

	svc, err := reg.Publish(ctx, Service{Name: "api", Port: 8080, OwnerPeerID: "peer-owner", AuthMode: "vpn-peer"})
	if err != nil {
		t.Fatal(err)
	}
	ok, err := acl.AllowConnect(ctx, svc, "peer-owner")
	if err != nil || !ok {
		t.Fatalf("owner should connect: ok=%v err=%v", ok, err)
	}
	ok, _ = acl.AllowConnect(ctx, svc, "peer-other")
	if ok {
		t.Fatal("other peer should not connect to private service")
	}
	_ = acl.Grant(ctx, svc.ID, "peer:peer-other", ActionConnect)
	ok, _ = acl.AllowConnect(ctx, svc, "peer-other")
	if !ok {
		t.Fatal("granted peer should connect")
	}
}