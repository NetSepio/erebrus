package gatewayauth

import "testing"

func TestValidatePurposeClaims(t *testing.T) {
	claims := &Claims{Role: roleGatewayCall, NodeID: "12D3node", Purpose: "drop_upload"}
	if err := validatePurposeClaims(claims, "12D3node", "drop_upload"); err != nil {
		t.Fatal(err)
	}
	if err := validatePurposeClaims(claims, "12D3node", "drop_read"); err == nil {
		t.Fatal("expected exact purpose mismatch")
	}
	if err := validatePurposeClaims(claims, "12D3other", "drop_upload"); err == nil {
		t.Fatal("expected exact node mismatch")
	}
}

func TestValidatePurposeClaimsAcceptsPeerIDAlias(t *testing.T) {
	claims := &Claims{Role: roleGatewayCall, PeerID: "12D3node", Purpose: "drop_status"}
	if err := validatePurposeClaims(claims, "12D3node", "drop_status"); err != nil {
		t.Fatal(err)
	}
}
