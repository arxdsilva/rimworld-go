package recreation

// JoyKind is the kind of fun an activity gives (AX-0 contract, A6; wiki:
// Recreation): a colonist tires of one kind it has had a lot of lately
// (tolerance) and seeks another. Nothing reads it until A6 adds
// tolerance; A7 uses Reading.
type JoyKind uint8

const (
	JoyNone JoyKind = iota
	JoySolitary
	JoySocial
	JoyCerebral
	JoyDexterity
	JoyReading
	JoyTelevision
	JoyTelescope
	JoyChemical
	JoyGluttonous
	NumJoyKinds
)

var joyKindNames = [NumJoyKinds]string{"", "solitary", "social", "cerebral", "dexterity", "reading", "television",
	"telescope", "chemical", "gluttonous"}

func (k JoyKind) String() string {
	if k >= NumJoyKinds {
		return ""
	}
	return joyKindNames[k]
}
