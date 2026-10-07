package ebiten

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

// Animals tab: the colony's tame animals, with slaughter and release.

const animalRowH = 30

func (a *App) tameAnimals() []game.AnimalView {
	var out []game.AnimalView
	for _, an := range a.renderer.world().Animals {
		if an.Tame {
			out = append(out, an)
		}
	}
	return out
}

func (a *App) animalsTabButtons() []button {
	r := workRect(a.sw, a.sh)
	var out []button
	for i, an := range a.tameAnimals() {
		y := r.y + 60 + float32(i*animalRowH)
		if y+animalRowH > r.y+r.h {
			break
		}
		id := strconv.FormatUint(uint64(an.ID), 10)
		slaughter := "Slaughter"
		if an.Designated {
			slaughter = "Spare"
		}
		out = append(out,
			button{id: "animals.slaughter." + id, label: slaughter, x: r.x + 430, y: y, w: 100, h: 24, selected: an.Designated},
			button{id: "animals.release." + id, label: "Release", x: r.x + 536, y: y, w: 90, h: 24},
		)
	}
	return out
}

func (a *App) applyAnimalsTab(id string) {
	parts := strings.Split(id, ".")
	if len(parts) != 3 || a.renderer == nil {
		return
	}
	n, err := strconv.ParseUint(parts[2], 10, 64)
	if err != nil {
		return
	}
	for _, an := range a.tameAnimals() {
		if uint64(an.ID) != n {
			continue
		}
		switch parts[1] {
		case "slaughter":
			kind := command.DesignateHunt
			if an.Designated {
				kind = command.DesignateCancel
			}
			a.renderer.enqueue(command.Command{Kind: kind, X: an.X, Y: an.Y, X2: an.X, Y2: an.Y})
		case "release":
			a.renderer.enqueue(command.Command{Kind: command.ReleaseAnimal, Pawn: n})
		}
	}
}

func (a *App) drawAnimalsTab(screen *ebiten.Image, sw, sh int) {
	r := workRect(sw, sh)
	drawPanel(screen, r.x, r.y, r.w, r.h)
	w := a.renderer.world()
	marked := 0
	for _, an := range w.Animals {
		if an.TameMark {
			marked++
		}
	}
	drawLabel(screen, int(r.x)+12, int(r.y)+10, fmt.Sprintf("Animals  (%d tame, %d marked for taming, %d pens)", len(a.tameAnimals()), marked, len(w.Pens)))
	drawLabel(screen, int(r.x)+12, int(r.y)+28, "Tame with Architect > Orders > Tame (hunters do it). Tame wolves guard; pens keep animals in place.")
	drawLabel(screen, int(r.x)+52, int(r.y)+44, "Name          Species     Health  Doing")
	mx, my := a.currentCursor()
	for i, an := range a.tameAnimals() {
		y := int(r.y) + 60 + i*animalRowH
		if y+animalRowH > int(r.y+r.h) {
			break
		}
		if a.renderer.art != nil {
			if img := a.renderer.art.image("ui_icon/"+an.Species, color.NRGBA{}); img != nil {
				drawIcon(screen, img, float64(r.x)+16, float64(y), 26)
			}
		}
		doing := an.Mode
		if an.Designated {
			doing = "marked for slaughter"
		}
		drawLabel(screen, int(r.x)+52, y+6, fmt.Sprintf("%-13s %-11s %3d%%    %s", an.Name, an.Species, an.Health, doing))
	}
	a.drawButtons(screen, a.animalsTabButtons(), mx, my)
	if len(a.tameAnimals()) == 0 {
		drawLabel(screen, int(r.x)+52, int(r.y)+66, "No tame animals yet.")
	}
}
