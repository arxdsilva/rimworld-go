package damage

// Type is what kind of harm an attack does.
type Type uint8

const (
	Sharp Type = iota // blades, bullets, claws, teeth
	Blunt             // fists, clubs, hooves
	Heat              // flames
	Bomb              // explosions
	Burn              // scorching, not open flame
	EMP               // electromagnetic pulse
	numTypes
)

// Category is the armor rating that resists a damage type.
type Category uint8

const (
	// NoArmor: nothing resists it (EMP).
	NoArmor Category = iota
	ArmorSharp
	ArmorBlunt
	ArmorHeat
)

// wiki: Damage (armor category per damage type; bullet/arrow/cut/stab/bite
// and bomb are sharp, flame and burn heat, EMP none). Sharp here stands for
// all of the sharp kinds.
var resisted = [numTypes]Category{
	Sharp: ArmorSharp,
	Blunt: ArmorBlunt,
	Heat:  ArmorHeat,
	Bomb:  ArmorSharp,
	Burn:  ArmorHeat,
	EMP:   NoArmor,
}

// Resisted returns the armor category that applies to t (NoArmor for
// unknown types).
func Resisted(t Type) Category {
	if t >= numTypes {
		return NoArmor
	}
	return resisted[t]
}

var names = [numTypes]string{"sharp", "blunt", "heat", "bomb", "burn", "EMP"}

// String is the player-facing name.
func (t Type) String() string {
	if t >= numTypes {
		return "unknown"
	}
	return names[t]
}
