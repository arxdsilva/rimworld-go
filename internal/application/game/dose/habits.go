package dose

import (
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/drug"
	"rimworld-go/internal/domain/pawn/recreation"
	"rimworld-go/internal/domain/sim/entity"
)

// moodAllows is the policy's mood gate for id (CSE-010): it reads the mood only
// on a policy that has one.
func (d Drugs) moodAllows(id entity.EntityID, p drug.Preset) bool {
	if p.MoodGate() == 0 {
		return true
	}
	mood, potential := d.h.Habits.Mood(id)
	return p.MoodAllows(mood, potential, d.h.Habits.PotentialMood())
}

// fromPack starts a dose out of id's pack (CSE-011), the first of kinds it
// holds: into its hands and down the throat where it stands. True once the dose
// has left the pack: a job that cannot start is left failed with the dose on it,
// so that the job's end puts it back or down, and the next look waits.
func (d Drugs) fromPack(id entity.EntityID, kinds []item.Kind, now uint64) bool {
	if !d.h.Habits.PackDoses() {
		return false
	}
	for _, k := range kinds {
		if !d.h.Habits.PackHas(id, k) {
			continue
		}
		stack, ok := d.h.Habits.Unpack(id, k)
		if !ok {
			return false
		}
		if !d.startHeld(id, stack) {
			d.State(id).Retry = now + retryBusyRechecks*d.recheck
		}
		return true
	}
	return false
}

// Allowed lists the drugs id's policy lets it take now for a reason other than
// its own boredom (a drink in company, a friend's gift), as if bored.
func (d Drugs) Allowed(id entity.EntityID) []item.Kind {
	s := (*d.states)[id]
	var crave item.Kind
	craving := false
	if s != nil {
		crave, craving = s.Craving()
	} else {
		s = &drug.State{Policy: drug.SocialDrinking}
	}
	return d.kinds(id, s, true, crave, craving, d.h.Pawns.Now())
}

// DrinkFor is the drink id takes along to relax in company (CSE-008): k, from
// the pack (stack 0) or the nearest stack at (x, y). No hard drug makes a
// sociable drink.
func (d Drugs) DrinkFor(id entity.EntityID) (k item.Kind, stack entity.EntityID, x, y int, ok bool) {
	kinds := d.Allowed(id)
	soft := kinds[:0]
	for _, c := range kinds {
		if def, has := drug.DefOf(c); has && def.OverdoseMax <= 0 {
			soft = append(soft, c)
		}
	}
	if d.h.Habits.PackDoses() {
		for _, c := range soft {
			if d.h.Habits.PackHas(id, c) {
				return c, 0, 0, 0, true
			}
		}
	}
	return d.nearestOf(id, soft)
}

// GiftFor is the dose visitor may bring patient (CSE-007), as DrinkFor, from
// what patient's policy allows: turn says which of them it thinks of first (the
// policy order shuffled, by the caller's hash).
func (d Drugs) GiftFor(visitor, patient entity.EntityID, turn uint64) (k item.Kind, stack entity.EntityID, x, y int, ok bool) {
	kinds := d.Allowed(patient)
	if len(kinds) == 0 {
		return 0, 0, 0, 0, false
	}
	first := int(turn % uint64(len(kinds)))
	for i := range kinds {
		c := kinds[(first+i)%len(kinds)]
		if d.h.Habits.PackHas(visitor, c) {
			return c, 0, 0, 0, true
		}
	}
	return d.nearestOf(visitor, append(kinds[first:len(kinds):len(kinds)], kinds[:first]...))
}

// nearestOf is the nearest free stack of the first of kinds that has one.
func (d Drugs) nearestOf(id entity.EntityID, kinds []item.Kind) (item.Kind, entity.EntityID, int, int, bool) {
	for i := range kinds {
		if stack, x, y := d.nearest(id, kinds[i:i+1]); stack != 0 {
			return kinds[i], stack, x, y, true
		}
	}
	return 0, 0, 0, 0, false
}

// TakeOne has id, at stack, pick up one dose of it in its hands, split off a
// bigger stack; false when it is gone or in other hands.
func (d Drugs) TakeOne(id, stack entity.EntityID) (entity.EntityID, bool) {
	st, ok := d.h.Stock.Stack(stack)
	if !ok || st.Qty <= 0 || st.Carrier != 0 || !st.Kind.IsDrug() {
		return 0, false
	}
	if st.Qty == 1 {
		st.Carrier = id
		d.h.Stock.PutStack(stack, st)
		return stack, true
	}
	one := st
	one.Qty = 1
	carry, ok := d.h.Habits.SpawnHeld(id, one)
	if !ok {
		return 0, false
	}
	st.Qty--
	d.h.Stock.PutStack(stack, st)
	return carry, true
}

// Held is the kind of the one dose id holds as stack; false: it holds none now.
func (d Drugs) Held(id, stack entity.EntityID) (item.Kind, bool) {
	st, ok := d.h.Stock.Stack(stack)
	if !ok || st.Carrier != id || !st.Kind.IsDrug() {
		return 0, false
	}
	return st.Kind, true
}

// JoyOf is the recreation a dose of k gives (recreation points).
func JoyOf(k item.Kind) int {
	def, ok := drug.DefOf(k)
	if !ok {
		return 0
	}
	return int(def.Joy) * recreation.Full / 1000
}

// Tolerance is id's tolerance to k, in percent.
func (d Drugs) Tolerance(id entity.EntityID, k item.Kind) int {
	if s := (*d.states)[id]; s != nil {
		return int(s.ToleranceOf(k)) / 10
	}
	return 0
}

// Swallow has patient take the dose feeder holds as stack: it leaves play, and
// the dose works on patient. It reports whether the dose killed patient.
func (d Drugs) Swallow(feeder, patient, stack entity.EntityID) bool {
	k, ok := d.Held(feeder, stack)
	if !ok {
		return false
	}
	d.h.Stock.DeleteItem(stack)
	d.h.Habits.Used(feeder)
	return d.Ingest(patient, k)
}
