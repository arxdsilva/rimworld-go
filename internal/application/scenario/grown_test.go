package scenario

import (
	"testing"
	"time"
)

func TestBuildCounts(t *testing.T) {
	g, n, err := Build(1, 120, 120)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("counts %+v", n)
	if n.Colonists != 20 || n.Tame != 50 || n.Stacks != 1000 {
		t.Fatalf("counts %+v", n)
	}
	if n.Buildings < 480 || n.Buildings > 540 { // ~430 in rooms, ~80 fences
		t.Fatalf("buildings = %d, want about 510", n.Buildings)
	}
	g.Step(1) // apply the zone and designation commands
	w := g.World()
	if len(w.Mines) != 200 {
		t.Errorf("mines = %d, want 200", len(w.Mines))
	}
	if len(w.Frames) != 50 {
		t.Errorf("blueprints = %d, want 50", len(w.Frames))
	}
	grow := 0
	for _, z := range w.Zones {
		t.Logf("zone %s cells %d", z.Kind, len(z.Cells))
		if z.Kind == "grow" {
			grow += len(z.Cells)
		}
	}
	if grow != 600 {
		t.Errorf("grow cells = %d, want 600", grow)
	}
}

func TestBuildTooSmall(t *testing.T) {
	if _, _, err := Build(1, 48, 40); err == nil {
		t.Fatal("want an error for a small map")
	}
}

func TestGrownColonyProbe(t *testing.T) {
	for _, n := range []int{250, 500} {
		t0 := time.Now()
		g, _, err := Build(1, n, n)
		if err != nil {
			t.Fatal(err)
		}
		t1 := time.Now()
		g.Step(500)
		t.Logf("%d: build %v, 500 ticks: %v", n, t1.Sub(t0), time.Since(t1))
	}
}

// TestGrownGolden pins the state of the grown colony after Build and
// 5 000 ticks (NG-1 §6.2): it exercises the scale paths (hundreds of
// buildings, stacks, zones, designations, a pen) that the start colony
// of TestScaleGolden never reaches.
func TestGrownGolden(t *testing.T) {
	if testing.Short() {
		t.Skip("long golden run")
	}
	// Re-pinned (ENG-003): StateHash's sparse encoding (encoding only, on
	// f957fc8a with no other change), then CompleteResearch also completing
	// the three new projects (Brewing, Herbal remedies, Prosthetics): after
	// Build and 1 000 and 5 000 ticks every saved field is identical except
	// Research, and the new social fields are empty.
	// Re-pinned (PLAY-010): animals react to every attack, predators charge
	// colonists who come close, unarmed colonists flee sooner, colonists
	// and tame animals walk through each other, and grass regrows from
	// shoots. The moved saved fields are exactly the ones those touch
	// (animals, positions/motion/jobs, hits/wounds/logs, RNG, grass
	// growth); checked field by field against 4b12749d on the scale runs.
	// Re-pinned (NG-1 E4): plants grow per chunk once per climate interval
	// (plant.GrowSpan) and frost kills in the same pass; after Build and
	// 1, 250, 1 000 and 5 000 ticks every saved field is identical to
	// f815d545 except the plants' Growth.
	// Re-pinned (NG-1 S3): colonists now save their bios and skills; every
	// other saved field is identical to the previous pin (checked field by
	// field against 4d68b9aa), so behaviour is unchanged.
	// Re-pinned earlier: CompleteResearch now also completes the NG-1 research
	// projects (more unlocks), and StateHash became format-independent;
	// identical with and without the E1 engine changes.
	// Re-pinned (ENG-003 M1-M10): bodies, social interactions, chat
	// mirroring, smoothed mood, romance, animal bonds and healing change
	// behaviour by design; stable over
	// -count=3. Re-pinned (ENG-003 polish): conversation memories land at
	// the speaker's Social impact (§13.2); stable over -count=3.
	// Re-pinned (MAPGEN-3): CompleteResearch also completes the new Bridges
	// project; with it left unfinished the hash is the previous pin
	// (f6040a57), so behaviour is unchanged.
	// Re-pinned (PLAY-015), behaviour changed by design: the grown set
	// piece builds a pen marker instead of painting a pen zone (one more
	// building, so later entity ids and the RNG stream shift), livestock
	// is held by gates and pens while the tame wolves use allowed areas,
	// and new colonists start with Firefight on. Checked: same animal
	// count, one extra building; the old run's single manhunter message is
	// a roll the shifted stream no longer makes. Stable over -count=3.
	const want uint64 = 0xc52d50891a1a788c
	g, _, err := Build(7, 120, 120)
	if err != nil {
		t.Fatal(err)
	}
	g.Step(5000)
	if got := g.StateHash(); got != want {
		t.Fatalf("state hash = %#x, want %#x", got, want)
	}
}
