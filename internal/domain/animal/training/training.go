package training

// Skill is a trainable behaviour (append only, saved).
type Skill uint8

const (
	Obedience Skill = iota
	Guard
	Release
	NumSkills
)

// TODO(M10): step chances, decay, bond chances.
