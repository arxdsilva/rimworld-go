package ebiten

import ebiten "github.com/hajimehoshi/ebiten/v2"

// The resource list (PLAY-018: HUD-TOP owns it): stored goods as icon +
// count rows, top-left, in the layout's Resources rect. Until migrated it
// is the legacy readouts at the start of the top strip.

// drawResources draws the resource list (legacy: the top strip readouts).
func (a *App) drawResources(screen *ebiten.Image, sw, sh int) {
	x := hudPad
	for _, l := range resourceReadouts(a.renderer.world()) { // Food, Wood, Stone, Steel (+ Silver, Gold)
		drawLabel(screen, x, 10, l)
		x += max(90, int(textW(l, tsBody))+18)
	}
}

// resourcesHeight is the resource list's content height the layout
// reserves (0 while drawn the legacy way).
func (a *App) resourcesHeight() float32 { return 0 }

// resourceButtons are the list's buttons (category headers, rows).
func (a *App) resourceButtons() []button { return nil }

// resourcesChrome is the list's legacy area (none: inside the top strip).
func (a *App) resourcesChrome() []rect { return nil }

// applyResourceClick handles a "res." click.
func (a *App) applyResourceClick(id string) {}

// rightClickResources answers a right-click over the list.
func (a *App) rightClickResources(mx, my int) bool { return false }

// resourcesUI declares the list's widget-kit parts (stub).
func (a *App) resourcesUI(sw, sh int) {}
