package animalview

import (
	"rimworld-go/internal/application/game/evlog"
	"rimworld-go/internal/domain/sim/entity"
)

// View is one living animal's read model (game.AnimalView).
type View struct {
	ID entity.EntityID
	// Species is the species key ("arctic_fox": the art key); Label its
	// display name ("arctic fox"); Attack its natural weapon and attack
	// frame set ("bite", "claw", "kick", "gore", "headbutt", "peck");
	// Female picks a dimorphic species' female art; Size its body
	// length in tiles (SIM-019).
	Species    string
	Label      string
	Attack     string
	Female     bool
	Size       float64
	X, Y       int
	Health     int // percent
	Mode       string
	Designated bool
	Hostile    bool // attacking a colonist
	// TargetID is who a hostile animal is chasing (0 = none, PLAY-017).
	TargetID  entity.EntityID
	StepTicks int // duration of the current step in ticks (for lerp)
	// Tame animals belong to the colony and have a Name; TameMark: marked
	// for taming; Manhunter: enraged, hunting colonists.
	Tame      bool
	Name      string
	TameMark  bool
	Manhunter bool
	// Motion, as on game.PawnView.
	StepTick     uint64
	NextX, NextY int
	HasNext      bool
	BlockedBy    entity.EntityID
	// Action, as on game.PawnView: "bite", "kick", "graze", "flee" or "".
	Action string
	// Gait is how the animal moves this step (PLAY-022): game.GaitAmble,
	// game.GaitWalk or game.GaitRun; "" when it is not moving (read as GaitWalk).
	Gait               string
	AttackCycle        float64
	HitTick            uint64
	HitFromX, HitFromY int
	HitDamage          int
	// Log (LOG-001) is what happened to the animal, newest first.
	Log          []evlog.Line
	FoodPolicy   string // FOOD-0 contract ("" until FOOD-4): the tame animal's food policy
	ToxicBuildup int    // FOOD-6: toxic buildup percent (body/cond_toxic.go)
	// TEMP-0 contract (0 until TEMP-3): comfortable range (tenths of a degree C), hypothermia and heatstroke percent.
	ComfortMin, ComfortMax, Hypothermia, Heatstroke int
	// FOOD-3 (wiki): the percent chance a wild one turns on whoever hurts
	// it (the difficulty's), its wildness percent and the Animals level a
	// handler needs to tame it.
	Revenge, Wildness, MinHandling int
	// Needs and life (SIM-024..026). State is the behaviour ("wandering",
	// "feeding", "resting", "hunting", "attacking", "fleeing", "leaving");
	// Food and Rest are percent. Stage is "young" or "adult" (or the
	// archetype's word for young); Scale the body size factor, 0.55..1 as
	// the young grow. Product is "milk", "wool", "ember oil" or "" with
	// ProductPct its fullness. Herd groups animals that move together.
	State       string
	Food, Rest  int
	Diet        string
	Hunts       bool // PLAY-017: a tame predator (hunts wild animals when hungry)
	AgeDays     int
	Adult       bool
	Stage       string
	Scale       float64
	Pregnant    bool
	PregPct     int
	EggsFertile bool
	Product     string
	ProductPct  int
	Sleeping    bool
	// Downed: too hurt or bled to stand (game/animal_health.go); Sleeping is
	// set with it, so the renderer draws the rest pose (tinted).
	Downed  bool
	Leaving bool
	Herd    uint32
	// Dormant (MAPGEN-3): a sealed vault's sleeping defender (game/vault.go).
	Dormant bool
	// Mech (DEF-4): a mechanoid, a hostile machine (raid/mech.go);
	// Stunned: an EMP has it standing dead still. Staggered (CBT-1): a
	// hit's stopping power slows it.
	Mech, Stunned, Staggered bool
	// PLAY-015: Penned livestock show Pen (the pen's name, "Roaming" or
	// "Being roped"); other tame animals have an allowed area AreaRef
	// (command.AreaUnrestricted, command.AreaHome or an area id).
	Penned  bool
	Pen     string
	AreaRef int
	// ENG-003 (M10): the master's and bonded colonist's names and ids
	// (tame animals), which training.Skill skills are learnt, and every
	// skill's row (Training, in training.Skill order). Mother is the
	// mother's name while she lives on the map; Offspring the names of
	// her young still on it.
	Master   string
	Bond     string
	Trained  []bool
	MasterID entity.EntityID
	// CanFollow (PLAY-015): trained enough to follow a drafted master;
	// Follows: it will (the player can turn it off).
	CanFollow, Follows bool
	BondID             entity.EntityID
	Training           []Training
	Mother             string
	Offspring          []string
	// MED-09 (game/animal_health.go): the health block. HealthState is the
	// headline ("Bleeding 25%/day · death in 31h", "Injured", "Healthy");
	// BleedPerDay permille of blood a day (0: not bleeding) with
	// BleedDeathHours (-1: not bleeding); BloodLossPct and BloodStage (""
	// or "minor".."extreme"); InfectionPct the worst infection's severity
	// (0: none), ImmunityPct its immunity and InfectionLosing on course to
	// kill; NeedsTend a tame animal a tend would help now; CareLevel its
	// medical care level (medcare.Policy).
	HealthState     string
	BleedPerDay     int
	BleedDeathHours int
	BloodLossPct    int
	BloodStage      string
	InfectionPct    int
	ImmunityPct     int
	InfectionLosing bool
	NeedsTend       bool
	CareLevel       uint8
	// MED-13 (game/animal_health.go): Parts are the hurt and lost parts of its
	// anatomy, in part order (a lost part's children are not listed
	// apart); MovingPct its moving capacity in percent (100 without a
	// body).
	Parts     []Part
	MovingPct int
}

// Training is one training skill of a tame animal: Allowed if
// its species can learn it, On if handlers train it, Steps of Needed.
type Training struct {
	Skill   int // training.Skill (SetTraining's Index)
	Label   string
	Allowed bool
	On      bool
	Learned bool
	Steps   int
	Needed  int
}

// Part is one hurt or lost part of an animal (MED-13): its
// label ("front left leg"), hit points left of MaxHP, lost, and bleeding
// (an open wound, or a fresh stump).
type Part struct {
	Label     string
	HP, MaxHP int
	Missing   bool
	Bleeding  bool
}
