package scenario

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/world/plant"
)

// WarmupTicks is the grown colony's warm-up before it is saved: two
// in-game days, so jobs, hauling, growth, dirt and wildlife have settled.
const WarmupTicks = 2 * clock.TicksPerDay

// RaidSize is the raid the raid variant receives right after warm-up.
const RaidSize = 12

// MinSide is the smallest map side the grown colony layout fits.
const MinSide = 110

type key struct {
	seed uint64
	w, h int
	raid bool
}

type memo struct {
	once sync.Once
	data []byte
	err  error
}

var (
	memoMu sync.Mutex
	memos  = map[key]*memo{}
)

// CacheDir, when set (benchmark runners, from a flag or environment
// variable), keeps built fixtures on disk across processes, keyed by the
// arguments and the save version. A stale cache is the caller's to clear
// whenever behaviour changes on purpose (the golden hashes re-pinned).
var CacheDir string

// CacheFile is the file under CacheDir that holds a fixture.
func CacheFile(seed uint64, w, h int, raid bool) string { return key{seed, w, h, raid}.file() }

func (k key) file() string {
	raid := ""
	if k.raid {
		raid = "-raid"
	}
	return filepath.Join(CacheDir, fmt.Sprintf("grown-v%d-s%d-%dx%d%s.sav", game.SaveVersion, k.seed, k.w, k.h, raid))
}

// GrownColony returns the save bytes of a grown colony on a w x h map
// generated from seed (see Build), after WarmupTicks; with raid, Raid
// (RaidSize) is called after warm-up. The result is built once per
// argument set per process and shared: callers must not modify it.
func GrownColony(seed uint64, w, h int, raid bool) []byte {
	b, err := grownColony(seed, w, h, raid)
	if err != nil {
		panic(err)
	}
	return b
}

func grownColony(seed uint64, w, h int, raid bool) ([]byte, error) {
	k := key{seed, w, h, raid}
	memoMu.Lock()
	m, ok := memos[k]
	if !ok {
		m = &memo{}
		memos[k] = m
	}
	memoMu.Unlock()
	m.once.Do(func() {
		if CacheDir != "" {
			if b, err := os.ReadFile(k.file()); err == nil {
				m.data = b
				return
			}
		}
		g, _, err := Build(seed, w, h)
		if err != nil {
			m.err = err
			return
		}
		g.Step(WarmupTicks)
		if raid {
			g.Raid(RaidSize)
		}
		var buf bytes.Buffer
		if err := g.Save(&buf, fmt.Sprintf("grown %dx%d seed %d", w, h, seed)); err != nil {
			m.err = err
			return
		}
		m.data = buf.Bytes()
		if CacheDir != "" {
			m.err = writeAtomic(k.file(), m.data)
		}
	})
	return m.data, m.err
}

// Load restores fixture bytes into a fresh game.
func Load(data []byte) (*game.Game, error) {
	g, _, err := game.Load(bytes.NewReader(data))
	return g, err
}

// Counts reports what Build placed (tests and the benchmark runner).
type Counts struct {
	Colonists, Tame, Buildings, Stacks, GrowCells, Mines, Blueprints int
}

// Build lays out the grown colony on a fresh generated map, before any
// warm-up. Everything goes through public dev hooks (BuildNow,
// SpawnNamedPawn, SpawnStack, SpawnAnimal, TameAnimal, SetGround,
// ClearPlants, Injure) and typed commands (zones and designations, applied
// on the first tick). The layout is fixed-size around the map centre:
//
//   - 20 colonists (the 3 of the start kit plus 17), some armed;
//   - 15 walled rooms with doors, furnished, one floored (~430
//     buildings), plus the pen's ~80 fence cells;
//   - a fenced pen with a gate, two troughs and 50 tame animals;
//   - 1 000 item stacks, 400 of them inside a stockpile zone;
//   - 600 grow-zone cells, half sown;
//   - a granite outcrop with 200 mine designations;
//   - 50 wall blueprints;
//   - three lightly wounded colonists (blood on the ground).
//
// Wildlife is whatever the generated map spawns (the biome's cap).
func Build(seed uint64, w, h int) (*game.Game, Counts, error) {
	b, err := build(seed, w, h)
	if err != nil {
		return nil, Counts{}, err
	}
	return b.g, b.n, nil
}

