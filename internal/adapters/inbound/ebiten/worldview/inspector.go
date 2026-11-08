package worldview

import (
	"fmt"
	"strconv"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/adapters/inbound/ebiten/units"
	"rimworld-go/internal/adapters/inbound/ebiten/worldinfo"
	"rimworld-go/internal/application/game"
	qd "rimworld-go/internal/domain/quest"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/world/caravan"
)

// The world inspector (MAIN_UI §33 selected tile, §35 caravan inspector):
// the selected object's facts and its gizmos, in the pane's place above
// the tabs. Every gizmo carries why it is disabled. Ids world.act.<name>.

const (
	inspPad   = 10
	gizmoH    = 26
	gizmoCols = 3
	lineH     = 18
	AbandonID = "world.abandon" // the confirm modal (its buttons: modal.world.abandon.ok / .cancel)
	AttackID  = "world.attack"  // the confirm modal for attacking a settlement that is not hostile (MAP-15)
	// attackGoodwill is what attacking a settlement costs its faction
	// (wiki: Faction base). The simulation applies it (maps.attackGoodwill);
	// no read model carries it, so the confirm names it from here.
	attackGoodwill = 200
	notYet         = "Not yet."
	stillForms     = "Still forming at the colony."
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
		k.Label(in.X, in.Y, v.sh.T("World"), gfx.WithColor(gfx.Label, uikit.Heading))
		k.Label(in.X, in.Y+24, v.sh.T("Click a colony, caravan, settlement or tile."), uikit.DimText())
		k.Label(in.X, in.Y+24+lineH, v.sh.T("Double-click one of our maps to show it."), uikit.DimText())
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
	case KindQuest:
		return v.inspectQuest(o)
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
	if m.Reinforce > 0 {
		lines = append(lines, line{"Reinforcements in", durationText(m.Reinforce)})
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
	abandon := gizmo{name: "abandon", label: "Abandon", tip: "Abandon\nLeave this map for good. Everyone and everything still on it is lost.",
		run: func() {
			body := v.sh.Tf("Everyone and everything still on %s will be lost. This cannot be undone.", o.Label)
			if v.endsGame(m) {
				body += v.sh.T(" Nobody would be left: this ends the game.")
			}
			v.kit().Confirm(AbandonID, v.sh.Tf("Abandon %s", o.Label), body, v.sh.T("Abandon"), true,
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

// endsGame reports that abandoning colony m leaves nobody of ours anywhere
// (MAP-14): it is the last colony and no caravan, pod or other map holds a
// colonist. The simulation's EveryoneGone is the authority once it happens;
// this only warns before.
func (v inspector) endsGame(m game.MapView) bool {
	if m.Kind != "colony" {
		return false
	}
	for i := range v.objs {
		switch o := &v.objs[i]; o.Kind {
		case KindCaravan, KindPod:
			return false
		case KindMap:
			if o.Map.ID != m.ID && (o.Map.Kind == "colony" || o.Map.Colonists > 0) {
				return false
			}
		}
	}
	return true
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
	default:
		if a, ok := v.sh.Env().Game.(Attacker); ok {
			attackWhy = a.AttackReason(id)
		}
	}
	gizmos := []gizmo{
		{name: "travel", label: "Travel", why: travelWhy, tip: "Travel\nPick a tile on the globe; the route and how long it takes show before you confirm.",
			run: func() { router(v).startRoute(id, tile) }},
		{name: "return", label: retLabel, why: retWhy, tip: "Return\nThe caravan turns round and comes home the way it went.",
			run: func() { v.send(command.Command{Kind: command.CaravanReturn, Target: id}) }},
		{name: "settle", label: "Settle here", why: settleWhy, tip: "Settle here\nThe caravan founds a new colony on this tile.",
			run: func() { v.send(command.Command{Kind: command.SettleHere, Target: id}) }},
		{name: "enter", label: "Enter", why: enterWhy, tip: "Enter\nThe caravan enters the map here (opening one when there is none yet).",
			run: func() { v.send(command.Command{Kind: command.EnterMap, Target: id}) }},
		{name: "attack", label: "Attack", why: attackWhy, tip: "Attack\nAttack the settlement here. Attacking a settlement that is not hostile costs its faction " + strconv.Itoa(attackGoodwill) + " goodwill and turns them hostile until it recovers.",
			run: func() { v.attackSettlement(id, site, settled) }},
		{name: "split", label: "Split", why: notYet, tip: "Split\nSplitting a caravan in two."},
	}
	return o.Label, lines, append(gizmos, v.questGizmos(c)...)
}

// questGizmos are the quest actions of caravan c standing on the tile of an
// open quest (QST-118, QST-119): "Fulfil request" on an active trade
// request, "Negotiate" on offered or active peace talks. The read model
// does not say what the caravan carries, so the game refuses a short load
// with a note.
func (v inspector) questGizmos(c game.CaravanView) []gizmo {
	why := ""
	if c.State == caravan.Forming.Label() {
		why = v.sh.T("Still forming at the colony.")
	}
	var out []gizmo
	for _, q := range v.quests {
		if q.Tile != c.Tile || c.Tile < 0 {
			continue
		}
		id, cid := q.ID, c.ID
		switch {
		case q.Kind == qd.Request && q.State == qd.Active:
			out = append(out, gizmo{name: "fulfil", label: v.sh.T("Fulfil request"), why: why,
				tip: v.sh.T("Fulfil request\nHand the requested goods over from this caravan."),
				run: func() { v.send(command.Command{Kind: command.FulfilRequest, Index: int(id), Target: cid}) }})
		case q.Kind == qd.Talks && (q.State == qd.Active || q.State == qd.Offered):
			out = append(out, gizmo{name: "negotiate", label: v.sh.T("Negotiate"), why: why,
				tip: v.sh.T("Negotiate\nHold peace talks with this caravan's envoys."),
				run: func() { v.send(command.Command{Kind: command.Negotiate, Index: int(id), Target: cid}) }})
		}
	}
	return out
}

// inspectQuest is a quest site: the quest's kind, who gives it, the time
// left and a line about it, with a gizmo that opens the Quests window on it.
func (v inspector) inspectQuest(o *Object) (string, []line, []gizmo) {
	q := o.Quest
	lines := []line{{v.sh.T("Faction"), v.questFaction(q)}}
	at, now := q.Deadline, uint64(v.sh.Tick())
	if q.State == qd.Offered {
		at = q.AcceptBy
	}
	if at > 0 {
		lines = append(lines, line{v.sh.T("Time left"), hoursLeft(at, now)})
	} else {
		lines = append(lines, line{v.sh.T("Time left"), v.sh.T("no limit")})
	}
	lines = append(lines, line{"", v.questLine(q)})
	id, tile := q.ID, o.Tile
	return o.Label, lines, []gizmo{{name: "quest", label: v.sh.T("View quest"),
		tip: v.sh.T("View quest\nOpen the Quests window on this quest."),
		run: func() { v.sh.Do(ports.ShowQuest{ID: uint32(id)}) }},
		v.sendGizmo(tile)}
}

// sendGizmo is "Send caravan" to tile: the Form caravan dialog of our first
// colony with people, bound for tile.
func (v inspector) sendGizmo(tile int) gizmo {
	home := -1
	for i := range v.objs {
		if o := &v.objs[i]; o.Kind == KindMap && o.Map.Kind == "colony" && o.Map.Colonists > 0 {
			home = int(o.Map.ID)
			break
		}
	}
	why := ""
	if home < 0 {
		why = v.sh.T("Nobody to form a caravan.")
	}
	return gizmo{name: "send", label: v.sh.T("Send caravan"), why: why,
		tip: v.sh.T("Send caravan\nChoose who and what travels here."),
		run: func() {
			if g := v.sh.Env().Game; g != nil && int(g.ViewedMap()) != home {
				v.sh.Do(ports.SwitchMap{Map: game.MapID(home)})
			}
			v.sh.FormCaravan(func() int { return tile })
		}}
}

// questFaction is the name of the faction behind quest q.
func (v inspector) questFaction(q game.QuestView) string {
	ref := q.Giver
	if ref == 0 {
		ref = q.Target
	}
	if g := v.sh.Env().Game; g != nil && ref > 0 {
		for _, f := range g.Factions() {
			if f.ID == ref-1 {
				return f.Name
			}
		}
	}
	return v.sh.T("Nobody in particular")
}

// questLine is one line about what the quest asks, by kind.
func (v inspector) questLine(q game.QuestView) string {
	switch q.Kind {
	case qd.Joiner:
		return v.sh.T("Someone in trouble asks to join us.")
	case qd.OreLump:
		return v.sh.T("Reach the ore before it is gone.")
	case qd.Stash:
		return v.sh.T("Goods lie hidden here under guard.")
	case qd.Refugee:
		return v.sh.T("A hurt traveller waits here for help.")
	case qd.Camp:
		return v.sh.T("A hostile camp to clear out.")
	case qd.Rescue:
		return v.sh.T("A prisoner is held here.")
	case qd.Request:
		return v.sh.T("Bring the goods they asked for.")
	case qd.Talks:
		return v.sh.T("Send a caravan to talk peace.")
	}
	return v.sh.T("A quest waits here.")
}

// attackSettlement sends caravan id's attack at once on a hostile
// settlement; on any other it first asks, naming the faction and the cost
// (MAP-15).
func (v inspector) attackSettlement(id uint64, site game.WorldSiteView, settled bool) {
	send := func() { v.send(command.Command{Kind: command.AttackSettlement, Target: id}) }
	if !settled || site.Relation == "hostile" {
		send()
		return
	}
	v.kit().Confirm(AttackID, v.sh.Tf("Attack %s", site.Faction), attackText(v.sh, site), v.sh.T("Attack"), true, send)
}

// attackConfirmText is the confirm's body: who is attacked, how they
// stand with us now, and what the attack costs.
func attackConfirmText(site game.WorldSiteView) string { return attackText(lang.English{}, site) }

func attackText(tr lang.Tr, site game.WorldSiteView) string {
	return lang.Tf(tr, "%s is %s toward us. Attacking their settlement costs %d goodwill and turns them hostile; goodwill recovers only slowly with time and gifts.", site.Faction, site.Relation, attackGoodwill)
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
