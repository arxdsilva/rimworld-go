package ebiten

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/application/game"
)

// Alert list (UI checklist §alerts): a stack on the right edge under the
// top bar, rebuilt from the read model each frame. Danger alerts are
// red; warnings amber. Clicking an alert centres the camera on its next
// target, cycling through them.

type alert struct {
	key     string // stable id for click cycling
	label   string
	danger  bool
	targets [][2]int
}

const (
	alertW   = 260
	alertH   = 24
	alertGap = 4
	// lowFoodPerColonist: fewer ground food items per colonist than
	// this raises "Low food".
	lowFoodPerColonist = 4
)

var warnClr = color.RGBA{R: 0xE0, G: 0xA8, B: 0x40, A: 0xFF}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return strconv.Itoa(n) + " " + many
}

// buildAlerts derives the current alerts from pawns and the world.
func buildAlerts(pawns []game.PawnView, w game.WorldView) []alert {
	var out []alert
	group := func(key string, danger bool, who []game.PawnView, one func(string) string, many string) {
		if len(who) == 0 {
			return
		}
		a := alert{key: key, danger: danger}
		if len(who) == 1 {
			a.label = one(who[0].Name)
		} else {
			a.label = fmt.Sprintf("%d %s", len(who), many)
		}
		for _, p := range who {
			a.targets = append(a.targets, [2]int{p.X, p.Y})
		}
		out = append(out, a)
	}

	var hostiles [][2]int
	species := ""
	for _, an := range w.Animals {
		if an.Hostile {
			hostiles = append(hostiles, [2]int{an.X, an.Y})
			species = an.Species
		}
	}
	if len(hostiles) > 0 {
		label := "A " + species + " is attacking!"
		if len(hostiles) > 1 {
			label = fmt.Sprintf("%d animals are attacking!", len(hostiles))
		}
		out = append(out, alert{key: "hostile", label: label, danger: true, targets: hostiles})
	}

	var rescue, starving, collapsed, unhappy []game.PawnView
	alive := 0
	for _, p := range pawns {
		switch {
		case p.Status == "dead":
			continue
		case p.Status == "downed" && p.Reason == "injured" && !p.InBed && p.CarriedBy == 0:
			rescue = append(rescue, p)
		case p.Status == "downed" && p.Reason == "no food":
			starving = append(starving, p)
		case p.Status == "downed" && p.Reason == "no bed":
			collapsed = append(collapsed, p)
		}
		alive++
		if p.Status == "alive" && p.Mood < 30 {
			unhappy = append(unhappy, p)
		}
	}
	group("rescue", true, rescue, func(n string) string { return n + " needs rescue" }, "colonists need rescue")
	group("starving", true, starving, func(n string) string { return n + " is starving" }, "colonists are starving")
	group("collapsed", false, collapsed, func(n string) string { return n + " collapsed from exhaustion" }, "colonists collapsed")
	if alive > 0 && w.Food < lowFoodPerColonist*alive {
		out = append(out, alert{key: "food", label: fmt.Sprintf("Low food (%d left)", w.Food)})
	}
	group("unhappy", false, unhappy, func(n string) string { return n + " is unhappy" }, "colonists are unhappy")
	return out
}

func (a *App) currentAlerts() []alert {
	if a.renderer == nil {
		return nil
	}
	return buildAlerts(a.renderer.snapshot(), a.renderer.world())
}

func alertRect(sw, i int) button {
	return button{x: float32(sw - alertW - hudPad), y: float32(hudTopH + hudPad + i*(alertH+alertGap)), w: alertW, h: alertH}
}

func (a *App) alertButtons(sw int) []button {
	if a.workOpen {
		return nil
	}
	var out []button
	for i, al := range a.currentAlerts() {
		r := alertRect(sw, i)
		r.id, r.label = "alert."+al.key, al.label
		out = append(out, r)
	}
	return out
}

func (a *App) drawAlerts(screen *ebiten.Image, sw int) {
	if a.workOpen {
		return
	}
	for i, al := range a.currentAlerts() {
		r := alertRect(sw, i)
		edge := warnClr
		bg := color.RGBA{48, 36, 22, 230}
		if al.danger {
			edge, bg = alertClr, color.RGBA{60, 18, 14, 235}
		}
		vector.FillRect(screen, r.x, r.y, r.w, r.h, bg, false)
		vector.FillRect(screen, r.x, r.y, 4, r.h, edge, false)
		vector.StrokeRect(screen, r.x, r.y, r.w, r.h, 1, edge, false)
		drawLabel(screen, int(r.x)+12, int(r.y)+6, al.label)
	}
}

// focusAlert centres the camera on the alert's next target.
func (a *App) focusAlert(id string) {
	key := strings.TrimPrefix(id, "alert.")
	for _, al := range a.currentAlerts() {
		if al.key != key || len(al.targets) == 0 {
			continue
		}
		if a.alertCycle == nil {
			a.alertCycle = map[string]int{}
		}
		t := al.targets[a.alertCycle[key]%len(al.targets)]
		a.alertCycle[key]++
		a.renderer.centerOn(float64(t[0])+0.5, float64(t[1])+0.5, a.sw, a.sh)
		return
	}
}

// centerOn moves the camera so world point (wx, wy) is mid-screen.
func (r *Renderer) centerOn(wx, wy float64, sw, sh int) {
	tile := r.tilePx()
	r.cam.offsetX = wx*tile - float64(sw)/2
	r.cam.offsetY = wy*tile - float64(sh)/2
}
