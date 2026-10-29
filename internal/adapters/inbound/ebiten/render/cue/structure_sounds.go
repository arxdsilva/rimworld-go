package cue

import (
	"sort"
	"strconv"
	"strings"

	"rimworld-go/internal/adapters/inbound/ebiten/camera"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/entity"
)

// Wave D structure sounds (DEF-3, PRD-3): bench work loops, a turret's
// new barrel, a charge arming and the powered door. The turret and mortar
// shots go by weapon name (weaponVoices, combat_sounds.go).

const (
	// workPrefix starts PawnView.Acting while a crafter works at a bench
	// ("work at <bench kind>", ActX/ActY the bench).
	workPrefix = "work at "
	// rearmRise is how far (permille) a building's Barrel must climb in one
	// frame to count as a fresh barrel, not wear creeping the other way.
	rearmRise = 200
	// iedMemoMax bounds the remembered charges.
	iedMemoMax = 1024
)

var autodoorKind = construction.Autodoor.String()

// benchLoops is the work loop of each bench kind, keyed by the pawn's
// Acting while it works there.
var benchLoops = map[string]string{
	workPrefix + construction.FueledSmithy.String():     "amb.bench_smithy",
	workPrefix + construction.ElectricSmithy.String():   "amb.bench_smithy",
	workPrefix + construction.MachiningTable.String():   "amb.bench_machining",
	workPrefix + construction.ElectricSmelter.String():  "amb.bench_smelter",
	workPrefix + construction.BiofuelRefinery.String():  "amb.bench_refinery",
	workPrefix + construction.ArtBench.String():         "amb.bench_sculpt",
	workPrefix + construction.FabricationBench.String(): "amb.bench_fabrication",
}

// iedKinds are the buried charges (their arming click).
var iedKinds = map[string]bool{
	construction.IEDHighExplosive.String(): true, construction.IEDIncendiary.String(): true,
	construction.IEDEMP.String(): true, construction.IEDSmoke.String(): true,
}

// benchWorker is a crafter at work: the loop it makes and the bench tile.
type benchWorker struct {
	id   string
	x, y int
}

// structureMemo is what the structure cues remember between frames. Its
// maps and slice are reused: nothing is allocated once warm.
type structureMemo struct {
	workers       []benchWorker
	barrels, next map[entity.EntityID]int // Barrel of each building, last frame and this
	ieds          map[entity.EntityID]bool
	seen          bool // a first frame was heard (what exists then is not new)
}

// listenStructures reads a frame for the structure cues: the crafters at
// work, a building whose Barrel climbed (a fresh barrel was fitted) and a
// charge that appeared (armed as it is set). The first frame only fills
// the memo.
func (b *Bus) listenStructures(w game.WorldView, pawns []game.PawnView) {
	m := &b.Struct
	m.workers = m.workers[:0]
	for i := range pawns {
		if !strings.HasPrefix(pawns[i].Acting, workPrefix) {
			continue
		}
		if id, ok := benchLoops[pawns[i].Acting]; ok {
			m.workers = append(m.workers, benchWorker{id, pawns[i].ActX, pawns[i].ActY})
		}
	}
	if m.barrels == nil {
		m.barrels, m.next, m.ieds = map[entity.EntityID]int{}, map[entity.EntityID]int{}, map[entity.EntityID]bool{}
	}
	if len(m.ieds) > iedMemoMax {
		clear(m.ieds)
	}
	clear(m.next)
	for i := range w.Buildings {
		bd := &w.Buildings[i]
		x, y := float64(bd.X)+0.5, float64(bd.Y)+0.5
		if bd.Barrel > 0 {
			if was, ok := m.barrels[bd.ID]; ok && bd.Barrel >= was+rearmRise {
				b.Cue("sfx.turret_rearm", x, y, 1)
			}
			m.next[bd.ID] = bd.Barrel
		}
		if iedKinds[bd.Kind] && !m.ieds[bd.ID] {
			m.ieds[bd.ID] = true
			if m.seen {
				b.Cue("sfx.ied_arm", x, y, 1)
			}
		}
	}
	m.barrels, m.next = m.next, m.barrels
	m.seen = true
}

// benchWork adds a positioned loop for each crafter at work, the nearest
// MaxMachineLoops of each sound (a row of smithies is one clang, not five).
func (b *Bus) benchWork(rv *render.View, want map[string]Loop, v camera.Viewport) {
	type heard struct {
		key, id  string
		vol, pan float64
	}
	var hs []heard
	for _, wk := range b.Struct.workers {
		if vol, pan, ok := Spatial(rv, float64(wk.x)+0.5, float64(wk.y)+0.5, v); ok {
			hs = append(hs, heard{"bench:" + strconv.Itoa(wk.x) + ":" + strconv.Itoa(wk.y), wk.id, vol, pan})
		}
	}
	sort.SliceStable(hs, func(i, j int) bool { return hs[i].vol > hs[j].vol })
	count := map[string]int{}
	for _, h := range hs {
		if count[h.id] == MaxMachineLoops {
			continue
		}
		count[h.id]++
		want[h.key] = Loop{ID: h.id, Vol: h.vol, Pan: h.pan}
	}
}
