package game

import (
	"strings"
	"testing"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/difficulty"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/mapgen/pipeline"
	"rimworld-go/internal/domain/world/terrain"
	"rimworld-go/internal/domain/world/vault"
)

// Sealed vault fixtures (MAPGEN-3). siteFixture's granite block (60..79)^2
// holds vault 1 at (63,63)-(71,71): an interior 64..70 square with four
// caskets (tops (65,64), (68,64), (65,69), (68,69)), three defender spots
// on row 67 and two loot stacks. A soil corridor (60..62, 67) leads from
// the open ground to its west wall at (63,67), so that wall shows and a
// colonist can stand next to it. Vault 2 at (73,63)-(77,71) holds one
// casket at (75,64) and has no way in.

const (
	v1X0, v1Y0, v1X1, v1Y1 = 63, 63, 71, 71
	v1DoorX, v1DoorY       = 63, 67 // the west wall the corridor meets
)

var v1Caskets = [4][2]int{{65, 64}, {68, 64}, {65, 69}, {68, 69}}

// mapVault builds a generator vault with one floor tile per interior cell,
// the full wall ring, caskets with the given occupants at tops, defender
// spots and loot.
func mapVault(id, x0, y0, x1, y1 int, tops [][2]int, occ []vault.Occupant, spots []mapgen.Point, loot []mapgen.Loot) mapgen.Vault {
	v := mapgen.Vault{ID: id, X0: x0, Y0: y0, X1: x1, Y1: y1, WallStuff: item.GraniteBlocks, DefenderSpots: spots, Loot: loot}
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			if x == x0 || x == x1 || y == y0 || y == y1 {
				v.Walls = append(v.Walls, mapgen.Point{X: x, Y: y})
			} else {
				v.Floors = append(v.Floors, mapgen.RuinFloor{X: x, Y: y, Stuff: item.GraniteBlocks})
			}
		}
	}
	for i, t := range tops {
		v.Caskets = append(v.Caskets, mapgen.Casket{X: t[0], Y: t[1], Occupant: occ[i%len(occ)]})
	}
	return v
}

// carveVault lays v's footprint as smooth stone (the generator's carving).
func carveVault(r *mapgen.Result, v mapgen.Vault) {
	for y := v.Y0; y <= v.Y1; y++ {
		for x := v.X0; x <= v.X1; x++ {
			r.Terrain.Set(x, y, terrain.StoneSmooth)
		}
	}
}

// withVaults adds vault 1 (occupants occ, cycled) and vault 2 (a Grateful
// sleeper) to the fixture, with the corridor to vault 1.
func withVaults(occ ...vault.Occupant) func(*mapgen.Result) {
	return func(r *mapgen.Result) {
		tops := make([][2]int, 0, 4)
		for _, c := range v1Caskets {
			tops = append(tops, c)
		}
		v1 := mapVault(1, v1X0, v1Y0, v1X1, v1Y1, tops, occ,
			[]mapgen.Point{{X: 64, Y: 67}, {X: 67, Y: 67}, {X: 70, Y: 67}},
			[]mapgen.Loot{{X: 66, Y: 67, Kind: item.Silver, Qty: 200}, {X: 69, Y: 67, Kind: item.Components, Qty: 6}})
		v2 := mapVault(2, 73, 63, 77, 71, [][2]int{{75, 64}}, []vault.Occupant{vault.Grateful},
			[]mapgen.Point{{X: 75, Y: 68}}, nil)
		carveVault(r, v1)
		carveVault(r, v2)
		for x := 60; x < v1X0; x++ {
			r.Terrain.Set(x, v1DoorY, terrain.Soil)
		}
		r.Vaults = append(r.Vaults, v1, v2)
	}
}

// vaultGame is an ember_clan Setup (colonists standing at once) colony on the fixture with the two
// vaults; setup edits the Setup, pre the game before the site is laid.
func vaultGame(t testing.TB, setup func(*Setup), pre func(*Game), occ ...vault.Occupant) *Game {
	t.Helper()
	if len(occ) == 0 {
		occ = []vault.Occupant{vault.Empty}
	}
	s := setupFor(t, "ember_clan", func(s *Setup) {
		s.MapSize = 100
		if setup != nil {
			setup(s)
		}
	})
	g := newSetupGame(Config{Setup: s}, s)
	if pre != nil {
		pre(g)
	}
	res := siteFixture(100)
	withVaults(occ...)(&res)
	g.applySite(res)
	g.finishSetup()
	return g
}

func vaultByID(g *Game, id int) *vaultSave {
	for i := range g.vaults {
		if g.vaults[i].ID == id {
			return &g.vaults[i]
		}
	}
	return nil
}

