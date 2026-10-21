package ebiten

import (
	"fmt"
	"image/color"
	"sort"
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

func inspectorRect() button {
	return button{x: hudPad, y: hudTopH + hudPad, w: 340, h: 390}
}

func (a *App) inspectorButtons() []button {
	if a.renderer == nil || !a.renderer.HasSelect() || a.workOpen || a.archOpen {
		return nil
	}
	r := inspectorRect()
	tabs := []buttonSpec{
		{id: "inspector.character", label: "Character", selected: a.inspectTab != "needs" && a.inspectTab != "gear" && a.inspectTab != "log", w: 72, h: 22},
		{id: "inspector.needs", label: "Needs", selected: a.inspectTab == "needs", w: 48, h: 22},
		{id: "inspector.health", label: "Health", disabled: true, reason: "not implemented", w: 52, h: 22},
		{id: "inspector.social", label: "Social", disabled: true, reason: "not implemented", w: 52, h: 22},
		{id: "inspector.gear", label: "Gear", selected: a.inspectTab == "gear", w: 42, h: 22},
		{id: "inspector.log", label: "Log", selected: a.inspectTab == "log", w: 36, h: 22},
	}
	out := inspectorTabRow(r, tabs)
	if p, ok := a.selectedPawn(); ok {
		out = append(out, a.pawnActionBar(p, r)...)
		if a.inspectTab == "gear" {
			out = append(out, a.equipNearbyButtons(p, r)...)
		}
	}
	out = append(out, button{
		id:    "inspector.close",
		label: "X",
		x:     r.x + r.w - 28,
		y:     r.y + 6,
		w:     22,
		h:     22,
	})
	return out
}

// pawnActionBar is the Draft / Clear orders row along the panel's bottom,
// shown on every tab. A dead or downed colonist's actions are disabled with
// the status as the reason.
func (a *App) pawnActionBar(p game.PawnView, r button) []button {
	label := "Draft (R)"
	if p.Drafted {
		label = "Undraft (R)"
	}
	y := r.y + r.h - 62
	draft := button{id: "pawn.draft", label: label, x: r.x + 12, y: y, w: 120, h: 24, selected: p.Drafted}
	clear := button{id: "pawn.clearorders", label: "Clear orders", x: r.x + 140, y: y, w: 120, h: 24}
	switch {
	case p.Status != "alive":
		draft.disabled, draft.reason = true, p.Status
		clear.disabled, clear.reason = true, p.Status
	case len(p.QueuedOrders) == 0:
		clear.disabled, clear.reason = true, "no queued orders"
	}
	return []button{draft, clear}
}

const (
	maxEquipShown      = 4  // buttons in the "Equip nearby" list
	maxEquipCandidates = 12 // nearest weapons whose orders are probed
)

// equipOption is one reachable weapon the selected colonist can equip.
type equipOption struct {
	Item  entity.EntityID
	Label string
	Cmd   command.Command
}

type equipCacheKey struct {
	r       *Renderer
	pawn    entity.EntityID
	tick    clock.Tick
	x, y    int
	items   int
	drafted bool
	weapon  string
	queued  int
}

// equipCacheState is the Gear tab's "Equip nearby" list, kept per App
// (App.equipCache) and rebuilt when its key changes.
type equipCacheState struct {
	key  equipCacheKey
	set  bool
	opts []equipOption
}

// equipNearby lists up to maxEquipShown nearest loose weapons (Chebyshev
// distance, ties by item id) for which the game offers an enabled Equip
// order. The result is cached while nothing it depends on changes.
func (a *App) equipNearby(p game.PawnView) []equipOption {
	if a.renderer == nil || p.Status != "alive" {
		return nil
	}
	items := a.renderer.world().Items
	key := equipCacheKey{r: a.renderer, pawn: p.ID, tick: a.renderer.game.Tick(), x: p.X, y: p.Y,
		items: len(items), drafted: p.Drafted, weapon: p.Weapon, queued: len(p.QueuedOrders)}
	if a.equipCache.set && a.equipCache.key == key {
		return a.equipCache.opts
	}
	type cand struct {
		it   game.ItemView
		dist int
	}
	var cands []cand
	for _, it := range items {
		if k, ok := item.ParseKind(it.Kind); it.Carried || !ok || !k.IsWeapon() {
			continue
		}
		cands = append(cands, cand{it, max(abs(it.X-p.X), abs(it.Y-p.Y))})
	}
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].dist != cands[j].dist {
			return cands[i].dist < cands[j].dist
		}
		return cands[i].it.ID < cands[j].it.ID
	})
	if len(cands) > maxEquipCandidates {
		cands = cands[:maxEquipCandidates]
	}
	var out []equipOption
	for _, c := range cands {
		if len(out) == maxEquipShown {
			break
		}
		for _, o := range a.renderer.game.OrdersAt(p.ID, c.it.X, c.it.Y) {
			if orderIs(command.OrderEquip)(o) && strings.HasPrefix(o.Label, "Equip") && !o.Disabled {
				out = append(out, equipOption{Item: c.it.ID, Cmd: o.Cmd,
					Label: fmt.Sprintf("%s \u2014 %d tiles", titleCase(c.it.Kind), c.dist)})
				break
			}
		}
	}
	a.equipCache.key, a.equipCache.set, a.equipCache.opts = key, true, out
	return out
}

