package devtools

import (
	"fmt"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/incident"
)

// StorySource is the storyteller's state for the story panel (EAI-034):
// the game answers it, read only.
type StorySource interface {
	StoryDebug() game.StoryDebugView
}

// storyView is the story state now (zero without a colony that answers).
func (p *Panel) storyView() game.StoryDebugView {
	if s, ok := p.sh.Env().Game.(StorySource); ok {
		return s.StoryDebug()
	}
	return game.StoryDebugView{}
}

// maxStoryRows is how many queued events the story panel lists.
const maxStoryRows = 8

// storyLines are the story panel's lines, title first: the threat points
// and the wealth they come from, the tick of the next threat and the
// queued events, soonest first.
func (p *Panel) storyLines() []string {
	v := p.storyView()
	now := uint64(p.sh.Tick())
	out := []string{
		fmt.Sprintf("Story  tick %d", now),
		fmt.Sprintf("threat points %d  wealth %d", v.Points, v.Wealth),
	}
	switch {
	case v.NextThreat == 0:
		out = append(out, "next threat: none planned")
	case v.NextThreat <= now:
		out = append(out, fmt.Sprintf("next threat: due (t%d)", v.NextThreat))
	default:
		out = append(out, fmt.Sprintf("next threat: t%d (in %d ticks)", v.NextThreat, v.NextThreat-now))
	}
	if len(v.Queue) == 0 {
		return append(out, "queue: empty")
	}
	out = append(out, fmt.Sprintf("queue: %d", len(v.Queue)))
	for i, q := range v.Queue {
		if i == maxStoryRows {
			out = append(out, fmt.Sprintf("... and %d more", len(v.Queue)-i))
			break
		}
		row := fmt.Sprintf("t%d  %s", q.At, incident.DefOf(q.ID).Key)
		if q.Points > 0 {
			row += fmt.Sprintf("  %d pts", q.Points)
		}
		out = append(out, row)
	}
	return out
}

// storySize is the story panel's size.
func (p *Panel) storySize() (w, h float32) {
	lines := p.storyLines()
	return gfx.MaxTextW(lines, gfx.Small) + 20, float32(len(lines)*devStep + 12)
}

// drawStory paints the story panel in r.
func (p *Panel) drawStory(screen *ebiten.Image, r gfx.Rect) {
	uikit.DrawPanel(screen, r.X, r.Y, r.W, r.H)
	for i, l := range p.storyLines() {
		gfx.Text(screen, l, float64(r.X)+8, float64(r.Y)+4+float64(i*devStep), gfx.Small)
	}
}

// storyActions are the debug menu's story entries: the panel's toggle and
// the events the story fires (through Tools.FireIncident).
func (p *Panel) storyActions(fire func(key, name string) Action) []Action {
	label := "Story panel: show"
	if p.State.Story {
		label = "Story panel: hide"
	}
	return []Action{
		{Label: label, Run: func(Tools) string { p.State.Story = !p.State.Story; return "" }},
		fire("refugee_chased", "chased refugee"),
	}
}
