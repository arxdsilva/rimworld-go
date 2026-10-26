package game

import (
	"bytes"
	"compress/gzip"
	"encoding/gob"
	"strings"
	"testing"

	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// loadAsVersionWith is loadAsVersion with edit applied to the decoded
// save state before it is re-encoded (to plant fields only older saves
// wrote).
func loadAsVersionWith(t *testing.T, g *Game, v int, edit func(*saveState)) *Game {
	t.Helper()
	var buf bytes.Buffer
	if err := g.Save(&buf, "old"); err != nil {
		t.Fatal(err)
	}
	zr, err := gzip.NewReader(&buf)
	if err != nil {
		t.Fatal(err)
	}
	dec := gob.NewDecoder(zr)
	var info SaveInfo
	var s saveState
	if err := dec.Decode(&info); err != nil {
		t.Fatal(err)
	}
	if err := dec.Decode(&s); err != nil {
		t.Fatal(err)
	}
	info.Version = v
	edit(&s)
	var out bytes.Buffer
	zw := gzip.NewWriter(&out)
	enc := gob.NewEncoder(zw)
	if err := enc.Encode(info); err != nil {
		t.Fatal(err)
	}
	if err := enc.Encode(&s); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(&out)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// TestOldSaveInfectionMigrates (save v6, MED-02): a v5 save's per-pawn
// infection becomes an infection condition on the pawn's worst open
// wound, with no immunity; a legacy open wound without an infection adds
// nothing; a v6 save never writes the legacy field.
func TestOldSaveInfectionMigrates(t *testing.T) {
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	ada, _ := g.SpawnNamedPawn(4, 4, "Ada")
	bo, _ := g.SpawnNamedPawn(6, 4, "Bo")
	addWound(t, g, ada, body.ArmL, 3, body.Cut)
	addWound(t, g, ada, body.LegL, 9, body.Bite)
	h := loadAsVersionWith(t, g, 5, func(s *saveState) {
		s.Wounds = map[entity.EntityID]legacyWound{ada: {Infection: 45, Care: 3}, bo: {At: 7}}
	})
	c, ok := h.worstInfection(ada)
	if !ok || c.Part != body.LegL || c.Sev != 450 || c.Immunity != 0 || c.Kind != body.CondInfection {
		t.Fatalf("migrated infection %+v (found %v)", c, ok)
	}
	if h.InfectionOf(ada) != 45 || !h.indexedPatient(ada) {
		t.Fatalf("infection %d%%, patient %v", h.InfectionOf(ada), h.indexedPatient(ada))
	}
	if _, ok := h.worstInfection(bo); ok || h.med.legacyWounds != nil {
		t.Fatal("an open wound alone migrates to nothing; the legacy map is dropped")
	}
	k := loadAsVersionWith(t, h, SaveVersion, func(s *saveState) {
		if s.Wounds != nil {
			t.Fatalf("a v%d save writes no legacy wounds: %v", SaveVersion, s.Wounds)
		}
	})
	if c2, _ := k.worstInfection(ada); c2 != c {
		t.Fatalf("the condition round-trips: %+v vs %+v", c2, c)
	}
}

// TestHealthViewConditions: the Health tab lists an infection with its
// severity, immunity and tend quality, whether it is due, the hours to
// its next tend, a tooltip with the quality, the headline and the death
// clock; part tooltips list each wound's tend quality; the pawn view
// says losing and how long a wound has waited.
func TestHealthViewConditions(t *testing.T) {
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	id, _ := g.SpawnNamedPawn(4, 4, "Ada")
	b := addWound(t, g, id, body.LegL, 6, body.Bruise)
	g.Infect(id, 20)
	hv, _ := g.healthView(id)
	if len(hv.Conditions) != 1 {
		t.Fatalf("conditions %+v", hv.Conditions)
	}
	cv := hv.Conditions[0]
	if cv.Label != "Infection (minor), left leg" || cv.SevPct != 20 || cv.ImmPct != 0 || !cv.Due || !cv.Losing || cv.DueInHours != 0 {
		t.Fatalf("untended row %+v", cv)
	}
	if hv.State != "Infection: losing" || hv.DeathInHours != 23 { // 800 left at 35 an hour
		t.Fatalf("headline %q, death in %d", hv.State, hv.DeathInHours)
	}
	if tip := hv.Parts[body.LegL].TendTip; tip != "bruise, 6 HP: untended" {
		t.Fatalf("part tip %q", tip)
	}
	for i := 0; i < 5; i++ {
		medHour(g)
	}
	pv := pawnView(g, id)
	if !pv.InfectionLosing || pv.UntendedHours != 5 || !pv.Wounded || pv.Infection < 20 {
		t.Fatalf("pawn view losing %v untended %dh wounded %v infection %d", pv.InfectionLosing, pv.UntendedHours, pv.Wounded, pv.Infection)
	}
	b.TendAll(900, uint64(g.clock.Now()))
	v, _ := g.vital.Get(id)
	g.bodyChanged(id, b, &v)
	hv, _ = g.healthView(id)
	cv = hv.Conditions[0]
	if cv.TendPct != 90 || cv.Due || cv.DueInHours != 12 || cv.Losing || !strings.Contains(cv.Tip, "Tended at 90% quality") {
		t.Fatalf("tended row %+v", cv)
	}
	if hv.State != "Infection: winning" || hv.DeathInHours != -1 {
		t.Fatalf("tended headline %q, death in %d", hv.State, hv.DeathInHours)
	}
	if tip := hv.Parts[body.LegL].TendTip; tip != "bruise, 5 HP: tended 90%" { // a point healed in the 5 hours
		t.Fatalf("tended part tip %q", tip)
	}
	if pv := pawnView(g, id); pv.InfectionLosing || pv.UntendedHours != 0 || pv.Wounded {
		t.Fatalf("tended pawn view %+v", pv)
	}
	g.clock.Advance(12 * clock.TicksPerHour)
	if hv, _ = g.healthView(id); !hv.Conditions[0].Due {
		t.Fatal("due again after its window")
	}
}
