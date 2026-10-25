// Package vault is the shared vocabulary of sealed vaults (MAPGEN-3): old
// chambers generated inside rock, holding loot, dormant defenders and
// sleeper caskets. Map generation places them; the game runs their
// lifecycle. Names and numbers are ours.
package vault

// Occupant is what sleeps in a casket. Append only: saved.
type Occupant uint8

const (
	NoOccupant        Occupant = iota // not a sealed casket
	Grateful                          // a sleeper who wakes and asks to stay
	Hostile                           // an armed sleeper who wakes in a rage
	SleeperAndCrawler                 // a sleeper with a hive crawler curled at its feet
	Beetles                           // two hive beetles
	Empty                             // nobody
	Remains                           // long-dead remains with old gear
	NumOccupants
)

var occupantNames = [NumOccupants]string{"", "grateful", "hostile", "sleeper_crawler", "beetles", "empty", "remains"}

func (o Occupant) String() string {
	if o >= NumOccupants {
		return ""
	}
	return occupantNames[o]
}

// State is a vault's lifecycle. Append only: saved.
type State uint8

const (
	Generated     State = iota // sealed and undiscovered
	Warned                     // a colonist sensed it ("Ominous walls")
	Breached                   // opened to the outside; defenders awake
	CasketsOpened              // its casket group has been opened
	Cleared                    // no defender or hostile occupant left
)

var stateNames = [...]string{"generated", "warned", "breached", "caskets_opened", "cleared"}

func (s State) String() string {
	if int(s) >= len(stateNames) {
		return ""
	}
	return stateNames[s]
}
