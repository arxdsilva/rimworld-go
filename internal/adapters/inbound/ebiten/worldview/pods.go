package worldview

import (
	"fmt"
	"strconv"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/caravan"
	"rimworld-go/internal/domain/world/planet"
)

// Transport pod targeting (MAIN_UI §36, MAP-10; wiki: Transport pod): a
// pod group's Launch opens the view in targeting mode. The tiles its fuel
// reaches (caravan.PodRange: 2.25 chemfuel a tile a pod, 66 at most) are
// found by one breadth-first search when targeting starts and their edge
// is pulsed on the relief as the range ring. A click on a target opens
// the landing menu there; each mode carries why it is refused (the
// simulation's LaunchReason when the game offers it, else the same rule
// mirrored here). Pick spot switches to our map at the tile and leaves a
// placement cursor that sends the landing cell as X2, Y2.

// podMode is one landing menu row: its id suffix, label, mode and tip.
type podMode struct {
	id, label string
	mode      uint8
	tip       string
}

// podModes are the landing menu's rows, in order (ids world.pod.<id>).
func podModes() []podMode {
	return []podMode{
		{"drop", "Drop at edge", command.PodArriveDrop, "Drop at edge\nThe pods come down at the edge of the map there (at home, at the drop spot); off any map the passengers wait as a caravan."},
		{"dropcentre", "Drop at centre", command.PodArriveDropCentre, "Drop at centre\nThe pods come down in the middle of the map there."},
		{"attackedge", "Attack: edge", command.PodArriveAttackEdge, "Attack from the edge\nLand at the settlement's edge and attack it. Its people turn hostile."},
		{"attackcentre", "Attack: centre", command.PodArriveAttackCentre, "Attack the centre\nLand in the middle of the settlement, among its defenders. Its people turn hostile."},
		{"found", "Found colony", command.PodArriveFound, "Found a colony\nThe passengers settle the tile as a new colony."},
		{"gift", "Send as gift", command.PodArriveGift, "Send as gift\nThe settlement keeps the goods and animals; goodwill rises with their value."},
		{"visit", "Visit", command.PodArriveVisit, "Visit\nThe passengers stay a day as guests, then wait for orders."},
		{"spot", "Pick spot", command.PodArriveSpot, "Pick spot\nShow our map there and click the cell the pods land on."},
	}
}

// PodCancelID closes the landing menu.
const (
	PodCancelID  = "world.pod.cancel"
	podModeCount = 8 // the rows of podModes
	podMenuW     = 210
	podRowH      = 28
)

// podState is pod targeting: the pod whose group flies, the map it
// stands on, its home tile, range and load, the breadth-first distances
// and the ring, and the landing menu (its target tile and anchor).
type podState struct {
	on     bool
	pod    entity.EntityID
	origin game.MapID
	home   int
	rng    int
	loadKg int
	dist   []int16 // hops from home within range, -1 beyond (nil until the planet is charted)
	ring   []int
	menu   bool
	target int
	ax, ay float32 // the menu's anchor, logical
	why    podWhy
}

// podWhy caches the menu rows' refusals for one target and object read.
type podWhy struct {
	ok     bool
	target int
	at     readStamp
	modes  []podMode
	text   []string
}

// spotState is the pick-spot cursor: the launch waiting for its cell.
type spotState struct {
	on    bool
	fresh bool
	cmd   command.Command
}

// LaunchFor opens the view picking the landing of pod's group
// (ports.LaunchPods, the pod's Launch gizmo).
func (v *View) LaunchFor(pod entity.EntityID) {
	env := v.sh.Env()
	if env.Game == nil {
		return
	}
	v.Show("")
	if !v.open {
		return
	}
	st := podState{on: true, pod: pod, origin: env.Game.ViewedMap(), home: env.Game.WorldMap().Tile, target: -1}
	for _, m := range env.Game.Maps() {
		if m.ID == st.origin && m.Tile >= 0 {
			st.home = m.Tile
		}
	}
	for _, b := range v.sh.World().Buildings {
		if b.ID == pod {
			st.rng, st.loadKg = b.AX.Range, b.AX.LoadKg
		}
	}
	v.route = routeState{}
	v.pods = st
	v.sel.home = false
	v.selectKey("")
	if v.g != nil {
		v.g.Center(st.home)
	}
}

