package creature

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// Host is what the creature layer reads of the root renderer: the
// renderer implements it with one pointer (no allocation per call).
type Host interface {
	Figures
	Marks
	Sounds
}

// Figures is a creature's look as the root resolves it.
type Figures interface {
	// FigureLayers appends p's sprites in art facing fs, bottom first.
	FigureLayers(dst []Layer, p game.PawnView, fs string) []Layer
	// PawnBlood and AnimalBlood are a figure's blood stain this frame
	// (lit.kage's Custom3; 0 in the shadow pass).
	PawnBlood(p game.PawnView) float32
	AnimalBlood(an game.AnimalView) float32
	// LayerBlood is the stain one layer of a pawn takes.
	LayerBlood(key string, blood float32) float32
	// Visual is a creature's visual record (facing, lying since).
	Visual(id entity.EntityID) Visual
	// ColonyCloths are the colony's cloth tints (bed blankets).
	ColonyCloths() []string
	// ItemSprite is a stack kind's sprite; StoneItem tints a stone load.
	ItemSprite(kind string) string
	StoneItem(key, kind string, cs sprite.ColorScale) (string, sprite.ColorScale)
}

// Marks are the root's world marks and lookups the creature overlays use.
type Marks interface {
	// HostileMarker floats the red chevron lift tiles over tile (wx, wy).
	HostileMarker(dst *ebiten.Image, p game.PawnView, wx, wy, lift, tile float64)
	// HealthColor is a health bar's fill at pct.
	HealthColor(pct int) color.RGBA
	// TurretMuzzle is a sentry turret's muzzle in world tiles.
	TurretMuzzle(id entity.EntityID) (Vec, bool)
	// BuildingAt is the building standing on tile (x, y).
	BuildingAt(w game.WorldView, x, y int) (game.BuildingView, bool)
	// Spray is a hit's spray: what the target bleeds, how many drops and
	// how fast.
	Spray(w game.WorldView, target entity.EntityID, damage int) (kind PartKind, n int, speed float64)
}

// Sounds are the cues the creature effects raise.
type Sounds interface {
	// WorkCue is a work strike's sound for action act at (x, y).
	WorkCue(act string, x, y float64)
	// BurstCue is an explosive death's bang at (x, y).
	BurstCue(x, y float64)
	// TraceSound is an attack's sound.
	TraceSound(t game.TraceView)
}
