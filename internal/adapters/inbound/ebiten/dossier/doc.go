// Package dossier draws the colonist card (RHUD): a player-edited tree of value
// widgets for one pawn or animal, docked in the inspect pane or floating. It
// reads Simulation read models through Reader and emits typed commands through
// Actions; it never touches game state. Pure logic lives in sheet.
package dossier
