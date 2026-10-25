package ebiten

import (
	"fmt"
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// The colonist bar (PLAY-018: HUD-TOP owns it; it moves top-centre and
// lays out in the layout's BarSlots).

func pawnLabel(p game.PawnView) string {
	if p.Name != "" {
		return p.Name
	}
	return fmt.Sprintf("Colonist %d", p.ID)
}

// barTip is a colonist bar slot's hover line: the name, the age and the
// adulthood title ("Ada, 31, Convoy mechanic"); a legacy colonist has
// neither, so only the name.
func barTip(p game.PawnView) string {
	parts := []string{pawnLabel(p)}
	if p.AgeYears > 0 {
		parts = append(parts, strconv.Itoa(p.AgeYears))
	}
	if p.Title != "" && p.Title != "—" {
		parts = append(parts, p.Title)
	}
	return strings.Join(parts, ", ")
}

// barClick is the last click on a colonist's bar slot.
type barClick struct {
	id entity.EntityID
	at uint64 // App.updates of the click; 0: none
}

// barDoubleClick records a bar click on id and reports it is the second
// of a double click (same colonist within doubleClickFrames, the map's
// window). A double click is spent: the next click starts a new pair.
func (a *App) barDoubleClick(id entity.EntityID) bool {
	last := a.barClk
	if last.at > 0 && last.id == id && a.updates-last.at <= doubleClickFrames {
		a.barClk = barClick{}
		return true
	}
	a.barClk = barClick{id: id, at: a.updates}
	return false
}

func (a *App) colonistBarButtons(sw, sh int) []button {
	if a.renderer == nil {
		return nil
	}
	pawns := a.barPawns()
	y := hudBarY(sh)
	out := make([]button, 0, len(pawns))
	for i, p := range pawns {
		sel := a.renderer.HasSelect() && a.renderer.Selected() == p.ID || a.inGroup(selPawn, p.ID)
		out = append(out, button{
			id:       "bar.pawn." + strconv.FormatUint(uint64(p.ID), 10),
			label:    "",
			tip:      barTip(p),
			x:        float32(hudPad + i*barSlot),
			y:        y + 4,
			w:        barSlot - 4,
			h:        hudBarH - 8,
			selected: sel,
		})
	}
	return out
}

// drawColonistBar draws the colonist bar: its band, slots and portraits.
func (a *App) drawColonistBar(screen *ebiten.Image, sw, sh int) {
	drawPanel(screen, 0, hudBarY(sh), float32(sw), hudBarH)
	mx, my := a.currentCursor()
	a.drawButtons(screen, a.colonistBarButtons(sw, sh), mx, my)
	a.drawColonistPortraits(screen, sh)
}

// barCount is the number of colonist bar entries the layout reserves
// (0 while the bar is drawn the legacy way, at the bottom).
func (a *App) barCount() int { return 0 }

// colonistBarChrome is the bar's area while it is drawn the legacy way.
func (a *App) colonistBarChrome(sw, sh int) []rect {
	return []rect{{0, hudBarY(sh), float32(sw), hudBarH}}
}

// colonistBarUI declares the bar's widget-kit parts (stub).
func (a *App) colonistBarUI(sw, sh int) {}

// barPawns are the colonists the colonist bar shows: everyone but
// prisoners (who stay selectable on the map).
func (a *App) barPawns() []game.PawnView {
	pawns := a.renderer.snapshot()
	for i, p := range pawns {
		if p.Prisoner {
			out := append(make([]game.PawnView, 0, len(pawns)), pawns[:i]...)
			for _, q := range pawns[i+1:] {
				if !q.Prisoner {
					out = append(out, q)
				}
			}
			return out
		}
	}
	return pawns
}

func (a *App) drawColonistPortraits(screen *ebiten.Image, sh int) {
	pawns := a.barPawns()
	y := float64(hudBarY(sh) + 6)
	for i, p := range pawns {
		x := float64(hudPad + i*barSlot + 6)
		a.drawPawnPortrait(screen, p, x, y, 40, 48)
		if p.Drafted {
			drawIcon(screen, a.icon("draft"), x+24, y-2, 18)
		}
		uiFillRect(screen, float32(x), float32(y)+46, 40*float32(p.Mood)/100, 3, moodColor(p.Mood), false)
		drawText(screen, ellipsize(pawnLabel(p), tsSmall, barSlot-4), x, float64(hudBarY(sh))+hudBarH-lineH(tsSmall)-1, tsSmall)
	}
}

func (a *App) drawPawnPortrait(screen *ebiten.Image, p game.PawnView, x, y, w, h float64) {
	if a.renderer.art != nil {
		a.renderer.drawPawnPortraitArt(screen, p, x, y, w, h)
	}
}