func lettersTitled(g *Game, title string) int {
	n := 0
	for _, m := range g.messages {
		if m.Letter && m.Title == title {
			n++
		}
	}
	return n
}

func messageWith(g *Game, part string) bool {
	for _, m := range g.messages {
		if strings.Contains(m.Text, part) || strings.Contains(m.Title, part) {
			return true
		}
	}
	return false
}

func historyKind(g *Game, kind string) int {
	n := 0
	for _, h := range g.history {
		if h.Kind == kind {
			n++
		}
	}
	return n
}

// breachV1 takes vault 1's west wall down.
func breachV1(t testing.TB, g *Game) {
	t.Helper()
	wid := g.layerBldgAt(v1DoorX, v1DoorY, construction.LayerObject)
	if wid == 0 {
		t.Fatal("no vault wall at the corridor")
	}
	g.removeBuilding(wid)
}

func TestVaultLaidSealedAndHidden(t *testing.T) {
	t.Parallel()
	g := vaultGame(t, nil, nil, vault.Grateful, vault.Hostile, vault.Empty, vault.Remains)
	w, ok := bldgAt(g, construction.Wall, v1DoorX, v1DoorY)
	if !ok || !w.Ancient || w.Stuff != item.GraniteBlocks || w.Ruin || w.Damage != 0 {
		t.Fatalf("vault wall %+v %v", w, ok)
	}
	if f, ok := bldgAt(g, construction.Floor, 66, 66); !ok || !f.Ancient {
		t.Fatalf("vault floor %+v %v", f, ok)
	}
	want := []vault.Occupant{vault.Grateful, vault.Hostile, vault.Empty, vault.Remains}
	for i, c := range v1Caskets {
		b, ok := bldgAt(g, construction.SleeperCasket, c[0], c[1])
		if !ok || !b.Ancient || vault.Occupant(b.Occupant) != want[i] || b.CasketGroup != 1 || b.Stuff != item.Steel {
			t.Fatalf("casket %d: %+v %v", i, b, ok)
		}
		if g.grid.Walkable(c[0], c[1]+1) {
			t.Fatal("a casket's lower cell blocks")
		}
	}
	if k, q := stackAt(g, 66, 67); k != item.Silver || q != 200 {
		t.Fatalf("loot %v x%d", k, q)
	}
	// sealed: the interior is hidden, the wall the corridor meets shows
	if !g.Fogged(66, 66) || !g.Fogged(65, 64) || !g.Fogged(66, 67) {
		t.Fatal("the vault's interior shows")
	}
	if g.Fogged(v1DoorX, v1DoorY) {
		t.Fatal("the wall at the corridor is hidden")
	}
	if v := vaultByID(g, 1); v == nil || v.State != vault.Generated || v.X0 != v1X0 || v.Y1 != v1Y1 {
		t.Fatalf("vault record %+v", v)
	}
	if !notColonyPiece(w) {
		t.Fatal("an ancient wall counts as the colony's")
	}
	for _, bv := range g.World().Buildings {
		if bv.Kind == construction.SleeperCasket.String() {
			t.Fatal("a sealed casket shows through the fog")
		}
	}
}

func TestVaultDefenderCount(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ pts, want int }{{0, 2}, {35, 2}, {124, 2}, {125, 3}, {175, 4}, {274, 5}, {275, 6}, {5000, 6}} {
		if got := vaultDefenderCount(c.pts); got != c.want {
			t.Errorf("vaultDefenderCount(%d) = %d, want %d", c.pts, got, c.want)
		}
	}
}

