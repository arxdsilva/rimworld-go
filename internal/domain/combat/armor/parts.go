package armor

import "rimworld-go/internal/domain/pawn/body"

// partGroups is the coverage groups of each humanlike part: the head's
// parts are in the whole head, the skull's (brain, ears) in the upper
// head too and the eyes in the eyes; the torso's organs are in the torso;
// an arm is in the arms and shoulders. wiki: Armor (coverage groups).
// original tuning: the body has no neck, shoulder or hand part, so the
// hands count as arms and the feet as legs.
var partGroups = [body.NumParts]Region{
	body.Brain: CoverFullHead | CoverUpperHead,
	body.EyeL:  CoverFullHead | CoverEyes, body.EyeR: CoverFullHead | CoverEyes,
	body.EarL: CoverFullHead | CoverUpperHead, body.EarR: CoverFullHead | CoverUpperHead,
	body.Nose: CoverFullHead, body.Jaw: CoverFullHead,
	body.Torso: CoverTorso, body.Heart: CoverTorso, body.LungL: CoverTorso, body.LungR: CoverTorso,
	body.Stomach: CoverTorso, body.KidneyL: CoverTorso, body.KidneyR: CoverTorso, body.Liver: CoverTorso,
	body.Spine: CoverTorso,
	body.ArmL:  CoverArms | CoverShoulders, body.ArmR: CoverArms | CoverShoulders,
	body.HandL: CoverArms, body.HandR: CoverArms,
	body.LegL: CoverLegs, body.LegR: CoverLegs, body.FootL: CoverLegs, body.FootR: CoverLegs,
}

// GroupOf is the coverage groups humanlike part p is in (0 for an unknown
// part: nothing covers it).
func GroupOf(p body.Part) Region {
	if p >= body.NumParts {
		return 0
	}
	return partGroups[p]
}
