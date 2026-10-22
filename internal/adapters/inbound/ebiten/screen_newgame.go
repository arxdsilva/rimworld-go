package ebiten

import (
	"fmt"
	"image/color"
	"math/rand/v2"
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/world/biome"
)

var scenarios = []struct {
	name, desc, summary string
}{
	{
		name:    "Emergency landing",
		desc:    "A small crew survives a hard landing with leftover gear and a working fabricator.",
		summary: "3 colonists  |  mid-tech  |  modest supplies",
	},
	{
		name:    "Frontier camp",
		desc:    "A low-tech group arrives with tools, seeds, and little else.",
		summary: "5 colonists  |  low-tech  |  scarce supplies",
	},
	{
		name:    "Sole survivor",
		desc:    "One colonist, a wreck, and a long walk to safety.",
		summary: "1 colonist  |  mixed tech  |  hard start",
	},
}

var storytellers = []struct {
	name, desc string
}{
	{"Measured tide", "Incidents ramp as the colony grows. Predictable pressure."},
	{"Quiet years", "Fewer threats, more room to build. Forgiving pacing."},
	{"Wild dice", "Quiet stretches and sudden spikes. High variance."},
}

var difficulties = []string{"Forgiving", "Standard", "Harsh"}

var stubPawns = []string{
	"Mira   grower / cook     no violence incapacity",
	"Hal    constructor       modest shooting",
	"Rin    researcher        frail, good intellectual",
}

func (a *App) scenarioButtons(sw, sh int) []button {
	specs := make([]buttonSpec, 0, 5)
	for i, s := range scenarios {
		specs = append(specs, buttonSpec{
			id:       "scenario." + itoa(i),
			label:    s.name,
			selected: a.scenario == i,
			w:        260,
			h:        40,
		})
	}
	left := stackButtons(56, 120, 260, 40, 8, specs)
	left = append(left, button{
		id:       "scenario.edit",
		label:    "Create / edit custom",
		x:        56,
		y:        300,
		w:        260,
		h:        32,
		disabled: true,
		reason:   "not implemented",
	})
	return append(left, a.footer(sw, sh, "Next", "wizard.next")...)
}

func (a *App) drawScenario(screen *ebiten.Image, sw, sh int) {
	s := scenarios[a.scenario]
	a.paintMenu(screen, sw, sh, "New Game — Scenario", []string{
		s.name,
		s.desc,
		s.summary,
	}, a.scenarioButtons(sw, sh))
}

func (a *App) storytellerButtons(sw, sh int) []button {
	specs := make([]buttonSpec, 0, 6)
	for i, s := range storytellers {
		specs = append(specs, buttonSpec{
			id:       "storyteller." + itoa(i),
			label:    s.name,
			selected: a.storyteller == i,
			w:        280,
			h:        40,
		})
	}
	for i, name := range difficulties {
		specs = append(specs, buttonSpec{
			id:       "difficulty." + itoa(i),
			label:    "Difficulty: " + name,
			selected: a.difficulty == i,
			w:        280,
			h:        32,
		})
	}
	left := stackButtons(56, 120, 280, 40, 8, specs)
	left = append(left, button{
		id:       "storyteller.custom",
		label:    "Customize",
		x:        56,
		y:        420,
		w:        280,
		h:        32,
		disabled: true,
		reason:   "not implemented",
	})
	return append(left, a.footer(sw, sh, "Next", "wizard.next")...)
}

func (a *App) drawStoryteller(screen *ebiten.Image, sw, sh int) {
	s := storytellers[a.storyteller]
	a.paintMenu(screen, sw, sh, "Storyteller & Difficulty", []string{
		"Selected: " + s.name,
		s.desc,
		"",
		"Preset: " + difficulties[a.difficulty],
		"Custom threat/economy/mood sliders are not wired yet.",
	}, a.storytellerButtons(sw, sh))
}

// wizardState is what the New Game wizard has chosen so far.
type wizardState struct {
	seed     uint64
	seedText string // the seed field's text while it is edited
	editing  bool   // the seed field has the keyboard
	biome    biome.ID
	biomes   []game.BiomeInfo
}

// maxSeedDigits keeps the typed seed inside a uint64.
const maxSeedDigits = 18