// podRing fills the targeting distances and ring once the planet exists.
func (v targeting) podRing(p *planet.Planet) {
	ps := &v.pods
	if !ps.on || ps.dist != nil {
		return
	}
	ps.dist, ps.ring = rangeRing(p, ps.home, min(ps.rng, caravan.PodMaxRange))
}

// rangeRing is the breadth-first hop distance from home of every tile
// within rng (-1 beyond), and the ring: the reached tiles at the range
// or next to an unreached one.
func rangeRing(p *planet.Planet, home, rng int) ([]int16, []int) {
	n := p.TileCount()
	dist := make([]int16, n)
	for i := range dist {
		dist[i] = -1
	}
	if home < 0 || home >= n {
		return dist, nil
	}
	dist[home] = 0
	front := []int32{int32(home)}
	var next []int32
	for d := 1; d <= rng && len(front) > 0; d++ {
		next = next[:0]
		for _, t := range front {
			for _, nb := range p.Neighbours(int(t)) {
				if dist[nb] < 0 {
					dist[nb] = int16(d)
					next = append(next, nb)
				}
			}
		}
		front, next = next, front
	}
	var ring []int
	for t, d := range dist {
		if d < 0 {
			continue
		}
		edge := int(d) == rng
		for _, nb := range p.Neighbours(t) {
			edge = edge || dist[nb] < 0
		}
		if edge && t != home {
			ring = append(ring, t)
		}
	}
	return dist, ring
}

// openPodMenu opens the landing menu for tile t anchored at (x, y).
func (v targeting) openPodMenu(t int, x, y float32) {
	v.pods.menu, v.pods.target, v.pods.ax, v.pods.ay = true, t, x, y
}

// podTarget is one quick pick of the targeting list.
type podTarget struct {
	tile  int
	label string
}

// podTargets are our maps, the settlements and the mineral sites within
// the pods' range, as quick picks (the globe works too).
func (v targeting) podTargets() []podTarget {
	ps := &v.pods
	var out []podTarget
	for i := range v.objs {
		o := &v.objs[i]
		if o.Kind == KindCaravan || o.Kind == KindPod || o.Tile >= len(ps.dist) || ps.dist[o.Tile] < 0 {
			continue
		}
		label := fmt.Sprintf("%s (%d tiles)", o.Label, ps.dist[o.Tile])
		if o.Kind == KindSettlement {
			label = fmt.Sprintf("%s (%s, %d tiles)", o.Label, o.Settlement.Relation, ps.dist[o.Tile])
		}
		out = append(out, podTarget{o.Tile, label})
	}
	return out
}

// declarePodTargets declares the targeting list in the inspector's place.
func (v targeting) declarePodTargets(r gfx.Rect) {
	k := v.kit()
	k.Panel(r)
	in := r.Inset(inspPad)
	k.Label(in.X, in.Y, v.sh.T("Targets in range"), gfx.WithColor(gfx.Label, uikit.Heading))
	body := gfx.Rect{X: in.X, Y: in.Y + 24, W: in.W, H: in.H - 24}
	targets := v.podTargets()
	const rowH = 30
	area := k.ScrollArea("world.pod.targets", body, float32(len(targets)*rowH), uikit.ScrollOpts{Step: rowH})
	y := area.View.Y - area.Off
	for _, tg := range targets {
		row := gfx.Rect{X: area.View.X, Y: y, W: area.View.W - 4, H: rowH - 4}
		y += rowH
		if row.Y < area.View.Y || row.Bottom() > area.View.Bottom() {
			continue
		}
		if k.Button("world.pod.target."+strconv.Itoa(tg.tile), row, gfx.Ellipsize(tg.label, gfx.Body, float64(row.W-12)),
			uikit.BtnOpts{Selected: v.pods.menu && v.pods.target == tg.tile}) {
			if v.g != nil {
				v.g.Center(tg.tile)
			}
			v.openPodMenu(tg.tile, r.Right(), row.Y)
		}
	}
}

