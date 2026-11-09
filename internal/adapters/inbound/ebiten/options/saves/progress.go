package saves

// Unsaved progress (SAV-013): how far the colony's saves have caught up
// with it, in ticks. A save covers the colony as of the tick it was
// encoded; ticks after that stay unsaved. A save clears the progress only
// once it is written: a synchronous one at once, a background one when its
// result comes back, and then only up to its encode tick. A failure never
// clears it. Loading a save and starting a colony set a new baseline.

// encoded is one background save: its slot and the tick it was encoded at.
type encoded struct {
	slot string
	at   int64
}

// progressBegun is the progress of a colony that starts at tick: nothing saved.
func progressBegun(tick int64) progress { return progress{dirtyAt: tick, savedAt: tick} }

// progressLoaded is the progress of a colony just loaded at tick: all of it saved.
func progressLoaded(tick int64) progress { return progress{dirtyAt: tick, savedAt: tick, known: true} }

// seen notes the colony's tick.
func (p *progress) seen(tick int64) { p.dirtyAt = max(p.dirtyAt, tick) }

// saved notes a save encoded at tick that was written.
func (p *progress) saved(at int64) {
	p.seen(at)
	p.savedAt, p.known = max(p.savedAt, at), true
}

// started notes a background save of slot encoded at tick.
func (p *progress) started(slot string, at int64) {
	p.seen(at)
	p.inflight = append(p.inflight, encoded{slot, at})
}

// finished notes a background save's result: the oldest save of that slot
// in flight is written (clearing up to its encode tick) or failed.
func (p *progress) finished(r SaveResult) {
	for i, e := range p.inflight {
		if e.slot != r.Slot {
			continue
		}
		p.inflight = append(p.inflight[:i:i], p.inflight[i+1:]...)
		if r.Err == nil {
			p.saved(e.at)
		}
		return
	}
}

// dirty reports progress that no written save holds.
func (p progress) dirty() bool { return !p.known || p.dirtyAt > p.savedAt }

// saving reports a background save still being written.
func (p progress) saving() bool { return len(p.inflight) > 0 }
