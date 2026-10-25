// Package vegetation is the MAPGEN-2 "vegetation" pass (.cursor/PLAY013_PLAN.md
// §3.6): a vegetation density field with clearings, then wild plants placed
// hierarchically (trees in groves, shrubs in thickets, ground plants in
// patches) under local saturation, and the starting grass mask.
//
// The species pool and its commonality are the biome's Plants permille
// list, so every species keeps today's total (permille × fertile cells)
// while its plants cluster instead of spreading uniformly.
package vegetation

import (
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/terrain"
	"rimworld-go/pkg/noise2"
	"rimworld-go/pkg/randx"
)

// Vegetation is the "vegetation" pass (MAPGEN-2): writes VegDensity, Plants, Grass.
type Vegetation struct{}

// Name is the step's stream and label key.
func (Vegetation) Name() string { return "vegetation" }

const (
	// startClear keeps plants this far (Chebyshev) from the landing spot.
	startClear = 6
	// bucket is the side of a saturation and lookup bucket.
	bucket = 8
	// passes visit the shuffled cells again until the totals are met: each
	// doubles the acceptance and loosens the bucket cap.
	passes = 6
	// pCap is the highest first-pass plant chance of one cell.
	pCap = 0.7
	// sandGrass is sand's grass weight: dunes stay mostly bare.
	sandGrass = 0.25
	// patchFade is the share of the map over which a group's patches fade
	// out to their floor.
	patchFade = 0.12
)

// Each group's patches cover patchShare of the land at full strength and
// fall to patchFloor elsewhere: trees in groves with few lone trees,
// shrubs in looser thickets, ground plants nearly everywhere.
var (
	patchShare = [numGroups]float64{0.22, 0.3, 0.4}
	patchFloor = [numGroups]float64{0, 0.06, 0.15}
)

// spot is a placed plant in the lookup buckets.
type spot struct {
	x, y int32
	sp   uint8 // local species index + 1 (0: a plant from before this step)
}

// state is one run's working data.
type state struct {
	c   *mapgen.Ctx
	r   *randx.Rand
	b   biome.Def
	occ []uint8 // 0 empty, else group + 1
	bw  int     // buckets per row
	// lists holds each bucket's placed plants (indices into spots).
	lists [][]int32
	spots []spot
	// species is the biome's pool in list order.
	species []specState
	gQuota  [numGroups]int
	gCount  [numGroups]int
	placed  int
	total   int // every species' quota
}

type specState struct {
	s     plant.Species
	g     group
	quota int
	count int
	tune  speciesTune
	in    []float32 // intensity on each eligible cell (first-pass chance)
}

// Run is the pass.
func (v Vegetation) Run(c *mapgen.Ctx) error {
	if c.W <= 0 || c.H <= 0 {
		return nil
	}
	st := &state{c: c, r: c.Rand(v.Name()), b: c.Cfg.Biome}
	d := dataOf(st.b.ID)
	n := c.W * c.H
	st.occ = make([]uint8, n)
	st.bw = (c.W + bucket - 1) / bucket
	st.lists = make([][]int32, st.bw*((c.H+bucket-1)/bucket))
	for _, p := range c.R.Plants {
		if c.In(p.X, p.Y) {
			st.occ[c.Idx(p.X, p.Y)] = uint8(groupOf(p.Species)) + 1
			st.add(p.X, p.Y, 0)
		}
	}

	growable := make([]bool, n)
	var elig []int32
	for i := range n {
		growable[i] = st.growable(i)
		if growable[i] && st.occ[i] == 0 && !c.F.Blocked[i] && !c.F.Road[i] && !c.F.Cave[i] && !st.nearStart(i) {
			elig = append(elig, int32(i))
		}
	}
	st.density(d, growable)
	st.plants(d, elig)
	st.grass()
	return nil
}

// growable reports ground anything grows on: fertile, dry, open land.
func (st *state) growable(i int) bool {
	c, t := st.c, st.c.R.Terrain.Cells[i]
	return t.Fertility() > 0 && !terrain.IsWater(t) && !c.F.Rock[i] && !c.F.Water[i].Wet()
}

func (st *state) nearStart(i int) bool {
	x, y := st.c.XY(i)
	s := st.c.R.Start
	return max(abs(x-s.X), abs(y-s.Y)) <= startClear
}

