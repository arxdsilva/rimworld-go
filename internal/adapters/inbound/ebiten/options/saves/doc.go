// Package saves is the client's book of saved games: saving the running
// colony to its slot (manual saves, the daily autosave and its rotation,
// the commitment colony's one slot), the background saves' reports, saving
// before leaving, and the Load screen's list. Book reaches the program
// only through Shell (deps.go); the root builds it and options.Screen
// holds it as Saves.
package saves
