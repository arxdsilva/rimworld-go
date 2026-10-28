package wizard

import (
	"fmt"
	"strconv"
	"time"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/globe"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/world/planet"
)

// Create World (NG-1 §2.6): the planet's seed, coverage, rainfall,
// temperature, population and faction list, then a progress modal that
// steps planet.Builder a fixed budget of work units per frame.

// Layout metrics.
const (
	worldRowGap   = 8
	worldFieldH   = 34
	worldSliderH  = 36
	worldLabelH   = 20
	worldFacRowH  = 44
	WorldProgress = "world.progress"
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
func (w worldPage) coverageOptions() []int {
	if w.sh.Env().DevMode || w.draft().World.Coverage == 5 {
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

// WorldPhaseName is the progress modal's phase text.
func WorldPhaseName(p planet.Phase) string {
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

// WorldReady reports a generated planet matches the draft's settings.
func (ng *Draft) WorldReady() bool {
	return ng.Planet != nil && ng.PlanetKey == ng.World.Key()
}

func (w worldPage) worldGenUI(sw, sh int) {
	uikit.OnScreenLayer(w.kit())
	if w.kit().SetupTooSmall(sw, sh) {
		return
	}
	ng := w.draft()
	_, _, content, footer := uikit.DialogFrame(sw, sh)

	// A planet made from other settings is stale: its landing tile goes.
	if ng.Planet != nil && ng.PlanetKey != ng.World.Key() {
		ng.Tile = -1
	}
	w.worldStep()

	cols := content.Columns(2, uikit.DialogPad)
	w.worldLeft(ng, cols[0])
	w.worldRight(ng, cols[1])

	label, tip := "Generate", ""
	if ng.WorldReady() {
		label = "Next"
	} else if ng.Planet != nil {
		tip = "These settings differ from the world you made. Generate it again."
	}
	uikit.StyledButton(w.kit(), "world.resetAll", gfx.Rect{X: uikit.FooterBack(footer).Right() + 12, Y: uikit.FooterBack(footer).Y, W: 120, H: uikit.FooterBtnH}, "Reset all",
		uikit.BtnOpts{Tip: "Reset all\nPut every world setting back to its default and roll a new seed."}, func() {
			w.kit().Confirm("world.resetAll.confirm", "Reset all world settings?", "Every setting on this screen goes back to its default and the seed is replaced.", "Reset", true, w.resetWorldAll)
		})
	w.ngFooter(footer, label, "world.generate", uikit.BtnOpts{Tip: tip}, w.worldGenerate)
}

// resetWorldAll restores the world settings of a fresh draft with a new seed.
func (w worldPage) resetWorldAll() {
	ng := w.draft()
	ng.World = planet.Params{SeedText: RandomWorldPhrase(), Coverage: DefaultCoverage, Rain: 2, Temp: 2, Pop: 1,
		Factions: faction.DefaultSlots()}
}

// ---- left column: the planet --------------------------------------------

func (w worldPage) worldLeft(ng *Draft, col gfx.Rect) {
	w.kit().Panel(col)
	in := col.Inset(14)
	hs := gfx.Large
	hs.Color = uikit.Heading
	w.kit().Label(in.X, in.Y, "Planet", hs)
	y := in.Y + float32(gfx.LineH(hs)) + 10

	// Seed.
	w.worldLabel("world.seed.lbl", gfx.Rect{X: in.X, Y: y, W: in.W, H: worldLabelH}, "Seed",
		"Seed\nThe same seed and settings always make the same world. Any text of 1 to 32 characters.")
	y += worldLabelH
	diceW := float32(80)
	w.kit().TextField("world.seed", gfx.Rect{X: in.X, Y: y, W: in.W - diceW - 8, H: worldFieldH}, &ng.World.SeedText, uikit.TextFieldSpec{
		MaxLen: MaxSeedText, Valid: SeedTextRune, Normalize: normalizeSeedText, Placeholder: "seed",
		Tip: "Seed\nLetters, digits, space, - and _ (1 to 32 characters).",
	})
	uikit.StyledButton(w.kit(), "world.dice", gfx.Rect{X: in.Right() - diceW, Y: y, W: diceW, H: worldFieldH}, "Dice",
		uikit.BtnOpts{Tip: "Randomize seed\nReplace the seed with a random phrase. It does not generate."},
		func() { ng.World.SeedText = RandomWorldPhrase() })
	y += worldFieldH + worldRowGap + 6

	// Coverage.
	opts := w.coverageOptions()
	idx := 0
	for i, v := range opts {
		if v == ng.World.Coverage {
			idx = i
		}
	}
	w.worldLabel("world.coverage.lbl", gfx.Rect{X: in.X, Y: y, W: in.W, H: worldLabelH}, "Globe coverage",
		"Globe coverage\nHow much of the planet is generated. Bigger worlds take longer. This is not your colony's map size.")
	y += worldLabelH
	if w.sliderInt("world.coverage", gfx.Rect{X: in.X, Y: y, W: in.W, H: worldSliderH}, &idx, uikit.SliderSpec{Stops: pctLabels(opts), Title: "Globe coverage"}) {
		ng.World.Coverage = opts[idx]
	}
	y += worldSliderH + worldRowGap + 6

	// Rainfall, temperature, population.
	for _, s := range []struct {
		id, label, tip string
		stops          []string
		v              *int
	}{
		{"world.rain", "Rainfall", "Rainfall\nHow wet the planet is. Dry worlds have more deserts, wet ones more swamps and jungle.", rainStops, &ng.World.Rain},
		{"world.temp", "Temperature", "Temperature\nHow warm the planet is overall. Cold worlds have larger ice caps.", tempStops, &ng.World.Temp},
		{"world.pop", "Population", "Population\nHow many other settlements exist. Doesn't change your colonists or raid sizes.", popStops, &ng.World.Pop},
	} {
		w.worldLabel(s.id+".lbl", gfx.Rect{X: in.X, Y: y, W: in.W, H: worldLabelH}, s.label, s.tip)
		y += worldLabelH
		w.sliderInt(s.id, gfx.Rect{X: in.X, Y: y, W: in.W, H: worldSliderH}, s.v, uikit.SliderSpec{Stops: s.stops, Title: s.label})
		y += worldSliderH + worldRowGap + 6
	}

	// Status.
	st := uikit.DimText()
	w.kit().Label(in.X, y, fmt.Sprintf("About %s tiles at this coverage.", groupDigits(worldTiles(ng.World.Coverage))), st)
	y += float32(gfx.LineH(st)) + 4
	switch {
	case ng.WorldReady():
		ok := gfx.Body
		ok.Color = uikit.Good
		w.kit().Label(in.X, y, "A world with these settings is ready. Next keeps it.", ok)
	case ng.Planet != nil:
		ws := gfx.Body
		ws.Color = uikit.Warning
		w.kit().Label(in.X, y, "Settings changed since the last world. Generate it again.", ws)
	}
}

// worldLabel is a dim row label with a hover card.
func (w worldPage) worldLabel(id string, r gfx.Rect, text, tip string) {
	st := gfx.Label
	w.kit().Label(r.X, r.Y, text, st)
	w.kit().Tip(id, r, tip)
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

func (w worldPage) worldRight(ng *Draft, col gfx.Rect) {
	w.kit().Panel(col)
	in := col.Inset(14)
	hs := gfx.Large
	hs.Color = uikit.Heading
	w.kit().Label(in.X, in.Y, "Factions", hs)
	y := in.Y + float32(gfx.LineH(hs)) + 4
	sub := gfx.Small
	sub.Color = uikit.TextDim
	w.kit().Label(in.X, y, "Who else lives on the planet. Each faction owns settlements; hostile ones send raids.", sub)
	y += float32(gfx.LineH(sub)) + 10

	slots := ng.World.Factions
	for i := range slots {
		s := &slots[i]
		d := faction.DefOf(s.Type)
		row := gfx.Rect{X: in.X, Y: y, W: in.W, H: worldFacRowH}
		w.worldFacRow(row, d, s, i)
		y += worldFacRowH + 4
		if len(ng.World.Factions) != len(slots) {
			break // a row was removed: the list is redeclared next frame
		}
	}
	if len(slots) == 0 {
		w.kit().Label(in.X+4, y+6, "No factions. The planet will be empty.", uikit.DimText())
		y += worldFacRowH
	}

	// The Swarm is always present.
	sw := gfx.Rect{X: in.X, Y: y, W: in.W, H: worldFacRowH}
	w.kit().Tip("world.fac.swarm", sw, "Always present\nHive swarms are part of every world. They are always hostile and have no settlements.")
	w.kit().Queue(func(dst *ebiten.Image) {
		uikit.FillRect(dst, sw, uikit.Field)
		uikit.StrokeRect(dst, sw, uikit.PanelBorder)
		w.DrawEmblem(dst, "ui_icon/faction_swarm", globe.Accent(faction.Swarm), sw.X+8, sw.Y+(sw.H-24)/2, 24)
		st := gfx.Label
		st.Color = uikit.TextDim
		gfx.Text(dst, "Hive swarms", float64(sw.X+44), float64(sw.Y)+(float64(sw.H)-gfx.LineH(st))/2, st)
		dt := gfx.Small
		dt.Color, dt.Align = uikit.TextDim, gfx.AlignRight
		gfx.Text(dst, "Always present", float64(sw.Right()-10), float64(sw.Y)+(float64(sw.H)-gfx.LineH(dt))/2, dt)
	})
	y += worldFacRowH + 12

	// Add, reset.
	addable := AddableTypes(ng.World.Factions)
	reason := ""
	if len(addable) == 0 {
		reason = "Every faction type is at its maximum."
	}
	uikit.StyledButton(w.kit(), "world.facAdd", gfx.Rect{X: in.X, Y: y, W: 150, H: 32}, "Add faction...", uikit.BtnOpts{Disabled: reason != "", Reason: reason}, w.pushAddFaction)
	uikit.StyledButton(w.kit(), "world.facReset", gfx.Rect{X: in.X + 158, Y: y, W: 150, H: 32}, "Reset factions", uikit.BtnOpts{Tip: "Reset factions\nBack to the default factions."},
		func() { ng.World.Factions = faction.DefaultSlots() })
	y += 32 + 12

	if !HostileCapable(ng.World.Factions) {
		ws := gfx.Body
		ws.Color = uikit.Warning
		for _, l := range gfx.WrapPx(NoHostileWarning, ws, float64(in.W)) {
			w.kit().Label(in.X, y, l, ws)
			y += float32(gfx.LineH(ws))
		}
	}
}

// NoHostileWarning is shown when no faction can raid the colony.
const NoHostileWarning = "No faction can raid you. Raids will be replaced by other threats."

func (w worldPage) worldFacRow(row gfx.Rect, d faction.Def, s *faction.Slot, i int) {
	w.kit().Queue(func(dst *ebiten.Image) {
		uikit.FillRect(dst, row, uikit.Field)
		uikit.StrokeRect(dst, row, uikit.PanelBorder)
		w.DrawEmblem(dst, "ui_icon/faction_"+d.Key, globe.Accent(d.Type), row.X+8, row.Y+(row.H-24)/2, 24)
		st := gfx.Label
		gfx.Text(dst, d.Label, float64(row.X+44), float64(row.Y)+5, st)
		ss := gfx.Small
		ss.Color = uikit.TextDim
		gfx.Text(dst, techLabel(d.Tech)+", up to "+strconv.Itoa(d.MaxCount), float64(row.X+44), float64(row.Y)+5+gfx.LineH(st), ss)
	})
	w.kit().Tip("world.fac."+d.Key+".tip", gfx.Rect{X: row.X, Y: row.Y, W: row.W - 190, H: row.H}, factionTypeTip(d))
	rm := gfx.Rect{X: row.Right() - 36, Y: row.Y + 8, W: 28, H: 28}
	uikit.StyledButton(w.kit(), "world.fac."+d.Key+".rm", rm, "x", uikit.BtnOpts{Tip: "Remove\nTake this faction type off the planet."}, func() {
		fs := w.draft().World.Factions
		w.draft().World.Factions = append(append([]faction.Slot(nil), fs[:i]...), fs[i+1:]...)
	})
	uikit.Stepper(w.kit(), "world.fac."+d.Key, gfx.Rect{X: rm.X - 118, Y: row.Y + 8, W: 112, H: 28}, &s.Count, 0, d.MaxCount)
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
func (w worldPage) pushAddFaction() {
	ng := w.draft()
	types := AddableTypes(ng.World.Factions)
	labels := make([]string, len(types))
	for i, t := range types {
		labels[i] = faction.DefOf(t).Label
	}
	uikit.Picker(w.kit(), "world.facAdd.pick", "Add faction", labels, -1, func(i int) {
		t := types[i]
		if k := SlotIndex(ng.World.Factions, t); k >= 0 {
			ng.World.Factions[k].Count++
			return
		}
		ng.World.Factions = append(ng.World.Factions, faction.Slot{Type: t, Count: 1})
	})
}

// ---- generation ---------------------------------------------------------

// worldGenerate is the forward button: Next when a matching planet exists,
// otherwise it starts generating behind the progress modal.
func (w worldPage) worldGenerate() {
	ng := w.draft()
	if w.kit().Editing() {
		w.kit().TextEnd(true)
	}
	ng.World.SeedText = normalizeSeedText(ng.World.SeedText)
	if ng.WorldReady() {
		w.forward()
		return
	}
	p := ng.World
	p.Factions = append([]faction.Slot(nil), ng.World.Factions...)
	ng.Builder = game.NewPlanetBuilder(p)
	ng.GenStep, ng.GenPhase, ng.GenFrac, ng.genErr = worldStepStart, planet.PhaseShape, 0, ""
	w.kit().Push(uikit.Modal{
		ID: WorldProgress, Title: "Generating world", W: 520, H: 190,
		Draw: func(body gfx.Rect) { w.worldProgressBody(body) },
		Buttons: []uikit.ModalButton{
			{ID: "cancel", Label: "Cancel", Closes: true, OnClick: w.cancelWorld},
		},
		OnClose: w.cancelWorld,
	})
}

// cancelWorld discards the builder in flight.
func (w worldPage) cancelWorld() {
	if ng := w.d; ng != nil {
		ng.Builder, ng.genErr = nil, ""
	}
}

func (w worldPage) worldProgressBody(body gfx.Rect) {
	ng := w.draft()
	if ng.genErr != "" {
		ws := gfx.Body
		ws.Color = uikit.Warning
		y := body.Y + 4
		for _, l := range gfx.WrapPx(ng.genErr, ws, float64(body.W)) {
			w.kit().Label(body.X, y, l, ws)
			y += float32(gfx.LineH(ws))
		}
		return
	}
	phase, frac := WorldPhaseName(ng.GenPhase), float64(ng.GenFrac)
	r := gfx.Rect{X: body.X, Y: body.Y + 8, W: body.W, H: 44}
	w.kit().Queue(func(dst *ebiten.Image) { uikit.DrawProgress(dst, r, phase, frac, "") })
}

// worldStep advances the builder by this frame's work-unit budget. The
// units per frame adapt to the frame time; the planet does not depend on
// them (it is identical for every step size).
func (w worldPage) worldStep() {
	ng := w.d
	if ng == nil || ng.Builder == nil {
		return
	}
	if ng.GenHold > 0 && ng.GenFrac >= ng.GenHold {
		return // a capture shows the generation part-way
	}
	t0 := w.sh.Env().Now
	phase, frac, done := ng.Builder.Step(ng.GenStep)
	ng.GenPhase, ng.GenFrac = phase, frac
	if !w.sh.Env().Capture && !t0.IsZero() {
		el := w.sh.Env().Now.Sub(t0)
		switch {
		case el < worldFrameBud/2:
			ng.GenStep = min(worldStepMax, ng.GenStep*3/2)
		case el > worldFrameBud*3/2:
			ng.GenStep = max(worldStepMin, ng.GenStep*2/3)
		}
	}
	if !done {
		return
	}
	p := ng.Builder.Planet()
	ng.Builder = nil
	if p == nil || p.RandomLanding(0) < 0 {
		ng.genErr = "Couldn't make a habitable world with these settings. Try another seed."
		ng.GenFrac = 1
		return
	}
	ng.Planet, ng.PlanetKey = p, p.Params.Key()
	ng.Tile, ng.LandRolls = -1, 0
	ng.Globe, ng.globeOf = nil, nil
	ng.Pool, ng.PoolKey = nil, ""
	w.kit().CloseModal(WorldProgress)
	w.forward()
}
