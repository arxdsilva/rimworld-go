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
	"rimworld-go/internal/platform/input"
)

// inspectorRect is the pawn inspector's panel. The Bio tab (bio_ui.go) is
// why it is 400 x 500: the seven tabs (Noto Sans, sized by measure) and the
// skill table need the room.
func inspectorRect() button {
	return button{x: hudPad, y: hudTopH + hudPad, w: 400, h: 500}
}

// Inspector rows are laid out on the body font's line height (NG-1 §1.1).
const (
	insStatusY  = 32  // the (up to two-line) status block
	insInfoY    = 72  // top of the portrait and the job/activity/tile/weapon rows
	insInfoLblW = 58  // the info rows' label column
	insBodyY    = 192 // top of the need bars / the gear lists
	insBarStep  = 21  // vertical step of a need bar
	insStatusN  = 2   // status lines kept (the rest is cut with an ellipsis)
)

// tabW is the width of a tab button for label: its text plus the button padding.
func tabW(label string) float32 {
	return textW(label, tsBody) + 16
}

func (a *App) inspectorButtons() []button {
	if a.renderer == nil || !a.renderer.HasSelect() || a.workOpen || a.archOpen {
		return nil
	}
	r := inspectorRect()
	bioOn := a.bioTabOn()
	bio := buttonSpec{id: "inspector.bio", label: "Bio", selected: bioOn, w: tabW("Bio"), h: 22}
	if p, ok := a.selectedPawn(); ok && !hasBio(p) {
		bio.disabled, bio.reason, bio.selected = true, "No biography for this colonist", false
	}
	prisonerTab, prisonerOn := a.prisonerTabSpec() // prisoner_ui.go
	tabs := []buttonSpec{
		{id: "inspector.character", label: "Character", selected: a.inspectTab != "needs" && a.inspectTab != "gear" && a.inspectTab != "log" &&
			a.inspectTab != "health" && a.inspectTab != "social" && !bioOn && !(prisonerOn && a.inspectTab == "prisoner"), w: tabW("Character"), h: 22},
		{id: "inspector.needs", label: "Needs", selected: a.inspectTab == "needs", w: tabW("Needs"), h: 22},
		bio,
		{id: "inspector.health", label: "Health", selected: a.inspectTab == "health", w: tabW("Health"), h: 22},
		{id: "inspector.social", label: "Social", selected: a.inspectTab == "social", w: tabW("Social"), h: 22},
		{id: "inspector.gear", label: "Gear", selected: a.inspectTab == "gear", w: tabW("Gear"), h: 22},
		{id: "inspector.log", label: "Log", selected: a.inspectTab == "log", w: tabW("Log"), h: 22},
	}
	if prisonerOn {
		tabs[5] = prisonerTab // a prisoner, disarmed, has its Prisoner tab where Gear was
	}
	out := inspectorTabRow(r, tabs)
	if p, ok := a.selectedPawn(); ok {
		out = append(out, a.pawnActionBar(p, r)...)
		if a.inspectTab == "gear" && !prisonerOn {
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
	label := "Draft" + a.keyHint(input.Draft)
	if p.Drafted {
		label = "Undraft" + a.keyHint(input.Draft)
	}
	y := r.y + r.h - 62
	draft := button{id: "pawn.draft", label: label, x: r.x + 12, y: y, w: 120, h: 24, selected: p.Drafted, quiet: true}
	clear := button{id: "pawn.clearorders", label: "Clear orders", x: r.x + 140, y: y, w: 120, h: 24, quiet: true}
	switch {
	case p.Status != "alive":
		draft.disabled, draft.reason = true, p.Status
		clear.disabled, clear.reason = true, p.Status
	case len(p.QueuedOrders) == 0:
		clear.disabled, clear.reason = true, "no queued orders"
	}
	return append([]button{draft, clear}, a.prisonerActions(p, r)...) // prisoner_ui.go: Capture / Arrest
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
	return r.y + insBodyY + gearLayoutFor(len(p.Apparel)).equipList
}

// equipNearbyButtons builds the Gear tab's "Equip nearby" list as buttons
// (pure: no drawing). Dead or downed colonists get one disabled line.
func (a *App) equipNearbyButtons(p game.PawnView, r button) []button {
	y := equipListY(r, p)
	limit := r.y + r.h - 66 // keep clear of the action bar
	if p.Status != "alive" {
		return []button{{id: "pawn.equip.none", label: "Equip nearby", x: r.x + 12, y: y, w: r.w - 24, h: 22,
			disabled: true, quiet: true, reason: p.Status}}
	}
	opts := a.equipNearby(p)
	if len(opts) == 0 {
		return []button{{id: "pawn.equip.none", label: "no weapons in reach", x: r.x + 12, y: y, w: r.w - 24, h: 22,
			disabled: true, quiet: true, reason: "no weapons in reach"}}
	}
	var out []button
	for _, o := range opts {
		if y+22 > limit {
			break
		}
		out = append(out, button{id: fmt.Sprintf("pawn.equip.%d", o.Item), label: o.Label, x: r.x + 12, y: y, w: r.w - 24, h: 22})
		y += 24
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
		x += w + 3
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
	if a.bioTabOn() || a.kitTabOn() || a.prisonerTabOn() {
		// bio_ui.go, social_tabs_ui.go and prisoner_ui.go declare and paint the tab's
		// content with the widget kit.
		mx, my := a.currentCursor()
		a.drawButtons(screen, a.inspectorButtons(), mx, my)
		return
	}
	rt := ""
	if a.renderer != nil {
		rt = roomLabelAt(a.renderer.world(), p.X, p.Y)
	}
	title := pawnLabel(p)
	if p.Temperament != "" {
		title += "  - " + p.Temperament
	}
	x, y := float64(r.x), float64(r.y)
	titleW := float64(r.w) - 12 - 36
	if rt != "" {
		// the room label sits right-aligned on the title row, clear of the close button
		drawText(screen, rt, x+float64(r.w)-36, y+10, textStyle{Size: tsBody.Size, Color: uiTextDim, Align: alignRight})
		titleW -= float64(textW(rt, tsBody)) + 12
	}
	drawText(screen, ellipsize(title, tsLabel, titleW), x+12, y+10, withColor(tsLabel, uiHeading))
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
	lines := wrapPx(status, tsSmall, float64(r.w)-24)
	if len(lines) > insStatusN {
		lines[insStatusN-1] = ellipsize(lines[insStatusN-1]+" "+strings.Join(lines[insStatusN:], " "), tsSmall, float64(r.w)-24)
		lines = lines[:insStatusN]
	}
	for i, ln := range lines {
		drawText(screen, ln, x+12, y+insStatusY+float64(i)*lineH(tsSmall), withColor(tsSmall, uiTextDim))
	}
	a.drawPawnPortrait(screen, p, x+12, y+insInfoY, 48, 64)
	info := [][2]string{
		{"job", p.Job},
		{"activity", fmt.Sprint(p.Activity)},
		{"tile", fmt.Sprintf("(%d, %d)  %s", p.X, p.Y, fmtTemp(p.Temp))},
		{"weapon", p.Weapon},
	}
	if ln := postponedLine(p.Postponed); ln != "" {
		info = append(info, [2]string{"", ln})
	}
	if ln := dirtyBootsLine(p); ln != "" {
		info = append(info, [2]string{"", ln})
	}
	rowH := float64(lineStep(tsBody))
	valX := x + 72 + insInfoLblW
	for i, kv := range info {
		ry := y + insInfoY + float64(i)*rowH
		if kv[0] != "" {
			drawText(screen, kv[0], x+72, ry, withColor(tsBody, uiTextDim))
		}
		vx := valX
		if kv[0] == "" {
			vx = x + 72
		}
		drawText(screen, ellipsize(kv[1], tsBody, x+float64(r.w)-12-vx), vx, ry, tsBody)
	}
	if a.inspectTab == "log" {
		a.drawPawnLog(screen, p)
		mx, my := a.currentCursor()
		a.drawButtons(screen, a.inspectorButtons(), mx, my)
		return
	}
	if a.inspectTab == "gear" {
		drawGear(screen, p, int(r.x)+12, int(r.y)+insBodyY)
		mx, my := a.currentCursor()
		a.drawButtons(screen, a.inspectorButtons(), mx, my)
		return
	}
	by := int(r.y) + insBodyY
	bw := int(r.w) - 24
	drawNeedBar(screen, int(r.x)+12, by, bw, 13, p.Health, "health")
	drawNeedBar(screen, int(r.x)+12, by+insBarStep, bw, 13, p.Hunger, "food")
	drawNeedBar(screen, int(r.x)+12, by+2*insBarStep, bw, 13, p.Rest, "rest")
	drawNeedBar(screen, int(r.x)+12, by+3*insBarStep, bw, 13, p.Recreation, "fun")
	drawNeedBar(screen, int(r.x)+12, by+4*insBarStep, bw, 13, p.Mood, "mood")
	drawText(screen, "feels "+p.MoodLabel, x+12, float64(by+5*insBarStep)+2, withColor(tsBody, uiTextDim))
	ty := float64(by+6*insBarStep) + 6
	for i, th := range p.Thoughts {
		if i >= 5 {
			break
		}
		sign := "+"
		if th.Value < 0 {
			sign = ""
		}
		drawText(screen, ellipsize(fmt.Sprintf("%s%d  %s", sign, th.Value, th.Label), tsSmall, float64(r.w)-32), x+20, ty, tsSmall)
		ty += float64(lineStep(tsSmall))
	}
	mx, my := a.currentCursor()
	a.drawButtons(screen, a.inspectorButtons(), mx, my)
}

// dirtyBootsLine is "Dirty boots N%" while the colonist carries dirt on
// their boots, else "".
func dirtyBootsLine(p game.PawnView) string {
	if p.Dirt <= 0 {
		return ""
	}
	return fmt.Sprintf("Dirty boots %d%%", p.Dirt)
}

func drawNeedBar(screen *ebiten.Image, x, y, w, h, pct int, label string) {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	drawText(screen, fmt.Sprintf("%s  %d", label, pct), float64(x), float64(y)+float64(h)/2-lineH(tsBody)/2, tsBody)
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
	const lw = 100 // label column ("health  100", measured: ~80 px)
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

// gearLayout is the Gear tab's vertical layout (offsets from the top of
// the body at insBodyY): drawGear paints it and equipListY puts the
// "Equip nearby" buttons under it.
type gearLayout struct {
	weaponVal, wearing, clothes, comfort, equipHead, equipList float32
}

func gearLayoutFor(apparel int) gearLayout {
	hs, ss := lineStep(tsLabel), lineStep(tsSmall)
	var g gearLayout
	g.weaponVal = hs
	g.wearing = hs + ss + 6
	g.clothes = g.wearing + hs
	g.comfort = g.clothes + float32(apparel+1)*ss + 6
	g.equipHead = g.comfort + ss + 6
	g.equipList = g.equipHead + hs + 2
	return g
}

// drawGear lists what the colonist carries and wears and the comfortable
// temperature range that gives it.
func drawGear(screen *ebiten.Image, p game.PawnView, x, y int) {
	g := gearLayoutFor(len(p.Apparel))
	fx, fy := float64(x), float64(y)
	head := withColor(tsLabel, uiHeading)
	drawText(screen, "Weapon", fx, fy, head)
	drawText(screen, p.Weapon, fx+12, fy+float64(g.weaponVal), tsSmall)
	drawText(screen, "Wearing", fx, fy+float64(g.wearing), head)
	ly := fy + float64(g.clothes)
	ss := float64(lineStep(tsSmall))
	drawText(screen, "everyday clothes", fx+12, ly, tsSmall)
	for _, a := range p.Apparel {
		ly += ss
		drawText(screen, gearLine(a), fx+12, ly, tsSmall)
	}
	drawText(screen, fmt.Sprintf("Comfortable  %s to %s", fmtTemp(p.ComfortMin), fmtTemp(p.ComfortMax)), fx, fy+float64(g.comfort), tsSmall)
	drawText(screen, "Equip nearby", fx, fy+float64(g.equipHead), head)
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
