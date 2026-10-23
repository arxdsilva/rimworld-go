// Package scenario builds deterministic benchmark fixtures for the engine
// scale work (NG-1 §6.2): grown colonies assembled only through the
// game's public dev hooks and typed commands, warmed up, and returned as
// save bytes so every benchmark Loads an identical starting state.
package scenario