// density writes F.VegDensity = Density × suitability × C on growable
// cells (0 elsewhere). C is a smooth canopy field times a warped clearing
// mask that is 0 on the Clearing share of the map.
func (st *state) density(d biomeVeg, growable []bool) {
	c := st.c
	s := c.Scale()
	canopy := c.NoiseSeed("vegetation/canopy")
	clearSeed, warpSeed := c.NoiseSeed("vegetation/clearing"), c.NoiseSeed("vegetation/clearwarp")
	cv := make([]float32, c.W*c.H)
	var sample []float32
	for i := range cv {
		if !growable[i] {
			continue
		}
		x, y := c.XY(i)
		wx, wy := noise2.Warp(warpSeed, float64(x), float64(y), 12*s, 30*s)
		cv[i] = float32(noise2.FBm(clearSeed, wx, wy, 2, 28*s, 0.5))
		if x%3 == 0 && y%3 == 0 {
			sample = append(sample, cv[i])
		}
	}
	q := float64(mapgen.Quantile(sample, d.Clearing))
	if d.Clearing <= 0 {
		q = -1
	}
	vd := c.F.VegDensity
	for i := range vd {
		if !growable[i] {
			vd[i] = 0
			continue
		}
		x, y := c.XY(i)
		smooth := stretch(noise2.FBm(canopy, float64(x), float64(y), 3, 45*s, 0.5), 0.1, 0.25)
		cm := (0.25 + 0.75*smooth) * mapgen.Smoothstep(q, q+0.035, float64(cv[i]))
		vd[i] = float32(mapgen.Clamp01(d.Density * st.suit(d, i) * cm))
	}
}

// stretch maps a noise value (mean 0.5, deviation sd) to [0, 1] with
// k per deviation around 0.5.
func stretch(v, sd, k float64) float64 { return mapgen.Clamp01(0.5 + (v-0.5)/sd*k) }

// suit is a cell's suitability in [0, 1]: soil fertility, the fertility
// field and the biome's lean toward wet or dry ground.
func (st *state) suit(d biomeVeg, i int) float64 {
	t := st.c.R.Terrain.Cells[i]
	soil := 0.25 + 0.75*min(float64(t.Fertility())/140, 1)
	if t.Wetness() > 0 && d.WetPref > 0 {
		soil = max(soil, 0.5+0.5*d.WetPref) // marsh and mud carry swamp growth
	}
	fert := 0.35 + 0.65*float64(st.c.F.Fertility[i])
	w := float64(st.c.F.Wetness[i])
	wet := min(2, max(0.1, 1+d.WetPref*(2*w-1))) / (1 + abs(d.WetPref))
	return mapgen.Clamp01(soil * fert * wet)
}

func abs[T int | float64](v T) T {
	if v < 0 {
		return -v
	}
	return v
}

// solve finds k with Σ min(hi, k·w) = target over w (w ≥ 0), to a
// relative 1e-4 by bisection; 0 when the target or the weights are 0.
func solve(w []float32, target, hi float64) float64 {
	tot := 0.0
	for _, v := range w {
		tot += float64(v)
	}
	if target <= 0 || tot <= 0 {
		return 0
	}
	sum := func(k float64) float64 {
		s := 0.0
		for _, v := range w {
			s += min(hi, k*float64(v))
		}
		return s
	}
	lo := target / tot // the clamp only lowers the sum: sum(lo) ≤ target
	if sum(lo) >= target*(1-1e-4) {
		return lo
	}
	up := 2 * lo
	for sum(up) < target && up < 1e12 {
		lo, up = up, up*2
	}
	for up-lo > 1e-4*up {
		mid := (lo + up) / 2
		if sum(mid) < target {
			lo = mid
		} else {
			up = mid
		}
	}
	return up
}

