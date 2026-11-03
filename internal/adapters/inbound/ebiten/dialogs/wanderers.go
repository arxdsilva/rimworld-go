package dialogs

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/pkg/randx"
)

// The New wanderers picker (MAIN_UI §38, wiki: Endings): after a game over,
// once the wait is over, the player chooses the group that sets out to start
// again, in a menu shaped like the game creation menu's colonist page: a
// list of slots with a reroll on each, and the chosen person's card (portrait,
// name, age, backstory, traits and skills with passion glyphs). Start emits NewWanderers (Name = the
// seeds, comma-separated decimal). The seeds come from the tick the picker
// opened at and a local reroll counter: UI state, no simulation access.

const (
	WanderersID        = "colony.wanderers"
	WanderersStartID   = "wanderers.start"
	WanderersCancelID  = "wanderers.cancel"
	WanderersSizeID    = "wanderers.size"
	wanderersW         = 780
	wanderersH         = 540
	wandererRowH       = 40
	wandererSkillRowH  = 17
	wandererPortraitW  = 90
	wandererPortraitH  = 112
	wanderersWaitWords = "New wanderers can't set out yet."
)

// WandererRowID, WandererRerollID and WandererSlotID are the picker's
// per-slot widget ids.
func WandererRowID(i int) string    { return "wanderers.slot." + strconv.Itoa(i) }
func WandererRerollID(i int) string { return "wanderers.reroll." + strconv.Itoa(i) }

// wandererSlot is one chosen person: the seed and its card.
type wandererSlot struct {
	seed uint64
	bio  game.BioView
}

// wandererDraft is the picker's state.
type wandererDraft struct {
	base  uint64
	next  uint64
	slots []wandererSlot
	n     int // how many of the slots set out
	sel   int
}

// fresh is a seed no slot holds.
func (d *wandererDraft) fresh() uint64 {
	for {
		d.next++
		s := randx.SplitMix64(d.base + d.next)
		if s == 0 {
			continue
		}
		dup := false
		for _, o := range d.slots {
			dup = dup || o.seed == s
		}
		if !dup {
			return s
		}
	}
}

// roll gives slot i a new person.
func (d *wandererDraft) roll(i int, rules pawngen.Rules) {
	s := d.fresh()
	d.slots[i] = wandererSlot{seed: s, bio: game.WandererBio(s, rules)}
}

// seeds is the command's Name: the seeds of the people who set out.
func (d *wandererDraft) seeds() string {
	parts := make([]string, 0, d.n)
	for _, s := range d.slots[:d.n] {
		parts = append(parts, strconv.FormatUint(s.seed, 10))
	}
	return strings.Join(parts, ",")
}

// OpenWanderers opens the picker, a full group to start from.
func (dl *Dealings) OpenWanderers() {
	ax := dl.sh.World().AX
	if dl.wand != nil || ax.WanderersMax <= 0 {
		return
	}
	d := &wandererDraft{base: uint64(dl.sh.Tick())*0x2545F4914F6CDD1D + uint64(ax.WanderersLeft), n: ax.WanderersMax}
	d.slots = make([]wandererSlot, ax.WanderersMax)
	for i := range d.slots {
		d.roll(i, ax.WanderersRules)
	}
	dl.wand = d
	dl.kit().Push(uikit.Modal{
		ID: WanderersID, Title: "New wanderers", W: wanderersW, H: wanderersH,
		Draw: func(body gfx.Rect) { dl.wanderersBody(d, body) },
		Buttons: []uikit.ModalButton{
			{ID: WanderersStartID, Label: "Start", Style: uikit.BtnPrimary, Closes: true,
				Disabled: func() string { return dl.wanderersReason(d) }, OnClick: func() { dl.startWanderers(d) }},
			{ID: WanderersCancelID, Label: "Cancel", Closes: true, OnClick: dl.closeWanderers},
		},
		OnClose: dl.closeWanderers,
	})
}

// WandererSeeds are the open picker's seeds, slot by slot (nil: closed).
func (dl *Dealings) WandererSeeds() []uint64 {
	if dl.wand == nil {
		return nil
	}
	out := make([]uint64, dl.wand.n)
	for i := range out {
		out[i] = dl.wand.slots[i].seed
	}
	return out
}

// closeWanderers forgets the draft (the display resumes with it).
func (dl *Dealings) closeWanderers() { dl.wand = nil }

// syncWanderers drops the draft when its modal is gone by another path.
func (dl *Dealings) syncWanderers() {
	if dl.wand == nil {
		return
	}
	for _, m := range dl.kit().Modals {
		if m.ID == WanderersID {
			return
		}
	}
	dl.wand = nil
}

// wanderersReason is why Start is off ("" it is on).
func (dl *Dealings) wanderersReason(d *wandererDraft) string {
	ax := dl.sh.World().AX
	switch {
	case ax.WanderersLeft == 0:
		return "No more groups"
	case !ax.WanderersOK:
		return wanderersWaitWords
	case d.n < 1:
		return "Pick at least one person."
	}
	return ""
}

// startWanderers emits NewWanderers for the chosen people.
func (dl *Dealings) startWanderers(d *wandererDraft) {
	if dl.wanderersReason(d) != "" {
		return
	}
	dl.sh.Enqueue(command.Command{Kind: command.NewWanderers, Name: d.seeds()})
	dl.wand = nil
}

