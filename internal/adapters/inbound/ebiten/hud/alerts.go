package hud

import (
	"cmp"
	"fmt"
	"math"
	"math/bits"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/render/creature"
	"rimworld-go/internal/adapters/inbound/ebiten/render/light"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"slices"
	"strconv"
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
	// Map is the map the targets are on (Current sets it to the viewed map
	// for every alert but a cross-map one, which names its own).
	Map game.MapID
	// Cross marks an alert about another open map than the one on screen;
	// its target (-1, -1) only switches to the map.
	Cross bool
}

const (
	// lowFoodDays: fewer days of food (WorldView.FoodDays, tenths) than
	// this raises "Low food" (FOOD-4: vanilla's 4 nutrition a colonist,
	// 2.5 days at 1.6 a day; the wiki's Alerts page was unavailable).
	lowFoodDays = 25
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

// BuildAlertsIn derives the current alerts from pawns and the world, most
// urgent first: dangers before warnings, then by key.
func BuildAlertsIn(tr lang.Tr, pawns []game.PawnView, w game.WorldView) []Alert {
	out := raiseAlertsIn(tr, pawns, w)
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
func raiseAlertsIn(tr lang.Tr, pawns []game.PawnView, w game.WorldView) []Alert {
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
	group := func(key string, danger bool, who []game.PawnView, one func(string) string, many string) { // many is the translated format over the count
		if len(who) == 0 {
			return
		}
		a := Alert{Key: key, Danger: danger}
		if len(who) == 1 {
			a.Label = one(who[0].Name)
		} else {
			a.Label = fmt.Sprintf(many, len(who))
		}
		for _, p := range who {
			a.Targets = append(a.Targets, [2]int{p.X, p.Y})
		}
		out = append(out, a)
	}

	if al, ok := hostileAlert(tr, allPawns, w); ok {
		out = append(out, al)
	}

	if al, ok := FireAlertIn(tr, w); ok {
		out = append(out, al)
	}
	out = append(out, AnimalAlertsIn(tr, w)...)

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
	group("nobed", false, bedless, func(n string) string { return lang.Tf(tr, "%s needs a bed", n) }, tr.T("%d colonists need beds"))
	group("bleeding", true, bleeding, func(n string) string { return lang.Tf(tr, "%s is bleeding out", n) }, tr.T("%d colonists are bleeding out"))
	group("infection", false, infected, func(n string) string { return lang.Tf(tr, "%s is losing to an infection", n) }, tr.T("%d colonists are losing to infections"))
	if len(diseased) == 1 { // MED-04: one patient names the disease
		p := diseased[0]
		group("disease", false, diseased, func(n string) string { return lang.Tf(tr, "%s is sick: %s", n, p.Disease) }, "")
	} else {
		group("disease", false, diseased, nil, tr.T("%d colonists are sick"))
	}
	group("overdose", overdoseDanger, overdosed, func(n string) string { return lang.Tf(tr, "%s has overdosed", n) }, tr.T("%d colonists have overdosed"))
	group("untended", false, untended, func(n string) string { return lang.Tf(tr, "%s has an untended wound", n) }, tr.T("%d colonists have untended wounds"))
	group("rescue", true, rescue, func(n string) string { return lang.Tf(tr, "%s needs rescue", n) }, tr.T("%d colonists need rescue"))
	group("starving", true, starving, func(n string) string { return lang.Tf(tr, "%s is starving", n) }, tr.T("%d colonists are starving"))
	group("collapsed", false, collapsed, func(n string) string { return lang.Tf(tr, "%s collapsed from exhaustion", n) }, tr.T("%d colonists collapsed"))
	if alive > 0 && w.FoodDays < lowFoodDays {
		out = append(out, Alert{Key: "food", Label: lang.Tf(tr, "Low food (%d.%d days)", w.FoodDays/10, w.FoodDays%10)})
	}

	if len(raiders) > 0 {
		label := tr.T("Raid: 1 raider")
		if len(raiders) > 1 {
			label = lang.Tf(tr, "Raid: %d raiders", len(raiders))
		}
		switch plan {
		case "siege":
			label = lang.Tf(tr, "Siege: %d raiders", len(raiders))
		case "sappers":
			label = lang.Tf(tr, "Sappers: %d raiders", len(raiders))
		case "staged assault": // EAI-016
			label = lang.Tf(tr, "Staging: %d raiders", len(raiders))
		case "steal":
			label = lang.Tf(tr, "Thieves: %d raiders", len(raiders))
		case "burn":
			label = lang.Tf(tr, "Arsonists: %d raiders", len(raiders))
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
		out = append(out, Alert{Key: "break", Danger: true, Label: lang.Tf(tr, "%s: mental break (%s)", p.Name, p.MentalBreak), Targets: [][2]int{{p.X, p.Y}}})
	} else if len(breaking) > 1 {
		group("break", true, breaking, func(n string) string { return n }, tr.T("%d colonists are having mental breaks"))
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
	group("hypothermia", true, hypo, func(n string) string { return lang.Tf(tr, "%s has hypothermia", n) }, tr.T("%d colonists have hypothermia"))
	group("heatstroke", true, heat, func(n string) string { return lang.Tf(tr, "%s has heatstroke", n) }, tr.T("%d colonists have heatstroke"))
	if al, ok := NotRecoveringAlertIn(tr, pawns); ok { // alerts_exposure.go (AIL-004)
		out = append(out, al)
	}
	if al, ok := NeedWarmClothesAlertIn(tr, pawns, w); ok { // alerts_temp.go (TEMP-5)
		out = append(out, al)
	}
	if al, ok := ColdAnimalAlertIn(tr, w); ok {
		out = append(out, al)
	}
	if al, ok := noFuelAlert(tr, w); ok { // alerts_power.go: campfires, torches, generators
		out = append(out, al)
	}
	out = append(out, brownoutAlerts(tr, w)...) // alerts_power.go (PLAY-021)
	if al, ok := FishDyingAlertIn(tr, w); ok {  // alerts_tank.go (FISH)
		out = append(out, al)
	}
	if al, ok := sparkingAlert(tr, w); ok { // alerts_power.go (MAP-21): a turret's fuse is lit
		out = append(out, al)
	}
	if al, ok := notConnectedAlert(tr, w); ok {
		out = append(out, al)
	}
	if al, ok := spoilingAlert(w); ok {
		out = append(out, al)
	}
	if al, ok := WaitingFramesAlertIn(tr, w); ok {
		out = append(out, al)
	}
	out = append(out, shipAlerts(tr, w)...) // ending.go (AX-8): the reactor's countdown, a ready ship
	group("unhappy", false, unhappy, func(n string) string { return lang.Tf(tr, "%s is unhappy", n) }, tr.T("%d colonists are unhappy"))
	if al, ok := GoneAlertIn(tr, w); ok { // ending.go (MAP-19)
		out = append(out, al)
	}
	if w.ResearchAlert { // RSCH-2: the game raises it (a bench and a researcher, no project)
		out = append(out, Alert{Key: ResearchAlertKey, Label: tr.T("Need research project")})
	}
	return out
}

// ResearchAlertKey is the "Need research project" alert; a click on it
// opens the Research tab instead of moving the camera.
const ResearchAlertKey = "research"

// hostileAlert is the "hostile" alert for animals attacking colonists: one
// animal says what it is and, when it is chasing someone, whom ("A timber
// wolf is chasing Ada!"); several are only counted. pawns is every pawn
// the target may be among.
func hostileAlert(tr lang.Tr, pawns []game.PawnView, w game.WorldView) (Alert, bool) {
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
	label := lang.Tf(tr, "%s is attacking!", withArticle(tr, speciesLabel(first)))
	if first.TargetID != 0 {
		if p, ok := creature.ChasedPawn(pawns, first.TargetID); ok && p.Name != "" {
			label = lang.Tf(tr, "%s is chasing %s!", withArticle(tr, speciesLabel(first)), p.Name)
		}
	}
	if len(targets) > 1 {
		label = lang.Tf(tr, "%d animals are attacking!", len(targets))
	}
	return Alert{Key: "hostile", Label: label, Danger: true, Targets: targets}, true
}

// withArticle puts "A" or "An" before a noun phrase ("An arctic fox").
func withArticle(tr lang.Tr, noun string) string {
	if noun != "" && strings.ContainsRune("aeiouAEIOU", rune(noun[0])) {
		return lang.Tf(tr, "An %s", noun)
	}
	return lang.Tf(tr, "A %s", noun)
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
	out := BuildAlertsIn(c.sh, snap, w)                             // colonists and raiders: raiseAlerts splits them
	if extra := TradeAlertsFromIn(c.sh, env.Game); len(extra) > 0 { // alerts_trade.go (PLAY-024)
		for i := range extra {
			extra[i].Sev, extra[i].Explain = SevWarning, AlertExplain(extra[i].Key)
		}
		out = append(out, extra...)
	}
	if al, ok := QuestExpiringAlertIn(c.sh, w, uint64(c.sh.Tick())); ok { // alerts_quest.go (QST-134)
		al.Sev, al.Explain = SevWarning, AlertExplain(al.Key)
		out = append(out, al)
	}
	out = markMaps(c.sh, out, env.Game)
	c.alerts = alertsMemo{key: k, out: out, ok: true}
	return out
}

// MapThreatKey is the key prefix of the cross-map threat alert
// ("mapthreat.<map id>").
const MapThreatKey = "mapthreat"

// markMaps stamps every alert with the viewed map and appends "Threat on
// <label>" for each other open map with hostiles (MAP-1).
func markMaps(tr lang.Tr, out []Alert, g Reader) []Alert {
	viewed := g.ViewedMap()
	for i := range out {
		out[i].Map = viewed
	}
	maps := g.Maps()
	if len(maps) < 2 {
		return out
	}
	for _, m := range maps {
		if m.ID == viewed || !m.Threat {
			continue
		}
		out = append(out, Alert{
			Key: MapThreatKey + "." + strconv.FormatUint(uint64(m.ID), 10), Label: lang.Tf(tr, "Threat on %s", m.Label),
			Danger: true, Sev: SevDanger, Explain: AlertExplain(MapThreatKey),
			Targets: [][2]int{{-1, -1}}, Map: m.ID, Cross: true,
		})
	}
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
	Al     Alert    // zero for the "+N more" row
	More   int      // > 0: the "+N more" row
	Hidden []Alert  // what the "+N more" row stands for
	Lines  []string // the label wrapped to the row's text width, at most alertMaxLines (QST-134)
}

const (
	alertMaxLines  = 2  // an alert wraps to at most this many lines
	alertLinePitch = 16 // a wrapped line's pitch
	alertRowPad    = 6  // the row's height beyond its lines
)

// alertLines is label wrapped to width w in at most alertMaxLines lines;
// the last line is ellipsized when the label goes on.
func alertLines(label string, w float32) []string {
	lines := gfx.WrapPx(label, gfx.Small, float64(w))
	if len(lines) > alertMaxLines {
		last := strings.TrimRight(strings.Join(lines[alertMaxLines-1:], " "), " ")
		lines = append(lines[:alertMaxLines-1], gfx.Ellipsize(last+"…", gfx.Small, float64(w)))
	}
	if len(lines) == 0 {
		lines = []string{""}
	}
	return lines
}

// alertRowH is the height of a row of n lines: one line keeps AlertRowH.
func alertRowH(n int) float32 {
	return max(float32(hudlayout.AlertRowH), float32(n*alertLinePitch+alertRowPad))
}

// AlertRowLabelIn is the text a row shows.
func AlertRowLabelIn(tr lang.Tr, r AlertRow) string {
	if r.More > 0 {
		return lang.Tf(tr, "+%d more", r.More)
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
	textW := l.Alerts.W - 2*hudlayout.AlertRight
	lines := make([][]string, len(als))
	total := float32(0)
	for i, al := range als {
		lines[i] = alertLines(al.Label, textW)
		total += alertRowH(len(lines[i]))
	}
	// everything shows when it fits the rect; else the rows that leave room
	// for the "+N more" row do (at least one)
	shown := len(als)
	if total > l.Alerts.H+0.5 || len(als) > l.AlertRows {
		shown, total = 0, 0
		for shown < len(als) && shown+1 < l.AlertRows && total+alertRowH(len(lines[shown]))+hudlayout.AlertRowH <= l.Alerts.H+0.5 {
			total += alertRowH(len(lines[shown]))
			shown++
		}
		shown = max(shown, min(1, len(als)))
	}
	rows := make([]AlertRow, 0, shown+1)
	y := l.Alerts.Y
	for i := 0; i < shown; i++ {
		h := alertRowH(len(lines[i]))
		rows = append(rows, AlertRow{R: gfx.Rect{X: l.Alerts.X, Y: y, W: l.Alerts.W, H: h}, Al: als[i], Lines: lines[i]})
		y += h
	}
	if more := len(als) - shown; more > 0 && l.AlertRows > 1 { // one row has no room for the count
		rows = append(rows, AlertRow{R: gfx.Rect{X: l.Alerts.X, Y: y, W: l.Alerts.W, H: hudlayout.AlertRowH}, More: more, Hidden: als[shown:]})
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

// textY is the y that centres s, drawn in st, in r: by the height the text
// measures (the font's own, taller than its reference pitch), so it stays
// inside r.
func textY(r gfx.Rect, s string, st gfx.TextStyle) float64 {
	_, h := gfx.Measure(s, st)
	return float64(r.Y) + (float64(r.H)-h)/2
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
		// UIS-030: plain right-aligned text, no panel; the severity is the
		// text colour, a danger's pulse its brightness, a shadow plate keeps
		// it legible over the map
		text := uikit.Warning
		switch {
		case row.More > 0:
			text = uikit.TextDim
		case row.Al.Sev == SevDanger:
			text = creature.Fade(uikit.Danger, pulse)
		}
		if row.More == 0 && r.Hit(mx, my) {
			text = uikit.Text
		}
		lines, plate := row.Lines, creature.Fade(uikit.Panel, 0.55)
		if row.More > 0 {
			lines = []string{AlertRowLabelIn(c.sh, row)}
		} else if strings.HasPrefix(row.Al.Key, QuestExpiringKey) {
			plate = creature.Fade(uikit.PanelBorder, 0.6) // QST-134: a grey band, not red
		}
		top := r.Y + (r.H-float32(len(lines)*alertLinePitch))/2
		for i, label := range lines {
			label = uikit.FitText(label, gfx.Small, r.W-2*hudlayout.AlertRight)
			w, _ := gfx.Measure(label, gfx.Small)
			x := float64(r.Right()) - hudlayout.AlertRight - w
			lr := gfx.Rect{X: r.X, Y: top + float32(i*alertLinePitch), W: r.W, H: alertLinePitch}
			uikit.FillRect(screen, gfx.Rect{X: float32(x) - 4, Y: lr.Y + 1, W: float32(w) + 8, H: lr.H - 1}, plate)
			gfx.Text(screen, label, x, textY(lr, label, gfx.Small), gfx.WithColor(gfx.Small, text))
		}
	}
}

// Focus centres the camera on the alert's next target.
func (c Alerts) Focus(id string) {
	key := strings.TrimPrefix(id, ids.PrefixAlert)
	if d, ok := c.sh.(ports.Dispatcher); ok && key == ResearchAlertKey {
		d.Do(ports.ShowWindow{Mode: "research"})
		return
	}
	if id, ok := questOfAlertKey(key); ok { // alerts_quest.go: open that quest
		showQuest(c.sh, id)
		return
	}
	if key == GoneAlertKey { // ending.go: back to the everyone-gone screen
		Right(c).openGone(c.sh.World())
		return
	}
	c.cycle(key, 1)
}

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
		c.sh.Jump(ports.Spot{Map: al.Map, X: t[0], Y: t[1]})
		return
	}
}

// hungryAlertBelow is the food percent under which a tame animal raises
// "Animals hungry".
const hungryAlertBelow = 25

// penFullDiv: a pen holding more animals than its cells / this is full.
const penFullDiv = 4

// AnimalAlertsIn are the animal ecology alerts, derived from the read
// model: hungry tame animals, wild animals eating crops, timbergnaws
// eating trees and full pens.
func AnimalAlertsIn(tr lang.Tr, w game.WorldView) []Alert {
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
		label := tr.T("Animals hungry")
		if len(hungry) > 1 {
			label = lang.Tf(tr, "Animals hungry (%d)", len(hungry))
		}
		out = append(out, Alert{Key: "animalhungry", Label: label, Targets: hungry})
	}
	if len(raiders) > 0 {
		out = append(out, Alert{Key: "cropraid", Label: tr.T("Wild animals eating crops"), Danger: true, Targets: raiders})
	}
	if len(gnaws) > 0 {
		label := tr.T("Timbergnaws eating trees")
		if w.TreesFelled > 0 {
			label = lang.Tf(tr, "Timbergnaws eating trees (%d felled)", w.TreesFelled)
		}
		out = append(out, Alert{Key: "timbergnaw", Label: label, Targets: gnaws})
	}
	out = append(out, penAlerts(tr, w)...)
	return out
}

// FireAlertIn is "Fire in the colony!" (PLAY-015) while any fire burns
// inside Home; clicking cycles the burning Home cells.
func FireAlertIn(tr lang.Tr, w game.WorldView) (Alert, bool) {
	var at [][2]int
	for _, f := range w.Fires {
		if i := f.Y*w.TerrainW + f.X; f.X >= 0 && f.X < w.TerrainW && i >= 0 && i < len(w.Home) && w.Home[i] != 0 {
			at = append(at, [2]int{f.X, f.Y})
		}
	}
	if len(at) == 0 {
		return Alert{}, false
	}
	return Alert{Key: "fire", Label: tr.T("Fire in the colony!"), Danger: true, Targets: at}, true
}

// penAlerts are the pen alerts (PLAY-015): each open pen ("Pen 2 is
// open"), each pen holding more tame animals than its cells / penFullDiv,
// and the livestock with no pen ("Unpenned livestock: N").
func penAlerts(tr lang.Tr, w game.WorldView) []Alert {
	var out []Alert
	inBits := func(b []uint64, x, y int) bool {
		i := y*w.TerrainW + x
		return w.TerrainW > 0 && x >= 0 && x < w.TerrainW && i >= 0 && i>>6 < len(b) && b[i>>6]&(1<<(uint(i)&63)) != 0
	}
	for _, p := range w.Pens {
		if !p.Enclosed {
			out = append(out, Alert{Key: fmt.Sprintf("penopen.%d", p.ID), Label: lang.Tf(tr, "%s is open", p.Name),
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
				Label:   lang.Tf(tr, "%s full: %d animals in %d tiles", p.Name, len(pos), cells),
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
		out = append(out, Alert{Key: "unpenned", Label: lang.Tf(tr, "Unpenned livestock: %d", len(roaming)), Targets: roaming})
	}
	return out
}

// WaitingFramesAlertIn flags blueprints whose material is not in stock
// anywhere (the frame status starts with "No "): clicking cycles them.
func WaitingFramesAlertIn(tr lang.Tr, w game.WorldView) (Alert, bool) {
	var at [][2]int
	for _, f := range w.Frames {
		if strings.HasPrefix(f.Status, "No ") {
			at = append(at, [2]int{f.X, f.Y})
		}
	}
	if len(at) == 0 {
		return Alert{}, false
	}
	return Alert{Key: "waitingframes", Label: lang.Tf(tr, "Blueprints waiting for materials (%d)", len(at)), Targets: at}, true
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
	"sparking":      "A turret is sparking and will explode when the fuse runs out. Keep colonists away from it until it stops, and put out any fire it starts.",
	"food":          "There is little food for the number of colonists. Plant and harvest crops, hunt, or stop food going to waste before anyone starves.",
	"raid":          "Armed raiders are on the map. Draft your fighters, take cover near your defences and hold a line.",
	"break":         "A colonist is having a mental break and may act on their own. Keep others out of the way, then work on lifting their mood.",
	"warmclothes":   "Winter is coming and these colonists would be cold in what they wear. Make or buy warm clothes (a parka, a tuque, a jacket) and let them dress; an outfit that allows them helps.",
	"animalcold":    "Tame animals are too cold. Shelter them in a heated room or pen, or bring them indoors beside a fire.",
	"hypothermia":   "Colonists are freezing. Bring them indoors beside a fire and give them warm clothes.",
	"heatstroke":    "Colonists are overheating. Move them into shade or a cool room and keep them off heavy work.",
	"notrecovering": "A downed colonist's heatstroke or hypothermia can't heal in the air where they lie: it fades only in air inside their comfortable range. A doctor carries them to a bed or a spot where it does. If there is none, cool or warm a room with a cooler, a heater or shade, and they will mend there.",
	"nofuel":        "A campfire, torch lamp or wood-fired generator has burned out of wood. Haulers refill them from the stockpile, so keep logs in stock; a generator makes no power until then.",
	"brownout":      "A grid is asked for more power than it makes and has switched devices off. Build another generator or a battery, or switch off what you can do without.",
	"notconnected":  "Some electrical devices have no grid in reach. Run conduit to within three tiles of each one, or place a power switch or battery on the line.",
	"spoiling":      "Stored food is about to rot. Eat it, or move it somewhere cool, before it is lost.",
	"waitingframes": "Some blueprints need materials that are not in stock. Gather or make what they need, or cancel the blueprints.",
	"research":      "A research bench and a colonist who does research are ready, but no project is picked. Open the Research tab, select a project and press Start research.",
	"fishdying":     "A fish tank's stock is dying: the power is out, the air is too hot or cold for the fish, or the feeder is empty. Fix what the tank's pane names, and keep feed in stock for the haulers.",
	"gone":          "Nobody is left in the colony. Click to open the screen again: eight hours after the last one went, new wanderers can set out to start over, up to seven groups in all.",
	"questexpiring": "A quest offer will lapse within a day. Click to read it in the Quests window and accept it, or let it go.",
	"unhappy":       "Colonists are in a poor mood and close to breaking. Look at what drags it down: hunger, rest, comfort and room to breathe.",
	"shipwaking":    "The ship reactor is waking, and its call draws raiders and machines until it is fully awake. Wall it in and keep colonists near it: if it falls, the waking is lost.",
	"shipready":     "The ship is complete and its reactor awake. Board colonists into its stasis caskets, then select any ship part and press Launch ship to leave the planet. This ends the colony's story.",
	"mapthreat":     "Hostiles are on another of your maps. Click to switch to it and deal with them; nobody is steering that map while you look elsewhere, only your colonists' own orders hold.",
	"allies":        "Fighters sent by a friendly faction are helping defend the colony. They fight raiders and hostile animals on their own and go home once all is quiet, within a day. Turrets and colonists never shoot them.",
}

// AlertExplain is the explanation for an alert key.
func AlertExplain(key string) string {
	if e, ok := alertExplanations[strings.SplitN(key, ".", 2)[0]]; ok {
		return e
	}
	return "Select this alert to look at the cause."
}
