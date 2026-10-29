// Package door is the pure door model (DEF-3): which kinds are doors, how
// fast one opens for its stuff and power, and the open/close state that a
// passing creature, a "hold open" order and the auto-close timer drive.
// Applying it (who passes, what an open door lets through) is the combat
// cluster's job (internal/application/game/combat/door.go).
package door
