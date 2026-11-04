package options

import (
	"strconv"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/domain/gameplay"
	"rimworld-go/internal/domain/sim/command"
)

// Options › Habits (CSE-049, CSE-023): the colony's habit rules, one check
// per switch with a one-line tooltip and a stepper per limit, shown from
// the read model (Story.Rules) and changed only by the typed commands
// SetGameplayToggle and SetGameplayLimit. The rules belong to the colony and
// live in its save, so the tab has controls only while a colony runs.

const (
	habitsID      = "opt.habits"
	habitsDefault = "opt.habits.defaults"
	habitHeadH    = 26
	habitRowH     = 28
	habitStepRowH = 32
	habitGroupGap = 8
	habitBtnW     = 28
	habitValW     = 84
)

// habitToggle is one switch row: its label and what it does.
type habitToggle struct {
	T          gameplay.Toggle
	Label, Tip string
}

// habitLimit is one number row.
type habitLimit struct {
	L          gameplay.Limit
	Label, Tip string
	Zero       string // what a value of 0 reads as ("" shows the number)
}

// habitGroup is a heading with its toggle rows and, for the tidying group,
// its limit rows.
type habitGroup struct {
	Title   string
	Toggles []habitToggle
	Limits  []habitLimit
}

// habitGroups are the tab's sections, in order. Every toggle appears once
// (TestHabitsRowsCoverEveryToggle).
func habitGroups() []habitGroup {
	return []habitGroup{
		{Title: "Free time", Toggles: []habitToggle{
			{gameplay.OutdoorsJoy, "Seek the outdoors for fun", "Colonists whose recreation runs low look for fun under open sky first."},
			{gameplay.JoyTopOff, "Top up recreation sooner", "Once a colonist has had a good break, the wish for another comes back at a lower level."},
			{gameplay.RelaxDrink, "Finish a drink while relaxing", "A drink taken to unwind is carried along and finished once it will not be wasted."},
			{gameplay.ShelterWander, "Wander under a roof in bad weather", "When it is unpleasant outside, idle colonists stroll indoors instead."},
			{gameplay.PoliteWander, "Stay out of private rooms when idle", "Idle colonists avoid bedrooms they do not own, hospitals, cells and cold stores."},
		}},
		{Title: "Food and drugs", Toggles: []habitToggle{
			{gameplay.SpareMeal, "Carry a spare meal", "A colonist with a free moment and a full-enough belly picks up a meal for later."},
			{gameplay.SeparateMeals, "Keep tainted meals apart", "Meals made with disliked ingredients no longer stack with clean ones."},
			{gameplay.OddMeatNormal, "Treat insect meat as ordinary", "Meals with insect meat stack with the rest instead of counting as tainted."},
			{gameplay.FlavourIngredients, "Give plain meals ingredients", "Meals not made at a bench get a plausible ingredient list."},
			{gameplay.SpoilingMeals, "Eat the meal that spoils first", "Food that will go off within a few days is preferred when choosing a meal."},
			{gameplay.DrugPotentialMood, "Hold doses while mood recovers", "A scheduled drug with a mood limit is refused when the mood is on its way up."},
			{gameplay.AnyDrugFromPack, "Take drugs from the pack", "A colonist may use a dose it is carrying before looking around the colony."},
			{gameplay.SickGifts, "Bring comfort to the sick", "A visit to a sick friend may become feeding them a treat."},
		}},
		{Title: "Tidying", Toggles: []habitToggle{
			{gameplay.CleanBeforeWork, "Clean before working", "Colonists sweep up the filth around a workplace before they start, and after delivering food."},
			{gameplay.CleanBeforeBill, "Clean before crafting", "Between gathering ingredients and crafting, the area around the bench is cleaned."},
			{gameplay.BenchCleanToggle, "Let each bench opt out of cleaning", "Adds a per-bench switch; benches switched off are never cleaned around."},
			{gameplay.CleanBeforeMeal, "Clean before eating", "Colonists tidy around their seat before they sit down to eat or relax."},
			{gameplay.CleanAfterTend, "Clean after tending", "A doctor cleans the room once a patient is treated, unless another patient needs care."},
			{gameplay.FullPathCheck, "Judge detours by real walking", "Decides whether a cleaning detour is worth it by the true path, not a straight line."},
			{gameplay.SnowWaitsForSky, "Wait out snowfall before shovelling", "Nobody clears snow while it is still falling."},
		}, Limits: []habitLimit{
			{gameplay.TidyStation, "Cells cleaned before work", "The most filthy cells a colonist cleans before starting work or after a delivery.", ""},
			{gameplay.TidyBench, "Cells cleaned before crafting", "The most filthy cells cleaned around a bench before the bill starts.", ""},
			{gameplay.TidyTend, "Cells cleaned after tending", "The most filthy cells a doctor cleans after a treatment. Unlimited cleans the whole room.", "Unlimited"},
		}},
		{Title: "Hauling and crafting", Toggles: []habitToggle{
			{gameplay.HaulAllIngredients, "Pack every ingredient first", "Crafters collect all the ingredients into their pack, then lay them out at the bench."},
			{gameplay.RespectCapacity, "Respect what a colonist can carry", "Collecting stops when the pack is full; the rest is fetched on another trip."},
			{gameplay.SpoilingIngredients, "Use ingredients that spoil first", "Bills pick the ingredients closest to going off, wherever they are stored."},
			{gameplay.StackToStockpile, "Stock a stack before a bill", "If its stockpile is nearer than the bench, a far-off stack is stored first."},
			{gameplay.PutBack, "Return carried things to the pack", "Something taken from the pack goes back into it when the job is interrupted."},
			{gameplay.ManualUnload, "Unload by hand", "Shows an unload button beside each carried item on the Gear tab."},
			{gameplay.FullRecipeList, "Name the ingredients in recipes", "Recipe summaries list the categories and items they accept, not a vague word."},
		}},
	}
}

