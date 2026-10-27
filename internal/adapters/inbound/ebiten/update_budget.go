package ebiten

// tickDebt is the Update time budget's state (PERF-002 HS7): at 3x and
// faster, an Update stops stepping once it has run for the budget and
// carries the ticks it still owes to the next Update, so a heavy raid tick
// delays the simulation a frame instead of stalling the display. The
// order of ticks never changes, only which Update runs them.
//
//   - owed: ticks carried over, at most one Update's nominal count;
//   - nominal: the speed tier's ticks per Update the debt was taken at;
//   - held: how many Updates stopped short (the probe and the overlay
//     report it);
//   - maxOwed: the largest debt seen.
//
// PERF-002 wave 0 contract: the type only; the HS7 agent adds the policy.
type tickDebt struct {
	owed, nominal, held, maxOwed int
}
