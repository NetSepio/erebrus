package transport

import "testing"

func TestDefaultLadderOrder(t *testing.T) {
	if DefaultLadder[0] != KindDirectWG {
		t.Fatalf("first = %s", DefaultLadder[0])
	}
	if DefaultLadder[1] != KindHysteria2 {
		t.Fatalf("second = %s", DefaultLadder[1])
	}
}

func TestScore(t *testing.T) {
	r := ProbeResult{Kind: KindDirectWG, Success: true, LatencyMs: 50, PacketLossPct: 1}
	s := Score(r, true, true)
	// 100 - 5 - 2 + 20 + 10 = 123
	if s != 123 {
		t.Fatalf("score = %d", s)
	}
}

func TestSelectBest(t *testing.T) {
	results := []ProbeResult{
		{Kind: KindDirectWG, Success: true, LatencyMs: 10},
		{Kind: KindHysteria2, Success: true, LatencyMs: 5},
		{Kind: KindVLESSReality, Success: false},
	}
	best, ok := SelectBest(results)
	if !ok || best.Kind != KindHysteria2 {
		t.Fatalf("best = %+v ok=%v", best, ok)
	}
}

func TestImplementedLadder(t *testing.T) {
	if len(ImplementedLadder(false)) != 1 {
		t.Fatal("expected wg only without stealth")
	}
	if len(ImplementedLadder(true)) != 3 {
		t.Fatal("expected 3 with stealth")
	}
}
