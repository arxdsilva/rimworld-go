package body

// Part is a body part (append only, saved).
type Part uint8

const (
	Brain Part = iota
	EyeL
	EyeR
	EarL
	EarR
	Nose
	Jaw
	Torso
	Heart
	LungL
	LungR
	Stomach
	ArmL
	ArmR
	HandL
	HandR
	LegL
	LegR
	FootL
	FootR
	NumParts
)

// TODO(M1): the part tree and per-part definitions.
