package ebiten

import (
	"fmt"
	"strconv"
	"time"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/world/planet"
)

// Create World (NG-1 §2.6): the planet's seed, coverage, rainfall,
// temperature, population and faction list, then a progress modal that
// steps planet.Builder a fixed budget of work units per frame.

func init() {
	registerScreenUI(screenWorldGen, func(a *App, sw, sh int) { a.worldGenUI(sw, sh) })
}

// Layout metrics.
const (
	worldRowGap   = 8
	worldFieldH   = 34
	worldSliderH  = 36
	worldLabelH   = 20
	worldFacRowH  = 44
	worldProgress = "world.progress"
	// worldStepStart is the work units stepped in a frame before the
	// frame time is known; stepping is bounded to keep a frame near 8 ms.
	worldStepStart = 1500
	worldStepMin   = 200
	worldStepMax   = 60000
	worldFrameBud  = 8 * time.Millisecond
)

// coverageStops are the Globe coverage choices (percent); 5 is dev-mode
// only (a tiny world for tests and quick looks).
var coverageStops = []int{30, 50, 100}

var (
	rainStops = []string{"Very dry", "Dry", "Normal", "Wet", "Very wet"}
	tempStops = []string{"Very cold", "Cold", "Normal", "Warm", "Very warm"}
	popStops  = []string{"Sparse", "Normal", "Crowded"}
)

// coverageOptions are the coverage values the slider offers now.
func (a *App) coverageOptions() []int {
	if a.devMode() || a.draft().world.Coverage == 5 {
		return append([]int{5}, coverageStops...)
	}
	return coverageStops
}

func pctLabels(v []int) []string {
	out := make([]string, len(v))
	for i, p := range v {
		out[i] = strconv.Itoa(p) + "%"
	}
	return out
}

// worldPhaseName is the progress modal's phase text.
func worldPhaseName(p planet.Phase) string {
	switch p {
	case planet.PhaseShape:
		return "Shaping the globe"
	case planet.PhaseElevation:
		return "Raising land"
	case planet.PhaseClimate:
		return "Climate"
	case planet.PhaseBiomes:
		return "Biomes"
	case planet.PhaseRivers:
		return "Rivers"
	case planet.PhaseFactions:
		return "Settling factions"
	case planet.PhaseRoads:
		return "Roads"
	}
	return "Done"
}

// worldTiles estimates the tile count of a coverage.
func worldTiles(cov int) int { return (10*planet.Frequency*planet.Frequency + 2) * cov / 100 }

// worldReady reports a generated planet matches the draft's settings.
func (ng *newGameState) worldReady() bool {
	return ng.planet != nil && ng.planetKey == ng.world.Key()
}

func (a *App) worldGenUI(sw, sh int) {
	a.declareOnScreenLayer()
	if a.setupTooSmall(sw, sh) {
		return
	}
	ng := a.draft()
	_, _, content, footer := dialogFrame(sw, sh)

	// A planet made from other settings is stale: its landing tile goes.
	if ng.planet != nil && ng.planetKey != ng.world.Key() {
		ng.tile = -1
	}
	a.worldStep()

	cols := content.columns(2, dialogPad)
	a.worldLeft(ng, cols[0])
	a.worldRight(ng, cols[1])

	label, tip := "Generate", ""
	if ng.worldReady() {
		label = "Next"
	} else if ng.planet != nil {
		tip = "These settings differ from the world you made. Generate it again."
	}
	a.ngButton("world.resetAll", rect{footerBack(footer).right() + 12, footerBack(footer).Y, 120, footerBtnH}, "Reset all",
		btnOpts{Tip: "Reset all\nPut every world setting back to its default and roll a new seed."}, func() {
			a.uiConfirm("world.resetAll.confirm", "Reset all world settings?", "Every setting on this screen goes back to its default and the seed is replaced.", "Reset", true, a.resetWorldAll)
		})
	a.ngFooter(footer, label, "world.generate", btnOpts{Tip: tip}, a.worldGenerate)
}

// resetWorldAll restores the world settings of a fresh draft with a new seed.
func (a *App) resetWorldAll() {
	ng := a.draft()
	ng.world = planet.Params{SeedText: randomWorldPhrase(), Coverage: defaultCoverage, Rain: 2, Temp: 2, Pop: 1,
		Factions: faction.DefaultSlots()}
}

// ---- left column: the planet --------------------------------------------

