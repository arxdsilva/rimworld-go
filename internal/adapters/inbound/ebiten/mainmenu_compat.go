package ebiten

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/mainmenu"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/platform/config"
	"rimworld-go/internal/platform/input"
)

// The main menu lives in mainmenu (ARCH-005). These aliases and one-line
// wrappers keep the root's call sites unchanged while the screens move out
// one package at a time; the last mover deletes this file (SOLID_GO.md §11).

type menuDiorama = mainmenu.Diorama

const (
	// DefaultMenuSplashRel is the original main-menu colonist illustration.
	DefaultMenuSplashRel = mainmenu.DefaultMenuSplashRel

	menuRailW    = mainmenu.RailW
	menuBtnH     = mainmenu.BtnH
	menuBtnGap   = mainmenu.BtnGap
	ramDismissID = mainmenu.RAMDismissID
)

// LocateMenuSplash walks parents of cwd looking for DefaultMenuSplashRel.
func LocateMenuSplash() (string, error) { return mainmenu.LocateMenuSplash() }

const (
	menuEmbers = mainmenu.EmberCount
	menuMotes  = mainmenu.MoteCount
)

func menuParticles(s config.Settings) (embers, motes int) { return mainmenu.Particles(s) }

func ramNoticeStyle() textStyle { return mainmenu.RAMNoticeStyle() }

func creditsLines() []string { return mainmenu.CreditsLines() }

func loadPNG(path string) (*ebiten.Image, error) { return mainmenu.LoadPNG(path) }

func loadMenuDiorama(root string) (*menuDiorama, error) { return mainmenu.LoadDiorama(root) }

func (a *App) mainMenuUI(sw, sh int)                    { a.menuScreen.Declare(sw, sh) }
func (a *App) drawMainMenu(s *ebiten.Image, sw, sh int) { a.menuScreen.Draw(s, sw, sh) }
func (a *App) mainMenuButtons(sw, sh int) []button      { return a.menuScreen.Buttons(sw, sh) }
func (a *App) loadSplash()                              { a.menuScreen.LoadSplash() }
func (a *App) loadMenuScene()                           { a.menuScreen.LoadScene() }

func (a *App) ramNoticeCard(sw, sh int, text string) rect {
	return a.menuScreen.RAMNoticeCard(sw, sh, text)
}

func (a *App) drawMenuDiorama(screen *ebiten.Image, sw, sh int, dim float64) bool {
	return a.menuScreen.DrawDiorama(screen, sw, sh, dim)
}

// newestSlot is the index of the loadable save written last (-1 when none).
func (a *App) newestSlot() int { return mainmenu.NewestSlot(a.opt.Slots) }

// creditsButtons is the Credits screen's Back button.
func (a *App) creditsButtons(sw, sh int) []button {
	return []button{{ID: "credits.back", Label: "Back", X: 40, Y: footerY(sh), W: 140, H: 36}}
}

func (a *App) drawCredits(screen *ebiten.Image, sw, sh int) {
	a.paintMenu(screen, sw, sh, "Credits", creditsLines(), a.creditsButtons(sw, sh))
}

// hasSaves reports a loadable save exists (for Continue).
func (a *App) hasSaves() bool {
	if !a.canLoad() {
		return false
	}
	if !a.opt.SlotsRead {
		a.opt.SlotsRead = true
		a.refreshSlots()
	}
	for _, sl := range a.opt.Slots {
		if sl.Err == "" {
			return true
		}
	}
	return false
}

// lowRAMNotice is the notice to show now and the amount of memory it is
// for: no text when the hardware is unknown, big enough, or the player
// already dismissed the notice for this amount (Gameplay.RAMNoticeSeen).
func (a *App) lowRAMNotice() (text string, total uint64) {
	total = a.hardwareTotal()
	if total == 0 || total >= minimumRAM() || a.settings.Gameplay.RAMNoticeSeen == total {
		return "", 0
	}
	return lowRAMText(total), total
}

// dismissRAMNotice remembers the notice was seen for total bytes of RAM.
func (a *App) dismissRAMNotice(total uint64) {
	s := a.settings
	s.Gameplay.RAMNoticeSeen = total
	a.setSettings(s)
}

// menuHost is what the main menu sees of the App: the ports roles it calls
// and mainmenu.Host.
type menuHost struct{ a *App }

var _ mainmenu.Shell = menuHost{}

func (h menuHost) Size() (int, int)              { return shellSize{h.a}.Size() }
func (h menuHost) Cursor() (int, int)            { return shellSize{h.a}.Cursor() }
func (h menuHost) MapCursor() (int, int)         { return shellSize{h.a}.MapCursor() }
func (h menuHost) MapScale() float64             { return shellSize{h.a}.MapScale() }
func (h menuHost) Settings() config.Settings     { return shellSettings{h.a}.Settings() }
func (h menuHost) KeyName(x input.Action) string { return shellSettings{h.a}.KeyName(x) }
func (h menuHost) KeyHint(x input.Action) string { return shellSettings{h.a}.KeyHint(x) }
func (h menuHost) Kit() *uikit.Kit               { return shellWidgets{h.a}.Kit() }

func (h menuHost) LoadSplash() (*ebiten.Image, error) {
	if h.a.cfg.LoadSplash == nil {
		return nil, mainmenu.ErrNoLoader
	}
	return h.a.cfg.LoadSplash()
}

func (h menuHost) LoadMenuScene() (*menuDiorama, error) {
	if h.a.cfg.LoadMenuScene == nil {
		return nil, mainmenu.ErrNoLoader
	}
	return h.a.cfg.LoadMenuScene()
}

func (h menuHost) Version() string                { return h.a.cfg.Version }
func (h menuHost) MenuTick() uint64               { return h.a.menuTick }
func (h menuHost) HasSaves() bool                 { return h.a.hasSaves() }
func (h menuHost) LowRAMNotice() (string, uint64) { return h.a.lowRAMNotice() }
func (h menuHost) DismissRAMNotice(total uint64)  { h.a.dismissRAMNotice(total) }
func (h menuHost) DrawFlame(dst *ebiten.Image, sx, sy float32, t, tile, k float64) {
	drawFlameScreen(dst, sx, sy, t, tile, k)
}
