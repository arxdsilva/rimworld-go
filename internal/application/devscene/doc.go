// Package devscene builds the developer and capture scenes of cmd/game's
// -scenario flag: set pieces (walls, lategame, zoo, power, …) placed on a
// fresh colony through the game's dev hooks (internal/application/game
// dev_hooks.go), the benchmark's grown colony and the social colony. The
// scenes are one explicit table (scenes.go); nothing registers itself.
package devscene