func (a *App) worldLeft(ng *newGameState, col rect) {
	a.uiPanel(col)
	in := col.inset(14)
	hs := tsLarge
	hs.Color = uiHeading
	a.uiLabel(in.X, in.Y, "Planet", hs)
	y := in.Y + float32(lineH(hs)) + 10

	// Seed.
	a.worldLabel("world.seed.lbl", rect{in.X, y, in.W, worldLabelH}, "Seed",
		"Seed\nThe same seed and settings always make the same world. Any text of 1 to 32 characters.")
	y += worldLabelH
	diceW := float32(80)
	a.uiTextField("world.seed", rect{in.X, y, in.W - diceW - 8, worldFieldH}, &ng.world.SeedText, textFieldSpec{
		MaxLen: maxSeedText, Valid: seedTextRune, Normalize: normalizeSeedText, Placeholder: "seed",
		Tip: "Seed\nLetters, digits, space, - and _ (1 to 32 characters).",
	})
	a.ngButton("world.dice", rect{in.right() - diceW, y, diceW, worldFieldH}, "Dice",
		btnOpts{Tip: "Randomize seed\nReplace the seed with a random phrase. It does not generate."},
		func() { ng.world.SeedText = randomWorldPhrase() })
	y += worldFieldH + worldRowGap + 6

	// Coverage.
	opts := a.coverageOptions()
	idx := 0
	for i, v := range opts {
		if v == ng.world.Coverage {
			idx = i
		}
	}
	a.worldLabel("world.coverage.lbl", rect{in.X, y, in.W, worldLabelH}, "Globe coverage",
		"Globe coverage\nHow much of the planet is generated. Bigger worlds take longer. This is not your colony's map size.")
	y += worldLabelH
	if a.uiSliderInt("world.coverage", rect{in.X, y, in.W, worldSliderH}, &idx, sliderSpec{Stops: pctLabels(opts), Title: "Globe coverage"}) {
		ng.world.Coverage = opts[idx]
	}
	y += worldSliderH + worldRowGap + 6

	// Rainfall, temperature, population.
	for _, s := range []struct {
		id, label, tip string
		stops          []string
		v              *int
	}{
		{"world.rain", "Rainfall", "Rainfall\nHow wet the planet is. Dry worlds have more deserts, wet ones more swamps and jungle.", rainStops, &ng.world.Rain},
		{"world.temp", "Temperature", "Temperature\nHow warm the planet is overall. Cold worlds have larger ice caps.", tempStops, &ng.world.Temp},
		{"world.pop", "Population", "Population\nHow many other settlements exist. Doesn't change your colonists or raid sizes.", popStops, &ng.world.Pop},
	} {
		a.worldLabel(s.id+".lbl", rect{in.X, y, in.W, worldLabelH}, s.label, s.tip)
		y += worldLabelH
		a.uiSliderInt(s.id, rect{in.X, y, in.W, worldSliderH}, s.v, sliderSpec{Stops: s.stops, Title: s.label})
		y += worldSliderH + worldRowGap + 6
	}

	// Status.
	st := dimText()
	a.uiLabel(in.X, y, fmt.Sprintf("About %s tiles at this coverage.", groupDigits(worldTiles(ng.world.Coverage))), st)
	y += float32(lineH(st)) + 4
	switch {
	case ng.worldReady():
		ok := tsBody
		ok.Color = uiGood
		a.uiLabel(in.X, y, "A world with these settings is ready. Next keeps it.", ok)
	case ng.planet != nil:
		w := tsBody
		w.Color = uiWarning
		a.uiLabel(in.X, y, "Settings changed since the last world. Generate it again.", w)
	}
}

// worldLabel is a dim row label with a hover card.
func (a *App) worldLabel(id string, r rect, text, tip string) {
	st := tsLabel
	a.uiLabel(r.X, r.Y, text, st)
	a.uiTip(id, r, tip)
}

// groupDigits writes n with thousands separators.
func groupDigits(n int) string {
	s := strconv.Itoa(n)
	out := ""
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out += ","
		}
		out += string(r)
	}
	return out
}

// ---- right column: factions ---------------------------------------------

