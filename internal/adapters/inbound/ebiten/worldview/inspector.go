package worldview

import (
	"fmt"
	"strconv"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/adapters/inbound/ebiten/units"
	"rimworld-go/internal/adapters/inbound/ebiten/worldinfo"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/world/caravan"
)

// The world inspector (MAIN_UI §33 selected tile, §35 caravan inspector):
// the selected object's facts and its gizmos, in the pane's place above
// the tabs. Every gizmo carries why it is disabled. Ids world.act.<name>.

const (
	inspPad    = 10
	gizmoH     = 26
	gizmoCols  = 3
	lineH      = 18
	AbandonID  = "world.abandon" // the confirm modal (its buttons: modal.world.abandon.ok / .cancel)
	notYet     = "Not yet."
	stillForms = "Still forming at the colony."
)

// line is one label/value row of the inspector.
type line struct{ label, value string }

// gizmo is one inspector action.
type gizmo struct {
	name, label, why, tip string
	run                   func()
}

// card caches the tile card of the last tile shown (worldinfo builds it
// from the planet, too dear for every frame).
type card struct {
	tile, day int
	f         bool
	info      worldinfo.TileInfo
	ok        bool
}

// inspectorRect is the inspector's place: the layout's pane, which the
// root grows over the pane's tab row while the view is open.
func inspectorRect(lay hudlayout.Layout) gfx.Rect { return lay.Pane }

// declareInspector declares the inspector for the selection at r (nothing
// selected: a hint).
func (v inspector) declareInspector(r gfx.Rect) {
	k := v.kit()
	k.Panel(r)
	in := r.Inset(inspPad)
	title, lines, gizmos := v.inspect()
	if title == "" {
		k.Label(in.X, in.Y, "World", gfx.WithColor(gfx.Label, uikit.Heading))
		k.Label(in.X, in.Y+24, "Click a colony, caravan, settlement or tile.", uikit.DimText())
		k.Label(in.X, in.Y+24+lineH, "Double-click one of our maps to show it.", uikit.DimText())
		return
	}
	k.Label(in.X, in.Y, gfx.Ellipsize(title, gfx.Label, float64(in.W)), gfx.WithColor(gfx.Label, uikit.Heading))
	rows := (len(gizmos) + gizmoCols - 1) / gizmoCols
	body := gfx.Rect{X: in.X, Y: in.Y + 24, W: in.W, H: in.H - 24 - float32(rows)*(gizmoH+4) - 4}
	area := k.ScrollArea("world.insp", body, float32(len(lines)*lineH), uikit.ScrollOpts{Step: lineH})
	y := area.View.Y - area.Off
	lw := area.View.W * 0.42
	for _, l := range lines {
		if y >= area.View.Y && y+lineH <= area.View.Bottom() {
			if l.label != "" {
				k.Label(area.View.X, y, gfx.Ellipsize(l.label, gfx.Small, float64(lw-6)), gfx.WithColor(gfx.Small, uikit.TextDim))
				k.Label(area.View.X+lw, y, gfx.Ellipsize(l.value, gfx.Small, float64(area.View.W-lw)), gfx.Small)
			} else {
				k.Label(area.View.X, y, gfx.Ellipsize(l.value, gfx.Small, float64(area.View.W)), gfx.Small)
			}
		}
		y += lineH
	}
	w := (in.W - float32(gizmoCols-1)*6) / gizmoCols
	for i, gz := range gizmos {
		col, row := i%gizmoCols, i/gizmoCols
		br := gfx.Rect{X: in.X + float32(col)*(w+6), Y: in.Bottom() - float32(rows-row)*(gizmoH+4) + 4, W: w, H: gizmoH}
		if k.Button("world.act."+gz.name, br, gz.label, uikit.BtnOpts{Disabled: gz.why != "", Reason: gz.why, Tip: gz.tip}) && gz.run != nil {
			gz.run()
		}
	}
}

// inspect is the selection's title, rows and gizmos ("" title: nothing).
func (v inspector) inspect() (string, []line, []gizmo) {
	o := v.selected()
	if o == nil {
		if t := v.selTile(); t >= 0 {
			return v.inspectTile(t)
		}
		return "", nil, nil
	}
	switch o.Kind {
	case KindMap:
		return v.inspectMap(o)
	case KindCaravan:
		return v.inspectCaravan(o)
	case KindSettlement:
		return v.inspectSettlement(o)
	case KindSite:
		return o.Label, []line{{"Found by", "the mineral scanner"}, {"Gone in", hoursLeft(o.Site.Expires, uint64(v.sh.Tick()))},
			{"", "Send a caravan or pods to mine it."}}, nil
	}
	f := o.Pod
	return o.Label, []line{{"Going to", v.tileName(int(f.To))}, {"Lands in", hoursLeft(f.Arrive, uint64(v.sh.Tick()))}}, nil
}

