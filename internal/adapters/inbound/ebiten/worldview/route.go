package worldview

import (
	"fmt"
	"math"
	"strconv"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/world/planet"
)

// Caravan route mode (MAIN_UI §33 Routing, §35 change destination): the
// caravan's Travel gizmo enters it; a click on a tile plans the cheapest
// overland path (planet.Route), drawn on the globe with the time it takes
// (game.TravelEstimate) and how the seasons change it; Confirm sends
// CaravanTravel.

// Route bar ids.
const (
	ConfirmID = "world.act.confirm"
	CancelID  = "world.act.cancel"
	routeBarW = 470
	routeBarH = 76
	// seasonGap is how much slower (a share) the worst season must be
	// before the note mentions it (original tuning).
	seasonGap = 0.1
)

// routeState is the route being planned: the caravan, its tile, the
// target tile (-1 none yet), the path and its texts.
type routeState struct {
	on      bool
	caravan uint64
	from    int
	to      int
	path    []int
	eta     string // "about 3 days", "—" while unknown
	note    string // the season and terrain line
	why     string // why the caravan cannot go there ("" it can)
}

// startRoute enters route mode for caravan id standing on tile from.
func (v router) startRoute(id uint64, from int) {
	v.pods = podState{}
	v.route = routeState{on: true, caravan: id, from: from, to: -1, eta: "—"}
}

// setRouteTarget plans the route to tile t.
func (v router) setRouteTarget(t int) {
	r := &v.route
	p := v.chart.planet
	r.to, r.path, r.why, r.eta, r.note = t, nil, "", "—", ""
	if p == nil {
		r.why = noMap
		return
	}
	if t == r.from {
		r.why = "The caravan is already there."
		return
	}
	r.path = p.Route(r.from, t)
	if r.path == nil {
		r.why = "No overland route reaches that tile."
		return
	}
	if est, ok := v.sh.Env().Game.(Estimator); ok {
		ticks, why := est.TravelEstimate(r.caravan, t)
		if why != "" {
			r.why = why
		} else {
			r.eta = durationText(ticks)
		}
	}
	r.note = seasonNote(p, r.path, v.dayOfYear())
}

// etaCache is the inspector's last travel time and distance: an estimate
// routes over the planet, too dear for every frame.
type etaCache struct {
	ok         bool
	caravan    uint64
	from, tile int
	bucket     clock.Tick
	ticks      int
	why        string
	known      bool // the game gives estimates
	hops       int
}

// etaTicks is how long (game ticks) a shown estimate stays.
const etaTicks = 250

// eta is caravan c on tile from's estimate to tile: the time text ("—"
// without an estimator), why it cannot go ("" it can), and the hops.
func (v *View) eta(c uint64, from, tile int) (text, why string, hops int) {
	e := &v.etaC
	b := v.sh.Tick() / etaTicks
	if !e.ok || e.caravan != c || e.from != from || e.tile != tile || e.bucket != b {
		*e = etaCache{ok: true, caravan: c, from: from, tile: tile, bucket: b}
		if est, ok := v.sh.Env().Game.(Estimator); ok {
			e.known = true
			e.ticks, e.why = est.TravelEstimate(c, tile)
		}
		if p := v.chart.planet; p != nil {
			e.hops = p.HopsWith(&v.hops, from, tile, maxHops)
		}
	}
	if !e.known {
		return "—", "", e.hops
	}
	if e.why != "" {
		return "—", e.why, e.hops
	}
	return durationText(e.ticks), "", e.hops
}

// maxHops caps the settlement inspector's distance count (original tuning).
const maxHops = 200

// dayOfYear is today's day of the year.
func (v *View) dayOfYear() int {
	d := v.sh.Date()
	return d.Quadrum*clock.DaysPerQuadrum + d.Day
}

// durationText is a travel time ("about 9 hours", "about 2.5 days").
func durationText(ticks int) string {
	h := float64(ticks) / clock.TicksPerHour
	switch {
	case h < 1:
		return "under an hour"
	case h < 48:
		return fmt.Sprintf("about %d hours", int(math.Round(h)))
	}
	return "about " + strconv.FormatFloat(h/24, 'f', 1, 64) + " days"
}

// routeCost is the summed movement difficulty of the tiles entered along
// path on day of the year day.
func routeCost(p *planet.Planet, path []int, day int) float64 {
	s := 0.0
	for _, t := range path[1:] {
		s += p.MoveDifficulty(t, day)
	}
	return s
}

// seasonNote is the route's terrain and season line: the mean movement
// difficulty now, and how much slower the worst quadrum makes it.
func seasonNote(p *planet.Planet, path []int, day int) string {
	if len(path) < 2 {
		return ""
	}
	now := routeCost(p, path, day)
	best, worst := now, now
	for q := 0; q < clock.QuadrumsPerYear; q++ {
		c := routeCost(p, path, q*clock.DaysPerQuadrum+clock.DaysPerQuadrum/2)
		best, worst = math.Min(best, c), math.Max(worst, c)
	}
	line := fmt.Sprintf("%d tiles, terrain %.1f a tile.", len(path)-1, now/float64(len(path)-1))
	switch {
	case best > 0 && now > best*(1+seasonGap):
		return line + fmt.Sprintf(" Winter slows it now: %d%% slower than in the best season.", int(math.Round((now/best-1)*100)))
	case now > 0 && worst > now*(1+seasonGap):
		return line + fmt.Sprintf(" In winter it is up to %d%% slower.", int(math.Round((worst/now-1)*100)))
	}
	return line + " The seasons barely change it."
}

// declareRouteBar declares the route bar at r: where, how long, the note
// and Confirm / Cancel.
func (v router) declareRouteBar(r gfx.Rect) {
	k := v.kit()
	rt := &v.route
	k.Panel(r)
	head := "Travel: click a tile on the globe."
	if rt.to >= 0 {
		head = "Travel to " + v.tileName(rt.to) + " · " + rt.eta
	}
	k.Label(r.X+10, r.Y+8, gfx.Ellipsize(head, gfx.Label, float64(r.W-20)), gfx.WithColor(gfx.Label, uikit.Heading))
	note := rt.note
	if rt.why != "" {
		note = rt.why
	}
	if note != "" {
		k.Label(r.X+10, r.Y+28, gfx.Ellipsize(note, gfx.Small, float64(r.W-20)), gfx.WithColor(gfx.Small, uikit.TextDim))
	}
	why := rt.why
	if rt.to < 0 {
		why = "Click a tile on the globe first."
	}
	by := r.Bottom() - 32
	if k.Button(ConfirmID, gfx.Rect{X: r.Right() - 220, Y: by, W: 100, H: 26}, "Confirm", uikit.BtnOpts{Style: uikit.BtnPrimary,
		Disabled: why != "", Reason: why, Tip: "Confirm\nThe caravan sets off along this route."}) {
		v.send(command.Command{Kind: command.CaravanTravel, Target: rt.caravan, Count: rt.to})
		v.route = routeState{}
	}
	if k.Button(CancelID, gfx.Rect{X: r.Right() - 110, Y: by, W: 100, H: 26}, "Cancel", uikit.BtnOpts{}) {
		v.route = routeState{}
	}
}

// tileName names world tile t: an object there, or its number (the
// planet's own names may be of a base since razed).
func (v *View) tileName(t int) string {
	v.tiles = onTile(v.tiles, v.objs, t)
	if len(v.tiles) > 0 {
		return v.objs[v.tiles[0]].Label
	}
	return "tile " + strconv.Itoa(t)
}