type builder struct {
	g      *game.Game
	cx, cy int
	n      Counts
}

func build(seed uint64, w, h int) (*builder, error) {
	if w < MinSide || h < MinSide {
		return nil, fmt.Errorf("grown colony: map %dx%d smaller than %d", w, h, MinSide)
	}
	g := game.NewColony(game.Config{Seed: seed, Width: w, Height: h})
	b := &builder{g: g, cx: w / 2, cy: h / 2}
	g.CompleteResearch()
	cx, cy := b.cx, b.cy
	// level the colony site: soil, no plants; a granite outcrop south-east
	g.SetGround(cx-52, cy-30, cx+47, cy+29, "soil", "", "")
	g.ClearPlants(cx-52, cy-30, cx+47, cy+29)
	g.SetGround(cx+25, cy+5, cx+44, cy+24, "rock", "granite", "")
	g.SetGround(cx+40, cy+20, cx+44, cy+24, "rock", "granite", "steel")
	if err := b.rooms(); err != nil {
		return nil, err
	}
	b.pen()
	b.stacks()
	b.fields()
	b.mines()
	b.blueprints()
	b.colonists()
	return b, nil
}

func (b *builder) buildNow(kind, stuff string, x, y int) {
	if b.g.BuildNow(kind, stuff, "south", x, y) {
		b.n.Buildings++
	}
}

// rooms: a 5 x 3 block of 7 x 7 stone rooms (outer walls), doors facing
// south, furnished by role.
func (b *builder) rooms() error {
	for j := 0; j < 3; j++ {
		for i := 0; i < 5; i++ {
			x0, y0 := b.cx-20+i*8, b.cy-28+j*8
			x1, y1 := x0+6, y0+6
			for x := x0; x <= x1; x++ {
				for y := y0; y <= y1; y++ {
					if x != x0 && x != x1 && y != y0 && y != y1 {
						continue
					}
					if x == x0+3 && y == y1 {
						b.buildNow("door", "wood", x, y)
						continue
					}
					b.buildNow("wall", "granite blocks", x, y)
				}
			}
			b.furnish(i+j*5, x0+1, y0+1)
		}
	}
	if b.n.Buildings < 360 {
		return fmt.Errorf("grown colony: only %d room buildings placed", b.n.Buildings)
	}
	return nil
}

// furnish fills the 5 x 5 interior at (x, y) by room role.
func (b *builder) furnish(room, x, y int) {
	switch room % 5 {
	case 0, 1: // bedrooms
		b.buildNow("bed", "wood", x, y)
		b.buildNow("bed", "wood", x+2, y)
		b.buildNow("bed", "wood", x+4, y)
		b.buildNow("lamp", "wood", x+4, y+4)
	case 2: // kitchen and dining
		b.buildNow("stove", "wood", x, y)
		b.buildNow("table", "wood", x+2, y+2)
		b.buildNow("chair", "wood", x+1, y+2)
		b.buildNow("chair", "wood", x+4, y+2)
	case 3: // workshop
		b.buildNow("workbench", "wood", x, y)
		b.buildNow("tailoring bench", "wood", x+3, y)
		b.buildNow("research bench", "wood", x, y+3)
	case 4: // storerooms; the first has a floor
		if room != 4 {
			return
		}
		for dy := 0; dy < 5; dy++ {
			for dx := 0; dx < 5; dx++ {
				b.buildNow("floor", "wood", x+dx, y+dy)
			}
		}
	}
}

// pen: a fenced 21 x 19 field east of the rooms with a gate, two troughs
// and 50 tame animals.
func (b *builder) pen() {
	x0, y0, x1, y1 := b.cx+25, b.cy-28, b.cx+45, b.cy-10
	for x := x0; x <= x1; x++ {
		for y := y0; y <= y1; y++ {
			if x != x0 && x != x1 && y != y0 && y != y1 {
				continue
			}
			if x == (x0+x1)/2 && y == y1 {
				b.buildNow("fence gate", "wood", x, y)
				continue
			}
			b.buildNow("fence", "wood", x, y)
		}
	}
	b.g.Enqueue(command.Command{Kind: command.DesignatePen, X: x0 + 1, Y: y0 + 1, X2: x1 - 1, Y2: y1 - 1})
	b.buildNow("feeding trough", "wood", x0+2, y0+2)
	b.buildNow("feeding trough", "wood", x0+2, y0+6)
	species := []string{"dustbuck", "dustbuck", "ridgewolf", "dustbuck", "dustbuck"}
	for i := 0; i < 50; i++ {
		x, y := x0+5+i%10, y0+3+i/10*2
		id, err := b.g.SpawnAnimal(species[i%len(species)], x, y)
		if err != nil {
			continue
		}
		b.g.TameAnimal(id, fmt.Sprintf("Pet %d", i+1))
		b.n.Tame++
	}
}

