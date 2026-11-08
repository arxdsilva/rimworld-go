// Package game is the client's composition root: it builds the concrete
// adapters (settings, saves, colonist card presets, custom scenarios,
// sound, hardware, usage data, the update check), the colony builders and
// the dev scenes, hands them to the Ebitengine App as its ports, and runs
// the window (Run). It moved out of cmd/game (2026-11, GAR); only cmd/
// imports it (internal/arch TestOnlyCmdImportsCompose), and it imports no
// third-party module: the Ebitengine calls go through Display.
package game
