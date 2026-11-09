package saves

import (
	"image/color"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game/savefmt"
)

// One save as a list shows it (SAV-011), shared by the Load screen and the
// Save dialog: line 1 is the save's title with its tags, line 2 the
// colony's summary, the tooltip all of it. A field the save's header does
// not have (an older save) is left out, never shown as zero.

// autosaveTag marks a save the game wrote itself.
const autosaveTag = "Autosave"

// The compat tags (savefmt.Compat) and the confirm and refusal texts.
const (
	olderTag   = "Older version"
	newerTag   = "Newer version"
	contentTag = "Different content"
	olderAsk   = "Saved by an older version. It will be upgraded when it loads; saving again writes the new format, which older versions can't read."
	newerWhy   = "Made with a newer version (build %s). Update the game to load it."
	contentAsk = "Made with different content: %s. Load it anyway?"
	sep        = " · "
)

// Row is a save's text as a list shows it.
type Row struct {
	Title string // the save's name
	Tags  []Tag  // drawn right-aligned on line 1, first rightmost
	Sub   string // line 2: colony, date, colonists, start, build
	// Bad is why the save cannot be loaded ("": it can); line 2 then shows
	// it in red.
	Bad string
	Tip string // every field on one line (the hover card)
}

// Tag is a short coloured label on a row's first line.
type Tag struct {
	Text  string
	Color color.RGBA
}

// FormatRow is sl as a list row, in tr's language.
func FormatRow(tr ports.Texts, sl ports.SaveSlot) Row {
	r := Row{Title: slotTitle(sl), Sub: subLine(tr, sl), Bad: badReason(tr, sl)}
	if !sl.Modified.IsZero() {
		r.Tags = append(r.Tags, Tag{sl.Modified.Format("Jan 2 15:04"), uikit.TextDim})
	}
	if t, ok := compatTag(tr, sl); ok {
		r.Tags = append(r.Tags, t)
	}
	if sl.Ending != "" {
		r.Tags = append(r.Tags, Tag{tr.T(EndingTag), uikit.Good})
	}
	if sl.Commitment {
		r.Tags = append(r.Tags, Tag{tr.T(commitTag), uikit.Accent})
	}
	if sl.Autosave {
		r.Tags = append(r.Tags, Tag{tr.T(autosaveTag), uikit.TextDim})
	}
	r.Tip = rowTip(tr, sl, r)
	return r
}

// slotTitle is the save's name: the title the player gave it, else its slot.
func slotTitle(sl ports.SaveSlot) string {
	if sl.Title != "" {
		return sl.Title
	}
	return sl.Name
}

// subLine is "Colony · Day N, Season · K colonists · storyteller/difficulty
// · biome · build X", without the parts the header lacks.
func subLine(tr ports.Texts, sl ports.SaveSlot) string {
	parts := []string{sl.ColonyName, dayText(tr, sl), colonistsText(tr, sl.Colonists),
		joinNonEmpty("/", sl.Storyteller, sl.Difficulty), sl.Biome}
	if sl.Build != "" {
		parts = append(parts, tr.Tf("build %s", sl.Build))
	}
	return joinNonEmpty(sep, parts...)
}

// dayText is "Day N, Season" from the header, else the save's date line;
// an older header has neither and gives "".
func dayText(tr ports.Texts, sl ports.SaveSlot) string {
	switch {
	case sl.Season != "":
		return tr.Tf("Day %d, %s", sl.Day, sl.Season)
	case sl.Day > 0:
		return tr.Tf("Day %d", sl.Day)
	}
	return sl.Date
}

func colonistsText(tr ports.Texts, n int) string {
	if n == 1 {
		return tr.T("1 colonist")
	}
	return tr.Tf("%d colonists", n)
}

// badReason is why sl cannot be loaded: its error, or a newer build's
// format.
func badReason(tr ports.Texts, sl ports.SaveSlot) string {
	switch {
	case sl.Err != "":
		return tr.Tf("cannot load: %s", sl.Err)
	case savefmt.Compat(sl.Compat) == savefmt.CompatNewer:
		return tr.Tf(newerWhy, buildOf(tr, sl))
	}
	return ""
}

// buildOf is the build that wrote sl, or "unknown".
func buildOf(tr ports.Texts, sl ports.SaveSlot) string {
	if sl.Build != "" {
		return sl.Build
	}
	return tr.T("unknown")
}

// compatTag is the tag of a save this build reads differently.
func compatTag(tr ports.Texts, sl ports.SaveSlot) (Tag, bool) {
	switch savefmt.Compat(sl.Compat) {
	case savefmt.CompatOlder:
		return Tag{tr.T(olderTag), uikit.Warning}, true
	case savefmt.CompatNewer:
		return Tag{tr.T(newerTag), uikit.Danger}, true
	case savefmt.CompatContent:
		return Tag{tr.T(contentTag), uikit.Warning}, true
	}
	return Tag{}, false
}

// rowTip is the whole row on one line, with what the lines leave out: the
// file's slot, map size, format version and notice.
func rowTip(tr ports.Texts, sl ports.SaveSlot, r Row) string {
	parts := []string{r.Title}
	if sl.Name != r.Title {
		parts = append(parts, tr.Tf("file %s", sl.Name))
	}
	for i := len(r.Tags) - 1; i >= 0; i-- {
		parts = append(parts, r.Tags[i].Text)
	}
	parts = append(parts, r.Sub)
	if sl.MapSize > 0 {
		parts = append(parts, tr.Tf("map %d", sl.MapSize))
	}
	if sl.Version > 0 {
		parts = append(parts, tr.Tf("format %d", sl.Version))
	}
	parts = append(parts, sl.Notice, r.Bad)
	return joinNonEmpty(sep, parts...)
}

func joinNonEmpty(by string, parts ...string) string {
	out := parts[:0:0]
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, by)
}

// DrawRow paints r in the w x h row at x, y: the title left and the tags
// right on line 1, the summary (or the red reason) on line 2, both cut to
// fit.
func DrawRow(dst *ebiten.Image, r Row, x, y, w, h float32) {
	bx, by := float64(x), float64(y)
	right := float64(x+w) - 12
	for _, t := range r.Tags {
		gfx.Text(dst, t.Text, right, by+4, gfx.TextStyle{Size: gfx.Small.Size, Color: t.Color, Align: gfx.AlignRight})
		right -= float64(gfx.TextW(t.Text, gfx.Small)) + 16
	}
	gfx.Text(dst, gfx.Ellipsize(r.Title, gfx.Label, right-bx-12), bx+12, by+3, gfx.Label)
	sub, c := r.Sub, color.Color(uikit.TextDim)
	if r.Bad != "" {
		sub, c = r.Bad, uikit.Danger
	}
	gfx.Text(dst, gfx.Ellipsize(sub, gfx.Small, float64(w)-24), bx+12, by+float64(h)-gfx.LineH(gfx.Small)-2, gfx.WithColor(gfx.Small, c))
}