func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// equipListY is the top of the "Equip nearby" buttons on the Gear tab,
// below the wearing list and comfort line.
func equipListY(r button, p game.PawnView) float32 {
	return r.y + 244 + float32(len(p.Apparel))*16
}

// equipNearbyButtons builds the Gear tab's "Equip nearby" list as buttons
// (pure: no drawing). Dead or downed colonists get one disabled line.
func (a *App) equipNearbyButtons(p game.PawnView, r button) []button {
	y := equipListY(r, p)
	limit := r.y + r.h - 66 // keep clear of the action bar
	if p.Status != "alive" {
		return []button{{id: "pawn.equip.none", label: "Equip nearby", x: r.x + 12, y: y, w: 300, h: 16,
			disabled: true, reason: p.Status}}
	}
	opts := a.equipNearby(p)
	if len(opts) == 0 {
		return []button{{id: "pawn.equip.none", label: "no weapons in reach", x: r.x + 12, y: y, w: 300, h: 16,
			disabled: true, reason: "no weapons in reach"}}
	}
	var out []button
	for _, o := range opts {
		if y+16 > limit {
			break
		}
		out = append(out, button{id: fmt.Sprintf("pawn.equip.%d", o.Item), label: o.Label, x: r.x + 12, y: y, w: 300, h: 16})
		y += 17
	}
	return out
}

// applyPawnEquipClick enqueues the Equip order behind a "pawn.equip.<id>"
// button. Routed from applyClick.
func (a *App) applyPawnEquipClick(id string) {
	n, err := strconv.ParseUint(strings.TrimPrefix(id, "pawn.equip."), 10, 64)
	if err != nil || a.renderer == nil {
		return
	}
	p, ok := a.selectedPawn()
	if !ok {
		return
	}
	for _, o := range a.equipNearby(p) {
		if uint64(o.Item) == n {
			a.renderer.enqueue(o.Cmd)
			a.equipCache.set = false
			return
		}
	}
}

func inspectorTabRow(r button, tabs []buttonSpec) []button {
	x := r.x + 8
	y := r.y + r.h - 28
	out := make([]button, 0, len(tabs))
	for _, t := range tabs {
		w := t.w
		if w == 0 {
			w = 56
		}
		out = append(out, button{
			id:       t.id,
			label:    t.label,
			x:        x,
			y:        y,
			w:        w,
			h:        22,
			disabled: t.disabled,
			reason:   t.reason,
			selected: t.selected,
			quiet:    t.disabled, // tabs are narrow: the reason shows on hover, no suffix
		})
		x += w + 4
	}
	return out
}

