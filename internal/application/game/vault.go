package game

import (
	"slices"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/difficulty"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/terrain"
	"rimworld-go/internal/domain/world/vault"
	"rimworld-go/pkg/randx"
)

// Sealed vaults (MAPGEN-3 §4.2). The generator cuts old chambers into the
// rock: an intact wall ring of ancient pieces, a tiled floor, sleeper
// caskets (casket.go) and loot. At landing each vault gets a few dormant
// hive insects (none on peaceful settings). A vault's life:
//
//	Generated → Warned (a colonist stands near: the "Ominous walls" letter)
//	→ Breached (fog reveals its interior: the defenders wake)
//	→ CasketsOpened (its casket group opened) → Cleared (no defender or
//	hostile occupant left and no sealed casket).
//
// The vault list is tiny (at most three per map) and every check is
// paced: approach and clearing every vaultCheckEvery ticks, the breach
// only after a fog reveal.

// vaultSave is one sealed vault, as kept and saved (saveState.Vaults).
type vaultSave struct {
	ID             int
	X0, Y0, X1, Y1 int
	State          vault.State
	Defenders      []uint64 // entity ids of its dormant (or woken) defenders
	// Hostiles are the hostile casket occupants released (raiders, hive
	// insects), counted for clearing; Caskets how many caskets it got.
	Hostiles []uint64
	Caskets  int
}

const (
	// vaultCheckEvery paces the approach warning and the clearing check.
	vaultCheckEvery = 60
	// vaultWarnRange is how far outside the wall ring a colonist senses a
	// vault.
	vaultWarnRange = 2
	// vaultPtsPerDefender is the threat points per dormant defender (our
	// divisor), within vaultDefMin..vaultDefMax.
	vaultPtsPerDefender = 50
	vaultDefMin         = 2
	vaultDefMax         = 6
	// vaultSalt seeds a vault's defender roll (never the main stream).
	vaultSalt = 0x5EA1ED

	vaultWarnTitle   = "Ominous walls"
	vaultBreachTitle = "Sealed vault breached"
	vaultBreachText  = "The vault is open. Hive insects stir inside!"
	vaultQuietText   = "The vault is open. Nothing stirs inside but old dust."
)

// inside reports a cell inside v's wall ring.
func (v *vaultSave) inside(x, y int) bool { return x > v.X0 && x < v.X1 && y > v.Y0 && y < v.Y1 }

func (v *vaultSave) centre() (int, int) { return (v.X0 + v.X1) / 2, (v.Y0 + v.Y1) / 2 }

// vaultDefenderCount is the dormant defenders a vault gets for pts threat
// points: one per vaultPtsPerDefender (rounded), 2..6.
func vaultDefenderCount(pts int) int {
	return min(max((pts+vaultPtsPerDefender/2)/vaultPtsPerDefender, vaultDefMin), vaultDefMax)
}

// vaultPeaceful reports settings that get no defenders: no threats, the
// most peaceful preset, or hive swarms disabled by the scenario. The
// rooms and their loot stay.
func (g *Game) vaultPeaceful() bool {
	s := g.ng.setup
	return s == nil || s.Difficulty.ThreatScale <= 0 || s.Preset == difficulty.Wanderer || g.incidentDisabled("hive_swarm")
}

// vaultThreatPoints is the threat points at landing: the colonists about
// to land (none stand yet when the site is laid) and the wealth so far.
func (g *Game) vaultThreatPoints() int {
	s := g.ng.setup
	return difficulty.ThreatPoints(s.Difficulty, g.Wealth(), len(s.Colonists), 0, int(g.ng.adapt.Value))
}

// vaultSeed is the base of the vaults' private rolls.
func (g *Game) vaultSeed() uint64 {
	if s := g.ng.setup; s != nil {
		return s.Seed
	}
	return g.streams.seed
}

