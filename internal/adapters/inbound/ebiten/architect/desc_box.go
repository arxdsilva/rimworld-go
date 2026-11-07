package architect

import (
	"strings"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
)

// The category panel's search row (UIS-053) and the description box's
// scroll (UIS-054), declared to the widget kit each Update (Panel.UI). The
// search filters the designators by name across every category: while it
// holds text the grid shows the matches instead of the open category.

// Widget ids of the panel's kit parts.
const (
	SearchID     = "arch.search"
	DescScrollID = "arch.desc"
)

// searchMaxLen caps the search text (runes).
const searchMaxLen = 32

// Panel is the Architect's search row and description-box scroll.
type Panel struct {
	*Tool
	query string // the search row's text
}

// SearchRect is the search row: the category panel's bottom ArchSearchH.
func SearchRect(l hudlayout.Layout) gfx.Rect {
	c := l.ArchCats
	if c.Empty() {
		return gfx.Rect{}
	}
	return gfx.Rect{X: c.X, Y: c.Bottom() - hudlayout.ArchSearchH, W: c.W, H: hudlayout.ArchSearchH}
}

// UI declares the search field and the description box's scroll view. The
// host calls it once per Update while the colony HUD is up.
func (p *Panel) UI() {
	if !p.Open || p.sh.Env().UIHidden {
		return
	}
	l := p.sh.Layout()
	if l.ArchCats.Empty() {
		return
	}
	k := p.sh.Kit()
	k.TextField(SearchID, SearchRect(l).Inset(2), &p.query, uikit.TextFieldSpec{
		MaxLen: searchMaxLen, Placeholder: p.sh.T("Search"), Tip: p.sh.T("Search"),
	})
	for i := len(k.Cur) - 1; i >= 0; i-- { // a click edits it; Tab stays the Architect's key
		if k.Cur[i].ID == SearchID {
			k.Cur[i].Focusable = false
			break
		}
	}
	if d := l.ArchDesc; !d.Empty() {
		k.ScrollArea(DescScrollID, d, p.Plan().H, uikit.ScrollOpts{Step: float32(gfx.LineStep(gfx.Body)) * 2})
	}
}

// Query is the search text, trimmed ("" shows the open category).
func (p *Panel) Query() string { return strings.TrimSpace(p.query) }

// SetQuery sets the search text (a test, or a host that clears it).
func (p *Panel) SetQuery(q string) { p.query = q }

// matches are the tools whose name (in the player's language) holds q,
// case-insensitively, in catalog order; a family shows one card.
func (p *Panel) matches(q string) []ToolDef {
	q = strings.ToLower(q)
	var out []ToolDef
	for _, d := range AllTools() {
		if d.Family != "" && d.Family != d.ID {
			continue
		}
		if strings.Contains(strings.ToLower(p.tx.localize(p.sh, d).Label), q) {
			out = append(out, d)
		}
	}
	return out
}

// Offset is the description box's scroll offset, held to its content.
func (p *Panel) Offset() float32 {
	if !p.Open {
		return 0
	}
	d := p.sh.Layout().ArchDesc
	k := p.sh.Kit()
	if d.Empty() || k == nil || k.Scroll == nil {
		return 0
	}
	return max(0, min(p.Plan().H-d.H, k.Scroll[DescScrollID]))
}

// ScrollTo sets the description box's scroll offset (clamped when read).
func (p *Panel) ScrollTo(off float32) {
	k := p.sh.Kit()
	if k.Scroll == nil {
		k.Scroll = map[string]float32{}
	}
	k.Scroll[DescScrollID] = off
}
