package devtools

import (
	"fmt"
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/platform/input"
)

// Developer mode (UI-016; Options › Gameplay): the Dev input context with
// ultrafast speed, a single-tick step while paused, a debug actions menu,
// an on-screen debug log and a debug inspector. The menu calls the
// game's developer hooks through Tools, a consumer-owned interface used
// only while developer mode is on; nothing else in the client touches the
// game outside commands.

// The speed tiers developer mode switches between (ports.Speed).
const (
	speedPaused = 0
	speedNormal = 1
	speedUltra  = 4
)

// needsFull is a colonist's full hunger and rest (domain needs.Full).
const needsFull = 10000

const devRowH = 20

// Mode reports developer mode is on.
func (p *Panel) Mode() bool { return p.sh.Env().On }

// active is developer mode with a colony running.
func (p *Panel) active() bool {
	e := p.sh.Env()
	return e.On && e.Game != nil
}

// pawnLabel is a colonist's name ("Colonist N" for an unnamed one).
func pawnLabel(p game.PawnView) string {
	if p.Name != "" {
		return p.Name
	}
	return fmt.Sprintf("Colonist %d", p.ID)
}

// Actions is the debug menu for the tile (tx, ty).
func (p *Panel) Actions(tx, ty int) []Action {
	spawn := func(species string) Action {
		return Action{Label: "Spawn " + strings.ReplaceAll(species, "_", " ") + " here", Run: func(d Tools) string {
			if _, err := d.SpawnAnimal(species, tx, ty); err != nil {
				return err.Error()
			}
			return "Spawned " + species
		}}
	}
	fire := func(key, name string) Action {
		return Action{Label: "Event: " + name, Run: func(d Tools) string {
			if d.FireIncident(key) {
				return name + " fired"
			}
			return name + " does not fit now"
		}}
	}
	out := []Action{
		spawn("deer"), spawn("timber_wolf"),
		{Label: "Add 100 wood here", Run: func(d Tools) string {
			if _, err := d.SpawnStack(item.Wood, 100, tx, ty); err != nil {
				return err.Error()
			}
			return "Added 100 wood"
		}},
		fire("herd_crossing", "herd crossing"), fire("farm_animals", "farm animals"), fire("mad_animal", "mad animal"),
		{Label: "Raid (3 raiders)", Run: func(d Tools) string { d.Raid(3); return "Raid sent" }},
	}
	sel := p.sh.Selected()
	kill := Action{Label: "Kill selected animal", Reason: "select an animal first"}
	if an := sel.Animal; an != nil {
		kill.Reason, kill.Run = "", func(d Tools) string { d.KillAnimal(an.ID); return "Killed " + an.Label }
	}
	feed := Action{Label: "Fill selected colonist's needs", Reason: "select a colonist first"}
	if pw := sel.Pawn; pw != nil {
		feed.Reason, feed.Run = "", func(d Tools) string {
			d.SetNeeds(pw.ID, needsFull, needsFull)
			return pawnLabel(*pw) + " fed and rested"
		}
	}
	lose := Action{Label: "Lose a part...", Reason: "select a colonist first"}
	spawnImp := Action{Label: "Spawn implants", Reason: "select a colonist first"}
	if pw := sel.Pawn; pw != nil {
		lose.Reason, lose.Run = "", func(d Tools) string { p.openLosePicker(*pw, d); return "" }
		spawnImp.Reason, spawnImp.Run = "", func(d Tools) string {
			if d.DevSpawnImplants(pw.ID) {
				return "Implants spawned near " + pawnLabel(*pw)
			}
			return "No room for implants"
		}
	}
	start := Action{Label: "Start fire here", Icon: "start_fire", Run: func(Tools) string {
		p.sh.Enqueue(command.Command{Kind: command.DevStartFire, X: tx, Y: ty})
		return "Fire started"
	}}
	return append(out, kill, feed, lose, spawnImp, start, p.mapFieldsAction())
}