// siteAt is the settlement on tile t, if any.
func (v *View) siteAt(t int) (game.WorldSiteView, bool) {
	for i := range v.objs {
		if o := &v.objs[i]; o.Kind == KindSettlement && o.Tile == t {
			return o.Settlement, true
		}
	}
	return game.WorldSiteView{}, false
}

// mapAt is our open map on tile t, if any.
func (v *View) mapAt(t int) (game.MapView, bool) {
	for i := range v.objs {
		if o := &v.objs[i]; o.Kind == KindMap && o.Tile == t {
			return o.Map, true
		}
	}
	return game.MapView{}, false
}

// podDist is the hop distance to t (beyond the range: counted up to one
// past the farthest flight).
func (v targeting) podDist(t int) int {
	ps := &v.pods
	if t >= 0 && t < len(ps.dist) && ps.dist[t] >= 0 {
		return int(ps.dist[t])
	}
	if p := v.chart.planet; p != nil {
		return p.HopsWith(&v.hops, ps.home, t, caravan.PodMaxRange+1)
	}
	return caravan.PodMaxRange + 1
}

// podReason is why the group would not land on t in mode ("" it would):
// the simulation's rule when the game offers it, else the mirror.
func (v targeting) podReason(t int, mode uint8) string {
	if mode == command.PodArriveSpot {
		if _, ok := v.mapAt(t); !ok {
			return "Pick spot needs one of our maps open there."
		}
	}
	if l, ok := v.sh.Env().Game.(Launcher); ok {
		why, _ := l.LaunchReason(v.pods.pod, t, mode)
		return why
	}
	return v.mirrorReason(t, mode)
}

// mirrorReason is LaunchReason's rule as the view can judge it from the
// read models (travel/launch.go is the authority).
func (v targeting) mirrorReason(t int, mode uint8) string {
	ps := &v.pods
	p := v.chart.planet
	if p == nil {
		return v.sh.T("This colony has no world map.")
	}
	if t < 0 || t >= p.TileCount() {
		return "Pick a tile on the world map."
	}
	if ps.loadKg <= 0 {
		return "The pods are empty: load them first."
	}
	if d := v.podDist(t); d > ps.rng {
		return fmt.Sprintf("Out of range: %d tiles away, the fuel reaches %d.", d, ps.rng)
	}
	site, settled := v.siteAt(t)
	hostile := settled && site.Relation == "hostile"
	switch mode {
	case command.PodArriveDrop, command.PodArriveDropCentre:
		if hostile {
			return "They would shoot the pods down: attack instead."
		}
		if settled {
			return "Land on a settlement as a visit or a gift."
		}
	case command.PodArriveAttackEdge, command.PodArriveAttackCentre:
		if !settled {
			return "Only a settlement can be attacked."
		}
	case command.PodArriveFound:
		if ok, why := p.ValidLanding(t); !ok {
			return why
		}
		return v.colonyCapReason()
	case command.PodArriveGift:
		if !settled {
			return "A gift needs a settlement to land on."
		}
	case command.PodArriveVisit:
		if !settled || !site.Visitable {
			return "Only a friendly settlement takes visitors."
		}
	}
	return ""
}

// colonyCapReason is why another colony cannot be founded ("" it can):
// Options, Gameplay, Max colonies (wiki: Colony).
func (v *View) colonyCapReason() string {
	limit := v.sh.Settings().Gameplay.Colonies()
	n := 0
	for i := range v.objs {
		if o := &v.objs[i]; o.Kind == KindMap && o.Map.Kind == "colony" {
			n++
		}
	}
	if n >= limit {
		return fmt.Sprintf("You hold the most colonies allowed (%d). Raise Max colonies in Options, Gameplay.", limit)
	}
	return ""
}

// podMenuRect is the landing menu's rectangle at its anchor, kept on
// screen.
func (v targeting) podMenuRect(sw, sh int) gfx.Rect {
	h := float32(34 + (podModeCount+1)*podRowH + 8)
	x := min(v.pods.ax+12, float32(sw)-podMenuW-8)
	y := min(max(v.pods.ay-h/2, 8), float32(sh)-h-8)
	return gfx.Rect{X: x, Y: y, W: podMenuW, H: h}
}

