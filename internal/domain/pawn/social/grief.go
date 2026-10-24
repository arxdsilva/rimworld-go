package social

// GriefDef is a death's mood effect on someone close and how long it lasts.
type GriefDef struct {
	Mood int
	Days int
}

// GriefFor is the grief over a person with relation k (friend, rival: the
// opinion bands; lost: missing rather than dead).
func GriefFor(k Kind, friend, rival, lost bool) (GriefDef, bool) {
	return GriefDef{}, false // TODO(M7)
}

// KillerOpinion is the opinion penalty toward whoever killed a person with
// relation k, centi-points (0: none).
func KillerOpinion(k Kind) int32 {
	return 0 // TODO(M7)
}
