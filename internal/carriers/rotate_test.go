package carriers

import (
	"testing"
)

func TestHashSecretStable(t *testing.T) {
	a := hashSecret("test-secret")
	b := hashSecret("test-secret")
	if a != b || len(a) != 64 {
		t.Fatalf("hash = %q", a)
	}
}