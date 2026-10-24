package ebiten

import (
	"fmt"
	"image/color"
	"math"
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
	// exposureAlert: hypothermia/heatstroke (permille) worth an alert.
	exposureAlert = 200
)

var warnClr = color.RGBA{R: 0xE0, G: 0xA8, B: 0x40, A: 0xFF}

// buildAlerts derives the current alerts from pawns and the world.
func buildAlerts(pawns []game.PawnView, w game.WorldView) []alert {
	var out []alert
	// raiders are listed with the colonists; split them off first
	var raiders [][2]int
	plan := ""
	colonists := pawns[:0:0]
	for _, p := range pawns {
		switch {
		case p.Prisoner:
		case !p.Hostile:
			colonists = append(colonists, p)
		case !p.Lying && !p.Fleeing:
			raiders = append(raiders, [2]int{p.X, p.Y})
			plan = p.RaidPlan
		}
	}
	pawns = colonists
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

	out = append(out, animalAlerts(w)...)

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

	if len(raiders) > 0 {
		label := "Raid: 1 raider"
		if len(raiders) > 1 {
			label = fmt.Sprintf("Raid: %d raiders", len(raiders))
		}
		switch plan {
		case "siege":
			label = fmt.Sprintf("Siege: %d raiders", len(raiders))
		case "sappers":
			label = fmt.Sprintf("Sappers: %d raiders", len(raiders))
		}
		out = append(out, alert{key: "raid", label: label, danger: true, targets: raiders})
	}
	var breaking []game.PawnView
	for _, p := range pawns {
		if p.MentalBreak != "" && p.Status != "dead" {
			breaking = append(breaking, p)
		}
	}
	if len(breaking) == 1 {
		p := breaking[0]
		out = append(out, alert{key: "break", danger: true, label: p.Name + ": mental break (" + p.MentalBreak + ")", targets: [][2]int{{p.X, p.Y}}})
	} else if len(breaking) > 1 {
		group("break", true, breaking, func(n string) string { return n }, "colonists are having mental breaks")
	}
	var hypo, heat []game.PawnView
	for _, p := range pawns {
		if p.Status == "dead" {
			continue
		}
		if p.Hypothermia >= exposureAlert {
			hypo = append(hypo, p)
		}
		if p.Heatstroke >= exposureAlert {
			heat = append(heat, p)
		}
	}
	group("hypothermia", true, hypo, func(n string) string { return n + " has hypothermia" }, "colonists have hypothermia")
	group("heatstroke", true, heat, func(n string) string { return n + " has heatstroke" }, "colonists have heatstroke")
	var cold [][2]int
	for _, b := range w.Buildings {
		if b.Fuel && b.FuelPct == 0 {
			cold = append(cold, [2]int{b.X, b.Y})
		}
	}
	if len(cold) > 0 {
		out = append(out, alert{key: "nofuel", label: "Campfire out of wood", targets: cold})
	}
	if al, ok := spoilingAlert(w); ok {
		out = append(out, al)
	}
	if al, ok := waitingFramesAlert(w); ok {
		out = append(out, al)
	}
	group("unhappy", false, unhappy, func(n string) string { return n + " is unhappy" }, "colonists are unhappy")
	return out
}

func (a *App) currentAlerts() []alert {
	if a.renderer == nil {
		return nil
	}
	// Several HUD passes ask every frame (chips, alert rows, hit layout):
	// build once per update while the read models are the same ones.
	r := a.renderer
	snap, w := r.game.Snapshot(), r.world()
	k := alertsKey{
		renderer: r, present: r.present, pawns: litFirst(snap), nPawns: len(snap),
		animals: litFirst(w.Animals), nAnimals: len(w.Animals), items: litFirst(w.Items), nItems: len(w.Items),
		bldgs: litFirst(w.Buildings), nBldgs: len(w.Buildings), frames: litFirst(w.Frames), nFrames: len(w.Frames),
		food: w.Food,
	}
	if m := &a.alerts; r.viewCached && m.ok && m.key == k {
		return m.out
	}
	out := buildAlerts(append(r.snapshot(), r.hostiles()...), w)
	a.alerts = alertsMemo{key: k, out: out, ok: true}
	return out
}

// alertsMemo is currentAlerts' last result and the read models it came
// from (identity of their lists, the update counter). Callers only read it.
type alertsMemo struct {
	key alertsKey
	out []alert
	ok  bool
}

