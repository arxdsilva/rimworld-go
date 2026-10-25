package ebiten

import (
	"fmt"
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/platform/input"
)

// Developer mode (UI-016; Options › Gameplay): the Dev input context with
// ultrafast speed, a single-tick step while paused, a debug actions menu,
// an on-screen debug log and a debug inspector. The menu calls the
// game's developer hooks through devTools, a consumer-owned interface used
// only while developer mode is on; nothing else in the client touches the
// game outside commands.

// devTools is the behaviour the debug actions menu needs.
type devTools interface {
	SpawnAnimal(species string, x, y int) (entity.EntityID, error)
	KillAnimal(id entity.EntityID)
	FireIncident(key string) bool
	Raid(n int)
	SpawnStack(kind item.Kind, qty, x, y int) (entity.EntityID, error)
	SetNeeds(id entity.EntityID, hunger, rest int)
	DevLosePart(pawn entity.EntityID, part string) bool
	DevSpawnImplants(near entity.EntityID) bool
}

var _ devTools = (*game.Game)(nil)

// devState is developer mode's presentation state.
type devState struct {
	menu   bool // debug actions menu open
	mx, my int  // where it opened (screen)
	tx, ty int  // the tile under it
	log    bool // debug log panel
	insp   bool // debug inspector panel

	fields mapFieldsOverlay // Map fields overlay (mapfields_overlay.go)
}

// devMode reports developer mode is on.
func (a *App) devMode() bool { return a.settings.Gameplay.DevMode && !a.commitment() } // never in a commitment colony

// devAction is one debug menu entry.
type devAction struct {
	label  string
	reason string // why it is disabled ("" enabled)
	run    func(d devTools) string
	icon   string // ui_icon/<name>, drawn at the row's right
}

// needsFull is a colonist's full hunger and rest (domain needs.Full).
const needsFull = 10000

// devActions is the debug menu for the tile (tx, ty).
func (a *App) devActions(tx, ty int) []devAction {
	spawn := func(species string) devAction {
		return devAction{label: "Spawn " + strings.ReplaceAll(species, "_", " ") + " here", run: func(d devTools) string {
			if _, err := d.SpawnAnimal(species, tx, ty); err != nil {
				return err.Error()
			}
			return "Spawned " + species
		}}
	}
	fire := func(key, name string) devAction {
		return devAction{label: "Event: " + name, run: func(d devTools) string {
			if d.FireIncident(key) {
				return name + " fired"
			}
			return name + " does not fit now"
		}}
	}
	out := []devAction{
		spawn("deer"), spawn("timber_wolf"),
		{label: "Add 100 wood here", run: func(d devTools) string {
			if _, err := d.SpawnStack(item.Wood, 100, tx, ty); err != nil {
				return err.Error()
			}
			return "Added 100 wood"
		}},
		fire("herd_crossing", "herd crossing"), fire("farm_animals", "farm animals"), fire("mad_animal", "mad animal"),
		{label: "Raid (3 raiders)", run: func(d devTools) string { d.Raid(3); return "Raid sent" }},
	}
	kill := devAction{label: "Kill selected animal", reason: "select an animal first"}
	if an, ok := a.selectedAnimal(); ok {
		kill.reason, kill.run = "", func(d devTools) string { d.KillAnimal(an.ID); return "Killed " + an.Label }
	}
	feed := devAction{label: "Fill selected colonist's needs", reason: "select a colonist first"}
	if p, ok := a.selectedPawn(); ok {
		feed.reason, feed.run = "", func(d devTools) string {
			d.SetNeeds(p.ID, needsFull, needsFull)
			return pawnLabel(p) + " fed and rested"
		}
	}
	lose := devAction{label: "Lose a part...", reason: "select a colonist first"}
	spawnImp := devAction{label: "Spawn implants", reason: "select a colonist first"}
	if p, ok := a.selectedPawn(); ok {
		lose.reason, lose.run = "", func(d devTools) string { a.openDevLosePicker(p, d); return "" }
		spawnImp.reason, spawnImp.run = "", func(d devTools) string {
			if d.DevSpawnImplants(p.ID) {
				return "Implants spawned near " + pawnLabel(p)
			}
			return "No room for implants"
		}
	}
	start := devAction{label: "Start fire here", icon: "start_fire", run: func(devTools) string {
		a.renderer.enqueue(command.Command{Kind: command.DevStartFire, X: tx, Y: ty})
		return "Fire started"
	}}
	return append(out, kill, feed, lose, spawnImp, start, a.mapFieldsAction())
}