func newWizardState(cfg AppConfig) wizardState {
	w := wizardState{seed: cfg.DefaultSeed, biome: cfg.DefaultBiome, biomes: game.Biomes()}
	if w.seed == 0 {
		w.seed = randomSeed()
	}
	w.seedText = strconv.FormatUint(w.seed, 10)
	return w
}

// randomSeed is a fresh non-zero seed for the wizard's dice. The wizard
// runs before any simulation exists, so presentation randomness is fine.
func randomSeed() uint64 {
	return rand.Uint64N(1_000_000_000) + 1
}

// commitSeed turns the field's text into the seed; an empty field keeps
// the previous seed.
func (w *wizardState) commitSeed() {
	w.editing = false
	if n, err := strconv.ParseUint(w.seedText, 10, 64); err == nil && n > 0 {
		w.seed = n
	}
	w.seedText = strconv.FormatUint(w.seed, 10)
}

func (a *App) worldGenButtons(sw, sh int) []button {
	label := "Seed: " + a.wiz.seedText
	if a.wiz.editing {
		label += "_"
	}
	btns := a.footer(sw, sh, "Next", "wizard.next")
	btns = append(btns,
		button{id: "world.seed", label: label, x: 56, y: 120, w: 260, h: 36, selected: a.wiz.editing},
		button{id: "world.dice", label: "Dice", x: 324, y: 120, w: 90, h: 36},
		button{id: "world.generate", label: "Generate", x: 196, y: footerY(sh), w: 140, h: 36},
	)
	return btns
}

func (a *App) drawWorldGen(screen *ebiten.Image, sw, sh int) {
	a.paintMenu(screen, sw, sh, "World Generation", []string{
		"The seed fixes the map: the same seed and biome always give the same colony site.",
		"Click the seed to type digits (Enter to accept); Dice picks a new one.",
		"",
		"Coverage: small  |  Temperature and rainfall follow the biome you land in",
		"Generate builds from this seed and goes on to choose the biome.",
	}, a.worldGenButtons(sw, sh))
}

// handleSeedInput feeds typed digits to the seed field while it is edited
// (the text context: Enter or Esc accept, Backspace deletes).
func (a *App) handleSeedInput() {
	if a.nav.screen() != screenWorldGen || !a.wiz.editing {
		return
	}
	for _, r := range a.in.raw.chars {
		if r >= '0' && r <= '9' && len(a.wiz.seedText) < maxSeedDigits {
			a.wiz.seedText += string(r)
		}
	}
	if a.in.key("Backspace") && a.wiz.seedText != "" {
		a.wiz.seedText = a.wiz.seedText[:len(a.wiz.seedText)-1]
	}
	if a.in.key("Enter") || a.in.key("NumpadEnter") || a.in.key("Escape") {
		a.wiz.commitSeed()
	}
}

// applyWizardClick handles the world and landing screens' buttons; it
// reports whether id was one of them.
func (a *App) applyWizardClick(id string) bool {
	switch {
	case id == "world.seed":
		a.wiz.editing = true
		a.wiz.seedText = ""
	case id == "world.dice":
		a.wiz.seed = randomSeed()
		a.wiz.seedText = strconv.FormatUint(a.wiz.seed, 10)
		a.wiz.editing = false
	case id == "world.generate":
		a.wiz.commitSeed()
		a.apply(intent{kind: intentPush, screen: screenLanding})
	case id == "wizard.next" || id == "wizard.start" || id == "wizard.back":
		a.wiz.commitSeed() // a half-typed seed counts
		return false
	case strings.HasPrefix(id, "landing.biome."):
		if n, err := strconv.Atoi(strings.TrimPrefix(id, "landing.biome.")); err == nil && n >= 0 && n < len(a.wiz.biomes) {
			a.wiz.biome = a.wiz.biomes[n].ID
		}
	default:
		return false
	}
	return true
}

const (
	landingListX  = 56
	landingListY  = 112
	landingListW  = 270
	landingRowH   = 30
	landingRowGap = 4
)

func (a *App) landingButtons(sw, sh int) []button {
	specs := make([]buttonSpec, 0, len(a.wiz.biomes))
	for i, b := range a.wiz.biomes {
		specs = append(specs, buttonSpec{
			id:       "landing.biome." + strconv.Itoa(i),
			label:    "    " + b.Name, // room for the biome icon drawn at the left
			selected: a.wiz.biome == b.ID,
			w:        landingListW,
			h:        landingRowH,
		})
	}
	btns := stackButtons(landingListX, landingListY, landingListW, landingRowH, landingRowGap, specs)
	return append(btns, a.footer(sw, sh, "Next", "wizard.next")...)
}

