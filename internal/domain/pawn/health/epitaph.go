package health

// CausePhrase is the epitaph's words for a saved cause of death (its
// String; " of wounds"), "" for a cause the stone leaves unsaid. Moved
// from the game's burial code (CBT-5 budget payer).
func CausePhrase(cause string) string {
	switch cause {
	case CauseInjury.String():
		return " of wounds"
	case CauseHypothermia.String():
		return " of the cold"
	case CauseHeatstroke.String():
		return " of heatstroke"
	case CauseInfection.String():
		return " of infection"
	case CauseNoFood.String():
		return " of hunger"
	case CauseBloodLoss.String():
		return " of blood loss"
	case CauseOrganFailure.String():
		return " of organ failure"
	case CauseDisease.String():
		return " of disease"
	case CauseOverdose.String():
		return " of an overdose"
	case CauseHeartAttack.String():
		return " of a heart attack"
	case CauseCarcinoma.String():
		return " of a carcinoma"
	}
	return ""
}
