// Package farm is the growing rules the game's plant pass applies
// (FOOD-2): growing light from the sun, sun lamps and plain lamps, the
// blight's strike and spread, wild plants sprouting back to the biome's
// density, the ripening estimate and the plant death notices. It is a
// consumer sub-package of internal/application/game (.cursor/SOLID_GO.md
// §10): pure functions over the domain tables, nothing from the game;
// chances come from plant.Roll, never the simulation's RNG.
package farm

import (
	"math"

	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/plant"
)

// Lamp is a sun lamp shining now.
type Lamp struct{ X, Y int }

// Sight reports whether (x, y) can see a light at (sx, sy).
type Sight func(sx, sy, x, y int) bool

// GrowLight is the growing light (permille) on (x, y): full daylight
// under the open sky (the growing hours are daylight), the best sun lamp
// in sight, and a plain lamp or fire's light art (0..1) at most
// plant.LampGrowLight (wiki: Sun lamp; a standing lamp's 50 % grows
// nothing).
func GrowLight(roofed bool, art float64, lamps []Lamp, x, y int, sees Sight) int {
	if !roofed {
		return plant.FullGrowLight
	}
	l := min(int(art*1000+0.5), plant.LampGrowLight)
	for _, s := range lamps {
		if v := plant.SunLampLight(x-s.X, y-s.Y); v > l && sees(s.X, s.Y, x, y) {
			l = v
		}
	}
	return l
}

// RipeIn is the calendar ticks a plant needs growing ticks of still
// takes: plants grow only in the GrowFromHour..GrowToHour hours.
func RipeIn(active int) int {
	const hours = plant.GrowToHour - plant.GrowFromHour
	return (active*24 + hours - 1) / hours
}

// Blight (wiki: Blight): the strike infects plants of one species within
// StrikeRadius of the origin in its room, more surely near it; a plant at
// plant.BlightSpread severity passes it on to a plant within
// plant.BlightRadius once in SpreadOdds passes (our tuning).
const (
	StrikeRadius = 11
	SpreadOdds   = 48
)

// Struck reports a plant d2 squared tiles from the strike's origin
// catching blight on roll r: always at the origin, 30 % at the edge.
func Struck(d2 int, r uint64) bool {
	if d2 > StrikeRadius*StrikeRadius {
		return false
	}
	d := int(math.Sqrt(float64(d2)))
	return int(r%100) < 100-70*d/StrikeRadius
}

// SpreadTo is the cell offset a spreading plant passes blight to on roll
// r, ok false when it doesn't this pass.
func SpreadTo(r uint64) (dx, dy int, ok bool) {
	if r%SpreadOdds != 0 {
		return 0, 0, false
	}
	const span = 2*plant.BlightRadius + 1
	dx, dy = int(r/SpreadOdds%span)-plant.BlightRadius, int(r/SpreadOdds/span%span)-plant.BlightRadius
	ok = (dx != 0 || dy != 0) && dx*dx+dy*dy <= plant.BlightRadius*plant.BlightRadius
	return dx, dy, ok
}

// Wild plants sprout back (wiki: Plants, plants grow spontaneously on
// free tiles): a chunk's pass sprouts one once in SproutOdds while it
// holds fewer wild plants than its biome's plant permille of its tiles.
// Wild healroot sprouts in the biomes the wiki names (Healroot) at
// healrootPermil. Our tuning.
const (
	SproutOdds     = 60
	healrootPermil = 3
)

// healrootLands are the biomes wild healroot grows in (wiki: Healroot).
var healrootLands = biome.Of(biome.TemperateForest, biome.TemperateSwamp, biome.BorealForest, biome.ColdBog, biome.Tundra)

// Sprout picks this pass's wild plant in the chunk (x0, y0)-(x1, y1) of
// biome b holding wild plants: a species by the biome's weights and a
// cell, ok false when none sprouts.
func Sprout(b biome.ID, x0, y0, x1, y1, wild int, now uint64, chunk int) (s plant.Species, x, y int, ok bool) {
	r := plant.Roll(now, uint64(chunk), 0x5eed)
	if r%SproutOdds != 0 || x1 <= x0 || y1 <= y0 {
		return 0, 0, 0, false
	}
	ps := biome.DefOf(b).Plants
	total := 0
	for _, p := range ps {
		total += p.Permil
	}
	heal := 0
	if healrootLands.Has(b) {
		heal = healrootPermil
	}
	area := (x1 - x0) * (y1 - y0)
	if total+heal == 0 || wild*1000 >= area*(total+heal) {
		return 0, 0, 0, false
	}
	r /= SproutOdds
	pick := int(r % uint64(total+heal))
	r /= uint64(total + heal)
	s = plant.Healroot
	for _, p := range ps {
		if pick < p.Permil {
			s = p.Species
			break
		}
		pick -= p.Permil
	}
	i := int(r % uint64(area))
	return s, x0 + i%(x1-x0), y0 + i/(x1-x0), true
}

// Field is the game's plants as the blight sees them (a consumer-owned
// role).
type Field interface {
	PlantIDs() []entity.EntityID
	Plant(id entity.EntityID) (plant.Plant, bool)
	SetPlant(id entity.EntityID, p plant.Plant)
	PlantAt(x, y int) entity.EntityID
	RoomAt(x, y int) int
}

// Origin picks the plant the blight incident strikes first: a healthy
// blightable crop (ok false: none, so the incident can't fire).
func Origin(f Field, now uint64) (entity.EntityID, bool) {
	var cands []entity.EntityID
	for _, id := range f.PlantIDs() {
		if p, ok := f.Plant(id); ok && p.Blightable() && p.Blight == 0 {
			cands = append(cands, id)
		}
	}
	if len(cands) == 0 {
		return 0, false
	}
	return cands[plant.Roll(now, uint64(len(cands)), 0xb2)%uint64(len(cands))], true
}

// Strike is the blight incident (wiki: Blight): plants of the origin's
// species within StrikeRadius in its room catch it at plant.BlightStart;
// it returns the origin and how many caught it (ok false: no origin).
func Strike(f Field, now uint64) (o plant.Plant, n int, ok bool) {
	oid, ok := Origin(f, now)
	if !ok {
		return o, 0, false
	}
	o, _ = f.Plant(oid)
	room := f.RoomAt(o.X, o.Y)
	for _, id := range f.PlantIDs() {
		p, ok := f.Plant(id)
		dx, dy := p.X-o.X, p.Y-o.Y
		if ok && p.Species == o.Species && p.Blightable() && p.Blight == 0 && f.RoomAt(p.X, p.Y) == room &&
			Struck(dx*dx+dy*dy, plant.Roll(now, uint64(id), 0xb3)) {
			p.Blight = plant.BlightStart
			f.SetPlant(id, p)
			n++
		}
	}
	return o, n, true
}

// Spread passes plant id's blight (p) to a healthy blightable plant
// within plant.BlightRadius once it is past plant.BlightSpread.
func Spread(f Field, id entity.EntityID, p plant.Plant, now uint64) {
	if p.Blight < plant.BlightSpread {
		return
	}
	dx, dy, ok := SpreadTo(plant.Roll(now, uint64(id), 0xb1))
	if !ok {
		return
	}
	tid := f.PlantAt(p.X+dx, p.Y+dy)
	if q, ok := f.Plant(tid); ok && q.Blightable() && q.Blight == 0 {
		q.Blight = plant.BlightStart
		f.SetPlant(tid, q)
	}
}