// inspectMap is one of our maps: a colony (Jump, Form caravan, Abandon)
// or a site, settlement or encounter map (Jump, Reform caravan, Abandon,
// the pack-up countdown).
func (v inspector) inspectMap(o *Object) (string, []line, []gizmo) {
	m := o.Map
	lines := []line{{"Kind", capFirst(m.Kind)}, {"Colonists", strconv.Itoa(m.Colonists)}}
	if m.Threat {
		lines = append(lines, line{"", "Hostiles on the map."})
	}
	if m.Countdown > 0 {
		lines = append(lines, line{"Packs up in", durationText(m.Countdown)})
	}
	switch {
	case o.razed:
		lines = append(lines, line{"", "The settlement that stood here is gone."})
	case o.Destroyed():
		lines = append(lines, line{"", "Nobody is left here."})
	}
	if g := v.sh.Env().Game; g != nil && g.ViewedMap() == m.ID {
		lines = append(lines, line{"", "This map is on screen."})
	}
	id := m.ID
	jump := gizmo{name: "jump", label: "Jump", tip: "Jump\nShow this map.", run: func() { v.jump(id) }}
	abandon := gizmo{name: "abandon", label: "Abandon", why: v.abandonReason(m.Kind), tip: "Abandon\nLeave this map for good. Everyone and everything still on it is lost.",
		run: func() {
			v.kit().Confirm(AbandonID, "Abandon "+o.Label, "Everyone and everything still on "+o.Label+" will be lost. This cannot be undone.", "Abandon", true,
				func() { v.send(command.Command{Kind: command.AbandonMap, Target: uint64(id)}) })
		}}
	if m.Kind == "colony" {
		why := ""
		if m.Colonists == 0 {
			why = "Nobody on this map to form a caravan."
		}
		form := gizmo{name: "form", label: "Form caravan", why: why, tip: "Form caravan\nChoose who and what travels, and where to.",
			run: func() {
				if g := v.sh.Env().Game; g != nil && g.ViewedMap() != id {
					v.sh.Do(ports.SwitchMap{Map: id})
				}
				v.sh.FormCaravan(func() int { return v.sel.pick })
			}}
		return o.Label, lines, []gizmo{jump, form, abandon}
	}
	why := ""
	if m.Colonists == 0 {
		why = "Nobody here to leave."
	}
	reform := gizmo{name: "reform", label: "Reform caravan", why: why, tip: "Reform caravan\nEveryone here packs up and leaves as a caravan.",
		run: func() { v.send(command.Command{Kind: command.CaravanReform, Target: uint64(id)}) }}
	return o.Label, lines, []gizmo{jump, reform, abandon}
}

// abandonReason is why a map of kind may not be abandoned ("" it may):
// never the last colony.
func (v inspector) abandonReason(kind string) string {
	if kind != "colony" {
		return ""
	}
	n := 0
	for i := range v.objs {
		if o := &v.objs[i]; o.Kind == KindMap && o.Map.Kind == "colony" {
			n++
		}
	}
	if n <= 1 {
		return "Your last colony cannot be abandoned."
	}
	return ""
}

// inspectCaravan is a caravan (§35): Travel, Return, Settle here, Enter,
// Attack, Split.
func (v inspector) inspectCaravan(o *Object) (string, []line, []gizmo) {
	c := o.Caravan
	lines := []line{{"State", c.State}, {"At", v.tileName(c.Tile)}}
	if c.Dest != c.Tile && c.Dest >= 0 {
		lines = append(lines, line{"Heading for", v.tileName(c.Dest)})
	}
	if c.State == caravan.Travelling.Label() || c.State == caravan.Returning.Label() {
		lines = append(lines, line{"Next tile", fmt.Sprintf("%d%%", c.Progress)})
		if eta, why, _ := v.eta(c.ID, c.Tile, c.Dest); why == "" {
			lines = append(lines, line{"Arrives in", eta})
		}
	}
	for i, m := range c.Members {
		h := "well"
		if i < len(c.Health) && c.Health[i] != "" {
			h = c.Health[i]
		}
		lines = append(lines, line{m, h})
	}
	forming := c.State == caravan.Forming.Label()
	id, tile := c.ID, c.Tile
	site, settled := v.siteAt(tile)
	travelWhy := ""
	if forming {
		travelWhy = stillForms
	}
	retLabel, retWhy := CaravanReturnLabel(c)
	settleWhy := v.settleReason(id, forming, tile)
	enterWhy := ""
	switch {
	case forming:
		enterWhy = stillForms
	case settled && site.Relation == "hostile":
		enterWhy = "A hostile settlement: attack it instead."
	case c.State != caravan.Arrived.Label():
		enterWhy = "Only a caravan waiting at its destination can enter."
	}
	attackWhy := ""
	switch {
	case forming:
		attackWhy = stillForms
	case !settled:
		attackWhy = "No settlement here."
	case site.Relation != "hostile":
		attackWhy = "Only hostile settlements can be attacked."
	default:
		if a, ok := v.sh.Env().Game.(Attacker); ok {
			attackWhy = a.AttackReason(id)
		}
	}
	return o.Label, lines, []gizmo{
		{name: "travel", label: "Travel", why: travelWhy, tip: "Travel\nPick a tile on the globe; the route and how long it takes show before you confirm.",
			run: func() { (router{v.View}).startRoute(id, tile) }},
		{name: "return", label: retLabel, why: retWhy, tip: "Return\nThe caravan turns round and comes home the way it went.",
			run: func() { v.send(command.Command{Kind: command.CaravanReturn, Target: id}) }},
		{name: "settle", label: "Settle here", why: settleWhy, tip: "Settle here\nThe caravan founds a new colony on this tile.",
			run: func() { v.send(command.Command{Kind: command.SettleHere, Target: id}) }},
		{name: "enter", label: "Enter", why: enterWhy, tip: "Enter\nThe caravan enters the map here (opening one when there is none yet).",
			run: func() { v.send(command.Command{Kind: command.EnterMap, Target: id}) }},
		{name: "attack", label: "Attack", why: attackWhy, tip: "Attack\nAttack the settlement here. Its people turn hostile for good.",
			run: func() { v.send(command.Command{Kind: command.AttackSettlement, Target: id}) }},
		{name: "split", label: "Split", why: notYet, tip: "Split\nSplitting a caravan in two."},
	}
}

