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

func TestChainLabel(t *testing.T) {
	if ChainLabel(ChainSOL) != "Solana" {
		t.Fatalf("sol label = %q", ChainLabel(ChainSOL))
	}
	if ChainLabel(ChainEthereum) != "Ethereum" {
		t.Fatalf("ethereum label = %q", ChainLabel(ChainEthereum))
	}
	if ChainLabel("") != "Solana" {
		t.Fatalf("empty label = %q", ChainLabel(""))
	}
}

func TestCanonicalChain(t *testing.T) {
	if CanonicalChain("sol") != ChainSolana {
		t.Fatalf("sol = %q", CanonicalChain("sol"))
	}
	if CanonicalChain("evm") != ChainEthereum {
		t.Fatalf("evm = %q", CanonicalChain("evm"))
	}
	if CanonicalChain("SOLANA") != ChainSolana {
		t.Fatalf("SOLANA = %q", CanonicalChain("SOLANA"))
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