// habitsContentH is the scroll content's height: headings, rows and gaps.
func habitsContentH() float32 {
	h := float32(0)
	for _, g := range habitGroups() {
		h += habitHeadH + float32(len(g.Toggles))*habitRowH + float32(len(g.Limits))*habitStepRowH + habitGroupGap
	}
	return h + 4
}

// habitsTab declares the tab: the disabled notice outside a colony, else
// the defaults button above the scrolling rows.
func (o *Screen) habitsTab(sw, sh int) {
	k := o.sh.Kit()
	panel := gfx.Rect{X: 40, Y: 96, W: float32(sw - 80), H: float32(sh - 180)}
	top := float32(optTabY + OptBtnH + 16)
	c := gfx.Rect{X: panel.X + 16, Y: top, W: panel.W - 32, H: panel.Bottom() - top - 16}
	if !o.sh.Env().Running {
		uikit.TextLines(k, c.X, c.Y, c.W, "Habits are rules of a colony and are saved with it, so they can only be changed while a colony is running. Open Options from the pause menu in a game to change them.", uikit.DimText())
		return
	}
	rules := o.sh.Rules()
	uikit.TextLines(k, c.X, c.Y+6, c.W-190, "How your colonists go about their day. Changes apply at once and are saved with this colony.", uikit.DimText())
	isDefault := o.habitsAreDefault(rules)
	uikit.StyledButton(k, habitsDefault, gfx.Rect{X: c.Right() - 170, Y: c.Y, W: 170, H: 28}, "Back to defaults",
		uikit.BtnOpts{Disabled: isDefault, Reason: "Every habit already has its default value.",
			Tip: "Back to defaults\nPut every habit and limit back to how a new colony starts."},
		func() { o.habitsReset(o.sh.Rules()) })

	area := k.ScrollArea(habitsID+".scroll", gfx.Rect{X: c.X, Y: c.Y + 36, W: c.W, H: c.H - 36}, habitsContentH(), uikit.ScrollOpts{Step: 40, Focusable: true})
	f := uikit.NewForm(area)
	for _, g := range habitGroups() {
		f.SectionTitle(o.sh, g.Title)
		for _, t := range g.Toggles {
			o.habitCheck(f, rules, t)
		}
		for _, l := range g.Limits {
			o.habitStepper(f, rules, l)
		}
		f.Gap(habitGroupGap)
	}
}