// placeVaults lays the generator's vaults (applySite): walls, floors and
// caskets as ancient pieces, the loot, then the dormant defenders. It
// draws nothing from the main random stream.
func (g *Game) placeVaults(vs []mapgen.Vault) {
	if len(vs) == 0 {
		return
	}
	peaceful := g.vaultPeaceful()
	n := 0
	if !peaceful {
		n = vaultDefenderCount(g.vaultThreatPoints())
	}
	for _, v := range vs {
		if !g.grid.InBounds(v.X0, v.Y0) || !g.grid.InBounds(v.X1, v.Y1) || v.X1-v.X0 < 2 || v.Y1-v.Y0 < 2 {
			continue
		}
		rec := vaultSave{ID: v.ID, X0: v.X0, Y0: v.Y0, X1: v.X1, Y1: v.Y1}
		g.carveVault(v)
		for _, f := range v.Floors {
			g.placeGenerated(construction.Building{Kind: construction.Floor, Stuff: f.Stuff, X: f.X, Y: f.Y, Ancient: true})
		}
		for _, w := range v.Walls {
			g.placeGenerated(construction.Building{Kind: construction.Wall, Stuff: v.WallStuff, X: w.X, Y: w.Y, Ancient: true})
		}
		for _, c := range v.Caskets {
			if c.Occupant == vault.NoOccupant || c.Occupant >= vault.NumOccupants {
				continue
			}
			if g.placeGenerated(construction.Building{Kind: construction.SleeperCasket, Stuff: item.Steel, X: c.X, Y: c.Y,
				Ancient: true, Occupant: uint8(c.Occupant), CasketGroup: v.ID}) != 0 {
				rec.Caskets++
			}
		}
		for _, l := range v.Loot {
			if l.Qty > 0 && g.dryWalkable(l.X, l.Y) {
				g.spawnUnclaimed(l.Kind, l.Qty, l.X, l.Y, false) // haul_claim.go: found, not the colony's
			}
		}
		if n > 0 {
			rec.Defenders = g.spawnDefenders(&rec, v.DefenderSpots, n)
		}
		g.vaults = append(g.vaults, rec)
	}
}

// carveVault keeps a hand-made Result safe: rock left in the footprint
// becomes smooth stone (the generator carves it already).
func (g *Game) carveVault(v mapgen.Vault) {
	for y := v.Y0; y <= v.Y1; y++ {
		for x := v.X0; x <= v.X1; x++ {
			if terrain.IsRockWall(g.terr.At(x, y)) {
				g.setTerrain(x, y, terrain.StoneSmooth)
			}
		}
	}
}

// spawnDefenders puts n dormant hive insects in vault v, on its defender
// spots first and then on free interior floor (row-major), at most one
// per cell; brutes are at most n/3. The species roll is the vault's own.
func (g *Game) spawnDefenders(v *vaultSave, spots []mapgen.Point, n int) []uint64 {
	r := randx.New(g.vaultSeed() ^ vaultSalt ^ uint64(v.ID)*0x9E3779B97F4A7C15)
	cells := make([][2]int, 0, len(spots))
	for _, p := range spots {
		if v.inside(p.X, p.Y) {
			cells = append(cells, [2]int{p.X, p.Y})
		}
	}
	for y := v.Y0 + 1; y < v.Y1; y++ {
		for x := v.X0 + 1; x < v.X1; x++ {
			cells = append(cells, [2]int{x, y})
		}
	}
	used := map[[2]int]bool{}
	var out []uint64
	brutes := 0
	for _, c := range cells {
		if len(out) == n {
			break
		}
		if used[c] || !g.dryWalkable(c[0], c[1]) {
			continue
		}
		used[c] = true
		s := animal.HiveBeetle
		switch r.IntN(3) {
		case 1:
			s = animal.HiveCrawler
		case 2:
			if brutes < n/3 {
				s, brutes = animal.HiveBrute, brutes+1
			} else {
				s = animal.HiveCrawler
			}
		}
		out = append(out, uint64(g.spawnDormant(s, c[0], c[1])))
	}
	return out
}

// spawnDormant places a sleeping defender (spawnAnimal without the main
// stream's think roll: it does not think until it wakes).
func (g *Game) spawnDormant(s animal.Species, x, y int) entity.EntityID {
	id := g.entities.Next()
	g.animalIDs = append(g.animalIDs, id)
	g.positions.Set(id, Position{X: x, Y: y})
	g.animals.Set(id, animal.State{Species: s, Food: animal.Full, Rest: animal.Full, HasSex: true,
		Female: animal.ArtFemale(uint64(id)), Dormant: true})
	g.invalidateCreatures() // animal_index.go
	return id
}

// wakeDormant rouses a sleeping defender: from now on it hunts colonists
// like a hive swarm and never calms down.
func (g *Game) wakeDormant(a *animal.State) {
	if !a.Dormant {
		return
	}
	now := uint64(g.clock.Now())
	a.Dormant = false
	a.Manhunter, a.Mode = hiveForever, animal.Attack
	a.NextThink, a.NextDecide = now, now
}

// vaultTick runs every vault's approach warning and clearing check, every
// vaultCheckEvery ticks (phaseThreats).
func (g *Game) vaultTick(now uint64) {
	if len(g.vaults) == 0 || now%vaultCheckEvery != 0 {
		return
	}
	for i := range g.vaults {
		v := &g.vaults[i]
		switch v.State {
		case vault.Generated:
			g.vaultWarn(v)
		case vault.Breached, vault.CasketsOpened:
			g.vaultClearCheck(v)
		}
	}
}