// wanderersBody declares the slot list and the chosen person's card.
func (dl *Dealings) wanderersBody(d *wandererDraft, body gfx.Rect) {
	ax := dl.sh.World().AX
	k := dl.kit()
	left, right := body.SplitH(280)
	right.X += 12
	right.W -= 12
	hs := gfx.WithColor(gfx.Label, uikit.Heading)
	k.Label(left.X, left.Y, fmt.Sprintf("Group (%d groups left)", ax.WanderersLeft), hs)
	size := d.n
	uikit.Stepper(k, WanderersSizeID, gfx.Rect{X: left.X, Y: left.Y + 22, W: 140, H: 28}, &size, 1, len(d.slots))
	d.n = size
	d.sel = max(0, min(d.sel, d.n-1))
	y := left.Y + 60
	for i := 0; i < d.n; i++ {
		row := gfx.Rect{X: left.X, Y: y, W: left.W - 84, H: wandererRowH - 4}
		b := d.slots[i].bio
		label := fmt.Sprintf("%s, %d", wandererName(b), b.AgeYears)
		if k.Button(WandererRowID(i), row, label, uikit.BtnOpts{Selected: i == d.sel, Tip: "Show this person's card."}) {
			d.sel = i
		}
		if k.Button(WandererRerollID(i), gfx.Rect{X: row.Right() + 6, Y: y, W: 72, H: row.H}, "Reroll", uikit.BtnOpts{Tip: "Reroll\nTrade this person for someone new."}) {
			d.roll(i, ax.WanderersRules)
			d.sel = i
		}
		y += wandererRowH
	}
	k.Panel(right)
	dl.wandererCard(d.slots[d.sel].bio, right.Inset(12))
}

// wandererName is a person's display name: nickname alone, else first and last.
func wandererName(b game.BioView) string {
	if b.Nick != "" {
		return b.Nick
	}
	return strings.TrimSpace(b.First + " " + b.Last)
}

// wandererCard draws a person's card: name, sex and age, backstories,
// traits, what they cannot do and their skills.
func (dl *Dealings) wandererCard(b game.BioView, in gfx.Rect) {
	k := dl.kit()
	pic := gfx.Rect{X: in.Right() - wandererPortraitW, Y: in.Y, W: wandererPortraitW, H: wandererPortraitH}
	k.Queue(func(dst *ebiten.Image) {
		uikit.FillRect(dst, pic, uikit.Field)
		dl.sh.DrawPortrait(dst, b.Look, float64(pic.X), float64(pic.Y), float64(pic.W), float64(pic.H))
	})
	wide := float64(in.W - wandererPortraitW - 12) // the lines beside the portrait
	full := strings.TrimSpace(b.First + " '" + b.Nick + "' " + b.Last)
	if b.Nick == "" {
		full = wandererName(b)
	}
	y := in.Y
	k.Label(in.X, y, full, gfx.WithColor(gfx.Body, uikit.Heading))
	y += 22
	k.Label(in.X, y, fmt.Sprintf("%s, age %d. Build: %s.", b.Sex, b.AgeYears, b.Build), uikit.DimText())
	y += 22
	for _, l := range [][2]string{{"Childhood", b.Childhood}, {"Adulthood", b.Adulthood}} {
		k.Label(in.X, y, l[0]+": "+gfx.Ellipsize(l[1], gfx.Body, wide-90), gfx.Body)
		y += 20
	}
	traits := "none"
	if len(b.Traits) > 0 {
		names := make([]string, len(b.Traits))
		for i, t := range b.Traits {
			names[i] = t.Label
		}
		traits = strings.Join(names, ", ")
	}
	k.Label(in.X, y, "Traits: "+gfx.Ellipsize(traits, gfx.Body, wide-70), gfx.Body)
	y += 20
	if len(b.Incapable) > 0 {
		k.Label(in.X, y, "Incapable of: "+gfx.Ellipsize(strings.Join(b.Incapable, ", "), gfx.Body, wide-110), gfx.WithColor(gfx.Body, uikit.Danger))
		y += 20
	}
	y = max(y, in.Y+wandererPortraitH) + 8
	k.Label(in.X, y, "Skills", gfx.WithColor(gfx.Label, uikit.Heading))
	y += 22
	for _, s := range b.Skills {
		if y+wandererSkillRowH > in.Bottom() {
			break
		}
		st := gfx.Body
		lvl := strconv.Itoa(s.Level)
		if s.Incapable {
			st, lvl = gfx.WithColor(gfx.Body, uikit.TextDisabled), "-"
		}
		k.Label(in.X, y, s.Label, st)
		dl.passionGlyph(float64(in.X+170), float64(y), s.Passion)
		k.Label(in.X+210, y, lvl, st)
		y += wandererSkillRowH
	}
}

// passionGlyph draws a skill's passion at (x, y), as the game creation menu's
// skill table does.
func (dl *Dealings) passionGlyph(x, y float64, passion int) {
	key := ""
	switch passion {
	case 1:
		key = "ui_icon/passion_minor"
	case 2:
		key = "ui_icon/passion_major"
	}
	if key == "" {
		return
	}
	dl.kit().Queue(func(dst *ebiten.Image) {
		if img := dl.sh.Icon(key); img != nil {
			gfx.Icon(dst, img, x, y, 14)
		}
	})
}