func (a *App) worldRight(ng *newGameState, col rect) {
	a.uiPanel(col)
	in := col.inset(14)
	hs := tsLarge
	hs.Color = uiHeading
	a.uiLabel(in.X, in.Y, "Factions", hs)
	y := in.Y + float32(lineH(hs)) + 4
	sub := tsSmall
	sub.Color = uiTextDim
	a.uiLabel(in.X, y, "Who else lives on the planet. Each faction owns settlements; hostile ones send raids.", sub)
	y += float32(lineH(sub)) + 10

	slots := ng.world.Factions
	for i := range slots {
		s := &slots[i]
		d := faction.DefOf(s.Type)
		row := rect{in.X, y, in.W, worldFacRowH}
		a.worldFacRow(row, d, s, i)
		y += worldFacRowH + 4
		if len(ng.world.Factions) != len(slots) {
			break // a row was removed: the list is redeclared next frame
		}
	}
	if len(slots) == 0 {
		a.uiLabel(in.X+4, y+6, "No factions. The planet will be empty.", dimText())
		y += worldFacRowH
	}

	// The Swarm is always present.
	sw := rect{in.X, y, in.W, worldFacRowH}
	a.uiTip("world.fac.swarm", sw, "Always present\nHive swarms are part of every world. They are always hostile and have no settlements.")
	a.uiSt().queue(func(dst *ebiten.Image) {
		fillRect(dst, sw, uiField)
		strokeRect(dst, sw, uiPanelBorder)
		a.drawEmblem(dst, "ui_icon/faction_swarm", globeAccent(faction.Swarm), sw.X+8, sw.Y+(sw.H-24)/2, 24)
		st := tsLabel
		st.Color = uiTextDim
		drawText(dst, "Hive swarms", float64(sw.X+44), float64(sw.Y)+(float64(sw.H)-lineH(st))/2, st)
		dt := tsSmall
		dt.Color, dt.Align = uiTextDim, alignRight
		drawText(dst, "Always present", float64(sw.right()-10), float64(sw.Y)+(float64(sw.H)-lineH(dt))/2, dt)
	})
	y += worldFacRowH + 12

	// Add, reset.
	addable := addableTypes(ng.world.Factions)
	reason := ""
	if len(addable) == 0 {
		reason = "Every faction type is at its maximum."
	}
	a.ngButton("world.facAdd", rect{in.X, y, 150, 32}, "Add faction...", btnOpts{Disabled: reason != "", Reason: reason}, a.pushAddFaction)
	a.ngButton("world.facReset", rect{in.X + 158, y, 150, 32}, "Reset factions", btnOpts{Tip: "Reset factions\nBack to the default factions."},
		func() { ng.world.Factions = faction.DefaultSlots() })
	y += 32 + 12

	if !hostileCapable(ng.world.Factions) {
		w := tsBody
		w.Color = uiWarning
		for _, l := range wrapPx(noHostileWarning, w, float64(in.W)) {
			a.uiLabel(in.X, y, l, w)
			y += float32(lineH(w))
		}
	}
}

// noHostileWarning is shown when no faction can raid the colony.
const noHostileWarning = "No faction can raid you. Raids will be replaced by other threats."

func (a *App) worldFacRow(row rect, d faction.Def, s *faction.Slot, i int) {
	a.uiSt().queue(func(dst *ebiten.Image) {
		fillRect(dst, row, uiField)
		strokeRect(dst, row, uiPanelBorder)
		a.drawEmblem(dst, "ui_icon/faction_"+d.Key, globeAccent(d.Type), row.X+8, row.Y+(row.H-24)/2, 24)
		st := tsLabel
		drawText(dst, d.Label, float64(row.X+44), float64(row.Y)+5, st)
		ss := tsSmall
		ss.Color = uiTextDim
		drawText(dst, techLabel(d.Tech)+", up to "+strconv.Itoa(d.MaxCount), float64(row.X+44), float64(row.Y)+5+lineH(st), ss)
	})
	a.uiTip("world.fac."+d.Key+".tip", rect{row.X, row.Y, row.W - 190, row.H}, factionTypeTip(d))
	rm := rect{row.right() - 36, row.Y + 8, 28, 28}
	a.ngButton("world.fac."+d.Key+".rm", rm, "x", btnOpts{Tip: "Remove\nTake this faction type off the planet."}, func() {
		fs := a.draft().world.Factions
		a.draft().world.Factions = append(append([]faction.Slot(nil), fs[:i]...), fs[i+1:]...)
	})
	a.ngStepper("world.fac."+d.Key, rect{rm.X - 118, row.Y + 8, 112, 28}, &s.Count, 0, d.MaxCount)
}