// stacks: one stack per tile over a 40 x 25 yard south of the rooms; the
// northern 40 x 10 is a stockpile, the rest waits to be hauled.
func (b *builder) stacks() {
	x0, y0 := b.cx-20, b.cy+3
	b.g.Enqueue(command.Command{Kind: command.DesignateStockpile, X: x0, Y: y0, X2: x0 + 39, Y2: y0 + 9})
	kinds := []item.Kind{item.Wood, item.GraniteBlocks, item.Berries, item.Meal, item.Leather, item.Steel, item.Food, item.Cloth}
	for i := 0; i < 1000; i++ {
		x, y := x0+i%40, y0+i/40
		if _, err := b.g.SpawnStack(kinds[i%len(kinds)], 5+i%26, x, y); err == nil {
			b.n.Stacks++
		}
	}
}

// fields: a 25 x 24 grow zone west of the rooms (600 cells), the northern
// half sown at mixed growth.
func (b *builder) fields() {
	x0, y0 := b.cx-50, b.cy-28
	potato, _ := plant.Parse("potato")
	b.g.Enqueue(command.Command{Kind: command.DesignateGrowZone, X: x0, Y: y0, X2: x0 + 24, Y2: y0 + 23, Build: uint8(potato)})
	b.n.GrowCells = 25 * 24
	for y := y0; y < y0+12; y++ {
		for x := x0; x < x0+25; x++ {
			_, _ = b.g.SpawnCrop("potato", x, y, (x*7+y*13)%90)
		}
	}
}

// mines: 200 mine designations on the outcrop's northern half.
func (b *builder) mines() {
	b.g.Enqueue(command.Command{Kind: command.DesignateMine, X: b.cx + 25, Y: b.cy + 5, X2: b.cx + 44, Y2: b.cy + 14})
	b.n.Mines = 20 * 10
}

// blueprints: 50 granite wall blueprints in two rows south-west.
func (b *builder) blueprints() {
	wall, stuff := buildID("wall"), stuffID("granite blocks")
	for i := 0; i < 50; i++ {
		x, y := b.cx-50+i%25, b.cy+8+i/25*4
		b.g.Enqueue(command.Command{Kind: command.DesignateBuild, X: x, Y: y, Build: wall, Stuff: stuff})
	}
	b.n.Blueprints = 50
}

// colonists: 17 more beside the start kit's three, a third armed, three
// lightly wounded (bleeding: stains).
func (b *builder) colonists() {
	b.n.Colonists = len(b.g.Snapshot())
	for i := 0; i < 17; i++ {
		x, y := b.cx-10+i, b.cy+1
		if i >= 8 && i <= 10 {
			y = b.cy + 2 // the start kit stands at cx-1..cx+1
		}
		if _, err := b.g.SpawnNamedPawn(x, y, fmt.Sprintf("Settler %d", i+1)); err == nil {
			b.n.Colonists++
		}
	}
	weapons := []item.Kind{item.Rifle, item.Spear, item.Knife, item.Club}
	for i, p := range b.g.Snapshot() {
		if i%3 == 0 {
			b.g.Arm(p.ID, weapons[i/3%len(weapons)])
		}
		if i%7 == 1 {
			b.g.Injure(p.ID, 30)
		}
	}
}

func buildID(name string) uint8 {
	for _, bv := range game.Buildables() {
		if bv.Kind == name {
			return bv.ID
		}
	}
	panic("scenario: unknown buildable " + name)
}

func stuffID(name string) uint8 {
	id, ok := game.StuffID(name)
	if !ok {
		panic("scenario: unknown stuff " + name)
	}
	return id
}

func writeAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("fixture cache: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("fixture cache: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("fixture cache: %w", err)
	}
	return nil
}
