package ebiten

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

func (r *Renderer) drawArt(screen *ebiten.Image, sw, sh int, tile float64) {
	r.worldView()
	r.pipeline().DrawArt(screen, sw, sh, tile)
}

func (r *Renderer) drawFloorLayers(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	r.worldView()
	r.pipeline().DrawFloorLayers(screen, w, sw, sh, tile)
}

func (r *Renderer) drawBuildingObj(dst *ebiten.Image, b *game.BuildingView, tile float64, sw, sh int) {
	r.worldView()
	r.pipeline().Ob.DrawBuildingObj(dst, b, tile, sw, sh)
}

func (r *Renderer) drawPlantObj(dst *ebiten.Image, w *game.WorldView, p *game.PlantView, tile float64) {
	r.worldView()
	r.pipeline().Ob.DrawPlantObj(dst, w, p, tile)
}

func (r *Renderer) drawPawnArt(screen *ebiten.Image, p game.PawnView, wx, wy, tile float64, sw, sh int) {
	r.worldView()
	r.pipeline().Ob.DrawPawnArt(screen, p, wx, wy, tile, sw, sh)
}

func (r *Renderer) drawForbiddenMark(screen *ebiten.Image, wx, wy int, tile float64, sw, sh int) {
	r.worldView()
	r.pipeline().Ob.DrawForbiddenMark(screen, wx, wy, tile, sw, sh)
}

func (r *Renderer) drawChatBubbles(screen *ebiten.Image, tile float64) {
	r.worldView()
	r.pipeline().Lb.DrawChatBubbles(screen, tile)
}

func (r *Renderer) drawSteamPuffs(screen *ebiten.Image, w game.WorldView, tile float64) {
	r.worldView()
	r.pipeline().Lb.DrawSteamPuffs(screen, w, tile)
}
