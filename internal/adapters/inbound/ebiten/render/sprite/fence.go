package sprite

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
)

// Fences (UI-017, ART fences) autotile among themselves, their gates and
// walls (one way, linkgrid.go). Every key may be missing from the art
// catalog: the vector fallback keeps nothing invisible.

// The fence kinds (construction names).
const (
	FenceKind = "fence"
	GateKind  = "fence gate"
)

// IsFenceKind reports a fence or a fence gate.
func IsFenceKind(kind string) bool { return kind == FenceKind || kind == GateKind }

// FenceSprite is structure/fence_<wood|stone>_<mask>.
func FenceSprite(stuff string, mask int) string {
	m := "wood"
	if IsStoneStuff(stuff) {
		m = "stone"
	}
	return "structure/fence_" + m + "_" + TwoDigits(mask)
}

// DrawFenceVector is the plain-shape fence used when the art has no
// sprite for it: a post with rails toward each joined neighbour; a gate
// is two posts with a bar across (swung aside when open).
func DrawFenceVector(screen *ebiten.Image, sx, sy, tile float32, kind string, mask int, open, stone bool, alpha uint8) {
	wood := color.RGBA{R: 140, G: 100, B: 60, A: alpha}
	if stone {
		wood = color.RGBA{R: 140, G: 140, B: 145, A: alpha}
	}
	cx, cy := sx+tile/2, sy+tile/2
	th := max(tile/10, 2)
	if kind == GateKind {
		vector.FillRect(screen, sx+tile*0.08, sy+tile*0.2, th*1.5, tile*0.6, wood, false)
		vector.FillRect(screen, sx+tile*0.92-th*1.5, sy+tile*0.2, th*1.5, tile*0.6, wood, false)
		if open {
			vector.FillRect(screen, sx+tile*0.08, sy+tile*0.2, th, tile*0.2, wood, false)
		} else {
			vector.FillRect(screen, sx+tile*0.08, cy-th/2, tile*0.84, th, wood, false)
		}
		return
	}
	vector.FillRect(screen, cx-th, cy-th, th*2, th*2, wood, false)
	if mask&art.WallN != 0 {
		vector.FillRect(screen, cx-th/2, sy, th, tile/2, wood, false)
	}
	if mask&art.WallS != 0 {
		vector.FillRect(screen, cx-th/2, cy, th, tile/2, wood, false)
	}
	if mask&art.WallW != 0 {
		vector.FillRect(screen, sx, cy-th/2, tile/2, th, wood, false)
	}
	if mask&art.WallE != 0 {
		vector.FillRect(screen, cx, cy-th/2, tile/2, th, wood, false)
	}
}
