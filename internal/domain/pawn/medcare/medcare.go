// Package medcare is the medical care policy (MED-03): how much care a
// patient gets, from none at all to the best medicine there is. Each
// patient has a policy; one not set by hand follows its group's default
// (colonists, prisoners, guests, animals). wiki: Doctoring (care levels
// and their defaults). Pure data, no state.
package medcare

import "rimworld-go/internal/domain/economy/item"

// Policy is a patient's medical care level, worst care first.
type Policy uint8

const (
	// NoCare: no doctor tends the patient (a rescue still happens).
	NoCare Policy = iota
	// NoMeds: tended, but never with medicine.
	NoMeds
	// HerbalMax: herbal medicine at most.
	HerbalMax
	// IndustrialMax: industrial medicine at most.
	IndustrialMax
	// Best: the best medicine to be had.
	Best
	// NumPolicies bounds Policy.
	NumPolicies
)

// Valid reports p is one of the policies.
func (p Policy) Valid() bool { return p < NumPolicies }

// Label is the player-facing name of p.
func (p Policy) Label() string {
	switch p {
	case NoCare:
		return "No care"
	case NoMeds:
		return "Care, no medicine"
	case HerbalMax:
		return "Herbal or less"
	case IndustrialMax:
		return "Industrial or less"
	case Best:
		return "Best available"
	}
	return "?"
}

// Tends reports a doctor may tend a patient under p (everything but
// NoCare).
func (p Policy) Tends() bool { return p != NoCare && p.Valid() }

// isMedicine reports a medicine item (drugs are filed under the same
// heading but tend nothing).
func isMedicine(k item.Kind) bool { return k.Category() == "Medicine" && !k.IsDrug() }

// Allows reports a tend under p may use medicine k. NoCare and NoMeds
// allow none; HerbalMax herbal only; IndustrialMax herbal and
// industrial; Best any medicine (a better one added later included).
// Anything that is not medicine is never allowed.
func Allows(p Policy, k item.Kind) bool {
	if !isMedicine(k) {
		return false
	}
	switch p {
	case HerbalMax:
		return k == item.HerbalMedicine
	case IndustrialMax:
		return k == item.HerbalMedicine || k == item.IndustrialMedicine
	case Best:
		return true
	}
	return false
}

// MaxMedicine is the best medicine p allows, or item.Food (the zero Kind:
// no medicine) for NoCare and NoMeds. Best names industrial, the best
// medicine so far.
func MaxMedicine(p Policy) item.Kind {
	switch p {
	case HerbalMax:
		return item.HerbalMedicine
	case IndustrialMax:
		return item.IndustrialMedicine
	case Best:
		return item.NanomedKit
	}
	return item.Food
}

// Group is who a default policy applies to.
type Group uint8

const (
	Colonists Group = iota
	Prisoners
	Guests
	Animals
	// NumGroups bounds Group.
	NumGroups
)

// Label is the player-facing name of g.
func (g Group) Label() string {
	switch g {
	case Colonists:
		return "Colonists"
	case Prisoners:
		return "Prisoners"
	case Guests:
		return "Guests"
	case Animals:
		return "Animals"
	}
	return "?"
}

// Default is group g's care until the player changes it: the best for
// colonists, herbal or less for everyone else. wiki: Doctoring.
func Default(g Group) Policy {
	if g == Colonists {
		return Best
	}
	return HerbalMax
}

// Defaults is every group's Default, in Group order.
func Defaults() [NumGroups]Policy {
	var d [NumGroups]Policy
	for g := Group(0); g < NumGroups; g++ {
		d[g] = Default(g)
	}
	return d
}
