package ebiten

// Fast-forward (CI-004). A journey that waits on the simulation (a kill, a
// meal, a sleeper) spent nearly all its time in the HUD: under -race one
// colony Update lays the screen out for about 3 ms, while Very fast's six
// ticks cost a fraction of that. ff keeps the real loop: a speed key starts
// the colony, every Update goes through App.Update (its commands, its pause
// on a threat, the renderer's sync), and Space stops it. After each Update
// that advanced the colony it steps the simulation on to the stride, as a
// faster speed tier would; an Update that ran no tick (paused) steps nothing.

// ffStride is the ticks one fast-forward Update covers by default.
const ffStride = 60

// ff runs the colony at Very fast until done reports true or maxTicks ticks
// have passed, calling done after every stride ticks, and pauses it again.
// It returns whether done held.
func (j *journey) ff(maxTicks, stride int, done func() bool) bool {
	j.t.Helper()
	return j.ffRun("Digit3", maxTicks, stride, nil, done)
}

// ffWatch is ff for a journey that samples something transient (a
// position, an item lying for a few ticks): the colony runs at Normal speed,
// one tick per Update, and watch runs after every tick, the Update's and
// each one stepped beyond it; the run stops on the tick watch reports true,
// so what it saw is still there. done runs after every stride ticks.
func (j *journey) ffWatch(maxTicks, stride int, watch, done func() bool) bool {
	j.t.Helper()
	return j.ffRun("Digit1", maxTicks, stride, watch, done)
}

// ffRun runs at most maxTicks/(the key's ticks per Update) Updates, the
// bound a plain loop at that speed over the same ticks has, so a colony it
// cannot move (a threat pause) fails after the same Updates.
func (j *journey) ffRun(key string, maxTicks, stride int, watch, done func() bool) bool {
	j.t.Helper()
	per := speedNormal.ticksPerFrame()
	if key == "Digit3" {
		per = speedVeryFast.ticksPerFrame()
	}
	stride = max(stride, per)
	start := j.g.Tick()
	j.key(key, 0)
	defer j.key("Space", 0)
	saw := func(moved bool) bool { return watch != nil && moved && watch() }
	if saw(j.g.Tick() != start) {
		return true
	}
	for i := 0; i <= maxTicks/per && int(j.g.Tick()-start) < maxTicks; i++ {
		before := j.g.Tick()
		j.update(1)
		if saw(j.g.Tick() != before) {
			return true
		}
		if ran := int(j.g.Tick() - before); ran > 0 {
			n := min(stride-ran, maxTicks-int(j.g.Tick()-start))
			switch {
			case n <= 0:
			case watch == nil:
				j.g.Step(n)
			default:
				for k := 0; k < n; k++ {
					j.g.Step(1)
					if watch() {
						return true
					}
				}
			}
		}
		if done != nil && done() {
			return true
		}
	}
	return false
}

// ffUntil replaces a Very fast run under until: the colony runs until cond
// holds, over the ticks maxUpdates Very fast Updates cover, and the journey
// fails as until does when it never holds. cond must be a state that lasts
// (a load that only grows, pods that landed), not a passing moment.
func (j *journey) ffUntil(cond func() bool, maxUpdates int) {
	j.t.Helper()
	if cond() {
		return
	}
	if !j.ff(maxUpdates*speedVeryFast.ticksPerFrame(), ffStride, cond) && !cond() {
		j.t.Fatalf("condition not met after %d Very fast Updates' ticks (tick %d)", maxUpdates, j.g.Tick())
	}
}
