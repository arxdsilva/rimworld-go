package pathstage

// Budget counts one tick's long searches (not saved: it starts afresh
// every tick). A search past the tick's limit is deferred: the caller
// fails it without postponing and asks again next tick.
type Budget struct {
	tick     uint64
	used     int
	deferred bool // the last Take was refused
}

// Take spends one of the limit searches tick now allows; false, and
// Deferred, once they are spent.
func (b *Budget) Take(now uint64, limit int) bool {
	if b.tick != now {
		b.tick, b.used = now, 0
	}
	if b.used >= limit {
		b.deferred = true
		return false
	}
	b.used++
	return true
}

// Clear forgets the last refusal (a new search is asked).
func (b *Budget) Clear() { b.deferred = false }

// Deferred reports that the last Take since Clear was refused.
func (b *Budget) Deferred() bool { return b.deferred }
