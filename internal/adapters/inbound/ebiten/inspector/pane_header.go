package inspector

import (
	"fmt"
	"image/color"
	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/selection"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/game/dossierview"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/medcare"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// The restyled pane's header row: the emblem, the name, and the small buttons
// between it and the close button. Everything here has an id under pane.head,
// so nothing collides with the inspector's own widgets or the card's.

// The header's widgets.
const (
	HeadTitleID   = "pane.head.title"
	HeadEmblemID  = "pane.head.emblem"
	HeadNextID    = "pane.head.next"
	HeadInfoID    = "pane.head.info"
	HeadRenameID  = "pane.head.rename"
	HeadThreatID  = "pane.head.threat"
	HeadCareID    = "pane.head.care"
	HeadSelfID    = "pane.head.selftend"
	HeadRenameFld = "pane.head.rename.field"
	HeadRenameOK  = "modal.pane.head.rename.dialog.ok"
	headRenameMod = "pane.head.rename.dialog"
)

// Header metrics (logical px).
const (
	headBtn     = 20
	headGap     = 2
	headMaxBtns = 6                                 // reserved beside the name
	headReserve = headMaxBtns * (headBtn + headGap) // what the name gives up
	headEmblem  = 22                                // the emblem's slot
)

// headButton is one small button of the header.
type headButton struct {
	id, label, icon, tip string
	reason               string // disabled, and why
	selected             bool
	run                  func()
}

// ThreatResponses are the threat response names in command order (default,
// flee, attack, ignore).
func ThreatResponses() []string {
	out := make([]string, len(threatResponses))
	for i, r := range threatResponses {
		out[i] = r.name
	}
	return out
}

// colonist reports p is one of ours: self-tend and renaming are for them.
func colonist(p game.PawnView) bool {
	return !p.Hostile && !p.Prisoner && !p.Visitor && !p.Ally
}

// declareHeader declares the name, the emblem and the buttons.
func (c CardPane) declareHeader(p gfx.Rect, spec Spec, g Geom) {
	u := c.kit()
	x := p.X + PaneInset
	if spec.Emblem != "" {
		if spec.EmblemTip != "" {
			u.Tip(HeadEmblemID, gfx.Rect{X: x, Y: p.Y + paneTop, W: headBtn, H: headBtn}, spec.EmblemTip)
		}
		x += headEmblem
	}
	tw := min(g.TitleW, gfx.TextW(spec.Title, gfx.Large))
	title := gfx.Rect{X: x, Y: p.Y + paneTop, W: tw, H: gfx.LineStep(gfx.Large)}
	if spec.TitleTip != "" || spec.TitleTab != "" {
		if u.Add(uikit.Widget{ID: HeadTitleID, R: title, Tip: spec.TitleTip}) && spec.TitleTab != "" && u.Press(title) {
			c.SelectPaneTab(ids.PrefixInspector + spec.TitleTab)
		}
	}
	bx := g.Close.X - headGap - headBtn
	if spec.Close == "" {
		bx = p.Right() - paneTop - headBtn
	}
	for _, b := range c.headButtons(spec) {
		c.declareHeadButton(b, gfx.Rect{X: bx, Y: p.Y + paneTop, W: headBtn, H: headBtn})
		bx -= headBtn + headGap
	}
}

// declareHeadButton is one button with its icon over it.
func (c CardPane) declareHeadButton(b headButton, r gfx.Rect) {
	u := c.kit()
	img := c.headIcon(b.icon)
	label := b.label
	if img != nil {
		label = ""
	}
	if u.Button(b.id, r, label, uikit.BtnOpts{Disabled: b.reason != "", Reason: b.reason, Tip: b.tip, Selected: b.selected, NoFocus: true}) && b.run != nil {
		b.run()
	}
	if img != nil {
		x, y, s := float64(r.X+2), float64(r.Y+2), float64(r.W-4)
		u.Queue(func(dst *ebiten.Image) { gfx.Icon(dst, img, x, y, s) })
	}
}

// headIcon is an icon of the art catalog (nil without art or without a key).
func (c CardPane) headIcon(key string) *ebiten.Image {
	art := c.env().Art
	if art == nil || key == "" {
		return nil
	}
	return art.Image(key, color.NRGBA{})
}

// headButtons are the buttons for the selection, right to left. A kind that
// cannot use one does not get it.
func (c CardPane) headButtons(spec Spec) []headButton {
	out := []headButton{c.nextButton(spec), c.infoButton()}
	switch spec.Kind {
	case selection.Pawn:
		p, ok := c.pawnView(spec.ID)
		if !ok {
			return nil
		}
		if colonist(p) {
			out = append(out, c.renameButton(p.ID, c.shell.PawnLabel(p)))
		}
		if th := threatCommandIn(c.shell, p); len(th) == 1 {
			out = append(out, c.threatButton(p, th[0]))
		}
		if !p.Hostile {
			out = append(out, c.careButton(p.ID, c.shell.PawnLabel(p), medcare.Policy(p.CareLevel), p.Status == "dead"))
		}
		if colonist(p) {
			out = append(out, c.selfTendButton(p))
		}
	case selection.Animal:
		for _, an := range c.shell.World().Animals {
			if an.ID != spec.ID {
				continue
			}
			if an.Tame {
				out = append(out, c.renameButton(an.ID, an.Name))
				if !an.Mech {
					out = append(out, c.careButton(an.ID, an.Name, medcare.Policy(an.CareLevel), false))
				}
			}
			break
		}
	}
	return out
}

// nextButton selects the next thing on this cell (RHUD-011). It looks only
// when clicked, so an idle frame pays nothing.
func (c CardPane) nextButton(spec Spec) headButton {
	return headButton{id: HeadNextID, label: ">", tip: "Next on this cell\nSelect the next thing on the same cell.",
		run: func() { c.selectNext(spec.Kind, spec.ID) }}
}

// selectNext moves the selection to what follows (k, id) on its cell.
func (c CardPane) selectNext(k selection.Kind, id entity.EntityID) {
	w := c.shell.World()
	x, y, ok := c.cellOf(w, k, id)
	if !ok {
		return
	}
	cands := c.sel.Selectables(w, x, y)
	at := -1
	for i, s := range cands {
		if s.Kind == k && s.ID == id {
			at = i
			break
		}
	}
	if len(cands) < 2 || at < 0 {
		c.shell.Hint(c.shell.T("Nothing else on this cell."))
		return
	}
	c.sel.Select(cands[(at+1)%len(cands)])
}

// cellOf is where the selection stands.
func (c CardPane) cellOf(w game.WorldView, k selection.Kind, id entity.EntityID) (x, y int, ok bool) {
	if k == selection.Pawn {
		p, ok := c.pawnView(id)
		return p.X, p.Y, ok
	}
	for _, an := range w.Animals {
		if an.ID == id {
			return an.X, an.Y, true
		}
	}
	return 0, 0, false
}

// infoButton opens the stat sheet (RHUD-012).
func (c CardPane) infoButton() headButton {
	return headButton{id: HeadInfoID, label: "i", tip: "Stats\nEvery figure this one is judged by.",
		run: func() {
			if h, _, on := c.host(); on {
				h.CardStats()
			}
		}}
}

// threatButton moves to the next threat response (RHUD-018): the grid's
// button in the header's size.
func (c CardPane) threatButton(p game.PawnView, b Button) headButton {
	cmds := b.Cmds
	return headButton{id: HeadThreatID, label: "!", icon: "ui_icon/" + b.Icon, tip: b.Desc,
		selected: p.ThreatResponse != "" && p.ThreatResponse != threatResponses[0].name,
		run: func() {
			for _, cmd := range cmds {
				c.shell.Enqueue(cmd)
			}
		}}
}

// careButton opens the care level picker (RHUD-019, 020).
func (c CardPane) careButton(id entity.EntityID, name string, cur medcare.Policy, dead bool) headButton {
	b := headButton{id: HeadCareID, label: "+", icon: careIcon(cur),
		tip: fmt.Sprintf("Medical care for %s: %s\n%s", name, cur.Label(), careTipIn(c.shell, cur)),
		run: func() { c.pickCare(id, cur) }}
	if dead {
		b.reason = "The dead need no care."
	}
	return b
}

// pickCare opens the five levels with the current one marked.
func (c CardPane) pickCare(id entity.EntityID, cur medcare.Policy) {
	opts := make([]string, medcare.NumPolicies)
	for p := medcare.Policy(0); p < medcare.NumPolicies; p++ {
		opts[p] = p.Label()
	}
	uikit.Picker(c.kit(), HeadCareID, c.shell.T("Medical care"), opts, int(cur), func(i int) {
		c.shell.Enqueue(command.Command{Kind: command.SetMedicalCare, Pawn: uint64(id), Mode: uint8(i)})
	})
}

// selfTendButton is the self-tend toggle (RHUD-021 to 024).
func (c CardPane) selfTendButton(p game.PawnView) headButton {
	st := selfTendState(c.Pane, p)
	on := false
	if h, ok := c.sim().Health(p.ID); ok {
		on = h.SelfTend
	}
	return headButton{id: HeadSelfID, label: "S", icon: "ui_icon/skill_medicine", tip: st.tip, reason: st.reason, selected: on,
		run: func() { c.setSelfTend(p, !on) }}
}

// setSelfTend sends the toggle, warning when it cannot do anything yet.
func (c CardPane) setSelfTend(p game.PawnView, on bool) {
	c.shell.Enqueue(command.Command{Kind: command.SetSelfTend, Pawn: uint64(p.ID), Enabled: on})
	if on && !p.WorkDoctor {
		c.shell.Hint(c.shell.Tf("%s has doctoring switched off.", c.shell.PawnLabel(p)))
	}
}

// renameButton opens the rename dialog (RHUD-025).
func (c CardPane) renameButton(id entity.EntityID, name string) headButton {
	return headButton{id: HeadRenameID, label: "N", icon: "ui_icon/zone_rename", tip: c.shell.T("Rename\nGive this one a new name."),
		run: func() { c.openRename(id, name) }}
}

// openRename opens the dialog over name. It accepts what the handler accepts;
// an empty name asks for the generated one back.
func (c CardPane) openRename(id entity.EntityID, name string) {
	u := c.kit()
	text := name
	send := func() {
		clean, ok := dossierview.CleanName(text)
		if ok {
			c.shell.Enqueue(command.Command{Kind: command.SetName, Pawn: uint64(id), Name: clean})
		}
	}
	u.Push(uikit.Modal{ID: headRenameMod, Title: c.shell.Tf("Rename %s", name), W: 380, H: 180,
		Draw: func(r gfx.Rect) { c.renameBody(r, &text) },
		Buttons: []uikit.ModalButton{
			{ID: "ok", Label: c.shell.T("Rename"), Style: uikit.BtnPrimary, Closes: true, OnClick: send, Disabled: func() string { return renameRefusal(c.shell, text) }},
			{ID: "cancel", Label: c.shell.T("Cancel"), Closes: true},
		}})
	u.Focus, u.TabTo = HeadRenameFld, HeadRenameFld
}

// renameBody declares the dialog's field and its hint.
func (c CardPane) renameBody(r gfx.Rect, text *string) {
	u := c.kit()
	u.TextField(HeadRenameFld, gfx.Rect{X: r.X, Y: r.Y, W: r.W, H: 28}, text, uikit.TextFieldSpec{Placeholder: c.shell.T("Generated name")})
	note := c.shell.T("Leave it empty for the name it started with.")
	st := uikit.DimText()
	if why := renameRefusal(c.shell, *text); why != "" {
		note = why
		st = gfx.WithColor(st, uikit.Warning)
	}
	u.Label(r.X, r.Y+36, note, st)
}

// renameRefusal is why a name will not do ("": it will).
func renameRefusal(tr lang.Tr, s string) string {
	if _, ok := dossierview.CleanName(strings.TrimSpace(s)); !ok {
		return tr.T("Names hold letters, spaces, apostrophes and hyphens, and stay short.")
	}
	return ""
}

// selfTend is how the self-tend control reads for one colonist.
type selfTend struct {
	reason string // disabled, and why
	tip    string
}

// selfTendState is the Health tab's and the header's self-tend control. With
// the card off it is today's: disabled only for the dead, with today's words.
func selfTendState(a *Pane, p game.PawnView) selfTend {
	st := selfTend{tip: a.shell.T("Self-tend\nWith no one else to help, this colonist tends its own wounds (at lower quality).")}
	if p.Status == "dead" {
		st.reason = a.shell.T("The dead need no care.")
	}
	if _, _, on := (CardPane{a}).host(); !on {
		return st
	}
	st.tip = a.shell.Tf("Self-tend\nWith no one else to help, this colonist tends its own wounds, at %d %% of a doctor's quality.", body.SelfTendFactor/10)
	if st.reason == "" {
		if r, ok := a.sim().(doctorReader); ok {
			st.reason = r.IncapableReason(p.ID, work.Doctor)
		}
	}
	return st
}

// doctorReader says why a colonist cannot do a kind of work; the game has it,
// asserted on Sim as bedPrisonReader is.
type doctorReader interface {
	IncapableReason(id entity.EntityID, t work.Type) string
}
