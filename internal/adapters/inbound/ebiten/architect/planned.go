package architect

// Placeholder cards (STK-AG, .agents/gaps/ARCHITECT_GAP_ANALYSIS.md): every
// core vanilla Architect item this game does not have yet sits in its
// vanilla category as a greyed card. It cannot be armed, placed or
// dragged, so it never emits a command; its tooltip says PlannedReason
// and one original sentence about the item. When an item lands, its
// placeholder goes and a working ToolDef takes its place in Tools.

// PlannedReason is a placeholder card's disabled reason.
const PlannedReason = "Not in this game yet"

// PlannedIcon is the generic greyed "planned" icon every placeholder shows
// (ui_icon/planned, STK-AR).
const PlannedIcon = "planned"

// PlannedPrefix starts every placeholder's tool id.
const PlannedPrefix = "planned_"

// Planned are the placeholder cards, by category in the vanilla order.
var Planned = []ToolDef{
	// Production

	// Power

	// Misc

	// Floors

	// Recreation

	// Ship
}