func (a *App) drawInspector(screen *ebiten.Image) {
	if a.renderer == nil || !a.renderer.HasSelect() || a.workOpen || a.archOpen {
		return
	}
	p, ok := a.selectedPawn()
	if !ok {
		return
	}
	r := inspectorRect()
	drawPanel(screen, r.x, r.y, r.w, r.h)
	title := pawnLabel(p)
	if p.Temperament != "" {
		title += "  - " + p.Temperament
	}
	drawLabel(screen, int(r.x)+12, int(r.y)+12, title)
	status := "colonist  " + p.Status
	if p.Reason != "" {
		status += " (" + p.Reason + ")"
	}
	if p.Drafted {
		status += "  drafted"
	}
	if line := waitingForLine(p.BlockedBy, a.creatureName); line != "" {
		status += "  " + line
	}
	if p.MentalBreak != "" {
		status = "MENTAL BREAK: " + p.MentalBreak
	}
	if p.Hypothermia > 0 {
		status += fmt.Sprintf("  hypothermia %d%%", p.Hypothermia/10)
	}
	if p.Heatstroke > 0 {
		status += fmt.Sprintf("  heatstroke %d%%", p.Heatstroke/10)
	}
	switch {
	case p.Infection > 0:
		status += fmt.Sprintf("  infection %d%%", p.Infection)
	case p.Wounded:
		status += "  wound untended"
	}
	if p.Tended && (p.Infection > 0 || p.Health < 100) {
		status += "  (tended)"
	}
	drawLabel(screen, int(r.x)+12, int(r.y)+32, status)
	a.drawPawnPortrait(screen, p, float64(r.x)+12, float64(r.y)+52, 48, 64)
	drawLabel(screen, int(r.x)+72, int(r.y)+56, fmt.Sprintf("job       %s", p.Job))
	drawLabel(screen, int(r.x)+72, int(r.y)+72, fmt.Sprintf("activity  %s", p.Activity))
	drawLabel(screen, int(r.x)+72, int(r.y)+88, fmt.Sprintf("tile      (%d, %d)  %s", p.X, p.Y, fmtTemp(p.Temp)))
	drawLabel(screen, int(r.x)+72, int(r.y)+104, fmt.Sprintf("weapon    %s", p.Weapon))
	if a.inspectTab == "log" {
		a.drawPawnLog(screen, p)
		mx, my := a.currentCursor()
		a.drawButtons(screen, a.inspectorButtons(), mx, my)
		return
	}
	if a.inspectTab == "gear" {
		drawGear(screen, p, int(r.x)+12, int(r.y)+126)
		mx, my := a.currentCursor()
		a.drawButtons(screen, a.inspectorButtons(), mx, my)
		return
	}
	drawNeedBar(screen, int(r.x)+12, int(r.y)+126, 300, 13, p.Health, "health")
	drawNeedBar(screen, int(r.x)+12, int(r.y)+146, 300, 13, p.Hunger, "food")
	drawNeedBar(screen, int(r.x)+12, int(r.y)+166, 300, 13, p.Rest, "rest")
	drawNeedBar(screen, int(r.x)+12, int(r.y)+186, 300, 13, p.Recreation, "fun")
	drawNeedBar(screen, int(r.x)+12, int(r.y)+206, 300, 13, p.Mood, "mood")
	drawLabel(screen, int(r.x)+12, int(r.y)+224, "feels "+p.MoodLabel)
	ty := int(r.y) + 242
	for i, th := range p.Thoughts {
		if i >= 5 {
			break
		}
		sign := "+"
		if th.Value < 0 {
			sign = ""
		}
		drawLabel(screen, int(r.x)+20, ty, fmt.Sprintf("%s%d  %s", sign, th.Value, th.Label))
		ty += 15
	}
	if a.renderer != nil {
		if lbl := roomLabelAt(a.renderer.world(), p.X, p.Y); lbl != "" {
			// right-aligned on the title row, clear of the status line
			drawLabel(screen, int(r.x+r.w)-36-len(lbl)*6, int(r.y)+12, lbl)
		}
	}
	mx, my := a.currentCursor()
	a.drawButtons(screen, a.inspectorButtons(), mx, my)
}