// habitsAreDefault reports that nothing differs from a new colony's rules.
func (o *Screen) habitsAreDefault(r gameplay.Settings) bool {
	return r == gameplay.Settings{}
}

// habitsReset sends the commands that put every changed habit back.
func (o *Screen) habitsReset(r gameplay.Settings) {
	for _, t := range gameplay.AllToggles() {
		if r.On(t) != gameplay.ToggleDefault(t) {
			o.setToggle(t, gameplay.ToggleDefault(t))
		}
	}
	for l := gameplay.Limit(0); l < gameplay.NumLimits; l++ {
		if r.LimitOf(l) != gameplay.LimitDefault(l) {
			o.setLimit(l, gameplay.LimitDefault(l))
		}
	}
}

// setToggle and setLimit are the tab's typed commands, applied at once: the
// colony is not stepping while Options is open.
func (o *Screen) setToggle(t gameplay.Toggle, on bool) {
	o.sh.Send(command.Command{Kind: command.SetGameplayToggle, Index: int(t), Enabled: on})
}

func (o *Screen) setLimit(l gameplay.Limit, n int) {
	o.sh.Send(command.Command{Kind: command.SetGameplayLimit, Index: int(l), Count: max(0, min(n, gameplay.MaxLimit))})
}

// habitCheck is one toggle row. The box shows the colony's value; a click
// sends the opposite, whether the kit toggled its copy this frame or on the
// next one (an id-driven click).
func (o *Screen) habitCheck(f *uikit.Form, rules gameplay.Settings, t habitToggle) {
	row, ok := f.Row(habitRowH)
	if !ok {
		return
	}
	on := rules.On(t.T)
	shown := on
	id := habitsID + ".t" + strconv.Itoa(int(t.T))
	if o.sh.Kit().Check(id, row, t.Label, &shown, uikit.CheckOpts{Tip: t.Label + "\n" + t.Tip}) {
		o.setToggle(t.T, !on)
	}
}

// habitStepper is one limit row: - value + at the right, clamped to
// 0..MaxLimit.
func (o *Screen) habitStepper(f *uikit.Form, rules gameplay.Settings, l habitLimit) {
	row, ok := f.Row(habitStepRowH)
	if !ok {
		return
	}
	k := o.sh.Kit()
	v := rules.LimitOf(l.L)
	id := habitsID + ".l" + strconv.Itoa(int(l.L))
	k.Label(row.X, row.Y+(row.H-float32(gfx.LineH(gfx.Body)))/2, l.Label, gfx.Body)
	k.Tip(id+".tip", gfx.Rect{X: row.X, Y: row.Y, W: row.W - 2*habitBtnW - habitValW - 8, H: row.H}, l.Label+"\n"+l.Tip)
	x := row.Right() - 2*habitBtnW - habitValW
	by := row.Y + 2
	bh := row.H - 4
	uikit.StyledButton(k, id+".dec", gfx.Rect{X: x, Y: by, W: habitBtnW, H: bh}, "-",
		uikit.BtnOpts{Disabled: v <= 0, Reason: "The minimum is 0."}, func() { o.setLimit(l.L, v-1) })
	uikit.StyledButton(k, id+".inc", gfx.Rect{X: x + habitBtnW + habitValW, Y: by, W: habitBtnW, H: bh}, "+",
		uikit.BtnOpts{Disabled: v >= gameplay.MaxLimit, Reason: "The maximum is " + strconv.Itoa(gameplay.MaxLimit) + "."}, func() { o.setLimit(l.L, v+1) })
	text := HabitLimitText(v, l.Zero)
	cx := float64(x + habitBtnW + habitValW/2)
	cy := float64(by) + (float64(bh)-gfx.LineH(gfx.Label))/2
	k.Queue(func(dst *ebiten.Image) {
		st := gfx.Label
		st.Align = gfx.AlignCenter
		gfx.Text(dst, text, cx, cy, st)
	})
}

// HabitLimitText is a limit's value as shown: the number, or zeroText for 0.
func HabitLimitText(v int, zeroText string) string {
	if v == 0 && zeroText != "" {
		return zeroText
	}
	return strconv.Itoa(v)
}
