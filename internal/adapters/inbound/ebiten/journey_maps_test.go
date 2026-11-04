package ebiten

import (
	"fmt"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/hud"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/platform/input"
)

// journey_maps_test.go: several simultaneous maps (MAP-1). The fixture is a
// colony with a second, blank map from the dev hook; everything after is keys
// and clicks through the real input path.
var _ = owe("journey_maps_test.go",
	[]string{},
	[]string{})

// mapsJourney is a paused colony of one colonist on map 0 and one on a
// second map (the dev hook's blank site map). away is the second map's id and
// its colonist.
func mapsJourney(t *testing.T) (j *journey, away game.MapID, awayPawn entity.EntityID) {
	t.Helper()
	g := game.New(game.Config{Seed: 1, Width: 24, Height: 24})
	if _, err := g.SpawnPawn(8, 4); err != nil {
		t.Fatal(err)
	}
	away, ok := game.DevOpenMap(g, 5)
	if !ok {
		t.Fatal("the dev hook opened no second map")
	}
	g.ViewMap(away)
	var err error
	if awayPawn, err = g.SpawnPawn(30, 25); err != nil {
		t.Fatal(err)
	}
	g.ViewMap(0)
	return newJourney(t, jGame(g)), away, awayPawn
}

func homePawn(j *journey) entity.EntityID {
	j.t.Helper()
	for _, p := range j.g.Snapshot() {
		return p.ID
	}
	j.t.Fatal("no colonist on the viewed map")
	return 0
}

// TestJourneyMapsBracketKeysCycleAndWrap: ] shows the next map and [ the
// previous one, both wrapping; the switch drops the selection.
func TestJourneyMapsBracketKeysCycleAndWrap(t *testing.T) {
	t.Parallel()
	j, away, _ := mapsJourney(t)
	home := homePawn(j)
	if !j.clickID(fmt.Sprintf("bar.pawn.%d", home)) || !j.a.renderer.HasSelect() {
		t.Fatal("the home colonist was not selected from the bar")
	}
	j.key("BracketRight", 0)
	if got := j.g.ViewedMap(); got != away {
		t.Fatalf("] showed map %d, want %d", got, away)
	}
	if j.a.renderer.HasSelect() {
		t.Fatal("the selection survived a map switch")
	}
	j.key("BracketRight", 0)
	if got := j.g.ViewedMap(); got != 0 {
		t.Fatalf("] on the last map showed %d, want a wrap to 0", got)
	}
	j.key("BracketLeft", 0)
	if got := j.g.ViewedMap(); got != away {
		t.Fatalf("[ on the first map showed %d, want a wrap to %d", got, away)
	}
	j.key("BracketLeft", 0)
	if got := j.g.ViewedMap(); got != 0 {
		t.Fatalf("[ showed %d, want 0", got)
	}
}

// TestJourneyMapsFrameClickSwitches: a map group's frame on the colonist
// bar switches to that map.
func TestJourneyMapsFrameClickSwitches(t *testing.T) {
	t.Parallel()
	j, away, _ := mapsJourney(t)
	if !j.clickID(hud.BarMapID(away)) {
		t.Fatal("the frame is disabled")
	}
	if got := j.g.ViewedMap(); got != away {
		t.Fatalf("the frame showed map %d, want %d", got, away)
	}
	if !j.clickID(hud.BarMapID(0)) || j.g.ViewedMap() != 0 {
		t.Fatalf("the home frame showed map %d, want 0", j.g.ViewedMap())
	}
}

// TestJourneyMapsBarColonistOfAnotherMapJumps: a portrait of a colonist on
// another map switches to it, selects him and centres the camera on him.
func TestJourneyMapsBarColonistOfAnotherMapJumps(t *testing.T) {
	t.Parallel()
	j, away, pawn := mapsJourney(t)
	if !j.clickID(fmt.Sprintf("bar.pawn.%d", pawn)) {
		t.Fatal("the portrait is disabled")
	}
	if got := j.g.ViewedMap(); got != away {
		t.Fatalf("the portrait showed map %d, want %d", got, away)
	}
	if !j.a.renderer.HasSelect() || j.a.renderer.Selected() != pawn {
		t.Fatalf("selected %v, want %d", j.a.renderer.Selected(), pawn)
	}
	j.draw(2)
	tx, ty := j.a.renderer.TileAt(j.a.sw/2, j.a.sh/2)
	if dx, dy := tx-30, ty-25; dx < -3 || dx > 3 || dy < -3 || dy > 3 {
		t.Fatalf("the screen centre shows tile (%d, %d), want the colonist's (30, 25)", tx, ty)
	}
}

