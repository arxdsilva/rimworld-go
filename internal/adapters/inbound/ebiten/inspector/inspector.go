package inspector

import (
	"fmt"
	"image/color"
	"sort"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
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
	sim     Sim
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
func (a Pawn) equipNearby(p game.PawnView) []equipOption {
	if a.sim() == nil || p.Status != "alive" {
		return nil
	}
	items := a.shell.World().Items
	key := equipCacheKey{sim: a.sim(), pawn: p.ID, tick: a.shell.Tick(), x: p.X, y: p.Y,
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
		for _, o := range a.sim().OrdersAt(p.ID, c.it.X, c.it.Y) {
			if OrderIs(command.OrderEquip)(o) && (strings.HasPrefix(o.Label, "Equip") || strings.HasPrefix(o.Label, "Wear")) && !o.Disabled {
				out = append(out, equipOption{Item: c.it.ID, Cmd: o.Cmd,
					Label: fmt.Sprintf("%s \u2014 %d tiles", o.Label, c.dist)})
				break
			}
		}
	}
	a.equipCache.key, a.equipCache.set, a.equipCache.opts = key, true, out
	return out
}

func TitleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// dirtyBootsLine is "Dirty boots N%" while the colonist carries dirt on
// their boots, else "".
func DirtyBootsLine(p game.PawnView) string {
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
	gfx.Text(screen, fmt.Sprintf("%s  %d", label, pct), float64(x), float64(y)+float64(h)/2-gfx.LineH(gfx.Body)/2, gfx.Body)
	fill := uikit.SliderFill
	switch label {
	case "mood":
		fill = MoodColor(pct)
	case "health":
		fill = HealthColor(pct)
	case "food", "rest", "fun":
		fill = needColor(pct)
	}
	const lw = 100 // label column ("health  100", measured: ~80 px)
	gfx.FillRect(screen, float32(x+lw), float32(y), float32(w-lw), float32(h), uikit.SliderTrack, false)
	fw := float32(w-lw) * float32(pct) / 100
	gfx.FillRect(screen, float32(x+lw), float32(y), fw, float32(h), fill, false)
}

// pawnView is colonist id in the snapshot.
func (a *Pane) pawnView(id entity.EntityID) (game.PawnView, bool) {
	if a.sim() == nil {
		return game.PawnView{}, false
	}
	for _, p := range a.shell.Pawns() {
		if p.ID == id {
			return p, true
		}
	}
	return game.PawnView{}, false
}

func (a *Pane) SelectedPawn() (game.PawnView, bool) {
	id, ok := a.shell.Selected()
	if !ok {
		return game.PawnView{}, false
	}
	return a.pawnView(id)
}

// moodColor is the mood bar's fill: bad, middling or good.
func MoodColor(pct int) color.RGBA {
	switch {
	case pct < 30:
		return uikit.MoodBad
	case pct < 55:
		return uikit.MoodMid
	}
	return uikit.MoodGood
}

// healthColor is the health bar's fill.
func HealthColor(pct int) color.RGBA {
	switch {
	case pct < 30:
		return uikit.Danger
	case pct < 70:
		return uikit.Warning
	default:
		return uikit.Good
	}
}

// needColor is a need bar's fill (food, rest, fun): calm while it is
// fine, amber when low, red when dire.
func needColor(pct int) color.RGBA {
	switch {
	case pct < 15:
		return uikit.Danger
	case pct < 30:
		return uikit.Warning
	}
	return uikit.SliderFill
}

// GearLayout is the Gear tab's vertical layout (offsets from the top of
// its content): drawGear paints it.
type GearLayout struct {
	WeaponVal, Wearing, Clothes, Comfort, Hint float32
}

// gearLayoutFor lays the tab out for lines Wearing rows below "everyday
// clothes".
func GearLayoutFor(lines int) GearLayout {
	hs, ss := gfx.LineStep(gfx.Label), gfx.LineStep(gfx.Small)
	var g GearLayout
	g.WeaponVal = hs
	g.Wearing = hs + ss + 6
	g.Clothes = g.Wearing + hs
	g.Comfort = g.Clothes + float32(lines+1)*ss + 6
	g.Hint = g.Comfort + ss + 6
	return g
}

// drawGear lists what the colonist carries and wears and the comfortable
// temperature range that gives it, inside in. Rows that do not fit are
// left out.
func drawGear(screen *ebiten.Image, p game.PawnView, in gfx.Rect, temp func(tenths int) string) {
	rows := GearRows(p)
	g := GearLayoutFor(len(rows) - 1)
	fx, fy := float64(in.X), float64(in.Y)
	head := gfx.WithColor(gfx.Label, uikit.Heading)
	ss := float64(gfx.LineStep(gfx.Small))
	fits := func(off float64, st gfx.TextStyle) bool {
		return float32(fy+off+float64(gfx.LineStep(st))) <= in.Bottom()
	}
	gfx.Text(screen, "Weapon", fx, fy, head)
	gfx.Text(screen, gfx.Ellipsize(p.Weapon, gfx.Small, float64(in.W)-12), fx+12, fy+float64(g.WeaponVal), gfx.Small)
	gfx.Text(screen, "Wearing", fx, fy+float64(g.Wearing), head)
	ly := fy + float64(g.Clothes)
	for _, row := range rows {
		if !fits(ly-fy, gfx.Small) {
			break
		}
		gfx.Text(screen, gfx.Ellipsize(row, gfx.Small, float64(in.W)-12), fx+12, ly, gfx.Small)
		ly += ss
	}
	if fits(float64(g.Comfort), gfx.Small) {
		gfx.Text(screen, fmt.Sprintf("Comfortable  %s to %s", temp(p.ComfortMin), temp(p.ComfortMax)), fx, fy+float64(g.Comfort), gfx.Small)
	}
	if fits(float64(g.Hint), gfx.Small) {
		gfx.Text(screen, gfx.Ellipsize("Weapons and clothes lying nearby: the Equip command.", gfx.Small, float64(in.W)), fx, fy+float64(g.Hint), gfx.WithColor(gfx.Small, uikit.TextDim))
	}
}

// drawGearTab is the Gear tab's body, drawn into r, the expanded panel's
// inner rectangle.
func (a Pawn) drawGearTab(screen *ebiten.Image, id entity.EntityID, r gfx.Rect) {
	if p, ok := a.pawnView(id); ok {
		drawGear(screen, p, r, a.shell.Temp)
		lines := GearStatLines(p)
		y := float64(r.Y) + float64(GearLayoutFor(len(GearRows(p))-1).Hint) + float64(gfx.LineStep(gfx.Small)) + 6
		for _, l := range lines {
			if float32(y)+gfx.LineStep(gfx.Small) > r.Bottom() {
				break
			}
			gfx.Text(screen, gfx.Ellipsize(l, gfx.Small, float64(r.W)), float64(r.X), y, gfx.Small)
			y += float64(gfx.LineStep(gfx.Small))
		}
	}
}

// drawNeedsTab is the Needs tab's body, drawn into r, the expanded panel's
// inner rectangle: every need as a bar, how the colonist feels, and the
// thoughts behind the mood. Rows that do not fit are left out.
func (a Pawn) drawNeedsTab(screen *ebiten.Image, id entity.EntityID, r gfx.Rect) {
	p, ok := a.pawnView(id)
	if !ok {
		return
	}
	in := r
	x, y, bw := int(in.X), int(in.Y)+3, int(in.W) // the first label stands a little above its bar
	needs := []struct {
		pct   int
		label string
	}{{p.Health, "health"}, {p.Hunger, "food"}, {p.Rest, "rest"}, {p.Recreation, "fun"}, {p.Comfort, "comfort"}, {p.Beauty, "beauty"}, {p.Outdoors, "outdoors"}, {p.Mood, "mood"}}
	for i, n := range needs {
		by := y + i*insBarStep
		if float32(by+13) > in.Bottom() {
			return
		}
		drawNeedBar(screen, x, by, bw, 13, n.pct, n.label)
	}
	fy := float64(y + len(needs)*insBarStep + 2)
	if float32(fy)+gfx.LineStep(gfx.Body) > in.Bottom() {
		return
	}
	gfx.Text(screen, "feels "+p.MoodLabel, float64(x), fy, gfx.WithColor(gfx.Body, uikit.TextDim))
	ty := fy + float64(gfx.LineStep(gfx.Body)) + 4
	for i, th := range p.Thoughts {
		if i >= 5 || float32(ty)+gfx.LineStep(gfx.Small) > in.Bottom() {
			break
		}
		sign := "+"
		if th.Value < 0 {
			sign = ""
		}
		gfx.Text(screen, gfx.Ellipsize(fmt.Sprintf("%s%d  %s", sign, th.Value, th.Label), gfx.Small, float64(in.W)-8), float64(x)+8, ty, gfx.Small)
		ty += float64(gfx.LineStep(gfx.Small))
	}
}

// gearRows is the Wearing list (PLAY-017): everyday clothes, each garment
// (by name and "(you chose this)" when the player ordered it, else with
// what it does), and the garment the colonist is walking to put on.
func GearRows(p game.PawnView) []string {
	rows := []string{"everyday clothes"}
	for i, a := range p.Apparel {
		l := GearLine(a)
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

func GearLine(a string) string {
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
func PostponedLine(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	ln := "put off: " + lines[0]
	if len(lines) > 1 {
		ln += fmt.Sprintf(" (+%d)", len(lines)-1)
	}
	return ln
}

func WaitingForLine(blockedBy entity.EntityID, name func(entity.EntityID) string) string {
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
func (a Pawn) CreatureName(id entity.EntityID) string {
	if a.sim() == nil {
		return ""
	}
	for _, ps := range [2][]game.PawnView{a.shell.Pawns(), a.shell.Hostiles()} {
		for _, p := range ps {
			if p.ID == id {
				return a.shell.PawnLabel(p)
			}
		}
	}
	for _, an := range a.shell.World().Animals {
		if an.ID == id {
			if an.Name != "" {
				return an.Name
			}
			return an.Species
		}
	}
	return ""
}
