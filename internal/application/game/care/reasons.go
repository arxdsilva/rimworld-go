package care

// Reasons a pawn-targeted order is disabled (SIM-062), shown after the
// option's label in the right-click menu. Shared by the order builder
// (orders.go), the forced jobs (forced.go) and capture (capture.go).
const (
	ReasonNoBed       = "no free bed"
	ReasonNoPrisonBed = "no free prisoner bed"
	ReasonCarried     = "carried by "
	ReasonSelf        = "cannot target itself"
)