// handleDevKeys runs the Dev context's actions.
func (a *App) handleDevKeys() {
	if !a.devMode() {
		return
	}
	f := &a.in
	r := a.renderer
	switch {
	case f.fired(input.DevUltrafast):
		if r.speed == speedUltra {
			r.setSpeed(speedNormal)
		} else {
			r.setSpeed(speedUltra)
		}
	case f.fired(input.DevStep):
		if r.speed == speedPaused {
			r.stepTicks++
		} else {
			a.toast("Pause first to step one tick")
		}
	case f.fired(input.DevActions):
		mx, my := a.currentCursor()       // logical: where the menu panel is drawn
		tx, ty := r.TileAt(a.mapCursor()) // physical: the map tile under the cursor
		a.dev.menu = !a.dev.menu
		a.dev.mx, a.dev.my, a.dev.tx, a.dev.ty = mx, my, tx, ty
	case f.fired(input.DevLog):
		a.dev.log = !a.dev.log
	case f.fired(input.DevInspector):
		a.dev.insp = !a.dev.insp
	}
}

const devRowH = 20

func (a *App) devMenuRect() button {
	n := len(a.devActions(a.dev.tx, a.dev.ty))
	sw, sh := a.size()
	w, h := float32(260), float32(24+n*devRowH)
	x, y := float32(a.dev.mx+8), float32(a.dev.my)
	x = min(x, float32(sw)-w-4)
	y = max(hudTopH, min(y, hudBarY(sh)-h-4))
	return button{x: x, y: y, w: w, h: h}
}

// devButtons are the debug menu's entries ("dev.act.N").
func (a *App) devButtons() []button {
	if !a.dev.menu || !a.devMode() || a.renderer == nil {
		return nil
	}
	r := a.devMenuRect()
	var out []button
	for i, act := range a.devActions(a.dev.tx, a.dev.ty) {
		out = append(out, button{id: "dev.act." + strconv.Itoa(i), label: act.label, x: r.x + 4, y: r.y + 22 + float32(i*devRowH),
			w: r.w - 8, h: devRowH - 2, disabled: act.reason != "", reason: act.reason, quiet: true})
	}
	return out
}

// applyDevClick runs a debug action through devTools.
func (a *App) applyDevClick(id string) {
	i, err := strconv.Atoi(strings.TrimPrefix(id, "dev.act."))
	acts := a.devActions(a.dev.tx, a.dev.ty)
	a.dev.menu = false
	if err != nil || i < 0 || i >= len(acts) || acts[i].run == nil || a.renderer == nil {
		return
	}
	d, ok := a.renderer.game.(devTools)
	if !ok {
		a.toast("Debug actions need the full game")
		return
	}
	if msg := acts[i].run(d); msg != "" {
		a.toast(msg)
	}
}

// drawDevPanels draws the debug menu, log and inspector.
func (a *App) drawDevPanels(screen *ebiten.Image, sw, sh int) {
	if !a.devMode() || a.renderer == nil {
		return
	}
	mx, my := a.currentCursor()
	if a.dev.menu {
		r := a.devMenuRect()
		drawPanel(screen, r.x, r.y, r.w, r.h)
		drawText(screen, fmt.Sprintf("Debug actions at (%d, %d)", a.dev.tx, a.dev.ty), float64(r.x)+8, float64(r.y)+3, tsLabel)
		acts := a.devActions(a.dev.tx, a.dev.ty)
		for i, b := range a.devButtons() {
			drawButton(screen, b, b.contains(mx, my))
			if i < len(acts) && acts[i].icon != "" {
				drawIcon(screen, a.icon(acts[i].icon), float64(b.x+b.w)-devRowH+2, float64(b.y), devRowH-4)
			}
		}
	}
	a.drawMapFieldLegend(screen)
	if a.dev.log {
		a.drawDebugLog(screen, sw, sh)
	}
	if a.dev.insp {
		a.drawDebugInspector(screen, sw)
	}
}

// debugLogLines is the last messages (with their tick) and every
// colonist's postponed work.
func debugLogLines(w game.WorldView, pawns []game.PawnView, n int) []string {
	var out []string
	msgs := w.Messages
	if len(msgs) > n {
		msgs = msgs[len(msgs)-n:]
	}
	for _, m := range msgs {
		out = append(out, fmt.Sprintf("t%-7d %s", m.Tick, m.Text))
	}
	for _, p := range pawns {
		for _, l := range p.Postponed {
			out = append(out, pawnLabel(p)+" put off: "+l)
		}
	}
	if len(out) == 0 {
		out = append(out, "no messages yet")
	}
	return out
}

func (a *App) drawDebugLog(screen *ebiten.Image, sw, sh int) {
	lines := debugLogLines(a.renderer.world(), a.renderer.snapshot(), 12)
	if len(lines) > 18 {
		lines = lines[:18]
	}
	var w float32
	for i, l := range lines {
		lines[i] = ellipsize(l, tsSmall, devLogMaxW)
		w = max(w, textW(lines[i], tsSmall))
	}
	pw, ph := w+20, float32(len(lines)*devStep+28)
	x, y := float32(sw)-pw-hudPad, hudBarY(sh)-ph-hudPad-120
	y = max(y, hudTopH+hudPad)
	drawPanel(screen, x, y, pw, ph)
	drawText(screen, "Debug log ("+a.bindings().Label(input.DevLog)+")", float64(x)+8, float64(y)+3, tsLabel)
	for i, l := range lines {
		drawText(screen, l, float64(x)+8, float64(y)+24+float64(i*devStep), tsSmall)
	}
}

