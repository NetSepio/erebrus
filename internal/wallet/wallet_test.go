package wallet

import "testing"

func TestSolanaDerivationStable(t *testing.T) {
	mnemonic := "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"
	addr1, err := AddressFromMnemonic(mnemonic, ChainSOL)
	if err != nil {
		t.Fatal(err)
	}
	addr2, err := AddressFromMnemonic(mnemonic, ChainSOL)
	if err != nil {
		t.Fatal(err)
	}
	if addr1 != addr2 || addr1 == "" {
		t.Fatalf("address = %q", addr1)
	}
}

func TestSignChallengeSolana(t *testing.T) {
	mnemonic := "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"
	msg := "I accept the Erebrus Terms of Service https://erebrus.network/terms. Challenge: test-flow"
	addr, pub, sig, err := SignChallengeWithMnemonic(mnemonic, ChainSOL, msg)
	if err != nil {
		t.Fatal(err)
	}
	if addr == "" || pub == "" || sig == "" {
		t.Fatalf("empty sign output addr=%q pub=%q sig=%q", addr, pub, sig)
	}
}
