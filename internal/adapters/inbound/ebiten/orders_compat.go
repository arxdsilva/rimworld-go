package ebiten

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/orders"
	"rimworld-go/internal/adapters/inbound/ebiten/selection"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// orders.Controller owns the right-click order flow (ARCH-005): the float
// menu, the formation drag and the Z search box. ordersHost is the root's
// side of orders.Host and newOrders builds the feature for NewApp. The
// aliases and one-line wrappers below keep the root's other call sites
// unchanged until the sweep (SOLID_GO.md §11).

type ordersHost struct{ a *App }

var (
	_ orders.Host   = ordersHost{}
	_ orders.Labels = ordersHost{}
)

// ordersShell is the feature's view of the App: the ports roles plus its Host.
type ordersShell struct {
	shell
	ordersHost
}

func newOrders(a *App) *orders.Controller {
	return orders.New(ordersShell{a.shell, ordersHost{a}})
}

func (h ordersHost) Frame() (*inputframe.Frame, uint64) { return &h.a.in, h.a.updates }
func (h ordersHost) Selection() *selection.Model        { return h.a.sel }
func (h ordersHost) Hostiles() []game.PawnView {
	if h.a.renderer == nil {
		return nil
	}
	return h.a.renderer.hostiles()
}
func (h ordersHost) ChromeHit(mx, my int) bool {
	sw, sh := h.a.size()
	return h.a.colonyChromeHit(mx, my, sw, sh)
}
func (h ordersHost) Layout() hudlayout.Layout       { return h.a.hud() }
func (h ordersHost) Icon(name string) *ebiten.Image { return h.a.icon(name) }
func (h ordersHost) OrdersAt(id entity.EntityID, tx, ty int) []game.OrderOption {
	if h.a.renderer == nil {
		return nil
	}
	return h.a.renderer.game.OrdersAt(id, tx, ty)
}
func (h ordersHost) CenterOnMap(wx, wy float64) {
	if h.a.renderer != nil {
		h.a.centerOnMap(wx, wy)
	}
}
func (h ordersHost) BuildingTitle(b game.BuildingView) string { return buildingTitle(b) }

const (
	floatRowH  = orders.FloatRowH
	floatHintH = orders.FloatHintH
)

type formationMove struct {
	pawn entity.EntityID
	x, y int
}

func formationCells(x, y, n int, free func(x, y int) bool) [][2]int {
	return orders.FormationCells(x, y, n, free)
}

func lineCells(x0, y0, x1, y1, n int, free func(x, y int) bool) [][2]int {
	return orders.LineCells(x0, y0, x1, y1, n, free)
}

func assignCells(pawns []game.PawnView, cells [][2]int) []formationMove {
	var out []formationMove
	for _, m := range orders.AssignCells(pawns, cells) {
		out = append(out, formationMove{pawn: m.Pawn, x: m.X, y: m.Y})
	}
	return out
}

func fireOrders(p game.PawnView, w game.WorldView, tx, ty int) []game.OrderOption {
	return orders.FireOrders(p, w, tx, ty)
}

func floatIcon(o game.OrderOption) string { return orders.FloatIcon(o) }

func routeTint(c color.RGBA, a uint8) color.RGBA { return orders.RouteTint(c, a) }

func (a *App) closeFloat()  { a.ord.CloseFloat() }
func (a *App) closeSearch() { a.ord.CloseSearch() }
func (a *App) openSearch()  { a.ord.OpenSearch() }
func (a *App) toggleDraft() { a.ord.ToggleDraft() }
func (a *App) handleOrders() {
	a.ord.HandleOrders()
}
func (a *App) chooseFloat(id string) { a.ord.ChooseFloat(id) }
func (a *App) openPick(x, y float32, up bool, entries []game.OrderOption, pick func(i int)) {
	a.ord.OpenPick(x, y, up, entries, pick)
}
func (a *App) openOrders(tx, ty int, sx, sy float32) {
	a.ord.OpenOrders(tx, ty, sx, sy)
}
func (a *App) rightClickTile(tx, ty int, sx, sy float32) {
	a.ord.RightClickTile(tx, ty, sx, sy)
}
func (a *App) floatRect() button { return a.ord.FloatRect() }
func (a *App) floatButtons() []button {
	return a.ord.FloatButtons()
}
func (a *App) drawFloat(screen *ebiten.Image)     { a.ord.DrawFloat(screen) }
func (a *App) drawFormation(screen *ebiten.Image) { a.ord.DrawFormation(screen) }
func (a *App) drawSearch(screen *ebiten.Image, sw int) {
	a.ord.DrawSearch(screen, sw)
}
func (a *App) searchRect(sw int) button { return a.ord.SearchRect(sw) }
func (a *App) searchButtons(sw int) []button {
	return a.ord.SearchButtons(sw)
}
func (a *App) searchTyping() { a.ord.SearchTyping() }
func (a *App) applySearchClick(id string) {
	a.ord.ApplySearchClick(id)
}

type searchHit struct {
	label, kind string
	c           selCand
	x, y        int
}

func (a *App) searchHits() []searchHit {
	var out []searchHit
	for _, h := range a.ord.SearchHits() {
		out = append(out, searchHit{label: h.Label, kind: h.Kind, c: h.C, x: h.X, y: h.Y})
	}
	return out
}
