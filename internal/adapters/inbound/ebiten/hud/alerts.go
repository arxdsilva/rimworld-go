package hud

import (
	"cmp"
	"fmt"
	"math"
	"math/bits"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/render/creature"
	"rimworld-go/internal/adapters/inbound/ebiten/render/light"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"slices"
	"strings"

	"rimworld-go/internal/application/game"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// Alert list (PLAY-018, MAIN_UI §32): a stack in the right column, in the
// layout's Alerts rect (hudlayout) from below the Learning Helper down
// to the letters, rebuilt from the read model each frame. Dangers come
// first (their backing pulses), then warnings, each group by key so the
// order never reshuffles while the alerts persist. Hovering a row shows
// what is wrong and what to do in a card to its left; a click centres the
// camera on the alert's next target, a right-click on the previous one.

// AlertSev is how urgent an alert is; the list is ordered by it.
type AlertSev uint8

const (
	SevDanger AlertSev = iota
	SevWarning
)

type Alert struct {
	Key     string // stable id for click cycling
	Label   string
	Danger  bool
	Sev     AlertSev // SevDanger for danger alerts (set by BuildAlerts)
	Explain string   // what is wrong and what to do (set by BuildAlerts)
	Targets [][2]int
}

const (
	// lowFoodPerColonist: fewer ground food items per colonist than
	// this raises "Low food".
	lowFoodPerColonist = 4
	// exposureAlert: hypothermia/heatstroke (permille) worth an alert.
	exposureAlert = 200
	// bleedAlertHours: a colonist this close to bleeding out raises the
	// "bleeding" alert (MED-01).
	bleedAlertHours = 12
	// untendedAlertHours: a colonist's open wound left untended this long
	// raises the "untended" alert (MED-02).
	untendedAlertHours = 4
	// overdoseExtremeStage: an overdose at this stage (PawnView.
	// OverdoseStage, 750 severity) makes the "overdose" alert a danger
	// (MED-07).
	overdoseExtremeStage = 2
)

// BuildAlerts derives the current alerts from pawns and the world, most
// urgent first: dangers before warnings, then by key.
func BuildAlerts(pawns []game.PawnView, w game.WorldView) []Alert {
	out := raiseAlerts(pawns, w)
	for i := range out {
		al := &out[i]
		al.Sev = SevWarning
		if al.Danger {
			al.Sev = SevDanger
		}
		al.Explain = AlertExplain(al.Key)
	}
	slices.SortStableFunc(out, func(a, b Alert) int {
		if c := cmp.Compare(a.Sev, b.Sev); c != 0 {
			return c
		}
		return strings.Compare(a.Key, b.Key)
	})
	return out
}

// raiseAlerts collects the alerts in the order the checks run.
func raiseAlerts(pawns []game.PawnView, w game.WorldView) []Alert {
	var out []Alert
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
		a := Alert{Key: key, Danger: danger}
		if len(who) == 1 {
			a.Label = one(who[0].Name)
		} else {
			a.Label = fmt.Sprintf("%d %s", len(who), many)
		}
		for _, p := range who {
			a.Targets = append(a.Targets, [2]int{p.X, p.Y})
		}
		out = append(out, a)
	}

	if al, ok := hostileAlert(allPawns, w); ok {
		out = append(out, al)
	}

	if al, ok := FireAlert(w); ok {
		out = append(out, al)
	}
	out = append(out, AnimalAlerts(w)...)

	var rescue, starving, collapsed, unhappy, bleeding, infected, untended, diseased []game.PawnView
	var overdosed, bedless []game.PawnView // MED-07, BED-3
	overdoseDanger := false
	alive := 0
	for _, p := range pawns {
		if p.Status != "dead" && p.Bleeding && p.BleedDeathHours <= bleedAlertHours {
			bleeding = append(bleeding, p)
		}
		if p.Status != "dead" && p.Infection > 0 && p.InfectionLosing { // MED-02
			infected = append(infected, p)
		}
		if p.Status != "dead" && p.Wounded && p.UntendedHours >= untendedAlertHours {
			untended = append(untended, p)
		}
		if p.Status != "dead" && p.Disease != "" { // MED-04
			diseased = append(diseased, p)
		}
		if p.Status != "dead" && p.OverdoseStage > 0 { // MED-07: major or worse
			overdosed = append(overdosed, p)
			overdoseDanger = overdoseDanger || p.OverdoseStage >= overdoseExtremeStage
		}
		if p.Status != "dead" && !p.HasBed { // BED-3: HasBed is state, so the alert holds until a bed exists
			bedless = append(bedless, p)
		}
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
	group("nobed", false, bedless, func(n string) string { return n + " needs a bed" }, "colonists need beds")
	group("bleeding", true, bleeding, func(n string) string { return n + " is bleeding out" }, "colonists are bleeding out")
	group("infection", false, infected, func(n string) string { return n + " is losing to an infection" }, "colonists are losing to infections")
	if len(diseased) == 1 { // MED-04: one patient names the disease
		p := diseased[0]
		group("disease", false, diseased, func(n string) string { return n + " is sick: " + p.Disease }, "")
	} else {
		group("disease", false, diseased, nil, "colonists are sick")
	}
	group("overdose", overdoseDanger, overdosed, func(n string) string { return n + " has overdosed" }, "colonists have overdosed")
	group("untended", false, untended, func(n string) string { return n + " has an untended wound" }, "colonists have untended wounds")
	group("rescue", true, rescue, func(n string) string { return n + " needs rescue" }, "colonists need rescue")
	group("starving", true, starving, func(n string) string { return n + " is starving" }, "colonists are starving")
	group("collapsed", false, collapsed, func(n string) string { return n + " collapsed from exhaustion" }, "colonists collapsed")
	if alive > 0 && w.Food < lowFoodPerColonist*alive {
		out = append(out, Alert{Key: "food", Label: fmt.Sprintf("Low food (%d left)", w.Food)})
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
		out = append(out, Alert{Key: "raid", Label: label, Danger: true, Targets: raiders})
	}
	var breaking []game.PawnView
	for _, p := range pawns {
		if p.MentalBreak != "" && p.Status != "dead" {
			breaking = append(breaking, p)
		}
	}
	if len(breaking) == 1 {
		p := breaking[0]
		out = append(out, Alert{Key: "break", Danger: true, Label: p.Name + ": mental break (" + p.MentalBreak + ")", Targets: [][2]int{{p.X, p.Y}}})
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
	if al, ok := noFuelAlert(w); ok { // alerts_power.go: campfires, torches, generators
		out = append(out, al)
	}
	out = append(out, brownoutAlerts(w)...) // alerts_power.go (PLAY-021)
	if al, ok := notConnectedAlert(w); ok {
		out = append(out, al)
	}
	if al, ok := spoilingAlert(w); ok {
		out = append(out, al)
	}
	if al, ok := WaitingFramesAlert(w); ok {
		out = append(out, al)
	}
	group("unhappy", false, unhappy, func(n string) string { return n + " is unhappy" }, "colonists are unhappy")
	return out
}

// hostileAlert is the "hostile" alert for animals attacking colonists: one
// animal says what it is and, when it is chasing someone, whom ("A timber
// wolf is chasing Ada!"); several are only counted. pawns is every pawn
// the target may be among.
func hostileAlert(pawns []game.PawnView, w game.WorldView) (Alert, bool) {
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
		return Alert{}, false
	}
	label := withArticle(speciesLabel(first)) + " is attacking!"
	if first.TargetID != 0 {
		if p, ok := creature.ChasedPawn(pawns, first.TargetID); ok && p.Name != "" {
			label = withArticle(speciesLabel(first)) + " is chasing " + p.Name + "!"
		}
	}
	if len(targets) > 1 {
		label = fmt.Sprintf("%d animals are attacking!", len(targets))
	}
	return Alert{Key: "hostile", Label: label, Danger: true, Targets: targets}, true
}

// withArticle puts "A" or "An" before a noun phrase ("An arctic fox").
func withArticle(noun string) string {
	if noun != "" && strings.ContainsRune("aeiouAEIOU", rune(noun[0])) {
		return "An " + noun
	}
	return "A " + noun
}

// Current is the alerts now, most urgent first (BuildAlerts over every
// pawn on the map, then the trade and allies alerts).
func (c Alerts) Current() []Alert {
	env := c.sh.Env()
	if env.Game == nil {
		return nil
	}
	// Several HUD passes ask every frame (chips, alert rows, hit layout):
	// build once per update while the read models are the same ones.
	snap, w := env.Game.Snapshot(), c.sh.World()
	k := NewAlertsKey(env.Session, c.sh.Present(), snap, w)
	if m := &c.alerts; env.ViewCached && m.ok && m.key == k {
		return m.out
	}
	out := BuildAlerts(snap, w)                             // colonists and raiders: raiseAlerts splits them
	if extra := TradeAlertsFrom(env.Game); len(extra) > 0 { // alerts_trade.go (PLAY-024)
		for i := range extra {
			extra[i].Sev, extra[i].Explain = SevWarning, AlertExplain(extra[i].Key)
		}
		out = append(out, extra...)
	}
	c.alerts = alertsMemo{key: k, out: out, ok: true}
	return out
}

// NewAlertsKey is the identity of the read models the alerts are built
// from (the colony session and presentation frame, the lists' identity,
// the power data included: the nets, their version and the flare).
func NewAlertsKey(session int, present uint64, snap []game.PawnView, w game.WorldView) AlertsKey {
	return AlertsKey{
		session: session, present: present, pawns: light.First(snap), nPawns: len(snap),
		animals: light.First(w.Animals), nAnimals: len(w.Animals), items: light.First(w.Items), nItems: len(w.Items),
		bldgs: light.First(w.Buildings), nBldgs: len(w.Buildings), frames: light.First(w.Frames), nFrames: len(w.Frames),
		food: w.Food, powerVer: w.PowerVer, nets: light.First(w.PowerNets), nNets: len(w.PowerNets), flare: w.SolarFlare,
	}
}

// alertsMemo is currentAlerts' last result and the read models it came
// from (identity of their lists, the update counter). Callers only read it.
type alertsMemo struct {
	key AlertsKey
	out []Alert
	ok  bool
}

// AlertsKey is what Current's memo compares.
type AlertsKey struct {
	session                          int
	present                          uint64
	pawns                            *game.PawnView
	animals                          *game.AnimalView
	items                            *game.ItemView
	bldgs                            *game.BuildingView
	frames                           *game.FrameView
	nPawns, nAnimals, nItems, nBldgs int
	nFrames, food                    int
	// power data (PLAY-021): a net's brownout moves PowerVer and the nets
	// list; the flare is a flag of its own
	powerVer uint64
	nets     *game.PowerNetView
	nNets    int
	flare    bool
}

// AlertRow is one row of the stack: an alert, or the "+N more" row that
// stands for the alerts that did not fit.
type AlertRow struct {
	R      gfx.Rect
	Al     Alert   // zero for the "+N more" row
	More   int     // > 0: the "+N more" row
	Hidden []Alert // what the "+N more" row stands for
}

// AlertRowLabel is the text a row shows.
func AlertRowLabel(r AlertRow) string {
	if r.More > 0 {
		return fmt.Sprintf("+%d more", r.More)
	}
	return r.Al.Label
}

// StackSplit says how many of total entries a stack of rows rows shows
// and how many it folds into its "+N" row. limit caps the entries shown
// before the "+N" row appears at all (the letter stack's eight). With one
// row there is no room for the count, so it shows an entry.
func StackSplit(total, rows, limit int) (shown, folded int) {
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

// Rows lays the alert stack out in l's Alerts rect: the most urgent
// alerts first, the last row counting the rest when they do not fit.
func (c Alerts) Rows(l hudlayout.Layout) []AlertRow {
	als := c.Current()
	if len(als) == 0 || l.AlertRows <= 0 || l.Alerts.Empty() {
		return nil
	}
	shown, more := StackSplit(len(als), l.AlertRows, len(als))
	rows := make([]AlertRow, 0, shown+1)
	at := func(i int) gfx.Rect {
		return gfx.Rect{X: l.Alerts.X, Y: l.Alerts.Y + float32(i)*(hudlayout.AlertRowH+hudlayout.HudGap), W: l.Alerts.W, H: hudlayout.AlertRowH}
	}
	for i := 0; i < shown; i++ {
		rows = append(rows, AlertRow{R: at(i), Al: als[i]})
	}
	if more > 0 {
		rows = append(rows, AlertRow{R: at(shown), More: more, Hidden: als[shown:]})
	}
	return rows
}

// Buttons are the alert rows as click targets, ids "alert.<key>".
func (c Alerts) Buttons(sw int) []uikit.Button { return c.ButtonsIn(c.sh.Layout()) }

func (c Alerts) ButtonsIn(l hudlayout.Layout) []uikit.Button {
	var out []uikit.Button
	for _, row := range c.Rows(l) {
		if row.More > 0 {
			continue // the count is not a place to jump to
		}
		out = append(out, uikit.Button{ID: ids.PrefixAlert + row.Al.Key, Label: row.Al.Label, X: row.R.X, Y: row.R.Y, W: row.R.W, H: row.R.H})
	}
	return out
}

// AlertPulse is the danger backing's brightness (0.55..1) at presentation
// frame present; reduced motion holds it steady.
func AlertPulse(present uint64, reduced bool) float64 {
	if reduced {
		return 1
	}
	return 0.775 + 0.225*math.Sin(float64(present)/9)
}

func (c Alerts) Draw(screen *ebiten.Image, sw int) { c.DrawIn(screen, c.sh.Layout()) }

func (c Alerts) DrawIn(screen *ebiten.Image, l hudlayout.Layout) {
	rows := c.Rows(l)
	if len(rows) == 0 {
		return
	}
	pulse := AlertPulse(c.sh.Present(), c.sh.Env().ReducedMotion)
	mx, my := c.sh.Cursor()
	for _, row := range rows {
		r := row.R
		edge, bg, text := uikit.Warning, uikit.AlertWarnBg, uikit.Text
		switch {
		case row.More > 0:
			edge, bg, text = uikit.PanelBorder, uikit.Panel, uikit.TextDim
		case row.Al.Sev == SevDanger:
			edge, bg = uikit.Danger, creature.Fade(uikit.AlertDangerBg, pulse)
		}
		uikit.FillRect(screen, r, bg)
		if row.More == 0 {
			uikit.FillRect(screen, gfx.Rect{X: r.X, Y: r.Y, W: 4, H: r.H}, edge)
		}
		border := edge
		if row.More > 0 || !r.Hit(mx, my) {
			border = creature.Fade(edge, 0.5)
		}
		uikit.StrokeRect(screen, r, border)
		gfx.Text(screen, uikit.FitText(AlertRowLabel(row), gfx.Body, r.W-20), float64(r.X)+12, float64(r.Y)+(float64(r.H)-gfx.LineH(gfx.Body))/2, gfx.WithColor(gfx.Body, text))
	}
}

// Focus centres the camera on the alert's next target.
func (c Alerts) Focus(id string) { c.cycle(strings.TrimPrefix(id, ids.PrefixAlert), 1) }

// cycle centres the camera on the alert's next target (dir 1) or
// previous one (dir -1), wrapping round. alertCycle holds one more than
// the index of the target on show (0: none yet), so a fresh alert going
// back starts at its last.
func (c Alerts) cycle(key string, dir int) {
	for _, al := range c.Current() {
		if al.Key != key || len(al.Targets) == 0 {
			continue
		}
		if c.alertCycle == nil {
			c.alertCycle = map[string]int{}
		}
		n, v := len(al.Targets), c.alertCycle[key]
		idx := v % n
		if dir < 0 {
			idx = n - 1
			if v > 0 {
				idx = ((v-2)%n + n) % n
			}
		}
		c.alertCycle[key] = idx + 1
		t := al.Targets[idx]
		c.sh.CenterOnMap(float64(t[0])+0.5, float64(t[1])+0.5)
		return
	}
}

// hungryAlertBelow is the food percent under which a tame animal raises
// "Animals hungry".
const hungryAlertBelow = 25

// penFullDiv: a pen holding more animals than its cells / this is full.
const penFullDiv = 4

// AnimalAlerts are the animal ecology alerts, derived from the read
// model: hungry tame animals, wild animals eating crops, timbergnaws
// eating trees and full pens.
func AnimalAlerts(w game.WorldView) []Alert {
	var out []Alert
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
		case an.Tame && an.State != "" && an.Food < hungryAlertBelow:
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
		out = append(out, Alert{Key: "animalhungry", Label: label, Targets: hungry})
	}
	if len(raiders) > 0 {
		out = append(out, Alert{Key: "cropraid", Label: "Wild animals eating crops", Danger: true, Targets: raiders})
	}
	if len(gnaws) > 0 {
		label := "Timbergnaws eating trees"
		if w.TreesFelled > 0 {
			label = fmt.Sprintf("Timbergnaws eating trees (%d felled)", w.TreesFelled)
		}
		out = append(out, Alert{Key: "timbergnaw", Label: label, Targets: gnaws})
	}
	out = append(out, penAlerts(w)...)
	return out
}

// FireAlert is "Fire in the colony!" (PLAY-015) while any fire burns
// inside Home; clicking cycles the burning Home cells.
func FireAlert(w game.WorldView) (Alert, bool) {
	var at [][2]int
	for _, f := range w.Fires {
		if i := f.Y*w.TerrainW + f.X; f.X >= 0 && f.X < w.TerrainW && i >= 0 && i < len(w.Home) && w.Home[i] != 0 {
			at = append(at, [2]int{f.X, f.Y})
		}
	}
	if len(at) == 0 {
		return Alert{}, false
	}
	return Alert{Key: "fire", Label: "Fire in the colony!", Danger: true, Targets: at}, true
}

// penAlerts are the pen alerts (PLAY-015): each open pen ("Pen 2 is
// open"), each pen holding more tame animals than its cells / penFullDiv,
// and the livestock with no pen ("Unpenned livestock: N").
func penAlerts(w game.WorldView) []Alert {
	var out []Alert
	inBits := func(b []uint64, x, y int) bool {
		i := y*w.TerrainW + x
		return w.TerrainW > 0 && x >= 0 && x < w.TerrainW && i >= 0 && i>>6 < len(b) && b[i>>6]&(1<<(uint(i)&63)) != 0
	}
	for _, p := range w.Pens {
		if !p.Enclosed {
			out = append(out, Alert{Key: fmt.Sprintf("penopen.%d", p.ID), Label: p.Name + " is open",
				Danger: true, Targets: [][2]int{{p.X, p.Y}}})
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
			out = append(out, Alert{Key: fmt.Sprintf("penfull.%d", p.ID),
				Label:   fmt.Sprintf("%s full: %d animals in %d tiles", p.Name, len(pos), cells),
				Targets: pos})
		}
	}
	var roaming [][2]int
	for _, an := range w.Animals {
		if an.Penned && an.Pen == "Roaming" {
			roaming = append(roaming, [2]int{an.X, an.Y})
		}
	}
	if len(roaming) > 0 {
		out = append(out, Alert{Key: "unpenned", Label: fmt.Sprintf("Unpenned livestock: %d", len(roaming)), Targets: roaming})
	}
	return out
}

// WaitingFramesAlert flags blueprints whose material is not in stock
// anywhere (the frame status starts with "No "): clicking cycles them.
func WaitingFramesAlert(w game.WorldView) (Alert, bool) {
	var at [][2]int
	for _, f := range w.Frames {
		if strings.HasPrefix(f.Status, "No ") {
			at = append(at, [2]int{f.X, f.Y})
		}
	}
	if len(at) == 0 {
		return Alert{}, false
	}
	return Alert{Key: "waitingframes", Label: fmt.Sprintf("Blueprints waiting for materials (%d)", len(at)), Targets: at}, true
}

// alertExplanations say, for each alert key, what is wrong and what to do
// (the card shown while the alert is hovered). Keys with a numeric suffix
// ("penopen.3") share the text of their prefix.
var alertExplanations = map[string]string{
	"tradeorbit":    "A trade ship is in radio range for a few more hours. Work the radio console to trade with it: goods within 6 tiles of a trade beacon are on offer, and purchases arrive by drop pod.",
	"tradecaravan":  "A trader caravan is camped near the colony and leaves soon. Right-click the trader to send a colonist to trade; the caravan buys and sells goods from your stockpiles.",
	"nobed":         "Some colonists have no bed of their own and sleep on the ground. Build a bed for each of them, or free one: sleeping rough costs rest and mood.",
	"bleeding":      "A colonist will bleed to death within hours. Get a doctor to tend the wounds now: a tend stops the bleeding at once, with or without medicine.",
	"infection":     "An infection is set to kill a colonist before their immunity builds. Keep it tended every 12 hours with the best medicine you have, by your most skilled doctor, ideally in a hospital bed.",
	"disease":       "A colonist has caught a disease. Flu, plague, malaria and sleeping sickness race the body's immunity and can kill: keep the sick in bed and tended on time, with good medicine and your best doctor. Gut worms and muscle parasites never kill but stay until tended away.",
	"overdose":      "A colonist has taken too much of a hard drug or prophyl. It wears off by a whole dose a day; at the extreme stage they collapse and may die. Keep them in bed, take spark tablets out of their drug policy and let no more doses reach them until it has passed.",
	"untended":      "A colonist's wound has gone untended for hours. Untended wounds bleed, heal slowly and fester more often. Let a doctor tend it, or let the colonist rest so a doctor can reach them.",
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
	"nofuel":        "A campfire, torch lamp or wood-fired generator has burned out of wood. Haulers refill them from the stockpile, so keep logs in stock; a generator makes no power until then.",
	"brownout":      "A grid is asked for more power than it makes and has switched devices off. Build another generator or a battery, or switch off what you can do without.",
	"notconnected":  "Some electrical devices have no grid in reach. Run conduit to within three tiles of each one, or place a power switch or battery on the line.",
	"spoiling":      "Stored food is about to rot. Eat it, or move it somewhere cool, before it is lost.",
	"waitingframes": "Some blueprints need materials that are not in stock. Gather or make what they need, or cancel the blueprints.",
	"unhappy":       "Colonists are in a poor mood and close to breaking. Look at what drags it down: hunger, rest, comfort and room to breathe.",
	"allies":        "Fighters sent by a friendly faction are helping defend the colony. They fight raiders and hostile animals on their own and go home once all is quiet, within a day. Turrets and colonists never shoot them.",
}

// AlertExplain is the explanation for an alert key.
func AlertExplain(key string) string {
	if e, ok := alertExplanations[strings.SplitN(key, ".", 2)[0]]; ok {
		return e
	}
	return "Select this alert to look at the cause."
}