// factionTypeTip explains a faction type on its row.
func factionTypeTip(d faction.Def) string {
	rel := "Starts neutral or friendly."
	switch {
	case d.Fixed && d.GoodwillMax < 0:
		rel = "Always hostile."
	case d.GoodwillMin <= -50:
		rel = "Can turn hostile and raid you."
	}
	return d.Label + "\n" + techLabel(d.Tech) + " people. " + rel
}

// pushAddFaction opens the "Add faction" list.
func (a *App) pushAddFaction() {
	ng := a.draft()
	types := addableTypes(ng.world.Factions)
	labels := make([]string, len(types))
	for i, t := range types {
		labels[i] = faction.DefOf(t).Label
	}
	a.pushPicker("world.facAdd.pick", "Add faction", labels, -1, func(i int) {
		t := types[i]
		if k := slotIndex(ng.world.Factions, t); k >= 0 {
			ng.world.Factions[k].Count++
			return
		}
		ng.world.Factions = append(ng.world.Factions, faction.Slot{Type: t, Count: 1})
	})
}

// ---- generation ---------------------------------------------------------

// worldGenerate is the forward button: Next when a matching planet exists,
// otherwise it starts generating behind the progress modal.
func (a *App) worldGenerate() {
	ng := a.draft()
	if a.uiTextEditing() {
		a.textEnd(true)
	}
	ng.world.SeedText = normalizeSeedText(ng.world.SeedText)
	if ng.worldReady() {
		a.forward()
		return
	}
	p := ng.world
	p.Factions = append([]faction.Slot(nil), ng.world.Factions...)
	ng.builder = game.NewPlanetBuilder(p)
	ng.genStep, ng.genPhase, ng.genFrac, ng.genErr = worldStepStart, planet.PhaseShape, 0, ""
	a.uiPushModal(modal{
		id: worldProgress, title: "Generating world", w: 520, h: 190,
		draw: func(a *App, body rect) { a.worldProgressBody(body) },
		buttons: []modalButton{
			{id: "cancel", label: "Cancel", closes: true, onClick: a.cancelWorld},
		},
		onClose: a.cancelWorld,
	})
}

// cancelWorld discards the builder in flight.
func (a *App) cancelWorld() {
	if ng := a.ng; ng != nil {
		ng.builder, ng.genErr = nil, ""
	}
}

func (a *App) worldProgressBody(body rect) {
	ng := a.draft()
	if ng.genErr != "" {
		w := tsBody
		w.Color = uiWarning
		y := body.Y + 4
		for _, l := range wrapPx(ng.genErr, w, float64(body.W)) {
			a.uiLabel(body.X, y, l, w)
			y += float32(lineH(w))
		}
		return
	}
	phase, frac := worldPhaseName(ng.genPhase), float64(ng.genFrac)
	r := rect{body.X, body.Y + 8, body.W, 44}
	a.uiSt().queue(func(dst *ebiten.Image) { drawProgress(dst, r, phase, frac, "") })
}

// worldStep advances the builder by this frame's work-unit budget. The
// units per frame adapt to the frame time; the planet does not depend on
// them (it is identical for every step size).
func (a *App) worldStep() {
	ng := a.ng
	if ng == nil || ng.builder == nil {
		return
	}
	if ng.genHold > 0 && ng.genFrac >= ng.genHold {
		return // a capture shows the generation part-way
	}
	t0 := a.now()
	phase, frac, done := ng.builder.Step(ng.genStep)
	ng.genPhase, ng.genFrac = phase, frac
	if a.cfg.Capture == nil && !t0.IsZero() {
		el := a.now().Sub(t0)
		switch {
		case el < worldFrameBud/2:
			ng.genStep = min(worldStepMax, ng.genStep*3/2)
		case el > worldFrameBud*3/2:
			ng.genStep = max(worldStepMin, ng.genStep*2/3)
		}
	}
	if !done {
		return
	}
	p := ng.builder.Planet()
	ng.builder = nil
	if p == nil || p.RandomLanding(0) < 0 {
		ng.genErr = "Couldn't make a habitable world with these settings. Try another seed."
		ng.genFrac = 1
		return
	}
	ng.planet, ng.planetKey = p, p.Params.Key()
	ng.tile, ng.landRolls = -1, 0
	ng.globe, ng.globeOf = nil, nil
	ng.pool, ng.poolKey = nil, ""
	a.uiCloseModal(worldProgress)
	a.forward()
}