// plants places the biome's wild plants on the eligible cells.
func (st *state) plants(d biomeVeg, elig []int32) {
	c, r := st.c, st.r
	e := len(elig)
	total := 0
	for _, pd := range st.b.Plants {
		q := (pd.Permil*e + 500) / 1000
		if pd.Permil <= 0 || q <= 0 {
			continue
		}
		g := groupOf(pd.Species)
		st.species = append(st.species, specState{s: pd.Species, g: g, quota: q, tune: tuneOf(st.b.ID, pd.Species)})
		st.gQuota[g] += q
		total += q
	}
	st.total = total
	if total == 0 || e == 0 {
		return
	}
	// A species' intensity is VegDensity × its group's patch field (trees
	// in groves, shrubs in thickets) × its wetness preference, scaled to
	// its quota. A cell's first-pass chance is the sum over the species,
	// and a bucket's target the sum of those chances.
	vd := make([]float32, e)
	any := false
	for k, i := range elig {
		vd[k] = c.F.VegDensity[i]
		any = any || vd[k] > 0
	}
	if !any {
		for k := range vd {
			vd[k] = 1
		}
	}
	s := c.Scale()
	cells := [numGroups]float64{d.TreeCell, d.ShrubCell, d.GroundCell}
	names := [numGroups]string{"vegetation/trees", "vegetation/shrubs", "vegetation/ground"}
	var gf [numGroups][]float32
	for g := range numGroups {
		if st.gQuota[g] > 0 {
			gf[g] = make([]float32, e)
			st.groupField(gf[g], elig, c.NoiseSeed(names[g]), cells[g]*s, g)
		}
	}
	w := make([]float32, e)
	for j := range st.species {
		sp := &st.species[j]
		sp.in = make([]float32, e)
		for k, i := range elig {
			sp.in[k] = vd[k] * gf[sp.g][k] * float32(wetPref(sp.tune, float64(c.F.Wetness[i])))
		}
		kj := solve(sp.in, float64(sp.quota), pCap)
		for k, v := range sp.in {
			sp.in[k] = float32(min(pCap, kj*float64(v)))
			w[k] += sp.in[k]
		}
	}
	bt := make([]float32, len(st.lists))
	for k, i := range elig {
		w[k] = min(pCap, w[k])
		x, y := c.XY(int(i))
		bt[st.bucketOf(x, y)] += w[k]
	}
	bc := make([]int32, len(st.lists))
	for _, sp := range st.spots {
		bc[st.bucketOf(int(sp.x), int(sp.y))]++
	}
	// shuffled visiting order (positions in elig)
	order := make([]int32, e)
	for k := range order {
		order[k] = int32(k)
	}
	for k := e - 1; k > 0; k-- {
		j := r.IntN(k + 1)
		order[k], order[j] = order[j], order[k]
	}
	for pass := range passes {
		if st.placed >= total {
			break
		}
		mult := float64(int(1) << pass)
		capMul := 1 + 0.3*float64(pass)
		for _, k := range order {
			i := int(elig[k])
			if st.occ[i] != 0 {
				continue
			}
			x, y := c.XY(i)
			b := st.bucketOf(x, y)
			if float64(bc[b]) >= float64(bt[b])*capMul+1 {
				continue // saturated
			}
			if r.Float64() >= float64(w[k])*mult {
				continue
			}
			sp := st.choose(x, y, int(k))
			if sp < 0 {
				continue
			}
			st.place(x, y, sp)
			bc[b]++
			if st.placed >= total {
				break
			}
		}
	}
}

// fits is the spacing rule: no tree beside a tree (4-neighbourhood), and
// no tree or shrub touching a shrub (8-neighbourhood); ground plants have
// none.
func (st *state) fits(x, y int, g group) bool {
	if g == gGround {
		return true
	}
	c := st.c
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if dx == 0 && dy == 0 || !c.In(x+dx, y+dy) {
				continue
			}
			o := st.occ[c.Idx(x+dx, y+dy)]
			if o == 0 || group(o-1) == gGround {
				continue
			}
			if g == gShrub || group(o-1) == gShrub || dx == 0 || dy == 0 {
				return false
			}
		}
	}
	return true
}

// choose picks the species for (x, y) in two draws. The group: the sum
// of its species' intensities here, each × its cluster boost (×(1 +
// ClusterW) with its own kind within ClusterR, so lone trees still come in
// small clumps), × the group's balance (above its share of the plants so
// far ×0.5, below ×1.5). A group that does not fit the spacing rule leaves
// the cell empty for a later pass, which keeps groves dense. Then the
// species: intensity × cluster boost × its balance within the group.
// -1 when nothing grows there.
func (st *state) choose(x, y, k int) int {
	var ws [16]float64
	wsl := ws[:0]
	if len(st.species) > len(ws) {
		wsl = make([]float64, 0, len(st.species))
	}
	var gw [numGroups]float64
	any := false
	for j, sp := range st.species {
		wt := 0.0
		if sp.count < sp.quota && sp.in[k] > 0 {
			wt = float64(sp.in[k])
			if sp.tune.ClusterW > 0 && st.near(x, y, uint8(j+1), sp.tune.ClusterR) {
				wt *= 1 + sp.tune.ClusterW
			}
			gw[sp.g] += wt
			any = true
		}
		wsl = append(wsl, wt)
	}
	if !any {
		return -1
	}
	for g := range numGroups {
		gw[g] *= balance(st.gCount[g], st.placed, st.gQuota[g], st.total)
	}
	g := group(pick(st.r, gw[:]))
	if !st.fits(x, y, g) {
		return -1
	}
	for j, sp := range st.species {
		if sp.g != g {
			wsl[j] = 0
		} else {
			wsl[j] *= balance(sp.count, st.gCount[g], sp.quota, st.gQuota[g])
		}
	}
	return pick(st.r, wsl)
}

// balance is ×0.5 when count/of is above quota/total, else ×1.5.
func balance(count, of, quota, total int) float64 {
	if of > 0 && count*total > quota*of {
		return 0.5
	}
	return 1.5
}

