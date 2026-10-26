package ebiten

import (
	"fmt"
	"image/color"
	"sort"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// insBarStep is the vertical step of a need bar on the Needs tab.
const insBarStep = 21

const (
	maxEquipShown      = 4  // entries in the Equip command's options
	maxEquipCandidates = 12 // nearest weapons and garments whose orders are probed
)

// equipOption is one reachable weapon or garment the selected colonist can
// pick up (Equip) or put on (Wear).
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

// equipCacheState is the Equip command's nearby list, kept per App
// (App.equipCache) and rebuilt when its key changes.
type equipCacheState struct {
	key  equipCacheKey
	set  bool
	opts []equipOption
}

// equipNearby lists up to maxEquipShown nearest loose weapons and garments
// (Chebyshev distance, ties by item id) for which the game offers an
// enabled Equip or Wear order (both are OrderEquip). The result is cached
// while nothing it depends on changes.
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
		if k, ok := item.ParseKind(it.Kind); it.Carried || !ok || !(k.IsWeapon() || k.IsApparel()) {
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
			if orderIs(command.OrderEquip)(o) && (strings.HasPrefix(o.Label, "Equip") || strings.HasPrefix(o.Label, "Wear")) && !o.Disabled {
				out = append(out, equipOption{Item: c.it.ID, Cmd: o.Cmd,
					Label: fmt.Sprintf("%s \u2014 %d tiles", o.Label, c.dist)})
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

// dirtyBootsLine is "Dirty boots N%" while the colonist carries dirt on
// their boots, else "".
func dirtyBootsLine(p game.PawnView) string {
	if p.Dirt <= 0 {
		return ""
	}
	return fmt.Sprintf("Dirty boots %d%%", p.Dirt)
}

// drawNeedBar draws one need as "label  pct" and a bar (the label column
// is 100 px wide, the bar fills the rest of w). The label picks the fill:
// health and mood have their own scales, any other need is calm until it
// runs low (a plant's growth stage takes the same bar).
func drawNeedBar(screen *ebiten.Image, x, y, w, h, pct int, label string) {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	drawText(screen, fmt.Sprintf("%s  %d", label, pct), float64(x), float64(y)+float64(h)/2-lineH(tsBody)/2, tsBody)
	fill := uiSliderFill
	switch label {
	case "mood":
		fill = moodColor(pct)
	case "health":
		fill = healthColor(pct)
	case "food", "rest", "fun":
		fill = needColor(pct)
	}
	const lw = 100 // label column ("health  100", measured: ~80 px)
	uiFillRect(screen, float32(x+lw), float32(y), float32(w-lw), float32(h), uiSliderTrack, false)
	fw := float32(w-lw) * float32(pct) / 100
	uiFillRect(screen, float32(x+lw), float32(y), fw, float32(h), fill, false)
}

// pawnView is colonist id in the snapshot.
func (a *App) pawnView(id entity.EntityID) (game.PawnView, bool) {
	if a.renderer == nil {
		return game.PawnView{}, false
	}
	for _, p := range a.renderer.snapshot() {
		if p.ID == id {
			return p, true
		}
	}
	return game.PawnView{}, false
}

func (a *App) selectedPawn() (game.PawnView, bool) {
	if a.renderer == nil || !a.renderer.HasSelect() {
		return game.PawnView{}, false
	}
	return a.pawnView(a.renderer.Selected())
}

// moodColor is the mood bar's fill: bad, middling or good.
func moodColor(pct int) color.RGBA {
	switch {
	case pct < 30:
		return uiMoodBad
	case pct < 55:
		return uiMoodMid
	}
	return uiMoodGood
}

// healthColor is the health bar's fill.
func healthColor(pct int) color.RGBA {
	switch {
	case pct < 30:
		return uiDanger
	case pct < 70:
		return uiWarning
	default:
		return uiGood
	}
}

// needColor is a need bar's fill (food, rest, fun): calm while it is
// fine, amber when low, red when dire.
func needColor(pct int) color.RGBA {
	switch {
	case pct < 15:
		return uiDanger
	case pct < 30:
		return uiWarning
	}
	return uiSliderFill
}

// gearLayout is the Gear tab's vertical layout (offsets from the top of
// its content): drawGear paints it.
type gearLayout struct {
	weaponVal, wearing, clothes, comfort, hint float32
}

// gearLayoutFor lays the tab out for lines Wearing rows below "everyday
// clothes".
func gearLayoutFor(lines int) gearLayout {
	hs, ss := lineStep(tsLabel), lineStep(tsSmall)
	var g gearLayout
	g.weaponVal = hs
	g.wearing = hs + ss + 6
	g.clothes = g.wearing + hs
	g.comfort = g.clothes + float32(lines+1)*ss + 6
	g.hint = g.comfort + ss + 6
	return g
}

// drawGear lists what the colonist carries and wears and the comfortable
// temperature range that gives it, inside in. Rows that do not fit are
// left out.
func drawGear(screen *ebiten.Image, p game.PawnView, in rect) {
	rows := gearRows(p)
	g := gearLayoutFor(len(rows) - 1)
	fx, fy := float64(in.X), float64(in.Y)
	head := withColor(tsLabel, uiHeading)
	ss := float64(lineStep(tsSmall))
	fits := func(off float64, st textStyle) bool { return float32(fy+off+float64(lineStep(st))) <= in.bottom() }
	drawText(screen, "Weapon", fx, fy, head)
	drawText(screen, ellipsize(p.Weapon, tsSmall, float64(in.W)-12), fx+12, fy+float64(g.weaponVal), tsSmall)
	drawText(screen, "Wearing", fx, fy+float64(g.wearing), head)
	ly := fy + float64(g.clothes)
	for _, row := range rows {
		if !fits(ly-fy, tsSmall) {
			break
		}
		drawText(screen, ellipsize(row, tsSmall, float64(in.W)-12), fx+12, ly, tsSmall)
		ly += ss
	}
	if fits(float64(g.comfort), tsSmall) {
		drawText(screen, fmt.Sprintf("Comfortable  %s to %s", fmtTemp(p.ComfortMin), fmtTemp(p.ComfortMax)), fx, fy+float64(g.comfort), tsSmall)
	}
	if fits(float64(g.hint), tsSmall) {
		drawText(screen, ellipsize("Weapons and clothes lying nearby: the Equip command.", tsSmall, float64(in.W)), fx, fy+float64(g.hint), withColor(tsSmall, uiTextDim))
	}
}

// drawGearTab is the Gear tab's body, drawn into r, the expanded panel's
// inner rectangle.
func (a *App) drawGearTab(screen *ebiten.Image, id entity.EntityID, r rect) {
	if p, ok := a.pawnView(id); ok {
		drawGear(screen, p, r)
	}
}

// drawNeedsTab is the Needs tab's body, drawn into r, the expanded panel's
// inner rectangle: every need as a bar, how the colonist feels, and the
// thoughts behind the mood. Rows that do not fit are left out.
func (a *App) drawNeedsTab(screen *ebiten.Image, id entity.EntityID, r rect) {
	p, ok := a.pawnView(id)
	if !ok {
		return
	}
	in := r
	x, y, bw := int(in.X), int(in.Y)+3, int(in.W) // the first label stands a little above its bar
	needs := []struct {
		pct   int
		label string
	}{{p.Health, "health"}, {p.Hunger, "food"}, {p.Rest, "rest"}, {p.Recreation, "fun"}, {p.Mood, "mood"}}
	for i, n := range needs {
		by := y + i*insBarStep
		if float32(by+13) > in.bottom() {
			return
		}
		drawNeedBar(screen, x, by, bw, 13, n.pct, n.label)
	}
	fy := float64(y + len(needs)*insBarStep + 2)
	if float32(fy)+lineStep(tsBody) > in.bottom() {
		return
	}
	drawText(screen, "feels "+p.MoodLabel, float64(x), fy, withColor(tsBody, uiTextDim))
	ty := fy + float64(lineStep(tsBody)) + 4
	for i, th := range p.Thoughts {
		if i >= 5 || float32(ty)+lineStep(tsSmall) > in.bottom() {
			break
		}
		sign := "+"
		if th.Value < 0 {
			sign = ""
		}
		drawText(screen, ellipsize(fmt.Sprintf("%s%d  %s", sign, th.Value, th.Label), tsSmall, float64(in.W)-8), float64(x)+8, ty, tsSmall)
		ty += float64(lineStep(tsSmall))
	}
}

// gearRows is the Wearing list (PLAY-017): everyday clothes, each garment
// (by name and "(you chose this)" when the player ordered it, else with
// what it does), and the garment the colonist is walking to put on.
func gearRows(p game.PawnView) []string {
	rows := []string{"everyday clothes"}
	for i, a := range p.Apparel {
		l := gearLine(a)
		if i < len(p.ApparelForced) && p.ApparelForced[i] {
			name, _, _ := strings.Cut(l, " (")
			l = strings.TrimSpace(name) + "  (you chose this)"
		}
		rows = append(rows, l)
	}
	if p.ChangingInto != "" {
		rows = append(rows, "Changing into: "+p.ChangingInto)
	}
	return rows
}

func gearLine(a string) string {
	switch a {
	case "parka":
		return "leather parka  (much warmer, hot in summer)"
	case "wool parka":
		return "wool parka  (much warmer, hot in summer)"
	case "tuque":
		return "knitted tuque  (warmer)"
	case "duster":
		return "leather duster (cooler in heat)"
	}
	return a
}

// waitingForLine is the read-only status fragment shown while a colonist is
// held up by another creature standing in its next cell.
// postponedLine is the colonist's work put off because it was blocked or
// out of reach (SIM-027): the first entry, plus how many more.
func postponedLine(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	ln := "put off: " + lines[0]
	if len(lines) > 1 {
		ln += fmt.Sprintf(" (+%d)", len(lines)-1)
	}
	return ln
}

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
