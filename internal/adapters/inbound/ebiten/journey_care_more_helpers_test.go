package ebiten

// journey_care_more_helpers_test.go: fixtures for journey_care_test.go:
// captured raiders, visitors, and a feeder that leaves one pawn hungry.

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/difficulty"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/scenario"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/site"
	"rimworld-go/pkg/randx"
)

// careFeedExcept keeps every colonist but skip fed and rested.
func careFeedExcept(j *journey, skip uint64) {
	for _, p := range j.g.Snapshot() {
		if !p.Hostile && uint64(p.ID) != skip {
			j.g.SetNeeds(p.ID, needs.Full, needs.Full)
		}
	}
}

// careCapturedRaider sends one club raider, downs it and has the colonists
// capture it into a prison bed built at (x, y); it returns that bed.
func careCapturedRaider(j *journey, x, y int) entity.EntityID {
	j.t.Helper()
	bed := careBed(j, "sleeping spot", x, y)
	carePrisonBed(j, bed)
	j.g.RaidArmed(item.Club)
	j.tick(2)
	var raider, doer uint64
	for _, p := range j.g.Snapshot() {
		if p.Hostile && raider == 0 {
			raider = uint64(p.ID)
		}
		if !p.Hostile && doer == 0 {
			doer = uint64(p.ID)
		}
	}
	if raider == 0 {
		j.t.Fatal("the raid brought no raider")
	}
	careDown(j, raider)
	j.g.Enqueue(command.Command{Kind: command.Capture, Pawn: doer, Target: raider})
	careRun(j, "Digit3", 8000, func() bool {
		careFeed(j)
		return j.pawn(raider).Prisoner
	})
	return bed
}

// careFirstPrisoner returns the first prisoner in the read model.
func careFirstPrisoner(j *journey) uint64 {
	j.t.Helper()
	for _, p := range j.g.Snapshot() {
		if p.Prisoner {
			return uint64(p.ID)
		}
	}
	j.t.Fatal("no prisoner in the colony")
	return 0
}

// careArrestedOutOfBed has the first colonist arrest the second and runs
// the colony until the arrest took; the arrester is then sent walking, so
// the prisoner is dropped out of any bed and nobody carries them. It
// returns the prisoner's id.
func careArrestedOutOfBed(j *journey) uint64 {
	j.t.Helper()
	ids := ordColonists(j)
	j.g.Enqueue(command.Command{Kind: command.Arrest, Pawn: ids[0], Target: ids[1]})
	careRun(j, "Digit2", 6000, func() bool {
		careFeed(j)
		return j.pawn(ids[1]).Prisoner
	})
	j.g.Enqueue(command.Command{Kind: command.MovePawn, Pawn: ids[0], X: 3, Y: 12})
	j.g.ApplyPending()
	j.tick(2)
	if p := j.pawn(ids[1]); p.InBed || p.CarriedBy != 0 {
		j.t.Fatalf("fixture: the arrested colonist is in a bed (%v) or carried (%d)", p.InBed, p.CarriedBy)
	}
	return ids[1]
}

// careRightClickSettled is careRightClick with the camera given a frame to
// settle on the tile first.
func careRightClickSettled(j *journey, x, y int) []string {
	j.t.Helper()
	j.lookAt(x, y)
	j.update(1)
	return careRightClick(j, x, y)
}

// careVisitorJourney is a landed set-up colony (broken_hull, 60x60) whose
// visitor group has arrived; it returns the journey and one visitor's id.
func careVisitorJourney(t *testing.T) (*journey, uint64) {
	t.Helper()
	d, ok := scenario.Builtin("broken_hull")
	if !ok {
		t.Fatal("no built-in scenario broken_hull")
	}
	s := &game.Setup{
		Scenario: d, Storyteller: "steady", Preset: difficulty.Seasoned, Difficulty: difficulty.Of(difficulty.Seasoned),
		Site: site.Site{Biome: biome.TemperateForest, Climate: biome.DefOf(biome.TemperateForest).Climate,
			Stones: []geology.Stone{geology.Granite, geology.Marble}},
		Factions: faction.Generate(faction.DefaultSlots(), randx.New(7)),
		MapSize:  60, Season: game.SeasonSpring, Seed: 4242,
	}
	s.Colonists = game.NewCandidatePool(s.Seed, s.Scenario.Apply()).Selected()
	b := game.NewColonyBuilder(game.Config{Setup: s})
	var g *game.Game
	for i := 0; i < 200 && g == nil; i++ {
		if _, _, done := b.Step(1); done {
			g = b.Game()
		}
	}
	if g == nil {
		t.Fatal("the colony never finished building")
	}
	for i := 0; i < 20; i++ {
		g.Step(60)
	}
	j := newJourney(t, jGame(g))
	if j.a.uiSt().ModalOpen() {
		j.key("Escape", 0)
	}
	if !g.FireIncident("visitor_group") {
		t.Fatal("the visitor group did not arrive (no neutral or friendly people?)")
	}
	j.update(1)
	for _, p := range g.Snapshot() {
		if p.Visitor && !p.Hostile {
			return j, uint64(p.ID)
		}
	}
	t.Fatal("no visitor in the read model")
	return nil, 0
}

// careDownGently injures id in small steps until it is downed, so a
// frail pawn (a visitor) is not killed on the way.
func careDownGently(j *journey, id uint64) {
	j.t.Helper()
	for i := 0; i < 200 && j.pawn(id).Status != "downed"; i++ {
		j.g.Injure(entity.EntityID(id), 3)
	}
	if st := j.pawn(id).Status; st != "downed" {
		j.t.Fatalf("could not down pawn %d (status %q)", id, st)
	}
}

// careSelect selects colonist id by clicking its tile with the camera on it.
func careSelect(j *journey, id uint64) {
	j.t.Helper()
	p := j.pawn(id)
	j.lookAt(p.X, p.Y)
	j.update(1)
	j.clickTile(p.X, p.Y, inputframe.Left, 0)
	j.draw(2)
	if got := uint64(j.a.renderer.Selected()); got != id {
		j.t.Fatalf("clicking pawn %d at (%d, %d) selected %d", id, p.X, p.Y, got)
	}
}

// careFirstColonist returns the first colonist (not a visitor, not hostile).
func careFirstColonist(j *journey) (uint64, game.PawnView) {
	j.t.Helper()
	for _, p := range j.g.Snapshot() {
		if !p.Hostile && !p.Visitor && !p.Prisoner {
			return uint64(p.ID), p
		}
	}
	j.t.Fatal("no colonist")
	return 0, game.PawnView{}
}