func TestVaultDefendersDormantUntilBreach(t *testing.T) {
	t.Parallel()
	g := vaultGame(t, nil, nil)
	v := vaultByID(g, 1)
	if n := len(v.Defenders); n < 2 || n > 6 {
		t.Fatalf("%d defenders", n)
	}
	brutes := 0
	type snap struct {
		pos  Position
		food int
	}
	before := map[entity.EntityID]snap{}
	for _, d := range v.Defenders {
		id := entity.EntityID(d)
		a, ok := g.animals.Get(id)
		if !ok || !a.Dormant || !animal.DefOf(a.Species).Hive || a.Enraged(uint64(g.clock.Now())) || g.hostile(a) {
			t.Fatalf("defender %d: %+v", id, a)
		}
		if a.Species == animal.HiveBrute {
			brutes++
		}
		p, _ := g.positions.Get(id)
		if p.X <= v1X0 || p.X >= v1X1 || p.Y <= v1Y0 || p.Y >= v1Y1 {
			t.Fatalf("defender outside the vault at %v", p)
		}
		before[id] = snap{p, a.Food}
	}
	if brutes > len(v.Defenders)/3 {
		t.Fatalf("%d brutes of %d", brutes, len(v.Defenders))
	}
	views := 0
	for _, av := range g.animalViews() {
		if _, ok := before[av.ID]; ok {
			if !av.Dormant || !av.Sleeping || av.Hostile {
				t.Fatalf("dormant view %+v", av)
			}
			views++
		}
	}
	if views != len(before) {
		t.Fatal("dormant defenders missing from the views")
	}
	g.Step(600)
	for id, s := range before {
		a, _ := g.animals.Get(id)
		p, _ := g.positions.Get(id)
		if !a.Dormant || p != s.pos || a.Food != s.food || a.Mode != animal.Wander {
			t.Fatalf("defender %d acted while dormant: %+v at %v", id, a, p)
		}
	}
	// breaching wakes them all
	breachV1(t, g)
	if v := vaultByID(g, 1); v.State != vault.Breached {
		t.Fatalf("state %v after the breach", v.State)
	}
	if g.Fogged(66, 66) {
		t.Fatal("the breach did not reveal the vault")
	}
	for id := range before {
		a, _ := g.animals.Get(id)
		if a.Dormant || a.Manhunter != hiveForever || a.Mode != animal.Attack {
			t.Fatalf("defender %d still asleep: %+v", id, a)
		}
	}
	if !messageWith(g, "The vault is open. Hive insects stir inside!") || historyKind(g, "vault") != 1 {
		t.Fatal("no breach letter or history line")
	}
	danger := false
	for _, m := range g.messages {
		if m.Text == "The vault is open. Hive insects stir inside!" {
			danger = m.Danger && m.Letter
		}
	}
	if !danger {
		t.Fatal("the breach letter is not a danger letter")
	}
	// breaching opens no casket, and vault 2 sleeps on
	for _, c := range v1Caskets {
		if b, _ := bldgAt(g, construction.SleeperCasket, c[0], c[1]); b.Occupant == 0 {
			t.Fatal("the breach opened a casket")
		}
	}
	if v2 := vaultByID(g, 2); v2.State != vault.Generated {
		t.Fatalf("vault 2 state %v", v2.State)
	}
	for _, d := range vaultByID(g, 2).Defenders {
		if a, _ := g.animals.Get(entity.EntityID(d)); !a.Dormant {
			t.Fatal("vault 2's defender woke")
		}
	}
}

func TestVaultHurtDefenderWakes(t *testing.T) {
	t.Parallel()
	g := vaultGame(t, nil, nil)
	id := entity.EntityID(vaultByID(g, 1).Defenders[0])
	g.hurt(id, 1, 0)
	a, _ := g.animals.Get(id)
	if a.Dormant || a.Manhunter != hiveForever {
		t.Fatalf("hurt defender %+v", a)
	}
	if b := entity.EntityID(vaultByID(g, 1).Defenders[1]); b != 0 {
		if o, _ := g.animals.Get(b); !o.Dormant {
			t.Fatal("hurting one woke another")
		}
	}
}

func TestVaultPeacefulHasNoDefenders(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		setup func(*Setup)
		pre   func(*Game)
	}{
		"wanderer":   {setup: func(s *Setup) { s.Preset, s.Difficulty = difficulty.Wanderer, difficulty.Of(difficulty.Wanderer) }},
		"no threats": {setup: func(s *Setup) { s.Difficulty.ThreatScale = 0 }},
		"hives off": {pre: func(g *Game) {
			if g.ng.fx.Disabled == nil {
				g.ng.fx.Disabled = map[string]bool{}
			}
			g.ng.fx.Disabled["hive_swarm"] = true
		}},
	}
	for name, c := range cases {
		g := vaultGame(t, c.setup, c.pre)
		for _, v := range g.vaults {
			if len(v.Defenders) != 0 {
				t.Fatalf("%s: vault %d has %d defenders", name, v.ID, len(v.Defenders))
			}
		}
		for _, id := range g.animalIDs {
			if a, _ := g.animals.Get(id); a.Dormant {
				t.Fatalf("%s: a dormant animal", name)
			}
		}
		if _, ok := bldgAt(g, construction.SleeperCasket, 65, 64); !ok {
			t.Fatalf("%s: the room is gone", name)
		}
		if k, _ := stackAt(g, 66, 67); k != item.Silver {
			t.Fatalf("%s: the loot is gone", name)
		}
		breachV1(t, g)
		if !messageWith(g, "The vault is open.") || messageWith(g, "Hive insects stir") {
			t.Fatalf("%s: wrong breach letter", name)
		}
	}
}