// HandleKeys runs the Dev context's actions.
func (p *Panel) HandleKeys(f Keys) {
	if !p.Mode() {
		return
	}
	st := p.State
	switch {
	case f.Fired(input.DevUltrafast):
		if p.sh.SpeedTier() == speedUltra {
			p.sh.SetSpeed(speedNormal)
		} else {
			p.sh.SetSpeed(speedUltra)
		}
	case f.Fired(input.DevStep):
		if p.sh.SpeedTier() == speedPaused {
			p.sh.StepTick()
		} else {
			p.sh.Toast("Pause first to step one tick")
		}
	case f.Fired(input.DevActions):
		mx, my := p.sh.Cursor()                 // logical: where the menu panel is drawn
		tx, ty := p.sh.TileAt(p.sh.MapCursor()) // physical: the map tile under the cursor
		st.Menu = !st.Menu
		st.MX, st.MY, st.TX, st.TY = mx, my, tx, ty
	case f.Fired(input.DevLog):
		st.Log = !st.Log
	case f.Fired(input.DevInspector):
		st.Insp = !st.Insp
	}
}

// MenuRect is the debug menu's rectangle.
func (p *Panel) MenuRect() uikit.Button {
	st := p.State
	n := len(p.Actions(st.TX, st.TY))
	sw, sh := p.sh.Size()
	w, h := float32(260), float32(24+n*devRowH)
	x, y := float32(st.MX+8), float32(st.MY)
	x = min(x, float32(sw)-w-4)
	y = max(hudlayout.HudMargin, min(y, float32(sh)-hudlayout.TabsH-h-4)) // clear of the tab strip
	return uikit.Button{X: x, Y: y, W: w, H: h}
}

// Buttons are the debug menu's entries ("dev.act.N").
func (p *Panel) Buttons() []uikit.Button {
	st := p.State
	if !st.Menu || !p.active() {
		return nil
	}
	r := p.MenuRect()
	var out []uikit.Button
	for i, act := range p.Actions(st.TX, st.TY) {
		out = append(out, uikit.Button{ID: "dev.act." + strconv.Itoa(i), Label: act.Label, X: r.X + 4, Y: r.Y + 22 + float32(i*devRowH),
			W: r.W - 8, H: devRowH - 2, Disabled: act.Reason != "", Reason: act.Reason, Quiet: true})
	}
	return out
}

// Click runs a debug action through Tools.
func (p *Panel) Click(id string) {
	st := p.State
	i, err := strconv.Atoi(strings.TrimPrefix(id, "dev.act."))
	acts := p.Actions(st.TX, st.TY)
	st.Menu = false
	g := p.sh.Env().Game
	if err != nil || i < 0 || i >= len(acts) || acts[i].Run == nil || g == nil {
		return
	}
	d, ok := g.(Tools)
	if !ok {
		p.sh.Toast("Debug actions need the full game")
		return
	}
	if msg := acts[i].Run(d); msg != "" {
		p.sh.Toast(msg)
	}
}

// Draw draws the debug menu, the map-fields legend, the log and the
// inspector. The three panels stack in the dev column (Stack).
func (p *Panel) Draw(screen *ebiten.Image, sw, sh int) {
	if !p.active() {
		return
	}
	mx, my := p.sh.Cursor()
	st := p.State
	if st.Menu {
		r := p.MenuRect()
		uikit.DrawPanel(screen, r.X, r.Y, r.W, r.H)
		gfx.Text(screen, fmt.Sprintf("Debug actions at (%d, %d)", st.TX, st.TY), float64(r.X)+8, float64(r.Y)+3, gfx.Label)
		acts := p.Actions(st.TX, st.TY)
		for i, b := range p.Buttons() {
			uikit.DrawButton(screen, b, b.Contains(mx, my))
			if i < len(acts) && acts[i].Icon != "" {
				gfx.Icon(screen, p.sh.Icon(acts[i].Icon), float64(b.X+b.W)-devRowH+2, float64(b.Y), devRowH-4)
			}
		}
	}
	stack := p.Stack()
	p.DrawMapFieldLegend(screen)
	if !stack.Log.Empty() {
		p.drawLog(screen, stack.Log)
	}
	if !stack.Insp.Empty() {
		p.drawInspector(screen, stack.Insp)
	}
}