// debugInspectLines is the raw record of the selection.
func (a *App) debugInspectLines() []string {
	if p, ok := a.selectedPawn(); ok {
		return []string{
			fmt.Sprintf("pawn #%d %q at (%d, %d)", p.ID, p.Name, p.X, p.Y),
			fmt.Sprintf("status=%s drafted=%v hostile=%v", p.Status, p.Drafted, p.Hostile),
			fmt.Sprintf("job=%q acting=%q", p.Job, p.Acting),
			fmt.Sprintf("queued=%d postponed=%d", len(p.QueuedOrders), len(p.Postponed)),
		}
	}
	if an, ok := a.selectedAnimal(); ok {
		return []string{fmt.Sprintf("animal #%d %s at (%d, %d)", an.ID, an.Species, an.X, an.Y),
			fmt.Sprintf("mode=%q health=%d tame=%v hostile=%v", an.Mode, an.Health, an.Tame, an.Hostile)}
	}
	if it, ok := a.selectedItem(); ok {
		return []string{fmt.Sprintf("item #%d %s x%d at (%d, %d)", it.ID, it.Kind, it.Qty, it.X, it.Y),
			fmt.Sprintf("forbidden=%v carried=%v fresh=%.1fd", it.Forbidden, it.Carried, it.FreshDays)}
	}
	if b, ok := a.selectedBuilding(); ok {
		return []string{fmt.Sprintf("building #%d %s (%s) at (%d, %d) %dx%d", b.ID, b.Kind, b.Stuff, b.X, b.Y, b.W, b.H),
			fmt.Sprintf("facing=%s hp=%d%% claimant=%d", b.Facing, b.HPPct, b.Claimant)}
	}
	if pl, ok := a.selectedPlant(); ok {
		return []string{fmt.Sprintf("plant #%d %s at (%d, %d) growth=%d%% stage=%d", pl.ID, pl.Species, pl.X, pl.Y, pl.Growth, pl.Stage)}
	}
	if f, ok := a.selectedFrame(); ok {
		return []string{fmt.Sprintf("frame #%d %s at (%d, %d)", f.ID, f.Kind, f.X, f.Y)}
	}
	if a.grouped() {
		return []string{fmt.Sprintf("group of %d: %v", len(a.group.ids), a.group.ids)}
	}
	return []string{"nothing selected"}
}

func (a *App) drawDebugInspector(screen *ebiten.Image, sw int) {
	lines := append([]string{fmt.Sprintf("Debug inspector  tick %d", a.renderer.game.Tick())}, a.debugInspectLines()...)
	pw, ph := maxTextW(lines, tsSmall)+20, float32(len(lines)*devStep+12)
	x := float32(sw) - pw - hudPad
	y := float32(hudTopH + 30)
	drawPanel(screen, x, y, pw, ph)
	for i, l := range lines {
		drawText(screen, l, float64(x)+8, float64(y)+4+float64(i*devStep), tsSmall)
	}
}

// Debug panels use the small type so a long log stays on one screen.
const (
	devStep    = 15  // line pitch
	devLogMaxW = 460 // widest log line, px
)

const devLosePickID, devLosePickList = "dev.lose", "dev.lose.list"

// devLoseParts are the part labels the Lose a part picker offers.
func devLoseParts() []string {
	out := make([]string, 0, body.NumParts)
	for p := body.Part(0); p < body.NumParts; p++ {
		out = append(out, p.Label())
	}
	return out
}

// openDevLosePicker lists the body parts; the pick is lost by p through d.
func (a *App) openDevLosePicker(p game.PawnView, d devTools) {
	parts := devLoseParts()
	sel := -1
	a.uiPushModal(modal{
		id: devLosePickID, title: "Lose a part", w: 300, h: 420,
		draw: func(a *App, r rect) {
			a.uiList(devLosePickList, r, len(parts), &sel, listSpec{RowH: 24, Label: func(i int) string { return parts[i] }})
		},
		buttons: []modalButton{
			{id: "dev.lose.ok", label: "Lose it", style: btnDanger, closes: true,
				disabled: func() string {
					if sel < 0 {
						return "Pick a part first."
					}
					return ""
				},
				onClick: func() {
					if sel < 0 || sel >= len(parts) {
						return
					}
					if d.DevLosePart(p.ID, parts[sel]) {
						a.toast(pawnLabel(p) + " lost the " + parts[sel])
					} else {
						a.toast("Could not remove " + parts[sel])
					}
				}},
			{id: "dev.lose.cancel", label: "Cancel", closes: true},
		},
	})
}
