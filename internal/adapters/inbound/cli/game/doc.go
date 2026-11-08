// Package game is cmd/game's command line: the flags, their usage text and
// Parse, which turns os.Args into Options for internal/compose/game. It
// moved out of cmd/game (2026-11, GAR); cmd/game/main.go only calls Parse
// and the composition root's Run.
package game