// standColonist puts the first colonist at (x, y).
func standColonist(g *Game, x, y int) entity.EntityID {
	id := g.pawnIDs[0]
	g.positions.Set(id, Position{X: x, Y: y})
	return id
}

func TestVaultWarningOnce(t *testing.T) {
	t.Parallel()
	g := vaultGame(t, nil, nil)
	g.vaultTick(vaultCheckEvery)
	if lettersTitled(g, vaultWarnTitle) != 0 {
		t.Fatal("warned with nobody near")
	}
	id := standColonist(g, 61, v1DoorY) // two from the wall ring
	g.vaultTick(vaultCheckEvery + 1)
	if lettersTitled(g, vaultWarnTitle) != 0 {
		t.Fatal("checked off the 60-tick beat")
	}
	g.vaultTick(2 * vaultCheckEvery)
	if lettersTitled(g, vaultWarnTitle) != 1 || vaultByID(g, 1).State != vault.Warned {
		t.Fatalf("no warning: state %v", vaultByID(g, 1).State)
	}
	want := g.personName(id) + " stops near the smooth walls in the rock. Something old is sealed inside, and it does not feel empty."
	if !messageWith(g, want) {
		t.Fatal("warning text")
	}
	g.vaultTick(3 * vaultCheckEvery)
	g.vaultTick(4 * vaultCheckEvery)
	if lettersTitled(g, vaultWarnTitle) != 1 {
		t.Fatal("warned twice")
	}
	if vaultByID(g, 2).State != vault.Generated {
		t.Fatal("vault 2 warned from afar")
	}
	// ignoring it leaves the vault as it was
	for _, d := range vaultByID(g, 1).Defenders {
		if a, _ := g.animals.Get(entity.EntityID(d)); !a.Dormant {
			t.Fatal("a warning woke a defender")
		}
	}
	if b, _ := bldgAt(g, construction.SleeperCasket, 65, 64); b.Occupant == 0 || !g.Fogged(66, 66) {
		t.Fatal("a warning opened something")
	}
}

func TestVaultNoWarningAfterBreach(t *testing.T) {
	t.Parallel()
	g := vaultGame(t, nil, nil)
	breachV1(t, g)
	standColonist(g, 62, v1DoorY)
	g.vaultTick(vaultCheckEvery)
	if lettersTitled(g, vaultWarnTitle) != 0 {
		t.Fatal("a warning after the breach")
	}
}

func TestVaultCleared(t *testing.T) {
	t.Parallel()
	g := vaultGame(t, nil, nil, vault.Empty, vault.Beetles)
	breachV1(t, g)
	for _, d := range vaultByID(g, 1).Defenders {
		g.killAnimal(entity.EntityID(d), 0)
	}
	g.vaultTick(vaultCheckEvery)
	if s := vaultByID(g, 1).State; s != vault.Breached {
		t.Fatalf("cleared with sealed caskets left: %v", s)
	}
	g.openCasketGroup(bldgIDAt(g, construction.SleeperCasket, 65, 64))
	if s := vaultByID(g, 1).State; s != vault.CasketsOpened {
		t.Fatalf("state %v after opening", s)
	}
	g.vaultTick(2 * vaultCheckEvery)
	if s := vaultByID(g, 1).State; s != vault.CasketsOpened {
		t.Fatal("cleared with beetles alive")
	}
	for _, h := range vaultByID(g, 1).Hostiles {
		g.killAnimal(entity.EntityID(h), 0)
	}
	g.vaultTick(3 * vaultCheckEvery)
	if s := vaultByID(g, 1).State; s != vault.Cleared {
		t.Fatalf("state %v, want cleared", s)
	}
	if historyKind(g, "vault") != 2 {
		t.Fatal("no cleared history line")
	}
	g.vaultTick(4 * vaultCheckEvery)
	if historyKind(g, "vault") != 2 {
		t.Fatal("cleared twice")
	}
}

