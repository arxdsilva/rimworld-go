package ebiten

import (
	"cmp"
	"fmt"
	"math"
	"math/bits"
	"slices"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// Alert list (PLAY-018, MAIN_UI §32): a stack in the right column, in the
// layout's Alerts rect (hud_layout.go) from below the Learning Helper down
// to the letters, rebuilt from the read model each frame. Dangers come
// first (their backing pulses), then warnings, each group by key so the
// order never reshuffles while the alerts persist. Hovering a row shows
// what is wrong and what to do in a card to its left; a click centres the
// camera on the alert's next target, a right-click on the previous one.

// alertSev is how urgent an alert is; the list is ordered by it.
type alertSev uint8

const (
	sevDanger alertSev = iota
	sevWarning
)

type alert struct {
	key     string // stable id for click cycling
	label   string
	danger  bool
	sev     alertSev // sevDanger for danger alerts (set by buildAlerts)
	explain string   // what is wrong and what to do (set by buildAlerts)
	targets [][2]int
}

const (
	// lowFoodPerColonist: fewer ground food items per colonist than
	// this raises "Low food".
	lowFoodPerColonist = 4
	// exposureAlert: hypothermia/heatstroke (permille) worth an alert.
	exposureAlert = 200
)

// buildAlerts derives the current alerts from pawns and the world, most
// urgent first: dangers before warnings, then by key.
func buildAlerts(pawns []game.PawnView, w game.WorldView) []alert {
	out := raiseAlerts(pawns, w)
	for i := range out {
		al := &out[i]
		al.sev = sevWarning
		if al.danger {
			al.sev = sevDanger
		}
		al.explain = alertExplain(al.key)
	}
	slices.SortStableFunc(out, func(a, b alert) int {
		if c := cmp.Compare(a.sev, b.sev); c != 0 {
			return c
		}
		return strings.Compare(a.key, b.key)
	})
	return out
}

// raiseAlerts collects the alerts in the order the checks run.
func raiseAlerts(pawns []game.PawnView, w game.WorldView) []alert {
	var out []alert
	allPawns := pawns // the chased colonist is named from the whole list
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

	if al, ok := hostileAlert(allPawns, w); ok {
		out = append(out, al)
	}

	if al, ok := fireAlert(w); ok {
		out = append(out, al)
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

// hostileAlert is the "hostile" alert for animals attacking colonists: one
// animal says what it is and, when it is chasing someone, whom ("A timber
// wolf is chasing Ada!"); several are only counted. pawns is every pawn
// the target may be among.
func hostileAlert(pawns []game.PawnView, w game.WorldView) (alert, bool) {
	var targets [][2]int
	var first game.AnimalView
	for _, an := range w.Animals {
		if !an.Hostile {
			continue
		}
		if len(targets) == 0 {
			first = an
		}
		targets = append(targets, [2]int{an.X, an.Y})
	}
	if len(targets) == 0 {
		return alert{}, false
	}
	label := withArticle(animalLabel(first)) + " is attacking!"
	if first.TargetID != 0 {
		if p, ok := chasedPawn(pawns, first.TargetID); ok && p.Name != "" {
			label = withArticle(animalLabel(first)) + " is chasing " + p.Name + "!"
		}
	}
	if len(targets) > 1 {
		label = fmt.Sprintf("%d animals are attacking!", len(targets))
	}
	return alert{key: "hostile", label: label, danger: true, targets: targets}, true
}

// withArticle puts "A" or "An" before a noun phrase ("An arctic fox").
func withArticle(noun string) string {
	if noun != "" && strings.ContainsRune("aeiouAEIOU", rune(noun[0])) {
		return "An " + noun
	}
	return "A " + noun
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

// alertRow is one row of the stack: an alert, or the "+N more" row that
// stands for the alerts that did not fit.
type alertRow struct {
	r      rect
	al     alert   // zero for the "+N more" row
	more   int     // > 0: the "+N more" row
	hidden []alert // what the "+N more" row stands for
}

// alertRowLabel is the text a row shows.
func alertRowLabel(r alertRow) string {
	if r.more > 0 {
		return fmt.Sprintf("+%d more", r.more)
	}
	return r.al.label
}

// stackSplit says how many of total entries a stack of rows rows shows
// and how many it folds into its "+N" row. limit caps the entries shown
// before the "+N" row appears at all (the letter stack's eight). With one
// row there is no room for the count, so it shows an entry.
func stackSplit(total, rows, limit int) (shown, folded int) {
	switch {
	case total <= 0 || rows <= 0:
		return 0, max(total, 0)
	case total <= limit && total <= rows:
		return total, 0
	case rows == 1:
		return 1, 0
	}
	shown = min(rows-1, limit)
	return shown, total - shown
}

// alertRows lays the alert stack out in l's Alerts rect: the most urgent
// alerts first, the last row counting the rest when they do not fit.
func (a *App) alertRows(l hudLayout) []alertRow {
	als := a.currentAlerts()
	if len(als) == 0 || l.AlertRows <= 0 || l.Alerts.empty() {
		return nil
	}
	shown, more := stackSplit(len(als), l.AlertRows, len(als))
	rows := make([]alertRow, 0, shown+1)
	at := func(i int) rect {
		return rect{l.Alerts.X, l.Alerts.Y + float32(i)*(alertRowH+hudGap), l.Alerts.W, alertRowH}
	}
	for i := 0; i < shown; i++ {
		rows = append(rows, alertRow{r: at(i), al: als[i]})
	}
	if more > 0 {
		rows = append(rows, alertRow{r: at(shown), more: more, hidden: als[shown:]})
	}
	return rows
}

// alertButtons are the alert rows as click targets, ids "alert.<key>".
func (a *App) alertButtons(sw int) []button { return a.alertButtonsIn(a.hud()) }

func (a *App) alertButtonsIn(l hudLayout) []button {
	var out []button
	for _, row := range a.alertRows(l) {
		if row.more > 0 {
			continue // the count is not a place to jump to
		}
		out = append(out, button{id: "alert." + row.al.key, label: row.al.label, x: row.r.X, y: row.r.Y, w: row.r.W, h: row.r.H})
	}
	return out
}

// alertPulse is the danger backing's brightness (0.55..1) at presentation
// frame present; reduced motion holds it steady.
func alertPulse(present uint64, reduced bool) float64 {
	if reduced {
		return 1
	}
	return 0.775 + 0.225*math.Sin(float64(present)/9)
}

func (a *App) drawAlerts(screen *ebiten.Image, sw int) { a.drawAlertsIn(screen, a.hud()) }

func (a *App) drawAlertsIn(screen *ebiten.Image, l hudLayout) {
	rows := a.alertRows(l)
	if len(rows) == 0 {
		return
	}
	pulse := alertPulse(a.renderer.present, a.renderer.reducedMotion)
	mx, my := a.currentCursor()
	for _, row := range rows {
		r := row.r
		edge, bg, text := uiWarning, uiAlertWarnBg, uiText
		switch {
		case row.more > 0:
			edge, bg, text = uiPanelBorder, uiPanel, uiTextDim
		case row.al.sev == sevDanger:
			edge, bg = uiDanger, fade(uiAlertDangerBg, pulse)
		}
		fillRect(screen, r, bg)
		if row.more == 0 {
			fillRect(screen, rect{r.X, r.Y, 4, r.H}, edge)
		}
		border := edge
		if row.more > 0 || !r.hit(mx, my) {
			border = fade(edge, 0.5)
		}
		strokeRect(screen, r, border)
		drawText(screen, fitText(alertRowLabel(row), tsBody, r.W-20), float64(r.X)+12, float64(r.Y)+(float64(r.H)-lineH(tsBody))/2, withColor(tsBody, text))
	}
}

// focusAlert centres the camera on the alert's next target.
func (a *App) focusAlert(id string) { a.cycleAlert(strings.TrimPrefix(id, "alert."), 1) }

// cycleAlert centres the camera on the alert's next target (dir 1) or
// previous one (dir -1), wrapping round. alertCycle holds one more than
// the index of the target on show (0: none yet), so a fresh alert going
// back starts at its last.
func (a *App) cycleAlert(key string, dir int) {
	for _, al := range a.currentAlerts() {
		if al.key != key || len(al.targets) == 0 {
			continue
		}
		if a.alertCycle == nil {
			a.alertCycle = map[string]int{}
		}
		n, v := len(al.targets), a.alertCycle[key]
		idx := v % n
		if dir < 0 {
			idx = n - 1
			if v > 0 {
				idx = ((v-2)%n + n) % n
			}
		}
		a.alertCycle[key] = idx + 1
		t := al.targets[idx]
		a.centerOnMap(float64(t[0])+0.5, float64(t[1])+0.5)
		return
	}
}

// centerOn moves the camera so world point (wx, wy) is mid-screen.
// sw x sh is the screen in the map's pixels (captures run at scale 1).
func (r *Renderer) centerOn(wx, wy float64, sw, sh int) {
	r.centerOnView(wx, wy, viewport{0, 0, float64(sw), float64(sh)}, colonyViewport(sw, sh))
}

// centerOnView centres the camera on a world point in the middle of the
// physical screen `screen`, clamped to the physical map viewport v.
func (r *Renderer) centerOnView(wx, wy float64, screen, v viewport) {
	tile := r.tilePx()
	r.cam.offsetX = wx*tile - (screen.x0+screen.x1)/2
	r.cam.offsetY = wy*tile - (screen.y0+screen.y1)/2
	r.cam.placed = true // an explicit jump; don't recentre on the map
	r.clampCamera(v)
}

// centerOnMap is the UI's "jump to": the target goes to the middle of the
// physical map viewport.
func (a *App) centerOnMap(wx, wy float64) {
	v := a.mapViewport()
	a.renderer.centerOnView(wx, wy, v, v)
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
	out = append(out, penAlerts(w)...)
	return out
}

// fireAlert is "Fire in the colony!" (PLAY-015) while any fire burns
// inside Home; clicking cycles the burning Home cells.
func fireAlert(w game.WorldView) (alert, bool) {
	var at [][2]int
	for _, f := range w.Fires {
		if i := f.Y*w.TerrainW + f.X; f.X >= 0 && f.X < w.TerrainW && i >= 0 && i < len(w.Home) && w.Home[i] != 0 {
			at = append(at, [2]int{f.X, f.Y})
		}
	}
	if len(at) == 0 {
		return alert{}, false
	}
	return alert{key: "fire", label: "Fire in the colony!", danger: true, targets: at}, true
}

// penAlerts are the pen alerts (PLAY-015): each open pen ("Pen 2 is
// open"), each pen holding more tame animals than its cells / penFullDiv,
// and the livestock with no pen ("Unpenned livestock: N").
func penAlerts(w game.WorldView) []alert {
	var out []alert
	inBits := func(b []uint64, x, y int) bool {
		i := y*w.TerrainW + x
		return w.TerrainW > 0 && x >= 0 && x < w.TerrainW && i >= 0 && i>>6 < len(b) && b[i>>6]&(1<<(uint(i)&63)) != 0
	}
	for _, p := range w.Pens {
		if !p.Enclosed {
			out = append(out, alert{key: fmt.Sprintf("penopen.%d", p.ID), label: p.Name + " is open",
				danger: true, targets: [][2]int{{p.X, p.Y}}})
			continue
		}
		cells := 0
		for _, word := range p.Bits {
			cells += bits.OnesCount64(word)
		}
		var pos [][2]int
		for _, an := range w.Animals {
			if an.Tame && inBits(p.Bits, an.X, an.Y) {
				pos = append(pos, [2]int{an.X, an.Y})
			}
		}
		if len(pos) > cells/penFullDiv {
			out = append(out, alert{key: fmt.Sprintf("penfull.%d", p.ID),
				label:   fmt.Sprintf("%s full: %d animals in %d tiles", p.Name, len(pos), cells),
				targets: pos})
		}
	}
	var roaming [][2]int
	for _, an := range w.Animals {
		if an.Penned && an.Pen == "Roaming" {
			roaming = append(roaming, [2]int{an.X, an.Y})
		}
	}
	if len(roaming) > 0 {
		out = append(out, alert{key: "unpenned", label: fmt.Sprintf("Unpenned livestock: %d", len(roaming)), targets: roaming})
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

// alertExplanations say, for each alert key, what is wrong and what to do
// (the card shown while the alert is hovered). Keys with a numeric suffix
// ("penopen.3") share the text of their prefix.
var alertExplanations = map[string]string{
	"hostile":       "Wild animals are attacking colonists. Draft your fighters and keep everyone else indoors until the threat is dead or gone.",
	"fire":          "Something inside the colony is burning, and fire spreads along wood and dry grass. Send colonists to put it out and clear what it can reach.",
	"animalhungry":  "Tame animals are running low on food. Put something they eat within reach, or build a feeding trough in their pen.",
	"cropraid":      "Wild animals are eating a field. Drive them off or fence the crops in before the harvest is lost.",
	"timbergnaw":    "Timbergnaws are gnawing trees down. Hunt them, or keep them away from the wood you still need.",
	"penopen":       "A pen has a gap in its fence, so the animals inside can wander out. Close the gap or add a gate.",
	"penfull":       "A pen holds more animals than it comfortably fits. Make it bigger or build a second pen.",
	"unpenned":      "Tame livestock are roaming with no pen. Give them one so they stay where you can feed and tend them.",
	"rescue":        "A wounded colonist is lying where they fell. Send a healthy colonist to carry them to a bed so they can heal.",
	"starving":      "A colonist has collapsed from hunger. Get food within reach and carry them to a bed to recover.",
	"collapsed":     "A colonist dropped from exhaustion with nowhere to sleep. Build enough beds so everyone can rest.",
	"food":          "There is little food for the number of colonists. Plant and harvest crops, hunt, or stop food going to waste before anyone starves.",
	"raid":          "Armed raiders are on the map. Draft your fighters, take cover near your defences and hold a line.",
	"break":         "A colonist is having a mental break and may act on their own. Keep others out of the way, then work on lifting their mood.",
	"hypothermia":   "Colonists are freezing. Bring them indoors beside a fire and give them warm clothes.",
	"heatstroke":    "Colonists are overheating. Move them into shade or a cool room and keep them off heavy work.",
	"nofuel":        "A fire has burned out of wood. Add wood to keep the room warm and lit.",
	"spoiling":      "Stored food is about to rot. Eat it, or move it somewhere cool, before it is lost.",
	"waitingframes": "Some blueprints need materials that are not in stock. Gather or make what they need, or cancel the blueprints.",
	"unhappy":       "Colonists are in a poor mood and close to breaking. Look at what drags it down: hunger, rest, comfort and room to breathe.",
}

// alertExplain is the explanation for an alert key.
func alertExplain(key string) string {
	if e, ok := alertExplanations[strings.SplitN(key, ".", 2)[0]]; ok {
		return e
	}
	return "Select this alert to look at the cause."
}