// settleReason is why caravan id on tile may not found a colony there: the
// simulation's rule when the game offers it, else the planet's and the
// colony cap's.
func (v inspector) settleReason(id uint64, forming bool, tile int) string {
	if why, ok := v.sh.SettleReason(id); ok {
		return why
	}
	if forming {
		return stillForms
	}
	if p := v.chart.planet; p != nil {
		if ok, why := p.ValidLanding(tile); !ok {
			return why
		}
	}
	return v.colonyCapReason()
}

// CaravanReturnLabel is the label of a caravan's Return gizmo and why it
// is disabled ("" it is not): a forming caravan is called off.
func CaravanReturnLabel(c game.CaravanView) (label, reason string) {
	switch c.State {
	case caravan.Forming.Label():
		return "Call off", ""
	case caravan.Returning.Label():
		return "Return", "Already on the way home."
	}
	return "Return", ""
}

// inspectSettlement is a faction's settlement: its people, their
// standing, and the distance and travel time from the caravan last
// selected.
func (v inspector) inspectSettlement(o *Object) (string, []line, []gizmo) {
	s := o.Settlement
	lines := []line{{"Faction", s.Faction}, {"Relation", s.Relation}}
	if g := v.sh.Env().Game; g != nil {
		for _, f := range g.Factions() {
			if f.Name == s.Faction {
				lines = append(lines, line{"Goodwill", strconv.Itoa(f.Goodwill)})
			}
		}
	}
	if s.Visitable {
		lines = append(lines, line{"", "Caravans are welcome."})
	}
	if ci := find(v.objs, "caravan:"+strconv.FormatUint(v.sel.caravan, 10)); ci >= 0 && v.chart.planet != nil {
		c := v.objs[ci].Caravan
		eta, why, d := v.eta(c.ID, c.Tile, s.Tile)
		if why != "" {
			eta = why
		}
		lines = append(lines, line{"From " + v.objs[ci].Label, fmt.Sprintf("%d tiles", d)}, line{"Travel time", eta})
	} else {
		lines = append(lines, line{"", "Select a caravan first to see how far it is."})
	}
	return o.Label, lines, nil
}

// inspectTile is a bare tile: the tile card (worldinfo).
func (v inspector) inspectTile(t int) (string, []line, []gizmo) {
	p := v.chart.planet
	if p == nil {
		return "", nil, nil
	}
	u := units.FromUnit(v.sh.Settings().Gameplay.TempUnit)
	day := v.dayOfYear()
	c := &v.card
	if !c.ok || c.tile != t || c.day != day || c.f != u.F {
		*c = card{tile: t, day: day, f: u.F, info: worldinfo.BuildTileInfoWith(p, t, worldinfo.Opts{Units: u, Day: day, Live: v.isLive}), ok: true}
	}
	ti := &c.info
	lines := make([]line, 0, len(ti.Rows)+2)
	if ti.Difficulty != "" {
		lines = append(lines, line{"Difficulty", ti.Difficulty})
	}
	for _, r := range ti.Rows {
		lines = append(lines, line{r.Label, r.Value})
	}
	for _, w := range ti.Warnings {
		lines = append(lines, line{"", w})
	}
	return ti.Name, lines, nil
}

// hoursLeft is the time from now to at ("3 h", "2 d").
func hoursLeft(at, now uint64) string {
	if at <= now {
		return "moments"
	}
	h := int((at - now) / clock.TicksPerHour)
	if h >= 48 {
		return fmt.Sprintf("%d d", h/24)
	}
	return fmt.Sprintf("%d h", max(1, h))
}