// TestJourneyMapsShiftClickAcrossMapsRefused: Shift on a portrait of
// another map neither switches nor groups; it says why.
func TestJourneyMapsShiftClickAcrossMapsRefused(t *testing.T) {
	t.Parallel()
	j, _, pawn := mapsJourney(t)
	home := homePawn(j)
	j.clickID(fmt.Sprintf("bar.pawn.%d", home))
	j.settle()
	j.clickWith(fmt.Sprintf("bar.pawn.%d", pawn), input.ModShift)
	if got := j.g.ViewedMap(); got != 0 {
		t.Fatalf("a Shift click showed map %d, want it to stay on 0", got)
	}
	if j.a.sel.Grouped() || j.a.renderer.Selected() != home {
		t.Fatalf("the Shift click changed the selection: group %v, selected %v", j.a.sel.Group.IDs(), j.a.renderer.Selected())
	}
	toasted := false
	for _, ts := range j.a.chrome.LiveToasts() {
		toasted = toasted || ts.Text == hud.BarCrossMapHint
	}
	if !toasted {
		t.Fatalf("no refusal toast: %+v", j.a.chrome.LiveToasts())
	}
}

// TestJourneyMapsLetterAndMessageJump: a letter and a message posted on the
// other map take the player there from map 0, through the letter stack's Jump
// to and the message row.
func TestJourneyMapsLetterAndMessageJump(t *testing.T) {
	t.Parallel()
	j, away, _ := mapsJourney(t)
	j.g.ViewMap(away)
	if !j.g.FireIncident("flu") || len(j.world().Letters) == 0 {
		t.Fatal("the flu outbreak on the second map posted no letter")
	}
	j.g.ViewMap(0)
	j.update(2)
	var letter game.LetterView
	for _, l := range j.g.World().Letters {
		if l.Map == away {
			letter = l
		}
	}
	if letter.ID == 0 {
		t.Fatalf("no letter carries map %d: %+v", away, j.g.World().Letters)
	}
	if !j.clickID(fmt.Sprintf("letter.%d", letter.ID)) || j.a.uiModalTop() == "" {
		t.Fatal("the letter row opened no window")
	}
	if !j.clickID(fmt.Sprintf("modal.letter.%d.jump", letter.ID)) {
		t.Fatal("Jump to is disabled")
	}
	if got := j.g.ViewedMap(); got != away {
		t.Fatalf("Jump to showed map %d, want %d", got, away)
	}

	j.key("BracketRight", 0) // back home
	if j.g.ViewedMap() != 0 {
		t.Fatalf("] left us on map %d", j.g.ViewedMap())
	}
	for _, b := range j.a.layoutHits(j.a.sw, j.a.sh) {
		if strings.HasPrefix(b.ID, "msg.") {
			if !j.clickID(b.ID) {
				t.Fatal("message row disabled")
			}
			if got := j.g.ViewedMap(); got != away {
				t.Fatalf("clicking %s showed map %d, want %d", b.ID, got, away)
			}
			return
		}
	}
	t.Fatalf("no message row on map 0's screen for the other map's news: %+v", j.g.World().Messages)
}

// TestJourneyMapsKeepTheirOwnCamera: moving the view on map 0, switching,
// moving on map 1 and switching back finds map 0's camera as it was left.
func TestJourneyMapsKeepTheirOwnCamera(t *testing.T) {
	t.Parallel()
	j, away, _ := mapsJourney(t)
	r := j.a.renderer
	pan := func() {
		j.draw(1)
		j.hold("D")
		j.update(1)
		j.draw(12)
		j.release()
		j.update(1)
		for i := 0; i < 400 && !r.smooth.Idle(); i++ {
			j.draw(1)
		}
	}
	pan()
	homeX, homeY := r.cam.OffsetX, r.cam.OffsetY
	j.key("BracketRight", 0)
	if j.g.ViewedMap() != away {
		t.Fatal("] did not switch")
	}
	j.draw(2)
	awayX, awayY := r.cam.OffsetX, r.cam.OffsetY
	pan()
	if r.cam.OffsetX == awayX && r.cam.OffsetY == awayY {
		t.Fatal("the pan did not move the second map's camera")
	}
	j.key("BracketLeft", 0)
	j.draw(2)
	if r.cam.OffsetX != homeX || r.cam.OffsetY != homeY {
		t.Fatalf("map 0's camera is (%v, %v) after the round trip, want (%v, %v)", r.cam.OffsetX, r.cam.OffsetY, homeX, homeY)
	}
}

// TestJourneyMapsSingleMapBarHasNoFrames: with one map open the bar is the
// plain one.
func TestJourneyMapsSingleMapBarHasNoFrames(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	j.update(2)
	for _, b := range j.a.layoutHits(j.a.sw, j.a.sh) {
		if strings.HasPrefix(b.ID, hud.BarMapPrefix) {
			t.Fatalf("a single-map bar has a frame %s", b.ID)
		}
	}
	j.key("BracketRight", 0)
	if j.g.ViewedMap() != 0 {
		t.Fatalf("] with one map showed map %d", j.g.ViewedMap())
	}
}
