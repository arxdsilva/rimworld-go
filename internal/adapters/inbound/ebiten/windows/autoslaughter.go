package windows

import (
	"fmt"
	"sort"
	"strconv"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// The auto-slaughter window (FOOD-5, MAIN_UI_SCREENS Animals): each tame
// species with a cap on adult males, adult females and young. A cap steps
// from none (-1) through 0..30; the surplus is slaughtered once a day. It
// reads WorldView.AutoSlaughter and sends SetAutoSlaughter.

const (
	AutoSlaughterBtnID   = "animals.autoslaughter"
	AutoSlaughterModalID = "colony.autoslaughter"
	autoW, autoH         = 520, 400
	autoRowH             = 30
	autoCapMax           = 30
	autoNameW            = 150
	autoCellW            = 110
)

// AnimalDietID is a tame animal's food policy button id.
func AnimalDietID(id entity.EntityID) string {
	return "animals.diet." + strconv.FormatUint(uint64(id), 10)
}

// AutoSlaughterStepID is the - or + button of cap field (0 males, 1
// females, 2 young) of species.
func AutoSlaughterStepID(species string, field int, up bool) string {
	d := "dec"
	if up {
		d = "inc"
	}
	return "autoslaughter." + species + "." + strconv.Itoa(field) + "." + d
}

// autoRow is one species line.
type autoRow struct {
	species, label string
	caps           [3]int // males, females, young; -1 none
}

// AutoRows are the tame species and the capped ones, by label.
func AutoRows(tame []game.AnimalView, caps []game.AutoSlaughterView) []autoRow {
	by := map[string]*autoRow{}
	for _, an := range tame {
		if _, ok := by[an.Species]; !ok {
			by[an.Species] = &autoRow{species: an.Species, label: an.Label, caps: [3]int{-1, -1, -1}}
		}
	}
	for _, c := range caps {
		r, ok := by[c.Species]
		if !ok {
			r = &autoRow{species: c.Species, label: c.Species}
			by[c.Species] = r
		}
		r.caps = [3]int{c.MaxMales, c.MaxFemales, c.MaxYoung}
	}
	out := make([]autoRow, 0, len(by))
	for _, r := range by {
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].label < out[j].label })
	return out
}

// StepCap is cap v moved one step: none -1 then 0..autoCapMax.
func StepCap(v int, up bool) (int, bool) {
	if up {
		return v + 1, v < autoCapMax
	}
	return v - 1, v > -1
}

// OpenAutoSlaughter opens the auto-slaughter window.
func (w Animals) OpenAutoSlaughter() {
	if !w.live() {
		return
	}
	w.OpenKit(uikit.Modal{
		ID: AutoSlaughterModalID, Title: "Auto-slaughter", W: autoW, H: autoH, Anchor: w.Anchor,
		Draw: func(body gfx.Rect) { w.autoBody(body) },
	})
}

func (w Animals) autoBody(body gfx.Rect) {
	hs := gfx.WithColor(gfx.Label, uikit.Heading)
	w.kit().Label(body.X, body.Y, "Species", hs)
	for i, h := range []string{"Males", "Females", "Young"} {
		w.kit().Label(body.X+autoNameW+float32(i)*autoCellW, body.Y, h, hs)
	}
	if w.kit().Button("autoslaughter.back", gfx.Rect{X: body.Right() - 90, Y: body.Y - 2, W: 90, H: 24}, "Animals", uikit.BtnOpts{Tip: "Back to the Animals window."}) {
		w.CloseQuietly()
		w.Show("animals")
		return
	}
	rows := AutoRows(w.tame(), w.sh.World().AutoSlaughter)
	if len(rows) == 0 {
		w.kit().Label(body.X, body.Y+32, "No tame animals yet.", uikit.DimText())
		return
	}
	list := gfx.Rect{X: body.X, Y: body.Y + 28, W: body.W, H: body.H - 28}
	area := w.kit().ScrollArea("autoslaughter.list", list, float32(len(rows))*autoRowH, uikit.ScrollOpts{Step: autoRowH * 2})
	for i, row := range rows {
		y := area.View.Y + float32(i)*autoRowH - area.Off
		if y < area.View.Y || y+autoRowH > area.View.Bottom() {
			continue
		}
		w.kit().Label(area.View.X, y+5, gfx.Ellipsize(row.label, gfx.Body, autoNameW-8), gfx.Body)
		for f := 0; f < 3; f++ {
			x := area.View.X + autoNameW + float32(f)*autoCellW
			text := "none"
			if row.caps[f] >= 0 {
				text = fmt.Sprint(row.caps[f])
			}
			for j, up := range []bool{false, true} {
				nv, ok := StepCap(row.caps[f], up)
				bx := x + float32(j)*(autoCellW-26)
				if w.kit().Button(AutoSlaughterStepID(row.species, f, up), gfx.Rect{X: bx, Y: y, W: 24, H: 24}, map[bool]string{false: "-", true: "+"}[up],
					uikit.BtnOpts{Disabled: !ok, Reason: "limit"}) {
					c := row.caps
					c[f] = nv
					w.sh.Enqueue(command.Command{Kind: command.SetAutoSlaughter, Name: row.species, X: c[0], Y: c[1], X2: c[2]})
				}
			}
			w.kit().Label(x+28, y+5, text, gfx.Body)
		}
	}
}
