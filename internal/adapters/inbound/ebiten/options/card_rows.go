package options

import (
	"fmt"

	"rimworld-go/internal/platform/config"
)

// The colonist card's group of rows (RHUD-001, 003, 028): whether the card is
// on, where it sits, how tall its pane is and where it floats. Everything else
// about the card is in its own setup dialog (the gear on the card).

// cardAnchors name the nine places of a floating card, row by row from the top
// left (config.HUDCard.Anchor).
var cardAnchors = []string{"Top left", "Top", "Top right", "Left", "Centre", "Right", "Bottom left", "Bottom", "Bottom right"}

const cardHeightStep = 20

// Why a card row is off.
const (
	cardPlainReason    = "The pane keeps its plain look, so the card floats. Turn on \"Restyle the pane\" in the card's setup to dock it."
	cardFloatingReason = "A floating card has its own size; only a docked card sets the pane's height."
	cardDockedReason   = "The card is docked in the pane. Let it float to place it on the map."
)

// cardRows are the card's rows, in the second column.
func cardRows() []OptionRow {
	return []OptionRow{
		{Key: "card", Label: "Colonist card", kind: rowToggle, Col: 1,
			Tip:   "A card of the selected colonist or animal: needs, skills, health and standing at a glance. Its gear opens the card's setup.",
			value: func(s config.Settings) string { return onOff(s.HUD.Enabled) },
			apply: func(s *config.Settings, _ int) { s.HUD.Enabled = !s.HUD.Enabled }},
		{Key: "carddock", Label: "Card sits", kind: rowChoice, Col: 1,
			Tip: "In the inspector pane, or floating over the map.",
			value: func(s config.Settings) string {
				if s.HUD.Docked {
					return "In the pane"
				}
				return "Over the map"
			},
			apply: func(s *config.Settings, _ int) { s.HUD.Docked = !s.HUD.Docked }},
		{Key: "cardheight", Label: "Pane height", kind: rowStep, Col: 1,
			Tip:   "How tall the inspector pane is while it holds the card.",
			value: func(s config.Settings) string { return fmt.Sprintf("%d", s.HUD.Pane.Height) },
			frac:  func(s config.Settings) float64 { return float64(s.HUD.Pane.Height-188) / (800 - 188) },
			apply: func(s *config.Settings, d int) {
				s.HUD.Pane.Height = max(188, min(800, s.HUD.Pane.Height+d*cardHeightStep))
			}},
		{Key: "cardspot", Label: "Floating card at", kind: rowChoice, Col: 1,
			Tip:   "Where on the screen a floating card starts. Its nudge is in the card's setup.",
			value: func(s config.Settings) string { return cardAnchors[max(0, min(len(cardAnchors)-1, s.HUD.Card.Anchor))] },
			apply: func(s *config.Settings, _ int) { s.HUD.Card.Anchor = (s.HUD.Card.Anchor + 1) % len(cardAnchors) }},
	}
}

// cardLock is why a card row cannot change with these settings ("" when it
// can).
func cardLock(key string, h config.HUD) string {
	switch key {
	case "carddock":
		if !h.Pane.Restyle {
			return cardPlainReason
		}
	case "cardheight":
		if !h.Pane.Restyle {
			return cardPlainReason
		}
		if !h.Docked {
			return cardFloatingReason
		}
	case "cardspot":
		if h.Docked {
			return cardDockedReason
		}
	}
	return ""
}