// declarePodMenu declares the landing menu at r.
func (v targeting) declarePodMenu(r gfx.Rect) {
	k := v.kit()
	ps := &v.pods
	k.Panel(r)
	head := v.sh.Tf("%s · %d of %d tiles", v.tileName(ps.target), v.podDist(ps.target), ps.rng)
	k.Label(r.X+10, r.Y+8, gfx.Ellipsize(head, gfx.Label, float64(r.W-20)), gfx.WithColor(gfx.Label, uikit.Heading))
	y := r.Y + 34
	w := &ps.why
	if !w.ok || w.target != ps.target || w.at != v.read {
		modes := podModes()
		text := make([]string, len(modes))
		for i, m := range modes {
			text[i] = v.podReason(ps.target, m.mode)
		}
		*w = podWhy{ok: true, target: ps.target, at: v.read, modes: modes, text: text}
	}
	for i, m := range w.modes {
		why := w.text[i]
		mode := m.mode
		if k.Button("world.pod."+m.id, gfx.Rect{X: r.X + 8, Y: y, W: r.W - 16, H: podRowH - 4}, m.label,
			uikit.BtnOpts{Disabled: why != "", Reason: why, Tip: m.tip}) {
			v.launch(mode)
			return // the menu is gone
		}
		y += podRowH
	}
	if k.Button(PodCancelID, gfx.Rect{X: r.X + 8, Y: y, W: r.W - 16, H: podRowH - 4}, v.sh.T("Cancel"), uikit.BtnOpts{}) {
		ps.menu = false
	}
}

// launch fires the group at the menu's target in mode: the command goes
// to the pods' own map; Pick spot first switches to the map at the target
// and leaves the placement cursor there.
func (v targeting) launch(mode uint8) {
	ps := &v.pods
	c := command.Command{Kind: command.LaunchPods, Target: uint64(ps.pod), Count: ps.target, Mode: mode, Map: uint32(ps.origin), MapSet: true}
	if mode == command.PodArriveSpot {
		m, ok := v.mapAt(ps.target)
		if !ok {
			return
		}
		v.spot = spotState{on: true, fresh: true, cmd: c}
		v.sh.Do(ports.SwitchMap{Map: m.ID})
		v.Close()
		return
	}
	if mode == command.PodArriveAttackEdge || mode == command.PodArriveAttackCentre {
		// MAP-15: an attack on a settlement that is not hostile asks first.
		if site, settled := v.siteAt(ps.target); settled && site.Relation != "hostile" {
			v.kit().Confirm(AttackID, v.sh.Tf("Attack %s", site.Faction), attackText(v.sh, site), v.sh.T("Attack"), true, func() {
				v.send(c)
				v.pods = podState{}
			})
			return
		}
	}
	v.send(c)
	v.pods = podState{}
}

// Spotting reports the pick-spot cursor is up (the root counts it as a
// tool: the map's own clicks wait).
func (v *View) Spotting() bool { return v.spot.on }

// HandleSpot feeds the pick-spot cursor this Update's input; true when it
// took it (selection must not also act on the click). A left click on a
// map cell sends the launch with that cell; a right click cancels.
func (v *View) HandleSpot() bool {
	if !v.spot.on {
		return false
	}
	f := v.kit().Host().Frame()
	mx, my := v.sh.Cursor()
	left, right := f.Clicked(inputframe.Left), f.Clicked(inputframe.Right)
	f.Take(inputframe.Left)
	f.Take(inputframe.Right)
	if v.spot.fresh {
		v.spot.fresh = false
		v.kit().Host().Hint(v.sh.T("Click the cell the pods land on; Esc or right-click cancels"))
		return true
	}
	if right {
		v.cancelSpot()
		return true
	}
	if !left || v.sh.ChromeHit(mx, my) {
		return true
	}
	tx, ty := v.sh.TileAt(v.sh.MapCursor())
	if w, h := v.sh.MapSize(); tx < 0 || ty < 0 || tx >= w || ty >= h {
		return true
	}
	c := v.spot.cmd
	c.X2, c.Y2 = tx, ty
	v.send(c)
	v.spot = spotState{}
	v.kit().Host().Hint("")
	return true
}

// cancelSpot drops the pick-spot cursor without launching.
func (v *View) cancelSpot() {
	v.spot = spotState{}
	v.kit().Host().Hint("")
}