func drawNeedBar(screen *ebiten.Image, x, y, w, h, pct int, label string) {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	drawLabel(screen, x, y-2, fmt.Sprintf("%s  %d", label, pct))
	bg := color.RGBA{R: 40, G: 30, B: 24, A: 255}
	fill := color.RGBA{R: 180, G: 90, B: 50, A: 255}
	switch label {
	case "rest":
		fill = color.RGBA{R: 70, G: 110, B: 180, A: 255}
	case "mood":
		fill = moodColor(pct)
	case "fun":
		fill = color.RGBA{R: 170, G: 120, B: 190, A: 255}
	case "health":
		fill = healthColor(pct)
	}
	const lw = 84 // label column ("health  100")
	vector.FillRect(screen, float32(x+lw), float32(y), float32(w-lw), float32(h), bg, false)
	fw := float32(w-lw) * float32(pct) / 100
	vector.FillRect(screen, float32(x+lw), float32(y), fw, float32(h), fill, false)
}

func (a *App) selectedPawn() (game.PawnView, bool) {
	if a.renderer == nil || !a.renderer.HasSelect() {
		return game.PawnView{}, false
	}
	id := a.renderer.Selected()
	for _, p := range a.renderer.snapshot() {
		if p.ID == id {
			return p, true
		}
	}
	return game.PawnView{}, false
}

func moodColor(pct int) color.RGBA {
	switch {
	case pct < 30:
		return color.RGBA{R: 200, G: 70, B: 60, A: 255}
	case pct < 55:
		return color.RGBA{R: 210, G: 170, B: 70, A: 255}
	}
	return color.RGBA{R: 100, G: 180, B: 90, A: 255}
}

func healthColor(pct int) color.RGBA {
	switch {
	case pct < 30:
		return color.RGBA{R: 200, G: 60, B: 50, A: 255}
	case pct < 70:
		return color.RGBA{R: 210, G: 160, B: 60, A: 255}
	default:
		return color.RGBA{R: 90, G: 170, B: 80, A: 255}
	}
}

// drawGear lists what the colonist carries and wears and the comfortable
// temperature range that gives it.
func drawGear(screen *ebiten.Image, p game.PawnView, x, y int) {
	drawLabel(screen, x, y, "Weapon")
	drawLabel(screen, x+12, y+16, p.Weapon)
	drawLabel(screen, x, y+40, "Wearing")
	ly := y + 56
	drawLabel(screen, x+12, ly, "everyday clothes")
	for _, a := range p.Apparel {
		ly += 16
		drawLabel(screen, x+12, ly, gearLine(a))
	}
	drawLabel(screen, x, ly+28, fmt.Sprintf("Comfortable  %s to %s", fmtTemp(p.ComfortMin), fmtTemp(p.ComfortMax)))
	drawLabel(screen, x, ly+48, "Equip nearby")
}

func gearLine(a string) string {
	switch a {
	case "parka":
		return "leather parka  (much warmer, hot in summer)"
	case "tuque":
		return "knitted tuque  (warmer)"
	case "duster":
		return "leather duster (cooler in heat)"
	}
	return a
}

// waitingForLine is the read-only status fragment shown while a colonist is
// held up by another creature standing in its next cell.
func waitingForLine(blockedBy entity.EntityID, name func(entity.EntityID) string) string {
	if blockedBy == 0 {
		return ""
	}
	n := name(blockedBy)
	if n == "" {
		n = "something"
	}
	return "waiting for " + n
}

// creatureName resolves a pawn's or animal's display name from the snapshot.
func (a *App) creatureName(id entity.EntityID) string {
	if a.renderer == nil {
		return ""
	}
	for _, p := range a.renderer.game.Snapshot() {
		if p.ID == id {
			return pawnLabel(p)
		}
	}
	for _, an := range a.renderer.world().Animals {
		if an.ID == id {
			if an.Name != "" {
				return an.Name
			}
			return an.Species
		}
	}
	return ""
}