// groupField fills f (per eligible cell) with group g's patch field: a
// smooth noise whose top patchShare[g] of the map is full strength,
// fading to patchFloor[g] elsewhere.
func (st *state) groupField(f []float32, elig []int32, seed uint64, cell float64, g group) {
	c := st.c
	var sample []float32
	for k, i := range elig {
		x, y := c.XY(int(i))
		f[k] = float32(noise2.FBm(seed, float64(x), float64(y), 2, cell, 0.5))
		if k%7 == 0 {
			sample = append(sample, f[k])
		}
	}
	hi := float64(mapgen.Quantile(sample, 1-patchShare[g]))
	lo := float64(mapgen.Quantile(sample, 1-patchShare[g]-patchFade))
	for k := range f {
		f[k] = float32(patchFloor[g] + (1-patchFloor[g])*mapgen.Smoothstep(lo, hi, float64(f[k])))
	}
}

// wetPref is 1 inside the species' wetness range, falling to 0.15 a
// quarter outside it.
func wetPref(t speciesTune, w float64) float64 {
	d := max(t.WetLo-w, w-t.WetHi, 0)
	return max(0.15, 1-d/0.25)
}

// near reports a plant of local species sp within rad (Chebyshev) of
// (x, y), through the buckets.
func (st *state) near(x, y int, sp uint8, rad int) bool {
	c := st.c
	bx0, by0 := max(0, x-rad)/bucket, max(0, y-rad)/bucket
	bx1, by1 := min(c.W-1, x+rad)/bucket, min(c.H-1, y+rad)/bucket
	for by := by0; by <= by1; by++ {
		for bx := bx0; bx <= bx1; bx++ {
			for _, pi := range st.lists[by*st.bw+bx] {
				p := st.spots[pi]
				if p.sp == sp && max(abs(int(p.x)-x), abs(int(p.y)-y)) <= rad {
					return true
				}
			}
		}
	}
	return false
}

func (st *state) bucketOf(x, y int) int { return (y/bucket)*st.bw + x/bucket }

func (st *state) add(x, y int, sp uint8) {
	b := st.bucketOf(x, y)
	st.lists[b] = append(st.lists[b], int32(len(st.spots)))
	st.spots = append(st.spots, spot{x: int32(x), y: int32(y), sp: sp})
}

// place puts local species k at (x, y) with a random starting growth:
// 10–100% grown, trees 25–100%.
func (st *state) place(x, y, k int) {
	sp := &st.species[k]
	def := plant.DefOf(sp.s)
	pct := 10 + st.r.IntN(91)
	if sp.g == gTree {
		pct = 25 + st.r.IntN(76)
	}
	st.c.R.Plants = append(st.c.R.Plants, mapgen.PlantSpawn{X: x, Y: y, Species: sp.s, Growth: def.GrowTicks * pct / 100})
	st.occ[st.c.Idx(x, y)] = uint8(sp.g) + 1
	st.add(x, y, uint8(k+1))
	sp.count++
	st.gCount[sp.g]++
	st.placed++
}

// pick is a weighted choice (weights ≥ 0, some > 0).
func pick(r *randx.Rand, w []float64) int {
	sum := 0.0
	for _, v := range w {
		sum += v
	}
	u := r.Float64() * sum
	last := 0
	for k, v := range w {
		if v <= 0 {
			continue
		}
		last = k
		if u < v {
			return k
		}
		u -= v
	}
	return last
}

// grass seeds the grass mask: the biome's grass share of the soft, dry,
// fertile, open cells, drawn more often where fertility, wetness and the
// vegetation field are high, and rarely on sand.
func (st *state) grass() {
	c := st.c
	permil := grassPermil(st.b)
	if permil <= 0 {
		return
	}
	var idx []int32
	var w []float32
	vmax := float32(0)
	for _, v := range c.F.VegDensity {
		vmax = max(vmax, v)
	}
	if vmax <= 0 {
		vmax = 1
	}
	for i := range c.R.Grass {
		t := c.R.Terrain.Cells[i]
		if t.Fertility() == 0 || t == terrain.Mud || terrain.IsWater(t) || c.F.Water[i].Wet() ||
			c.F.Blocked[i] || c.F.Rock[i] || c.F.Cave[i] {
			continue
		}
		wt := (0.3 + 0.7*float64(c.F.Fertility[i])) * (0.5 + 0.5*float64(c.F.Wetness[i])) *
			(0.35 + 0.65*float64(c.F.VegDensity[i]/vmax))
		if t == terrain.Sand {
			wt *= sandGrass
		}
		idx = append(idx, int32(i))
		w = append(w, float32(wt))
	}
	k := solve(w, float64(permil)*float64(len(idx))/1000, 1)
	for j, i := range idx {
		if st.r.Float64() < min(1, k*float64(w[j])) {
			c.R.Grass[i] = true
		}
	}
}
