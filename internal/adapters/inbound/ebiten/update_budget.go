package ebiten

import "time"

// The Update time budget (PERF-002 HS7): at 3x and faster, an Update stops
// stepping once it has run for updateBudget and carries the ticks it still
// owes to the next Update, so a heavy raid tick delays the simulation a
// frame instead of stalling the display. The order of Step calls never
// changes, only which Update runs them; captures never use the budget.
//
// Nothing here can flap: the decision to stop is stateless (wall time
// spent in this Update only) and the debt is capped at one Update's
// nominal ticks, so sustained overload settles at the k ticks an Update
// affords (alternating k and k+1 is the intended effective rate) and an
// idle Update pays the whole debt off. Interpolation stays continuous:
// animT at alpha 0 is Tick - stepped, the previous frame's Tick, whatever
// the count stepped.

// updateBudget is the wall time an Update may spend stepping before it
// carries the remaining ticks over (tuned with the fps probe).
const updateBudget = 5 * time.Millisecond

// tickDebt is the budget's state:
//
//   - owed: ticks carried over, at most one Update's nominal count;
//   - nominal: the speed tier's ticks per Update the debt was taken at;
//   - held: how many Updates stopped short (the probe and the overlay
//     report it);
//   - maxOwed: the largest debt seen.
type tickDebt struct {
	owed, nominal, held, maxOwed int
}

// advanceColony runs this Update's ticks (the tier's plus any owed) under
// the budget and stamps the Update Draw interpolates from.
func (a *App) advanceColony() {
	r := a.renderer
	d := &a.debt
	nominal := r.ticksWanted()
	if nominal != d.nominal { // a speed change or pause drops the debt
		d.owed, d.nominal = 0, nominal
	}
	want := nominal + d.owed
	var more func(int) bool
	if nominal > 1 && a.cfg.Capture == nil {
		start := a.now()
		more = func(int) bool { return a.now().Sub(start) < updateBudget }
	}
	done := r.advanceTicks(want, more)
	d.owed = min(want-done, nominal)
	if done < want {
		d.held++
	}
	d.maxOwed = max(d.maxOwed, d.owed)
	a.markUpdate()
}

// clearTickDebt drops owed ticks while the colony does not advance (pause
// overlay, background pause), so resuming never bursts.
func (a *App) clearTickDebt() { a.debt.owed = 0 }