func TestVaultAncientPieces(t *testing.T) {
	t.Parallel()
	g := vaultGame(t, nil, nil)
	breachV1(t, g)
	// the walls can be taken down like any other once reachable
	g.markDeconstruct(v1DoorX, v1DoorY-1, v1DoorX, v1DoorY-1, true)
	if b, _ := bldgAt(g, construction.Wall, v1DoorX, v1DoorY-1); !b.Deconstruct {
		t.Fatal("an ancient wall can't be deconstructed")
	}
	// smashed down: no blueprint to rebuild it
	wid := bldgIDAt(g, construction.Wall, v1DoorX, v1DoorY+1)
	g.damageBuilding(wid, 1<<20)
	if g.layerFrameAt(v1DoorX, v1DoorY+1, construction.LayerObject) != 0 {
		t.Fatal("a destroyed ancient wall left a blueprint")
	}
	// rebuilt in another material, it is the colony's
	sid := bldgIDAt(g, construction.Wall, v1DoorX, v1DoorY-2)
	sw, _ := g.bldgs.Get(sid)
	g.completeSwap(0, construction.Frame{Kind: construction.Wall, Stuff: item.Wood, X: sw.X, Y: sw.Y, Swaps: sid}, sw)
	if b, _ := g.bldgs.Get(sid); b.Ancient || b.Stuff != item.Wood {
		t.Fatalf("swapped vault wall %+v", b)
	}
	// no Home anchor
	for _, id := range g.bldgIDs {
		if b, _ := g.bldgs.Get(id); b.Ancient && g.inHome(b.Y*g.grid.Width()+b.X) {
			t.Fatalf("Home around an ancient piece at (%d,%d)", b.X, b.Y)
		}
	}
}

func TestVaultSaveRoundTrip(t *testing.T) {
	t.Parallel()
	g := vaultGame(t, nil, nil, vault.Beetles, vault.Hostile)
	breachV1(t, g)
	standColonist(g, 61, 61)
	g.vaultTick(vaultCheckEvery)
	g2 := saveLoad(t, g)
	if len(g2.vaults) != 2 {
		t.Fatalf("%d vaults after load", len(g2.vaults))
	}
	for i := range g.vaults {
		a, b := g.vaults[i], g2.vaults[i]
		if a.ID != b.ID || a.State != b.State || a.X0 != b.X0 || a.Y1 != b.Y1 || len(a.Defenders) != len(b.Defenders) || a.Caskets != b.Caskets {
			t.Fatalf("vault %d: %+v vs %+v", i, a, b)
		}
	}
	for _, d := range vaultByID(g2, 2).Defenders {
		if a, _ := g2.animals.Get(entity.EntityID(d)); !a.Dormant {
			t.Fatal("Dormant lost in the save")
		}
	}
	c, ok := bldgAt(g2, construction.SleeperCasket, 68, 64)
	if !ok || !c.Ancient || vault.Occupant(c.Occupant) != vault.Hostile || c.CasketGroup != 1 {
		t.Fatalf("casket after load %+v", c)
	}
	// a legacy colony saves no vaults
	if l := New(Config{Seed: 1, Width: 20, Height: 20}); len(saveLoad(t, l).vaults) != 0 {
		t.Fatal("vaults in a legacy colony")
	}
}

// TestVaultOnGeneratedMap lays a real generator's vault: every wall and
// casket stands, its interior starts hidden and its defenders sleep.
func TestVaultOnGeneratedMap(t *testing.T) {
	t.Parallel()
	for _, seed := range []uint64{4242, 2, 7, 99, 1, 3} {
		s := setupFor(t, "ember_clan", func(s *Setup) { s.Seed = seed })
		res, err := pipeline.Generate(mapgen.Config{Seed: s.Seed, W: 100, H: 100, Site: s.Site, Biome: biome.DefOf(s.Site.Biome)})
		if err != nil || len(res.Vaults) == 0 {
			continue
		}
		g := buildSetup(t, s)
		if len(g.vaults) != len(res.Vaults) {
			t.Fatalf("seed %d: %d of %d vaults kept", seed, len(g.vaults), len(res.Vaults))
		}
		for i, v := range res.Vaults {
			for _, w := range v.Walls {
				if b, ok := bldgAt(g, construction.Wall, w.X, w.Y); !ok || !b.Ancient {
					t.Fatalf("seed %d: no ancient wall at (%d,%d)", seed, w.X, w.Y)
				}
			}
			for _, c := range v.Caskets {
				if b, ok := bldgAt(g, construction.SleeperCasket, c.X, c.Y); !ok || vault.Occupant(b.Occupant) != c.Occupant {
					t.Fatalf("seed %d: casket at (%d,%d) %+v", seed, c.X, c.Y, b)
				}
			}
			rec := &g.vaults[i]
			if g.vaultInteriorShows(rec) {
				t.Fatalf("seed %d: vault %d starts revealed", seed, v.ID)
			}
			if n := len(rec.Defenders); n < vaultDefMin || n > vaultDefMax {
				t.Fatalf("seed %d: %d defenders", seed, n)
			}
			for _, d := range rec.Defenders {
				if a, _ := g.animals.Get(entity.EntityID(d)); !a.Dormant {
					t.Fatalf("seed %d: an awake defender", seed)
				}
			}
		}
		return
	}
	t.Fatal("no seed generated a vault")
}
