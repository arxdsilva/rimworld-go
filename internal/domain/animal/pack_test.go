package animal

import "testing"

// EAI-005: a pack holds as many animals as its threat points buy: more
// points never mean a smaller pack, a costlier species comes in fewer,
// and every pack has at least one and at most MaxPack.
func TestPackSizeScalesWithPoints(t *testing.T) {
	t.Parallel()
	for _, s := range All() {
		c := ThreatCost(DefOf(s))
		if c < MinThreatCost {
			t.Fatalf("%s costs %d, under the floor %d", s, c, MinThreatCost)
		}
		prev := 0
		for pts := 0; pts <= 5000; pts += 50 {
			n := PackSize(pts, s)
			if n < 1 || n > MaxPack {
				t.Fatalf("%s at %d points: %d animals, want 1..%d", s, pts, n, MaxPack)
			}
			if n < prev {
				t.Fatalf("%s: %d points send %d, fewer than %d for less", s, pts, n, prev)
			}
			prev = n
		}
	}
	if ThreatCost(DefOf(Elephant)) <= ThreatCost(DefOf(TimberWolf)) || ThreatCost(DefOf(TimberWolf)) <= ThreatCost(DefOf(Squirrel)) {
		t.Fatal("a bigger, harder-hitting animal costs more")
	}
	if PackSize(400, Squirrel) <= PackSize(400, TimberWolf) || PackSize(400, TimberWolf) <= PackSize(400, Elephant) {
		t.Fatal("the same points buy more small animals than big ones")
	}
	if PackSize(800, TimberWolf) <= PackSize(300, TimberWolf) {
		t.Fatal("more points send a bigger pack")
	}
}
