package ebiten

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/inspector"
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
func (h ordersHost) Icon(name string) *ebiten.Image { return h.a.arch.Icon(name) }
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
func (h ordersHost) BuildingTitle(b game.BuildingView) string { return inspector.BuildingTitle(b) }

type formationMove struct {
	pawn entity.EntityID
	x, y int
}

func (a *App) chooseFloat(id string) { a.ord.ChooseFloat(id) }

func (a *App) applySearchClick(id string) {
	a.ord.ApplySearchClick(id)
}

type searchHit struct {
	label, kind string
	c           selection.Cand
	x, y        int
}