// vaultWarn posts "Ominous walls" once, when a colonist stands within
// vaultWarnRange of the wall ring of a vault nobody has opened.
func (g *Game) vaultWarn(v *vaultSave) {
	for _, id := range g.pawnIDs {
		if !g.standingPawn(id) {
			continue
		}
		p, ok := g.positions.Get(id)
		if !ok || p.X < v.X0-vaultWarnRange || p.X > v.X1+vaultWarnRange || p.Y < v.Y0-vaultWarnRange || p.Y > v.Y1+vaultWarnRange {
			continue
		}
		v.State = vault.Warned
		g.letter(letterCaution, vaultWarnTitle, g.personName(id)+" stops near the smooth walls in the rock. Something old is sealed inside, and it does not feel empty.", id)
		return
	}
}

// vaultThreatsLeft reports a living defender or hostile occupant.
func (g *Game) vaultThreatsLeft(v *vaultSave) bool {
	for _, d := range v.Defenders {
		if a, ok := g.animals.Get(entity.EntityID(d)); ok && !a.Dead() && !a.Tame {
			return true
		}
	}
	for _, h := range v.Hostiles {
		id := entity.EntityID(h)
		if a, ok := g.animals.Get(id); ok {
			if !a.Dead() && !a.Tame {
				return true
			}
			continue
		}
		if g.standingRaider(id) {
			return true
		}
	}
	return false
}

// vaultClearCheck marks an opened vault Cleared once its caskets are open
// (or it had none) and no defender or hostile occupant is left.
func (g *Game) vaultClearCheck(v *vaultSave) {
	if v.State == vault.Breached && v.Caskets > 0 && g.sealedCaskets(v.ID) > 0 {
		return
	}
	if g.vaultThreatsLeft(v) {
		return
	}
	v.State = vault.Cleared
	g.addHistory("vault", "The sealed vault has been cleared.")
}

// vaultsRevealed is the breach check after a fog reveal (fogOpen): a
// vault not yet breached whose interior now shows is open.
func (g *Game) vaultsRevealed() {
	for i := range g.vaults {
		v := &g.vaults[i]
		if v.State >= vault.Breached {
			continue
		}
		if g.vaultInteriorShows(v) {
			g.breachVault(v)
		}
	}
}

// vaultInteriorShows reports an explored cell inside v's ring.
func (g *Game) vaultInteriorShows(v *vaultSave) bool {
	for y := v.Y0 + 1; y < v.Y1; y++ {
		for x := v.X0 + 1; x < v.X1; x++ {
			if !g.fogged(x, y) {
				return true
			}
		}
	}
	return false
}

// breachVault opens v: its defenders wake, a letter and a History line.
// The caskets stay sealed.
func (g *Game) breachVault(v *vaultSave) {
	v.State = vault.Breached
	woke := 0
	for _, d := range v.Defenders {
		id := entity.EntityID(d)
		a, ok := g.animals.Get(id)
		if !ok || a.Dead() {
			continue
		}
		if a.Dormant {
			g.wakeDormant(&a)
			g.animals.Set(id, a)
		}
		woke++
	}
	g.invalidateCreatures()
	x, y := v.centre()
	m := MessageView{Tick: uint64(g.clock.Now()), Text: vaultQuietText, X: x, Y: y, Letter: true, Title: vaultBreachTitle, Kind: letterInfo}
	if woke > 0 {
		m.Text, m.Danger, m.Kind = vaultBreachText, true, letterThreat
	}
	g.post(m)
	g.addHistory("vault", "The colony broke into a sealed vault.")
}

// vaultOf is the vault with id (nil: none).
func (g *Game) vaultOf(id int) *vaultSave {
	for i := range g.vaults {
		if g.vaults[i].ID == id {
			return &g.vaults[i]
		}
	}
	return nil
}

// fillDormantView shows a dormant defender asleep.
func fillDormantView(av *AnimalView, a animal.State) {
	if !a.Dormant {
		return
	}
	av.Dormant, av.Sleeping, av.Action, av.State = true, true, "", "dormant"
}

// exportVaults is the vaults for a save (nil: none).
func (g *Game) exportVaults() []vaultSave { return cloneVaults(g.vaults) }

// importVaults restores saved vaults; an older save has none.
func (g *Game) importVaults(vs []vaultSave) { g.vaults = cloneVaults(vs) }

func cloneVaults(vs []vaultSave) []vaultSave {
	if len(vs) == 0 {
		return nil
	}
	out := slices.Clone(vs)
	for i := range out {
		out[i].Defenders = slices.Clone(out[i].Defenders)
		out[i].Hostiles = slices.Clone(out[i].Hostiles)
	}
	return out
}
