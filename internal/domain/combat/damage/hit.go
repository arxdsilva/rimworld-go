package damage

// Form is the vanilla damage kind a hit deals, finer than Type: it picks
// the wound a hit leaves and how readily overkill destroys the part it
// strikes (append only). wiki: Damage types
type Form uint8

const (
	FormBlunt   Form = iota // fists, clubs, hooves; a sharp hit armor halved
	FormCut                 // edges: knives, swords, shrapnel
	FormStab                // points: spears, horns, spike traps
	FormBullet              // gunshots
	FormArrow               // bows and thrown spears
	FormScratch             // claws and beaks
	FormBite                // jaws
	FormCrush               // crushing weight
	FormBurn                // scorching, not open flame
	FormFlame               // open flame
	FormBomb                // blasts
	FormEMP                 // electromagnetic pulse
	FormStun                // stuns without wounding
	NumForms
)

// wiki: Damage types (armor category: bullet, arrow, cut, stab, scratch,
// bite and bomb are sharp; blunt and crush blunt; flame and burn heat).
var formType = [NumForms]Type{
	FormBlunt: Blunt, FormCut: Sharp, FormStab: Sharp, FormBullet: Sharp,
	FormArrow: Sharp, FormScratch: Sharp, FormBite: Sharp, FormCrush: Blunt,
	FormBurn: Burn, FormFlame: Heat, FormBomb: Bomb, FormEMP: EMP, FormStun: Stun,
}

// Type is the damage type of f (Blunt for an unknown form).
func (f Form) Type() Type {
	if f >= NumForms {
		return Blunt
	}
	return formType[f]
}

// FormOf is the form a hit of plain type t deals when nothing names a
// finer one (a sharp hit cuts, heat is flame).
func FormOf(t Type) Form {
	switch t {
	case Sharp:
		return FormCut
	case Heat:
		return FormFlame
	case Bomb:
		return FormBomb
	case Burn:
		return FormBurn
	case EMP:
		return FormEMP
	case Stun:
		return FormStun
	}
	return FormBlunt
}

var formNames = [NumForms]string{"blunt", "cut", "stab", "bullet", "arrow", "scratch", "bite", "crush", "burn", "flame", "bomb", "EMP", "stun"}

// String is the player-facing name.
func (f Form) String() string {
	if f >= NumForms {
		return "unknown"
	}
	return formNames[f]
}

// Hit is one blow, round, blast share or trap as it reaches its target,
// before armor: the weapon that dealt it (its name, "" for none), its
// form, amount, armor penetration (permille) and stopping power
// (permille of body size: a target no bigger staggers; 0 none).
type Hit struct {
	Weapon   string
	Form     Form
	Amount   int
	Pen      int
	Stopping int
}

// Type is the damage type of h's form.
func (h Hit) Type() Type { return h.Form.Type() }

// Of is a hit of amount of plain type t with penetration pen (a blast's
// share, a fixture).
func Of(t Type, amount, pen int) Hit { return Hit{Form: FormOf(t), Amount: amount, Pen: pen} }

// overkill is each form's window of overkill (permille of the struck
// part's max hit points past what it had left) over which the chance to
// destroy the part rises from none to certain. wiki: Damage types (cut
// and bite 0-10 %, bullet, arrow and scratch 0-70 %, blunt, crush and
// stab 40-100 %). original tuning: the forms the wiki gives no window
// (burns, flame, bombs) use the bullet's.
var overkill = [NumForms][2]int{
	FormBlunt: {400, 1000}, FormCut: {0, 100}, FormStab: {400, 1000}, FormBullet: {0, 700},
	FormArrow: {0, 700}, FormScratch: {0, 700}, FormBite: {0, 100}, FormCrush: {400, 1000},
	FormBurn: {0, 700}, FormFlame: {0, 700}, FormBomb: {0, 700}, FormEMP: {0, 700}, FormStun: {0, 700},
}

// Overkill is f's destroy window, lo to hi permille of overkill.
func (f Form) Overkill() (lo, hi int) {
	if f >= NumForms {
		f = FormBullet
	}
	return overkill[f][0], overkill[f][1]
}

// DestroyChance is the permille chance a hit overkilling a part by over
// permille of its max hit points destroys it in window lo..hi: none at or
// under lo, certain at or over hi, linear between. wiki: Injury
func DestroyChance(over, lo, hi int) int {
	switch {
	case over <= lo:
		return 0
	case over >= hi:
		return 1000
	}
	return (over - lo) * 1000 / (hi - lo)
}

// Staggers reports that a hit of stopping power stopping (permille) slows
// a body of size size (permille, a human 1000): the body is no bigger.
// wiki: Combat (stopping power)
func Staggers(stopping, size int) bool { return stopping > 0 && size <= stopping }
