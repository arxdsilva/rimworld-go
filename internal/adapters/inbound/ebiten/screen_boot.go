package ebiten

import (
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

func (a *App) drawBoot(screen *ebiten.Image, sw, sh int) {
	phase := a.bootPhase()
	if phase >= len(bootPhases) {
		phase = len(bootPhases) - 1
	}
	body := []string{
		"initializing...",
		"",
		"phase: " + bootPhases[phase],
		fmtProgress(phase+1, len(bootPhases)),
	}
	if a.artWarn != "" {
		body = append(body, "", "warning: "+a.artWarn)
	}
	a.fillMenu(screen, sw, sh, "rimworld-go", body, bodyDefaultX)
	gfx.Text(screen, "escape to skip", 56, float64(sh-40), gfx.WithColor(gfx.Small, uikit.TextDim))
}

func fmtProgress(n, total int) string {
	if total < 1 {
		return ""
	}
	filled := n * 20 / total
	bar := make([]byte, 20)
	for i := range bar {
		if i < filled {
			bar[i] = '#'
		} else {
			bar[i] = '-'
		}
	}
	return "[" + string(bar) + "]"
}
