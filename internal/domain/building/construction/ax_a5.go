package construction

// plannedA5 is A5's kinds not live yet (fire foam and security; AX-0
// contract). AX-5 made all five live (the foam popper, foam charge, foam
// turret, warhead charge and rocket battery), so none is planned. This
// file is A5's, and so are the predicates below for its own kinds.
func plannedA5(Kind) bool { return false }

// IsCharge reports a buried charge (an IED): the four DEF-3 charges and
// AX-5's foam and warhead charges. An explicit set, never a range of the
// enum (the AX kinds sit far past the DEF-3 ones).
func (k Kind) IsCharge() bool {
	switch k {
	case IEDHighExplosive, IEDIncendiary, IEDEMP, IEDSmoke, IEDFoam, IEDWarhead:
		return true
	}
	return false
}

// BurstsWhenDestroyed reports a defence that goes off as it is smashed
// (AX-5 adds the foam popper, foam turret and rocket battery to the
// sentry turret). wiki: Firefoam popper, Foam turret, Rocketswarm
// launcher, Mini-turret
func (k Kind) BurstsWhenDestroyed() bool {
	switch k {
	case SentryTurret, FoamPopper, FoamTurret, RocketBattery:
		return true
	}
	return false
}