type alertsKey struct {
	renderer                         *Renderer
	present                          uint64
	pawns                            *game.PawnView
	animals                          *game.AnimalView
	items                            *game.ItemView
	bldgs                            *game.BuildingView
	frames                           *game.FrameView
	nPawns, nAnimals, nItems, nBldgs int
	nFrames, food                    int
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
			pulse := 0.55 + 0.45*math.Sin(float64(a.renderer.present)/9)
			edge, bg = fade(alertClr, pulse), color.RGBA{60, 18, 14, 235}
		}
		vector.FillRect(screen, r.x, r.y, r.w, r.h, bg, false)
		vector.FillRect(screen, r.x, r.y, 4, r.h, edge, false)
		vector.StrokeRect(screen, r.x, r.y, r.w, r.h, 1, edge, false)
		drawFitSmall(screen, al.label, float64(r.x)+12, float64(r.y)+(float64(r.h)-lineH(tsBody))/2, float64(r.w)-18, tsBody)
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
	r.cam.placed = true // an explicit jump; don't recentre on the map
	r.clampCamera(colonyViewport(sw, sh))
}

// hungryAlertBelow is the food percent under which a tame animal raises
// "Animals hungry".
const hungryAlertBelow = 25

// penFullDiv: a pen holding more animals than its cells / this is full.
const penFullDiv = 4

// animalAlerts are the animal ecology alerts, derived from the read
// model: hungry tame animals, wild animals eating crops, timbergnaws
// eating trees and full pens.
func animalAlerts(w game.WorldView) []alert {
	var out []alert
	var hungry, raiders, gnaws [][2]int
	growCells := map[[2]int]bool{}
	for _, z := range w.Zones {
		if z.Kind == game.ZoneGrow {
			for _, c := range z.Cells {
				growCells[c] = true
			}
		}
	}
	for _, an := range w.Animals {
		pos := [2]int{an.X, an.Y}
		switch {
		case an.Tame && animalHasNeeds(an) && an.Food < hungryAlertBelow:
			hungry = append(hungry, pos)
		case !an.Tame && !an.Leaving && an.Species == "timbergnaw":
			gnaws = append(gnaws, pos)
		case !an.Tame && an.State == "feeding" && growCells[pos]:
			raiders = append(raiders, pos)
		}
	}
	if len(hungry) > 0 {
		label := "Animals hungry"
		if len(hungry) > 1 {
			label = fmt.Sprintf("Animals hungry (%d)", len(hungry))
		}
		out = append(out, alert{key: "animalhungry", label: label, targets: hungry})
	}
	if len(raiders) > 0 {
		out = append(out, alert{key: "cropraid", label: "Wild animals eating crops", danger: true, targets: raiders})
	}
	if len(gnaws) > 0 {
		label := "Timbergnaws eating trees"
		if w.TreesFelled > 0 {
			label = fmt.Sprintf("Timbergnaws eating trees (%d felled)", w.TreesFelled)
		}
		out = append(out, alert{key: "timbergnaw", label: label, targets: gnaws})
	}
	for _, z := range w.Zones {
		if z.Kind != game.ZonePen || len(z.Cells) == 0 {
			continue
		}
		in := map[[2]int]bool{}
		for _, c := range z.Cells {
			in[c] = true
		}
		var pos [][2]int
		for _, an := range w.Animals {
			if an.Tame && in[[2]int{an.X, an.Y}] {
				pos = append(pos, [2]int{an.X, an.Y})
			}
		}
		if len(pos) > len(z.Cells)/penFullDiv {
			out = append(out, alert{key: fmt.Sprintf("penfull.%d", z.ID),
				label:   fmt.Sprintf("Pen full: %d animals in a %d-tile pen", len(pos), len(z.Cells)),
				targets: pos})
		}
	}
	return out
}

// waitingFramesAlert flags blueprints whose material is not in stock
// anywhere (the frame status starts with "No "): clicking cycles them.
func waitingFramesAlert(w game.WorldView) (alert, bool) {
	var at [][2]int
	for _, f := range w.Frames {
		if strings.HasPrefix(f.Status, "No ") {
			at = append(at, [2]int{f.X, f.Y})
		}
	}
	if len(at) == 0 {
		return alert{}, false
	}
	return alert{key: "waitingframes", label: fmt.Sprintf("Blueprints waiting for materials (%d)", len(at)), targets: at}, true
}
