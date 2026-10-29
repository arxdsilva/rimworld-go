// Package inspector is the selection's inspector: the pane with its tabs
// (the pawn's needs, gear, health, social, bio and log; animals, buildings,
// frames, items, plants, pens, troughs, caskets, groups), the command grid
// under it and the command producers for every selectable kind, and the
// marks drawn over what is selected. Pane reads the selection and the
// world through Shell and emits typed commands; the root plugs the
// Architect's tool commands in for an empty selection.
package inspector
