package craft

import (
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/entity"
)

// Bench work speed (PRD-3, ENG-003). A bench works at its own factor
// (wiki: Crafting spot 0.5; the butcher spot works at full speed, its
// 70 % is yield: ButcherSpotYield) plus its linked facilities
// and an electric twin's bonus, times the light at its work spot, the
// air temperature there and its room's term.
const (
	// craftingSpotSpeed is the crafting spot's factor, permille (wiki:
	// Crafting spot).
	craftingSpotSpeed = 500
	// ButcherSpotYield is the butcher spot's meat and leather, permille of
	// the table's (FOOD-6, wiki: Butcher spot: 70 % work efficiency).
	ButcherSpotYield = 700
	// darkLight is the light (permille) below which work slows, down to
	// darkSpeed in full dark (wiki: Light: dark below 30 %, 0.8 at 0 %;
	// the straight ramp between is ours).
	darkLight = 300
	darkSpeed = 800
	// outdoorsSpeed is a bench outside any room or under the open sky
	// (wiki: Cleanliness: a production bench with no roof works at 0.8).
	outdoorsSpeed = 800
	// speedEvery is how many ticks a bench's speed is kept before it is
	// worked out again (ours: light changes slowly; the view stays put).
	// It is also the bad-temperature term's minimum stay: air wobbling
	// around 10 C moves a bench's speed at most once per window.
	speedEvery = 250
	// FOOD-4 (wiki: Temperature): a bench works at badTempSpeed outside
	// goodTempMin..goodTempMax C at its work spot.
	goodTempMin  = 10
	goodTempMax  = 35
	badTempSpeed = 700
)

// tempSpeed is the air temperature factor, permille, at t whole degrees.
func tempSpeed(t int) int {
	if t < goodTempMin || t > goodTempMax {
		return badTempSpeed
	}
	return 1000
}

// cleanSpeed is the room-cleanliness factor, permille, by the room's
// cleanliness band (very dirty, dirty, slightly dirty; clean and sterile
// are full speed). The wiki names cleanliness as a work-speed term but
// publishes no numbers: these are ours.
func cleanSpeed(hundredths int) int {
	switch {
	case hundredths < -110:
		return 850
	case hundredths < -40:
		return 900
	case hundredths < -5:
		return 950
	}
	return 1000
}

// lightSpeed is the light factor, permille, at light l (permille).
func lightSpeed(l int) int {
	if l >= darkLight {
		return 1000
	}
	return darkSpeed + (1000-darkSpeed)*max(0, l)/darkLight
}

// baseSpeed is bench kind k's own work speed, permille.
func baseSpeed(k construction.Kind) int {
	switch k {
	case construction.CraftingSpot:
		return craftingSpotSpeed
	}
	return 1000
}

// Stations is each bench's work speed (BuildingView.WorkSpeed) and the
// view lines of benches and sculptures; cached, never saved.
type Stations struct {
	h          Host
	facilities *Facilities
	art        *Art
	speed      map[entity.EntityID]speedRec
}

// speedRec is a bench's speed worked out at tick At for StructVer Ver.
type speedRec struct {
	At, Ver uint64
	V       int
}

// Speed is bench bid's work speed in permille (0: no bench): kept for
// speedEvery ticks or until the buildings change.
func (s *Stations) Speed(bid entity.EntityID) int {
	h := s.h
	now, ver := h.Now(), h.StructVer()
	if r, ok := s.speed[bid]; ok && r.Ver == ver && now >= r.At && now < r.At+speedEvery {
		return r.V
	}
	b, ok := h.Bench(bid)
	if !ok {
		return 0
	}
	v := s.compute(bid, b)
	if s.speed == nil || len(s.speed) > 4*len(h.BuildingIDs())+16 {
		s.speed = map[entity.EntityID]speedRec{} // dropped benches never pile up
	}
	s.speed[bid] = speedRec{At: now, Ver: ver, V: v}
	return v
}

// compute is bench b's speed now: its factor, facilities and electric
// bonus, then light, air temperature (FOOD-4) and room at the work spot.
func (s *Stations) compute(bid entity.EntityID, b construction.Building) int {
	h := s.h
	v := baseSpeed(b.Kind) + 10*(s.facilities.Bonus(bid)+ElectricPct(b.Kind))
	_, spot, ok := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
	if !ok {
		spot = [2]int{b.X, b.Y}
	}
	x, y := spot[0], spot[1]
	v = v * lightSpeed(h.Light(x, y)) / 1000 * tempSpeed(h.TemperatureAt(x, y)) / 1000
	if c, indoors := h.Cleanliness(x, y); indoors {
		return v * cleanSpeed(c) / 1000
	}
	return v * outdoorsSpeed / 1000
}

// View is building id's PRD-3 view fields: a bench's linked facilities
// and work speed (permille), and a sculpture's title in place of
// epitaph (any other building keeps the epitaph it has).
func (s *Stations) View(id entity.EntityID, b construction.Building, epitaph string) (linked []string, speed int, title string) {
	if construction.DefOf(b.Kind).Bench {
		return s.facilities.Linked(id), s.Speed(id), epitaph
	}
	if t := s.art.Title(id, b); t != "" {
		return nil, 0, t
	}
	return nil, 0, epitaph
}