// biomeInfo is the picker card of the chosen biome.
func (a *App) biomeInfo() game.BiomeInfo {
	for _, b := range a.wiz.biomes {
		if b.ID == a.wiz.biome {
			return b
		}
	}
	return game.BiomeInfoOf(a.wiz.biome)
}

// landingDetail is the picker's detail panel text for a biome card.
func landingDetail(b game.BiomeInfo, cols int) []string {
	title := b.Name
	if b.Difficulty != "" {
		title += "  [" + b.Difficulty + "]"
	}
	out := []string{title, ""}
	out = append(out, wrapText(b.Blurb, cols)...)
	out = append(out, "",
		fmt.Sprintf("Temperature  %dC winter low, %dC summer high", b.WinterLow, b.SummerHigh),
		fmt.Sprintf("Growing      about %d days a year", b.GrowingDays),
	)
	out = append(out, wrapLabeled("Ground", b.Ground, cols)...)
	out = append(out, wrapLabeled("Plants", b.Plants, cols)...)
	out = append(out, wrapLabeled("Wildlife", b.Wildlife, cols)...)
	return out
}

// wrapLabeled is "Label      a, b, c" wrapped under its label column.
func wrapLabeled(label string, items []string, cols int) []string {
	const lw = 13
	text := "none"
	if len(items) > 0 {
		text = strings.Join(items, ", ")
	}
	rows := wrapText(text, cols-lw)
	for i := range rows {
		if i == 0 {
			rows[i] = fmt.Sprintf("%-*s%s", lw, label, rows[i])
		} else {
			rows[i] = strings.Repeat(" ", lw) + rows[i]
		}
	}
	return rows
}

func (a *App) drawLanding(screen *ebiten.Image, sw, sh int) {
	a.fillMenu(screen, sw, sh, "Landing Site - choose where to land", []string{""}, 0)
	btns := a.landingButtons(sw, sh)
	mx, my := a.currentCursor()
	a.drawButtons(screen, btns, mx, my)
	for i, b := range a.wiz.biomes {
		r := btns[i]
		if img := a.art.image(b.Icon, color.NRGBA{}); img != nil {
			drawIcon(screen, img, float64(r.x)+4, float64(r.y)+3, landingRowH-6)
		}
		if b.Difficulty != "" {
			drawLabel(screen, int(r.x+r.w)-6*(len(b.Difficulty)+2)-4, int(r.y)+9, "("+b.Difficulty+")")
		}
	}
	x := landingListX + landingListW + 24
	w := sw - 40 - x - 12
	drawPanel(screen, float32(x-12), landingListY, float32(w+12), float32(sh-landingListY-96))
	cols := max(20, (w-12)/6)
	if img := a.art.image(a.biomeInfo().Icon, color.NRGBA{}); img != nil {
		drawIcon(screen, img, float64(x+w-64), landingListY+10, 56)
	}
	y := landingListY + 12
	for _, line := range landingDetail(a.biomeInfo(), cols-12) {
		drawLabel(screen, x, y, line)
		y += 16
	}
	drawLabel(screen, x, sh-96-24, fmt.Sprintf("Seed %d", a.wiz.seed))
}

func (a *App) pawnButtons(sw, sh int) []button {
	btns := stackButtons(56, 120, 140, 32, 8, []buttonSpec{
		{id: "pawns.reroll", label: "Reroll", disabled: true, reason: "not implemented"},
		{id: "pawns.rename", label: "Rename", disabled: true, reason: "not implemented"},
	})
	return append(btns, a.footer(sw, sh, "Start", "wizard.start")...)
}

func (a *App) drawPawns(screen *ebiten.Image, sw, sh int) {
	body := []string{
		"Candidate roster (placeholder; not the live simulation):",
		"",
	}
	body = append(body, stubPawns...)
	body = append(body, "", "Team warnings will flag missing doctor/cook/grower later.")
	if a.sessionErr != "" {
		body = append(body, "", "start failed: "+a.sessionErr)
	}
	a.paintMenu(screen, sw, sh, "Starting Pawns", body, a.pawnButtons(sw, sh))
}

func itoa(i int) string {
	return string(rune('0' + i))
}
