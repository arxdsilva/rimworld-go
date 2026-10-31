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
	// Re-pinned (PLAY-017): one charge now posts a chase warning ("A
	// timber wolf charges Settler 8!"); checked field by field against
	// c7d739ac6, only Messages differs.
	// Re-pinned (PLAY-020): injured colonists now rest in the set piece's
	// beds until healed (bed rest by default); the same value with and
	// without the zone settings work.
	// Re-pinned (PLAY-021 contract): CompleteResearch also completes the
	// five new power projects (Electricity, Batteries, Solar, Wind and
	// Geothermal power); with them left unfinished the hash is the
	// previous pin (7aab2f77), so behaviour is unchanged.
	// Re-pinned (PLAY-021): the set piece's lamps are power devices now, so
	// the colony saves a power state (unpowered lamps, no grid).
	// Re-pinned (MED-01), behaviour changed by design: wounds bleed and
	// blood loss downs and kills, pain shock and capacities decide downed,
	// raiders bleed for real instead of a timer. The previous pin still
	// holds at MED-00 (acd15b55); the new value is stable over -count=2.
	// Re-pinned (MED-02), behaviour changed by design: tend quality, wiki
	// healing rates and per-wound infection racing immunity; stable over
	// -count=2.
	// Re-pinned (MED-03 care policy, MED-05 organs): new organs shift where
	// hits land, three new capacities, the doctor's work runs as ordered
	// steps under care levels; MED-05 alone gives this value on a clean
	// copy; stable over -count=2.
	// Re-pinned (MED-09 animal medicine): hurt animals now get a body that
	// bleeds, heals at the wiki rates and can be infected; with that combat
	// hook off the hash is the previous pin, so MED-07 and MED-08 do not move
	// it (the run never reaches midnight or an overdose); stable over -count=2.
	// Re-pinned (PLAY-024 contract): CompleteResearch also completes the
	// two new projects (Sentry guns, Radio relay); with them left
	// unfinished the hash is the previous pin (925a9872), so behaviour is
	// unchanged; stable over -count=2.
	// Re-pinned (MED-13 animal anatomy): a hurt animal's hits land on its
	// species' parts (an extra medical-stream draw per hit), pain and lost
	// legs slow it. A dump of pawns, animals and messages every 100 ticks
	// matches the previous pin's run (960cb818) through tick 1000, the
	// first wolf wound included; from there the fight around the wounded
	// wolves differs. Stable over -count=2.
	// Re-pinned (PLAY-022: animals amble at half speed when idle): a
	// wandering animal's steps take twice the ticks (errands walk at the
	// old step, fleeing at the flee step, hunting and charging keep the
	// old step), the slow speed class is wider (+12 ticks, was +6), and
	// every animal saves two new fields (Errand, StepGait). With the
	// amble and the slow class set back to the old numbers only those
	// two fields move the hash. Stable over -count=2.
	// Re-pinned (SIM-063…066 pawn care and mental breaks): downed pawns get
	// hungry and doctors feed them, every downed cause but collapse-asleep
	// is rescued to a safer-ranked bed, friends visit the sick, and mental
	// breaks follow the wiki tiers and MTBs with new kinds; the previous
	// pin held through SIM-062 (the committed full suite), the benchmark
	// fixtures' hashes moved with the same change set. Stable over -count=2.
	// Re-pinned (BED/PRD/DEF wave B): rest gain scales with the bed's rest
	// effectiveness (ground and sleeping spots 80 %), shots fly as
	// projectiles with per-weapon warmup, bands and armor, and bills gather
	// ingredients across stacks; the previous pin held through wave A
	// (the committed full suite). Stable over -count=2.
	// Re-pinned (BED/PRD/DEF wave C): crafted and built things roll quality
	// and carry stuff, items wear outdoors and apparel turns ratty, rooms
	// score the vanilla impressiveness with role thoughts on waking, and
	// raiders carry kit weapons by tech (grenades and explosions); the
	// previous pin held through wave B (the committed full suite). Stable
	// over -count=2.
	// Re-pinned (BED/PRD/DEF wave D): new benches and research projects
	// (the sandbox completes every project), work speed from light, room
	// and facilities, the comfort, beauty and outdoors needs with sitting,
	// wiki cover values (walls and doors 75 %), door open times and the
	// door hooks in movement; the previous pin held through wave C (the
	// committed full suite). Stable over -count=2.
	// Re-pinned (BED/PRD/DEF wave E): the recipe catalogue and research
	// projects (the sandbox completes every project), wiki stack limits and
	// the stockpile filter tree in hauling, the raid strategies and drop
	// pods with the storyteller's big-threat face (ship parts, mechanoid
	// raids); the previous pin held through wave D (the committed full
	// suite), and the raid extraction alone kept the benchmark hashes.
	// Stable over -count=2.
	// Re-pinned (HAUL-002): stockpile hauls gather nearby compatible piles
	// (8-tile chain, up to the carry limit and the destination cell's
	// room) and deliver once; the previous pin held through wave E (the
	// committed full suite). Stable over -count=2.
	// Re-pinned (wave 2: RES-1, STO-2, CBT-1, CBT-2): shared partial-stack
	// claims, vanilla stockpile defaults and stack limits, layered armor,
	// bleeding gunshot and stab wounds, overkill, the wiki melee curves,
	// Euclidean ranges, weather and cover-angle accuracy; the previous pin
	// held through FEED-001 and STO-1 (the committed full suite). Stable
	// over -count=2.
	// Re-pinned (wave 3: CBT-3, CBT-4, CBT-5): weapons keep their make,
	// raider armor kits, projectile turret shots, raider cover seeking and
	// threat targeting, threat response, explosion item damage and
	// cook-off; the previous pin held through wave 2 (the committed full
	// suite). Stable over -count=2.
	// Re-pinned (FOOD-0 contract, no behaviour change): the sandbox's
	// CompleteResearch also completes the new Hydroponics project, and new
	// stockpiles allow every storable kind, so their filters gain the seven
	// new food kinds' bits; with both hidden the previous pin comes back
	// exactly. Stable over -count=2.
	// Re-pinned (FOOD-1..3): vanilla nutrition and hunger with malnutrition,
	// wiki crop and tree growth with night rest and the light rule,
	// butchering by body size, per-species animal products, taming food and
	// auto-slaughter; the previous pin held through FOOD-0 (the committed
	// full suite). Stable over -count=2.
	const want uint64 = 0xb944008f031aeff4
	g, _, err := Build(7, 120, 120)
	if err != nil {
		t.Fatal(err)
	}
	g.Step(5000)
	if got := g.StateHash(); got != want {
		t.Fatalf("state hash = %#x, want %#x", got, want)
	}
}
