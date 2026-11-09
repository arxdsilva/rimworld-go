package savefmt

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// InfoVersion is the first save version whose header names its colony
// (SAV-003: Colony, ColonyName, Base, Autosave, Season, Build, Content).
const InfoVersion = 16

// Info is a save's header: enough to list it without loading. The fields
// after Colonists are v2 (zero in a v1 header); the colony fields are v16
// (Upgrade fills them for an older header). game.SaveInfo becomes an alias
// of it (SAV-003), so the names and types of the v1…v15 fields never change:
// gob matches fields by name.
type Info struct {
	Version   int
	Name      string // the slot written (the file name before SlotKey)
	Date      string // in-game date
	Day       int    // days since landing
	Colonists int    // alive
	// Commitment: one save per colony, written by the game itself
	// (NG-1 §2.13); Storyteller and Difficulty name the colony's ("" for a
	// legacy colony); MapSize is the map side; Biome the biome's name.
	Commitment  bool
	Storyteller string
	Difficulty  string
	MapSize     int
	Biome       string
	// Ending (AX-8) is "ship" once the colony's ship has left: the
	// save's story is complete ("" while it goes on).
	Ending string

	// v16 (SAV-003).
	Colony     string  // the colony's stable id, shared by all its saves
	ColonyName string  // the colony's name as the player sees it
	Base       string  // the colony's slot, which its autosaves are named after
	Autosave   bool    // written by the game, not the player
	Season     string  // the in-game season when written
	Build      string  // the game build that wrote the save
	Content    Content // the content the save was played with
}

// Content is what the colony was played with beyond the game itself: a
// hash of the content definitions and the plugins in load order.
type Content struct {
	Hash    uint64
	Plugins []string
}

// Stamp is what the client and the build add to a header when they save:
// everything the simulation does not know of itself.
type Stamp struct {
	Name, Colony, ColonyName, Base string
	Autosave                       bool
	Season, Build                  string
	Content                        Content
}

// Apply writes the stamp into info.
func (s Stamp) Apply(info *Info) {
	info.Name, info.Colony, info.ColonyName, info.Base = s.Name, s.Colony, s.ColonyName, s.Base
	info.Autosave, info.Season, info.Build, info.Content = s.Autosave, s.Season, s.Build, s.Content
}

// Upgrade fills the v16 colony fields of an older header from its name:
// an autosave <slot>-autosave[-N] belongs to <slot>'s colony, whose id is
// "slot:"+<slot>. A v16 header, or one that already names its colony, is
// left alone; Version is never changed (Load still migrates by it).
func (i *Info) Upgrade() {
	if i.Version >= InfoVersion || i.Colony != "" {
		return
	}
	i.Base = BaseSlot(i.Name)
	i.Autosave = i.Name != i.Base
	i.Colony = "slot:" + i.Base
	i.ColonyName = i.Base
}

// BaseSlot is a colony's slot name from the name of one of its autosaves
// (<slot>-autosave or <slot>-autosave-N); any other name is its own.
func BaseSlot(name string) string {
	if i := strings.LastIndex(name, "-autosave-"); i >= 0 {
		if _, err := strconv.Atoi(name[i+len("-autosave-"):]); err == nil {
			return name[:i]
		}
	}
	return strings.TrimSuffix(name, "-autosave")
}

// SlotKey makes a slot name safe as a file name: letters, digits, '-' and
// '_' are kept, spaces become '-', anything else goes; an empty result is
// "colony".
func SlotKey(name string) string {
	name = strings.TrimSpace(name)
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		case r == ' ':
			b.WriteRune('-')
		}
	}
	if b.Len() == 0 {
		return "colony"
	}
	return b.String()
}

// CheckName is why name cannot name a save ("" when it can). SAV-001
// adds its rules (length, reserved names, the autosave suffix); today only
// a blank name is refused.
func CheckName(name string) (reason string) {
	if strings.TrimSpace(name) == "" {
		return "Name the save"
	}
	return ""
}

// Compat is how this build reads a save (ports.SaveSlot.Compat; 0 fine).
type Compat int

const (
	CompatOK      Compat = iota // same version and content
	CompatOlder                 // an older version: Load migrates it
	CompatNewer                 // written by a newer build: Load refuses it
	CompatContent               // other content or plugins: loads with a notice
)

// Runtime is what this build is, for CompatOf.
type Runtime struct {
	Version int
	Build   string
	Content Content
}

// CompatOf is how a build at rt reads a save with header info. Today it
// compares versions only; the content check (CompatContent) comes with
// the plugins' hashes.
func CompatOf(info Info, rt Runtime) Compat {
	switch {
	case info.Version > rt.Version:
		return CompatNewer
	case info.Version < rt.Version:
		return CompatOlder
	}
	return CompatOK
}

// ErrCorrupt means the save file is damaged: cut short or failing its
// checksum.
var ErrCorrupt = errors.New("save file is damaged")

// Drain reads the rest of a save stream (the reader from NewReader), so
// the gzip checksum at its end is checked; a short or damaged file is
// ErrCorrupt.
func Drain(r io.Reader) error {
	if _, err := io.Copy(io.Discard, r); err != nil {
		return fmt.Errorf("%w: %v", ErrCorrupt, err)
	}
	return nil
}

// Clamp cuts s to at most n runes: header text from a file is shown in the
// load list, and a damaged or hand-made header must not flood it.
func Clamp(s string, n int) string {
	if n <= 0 {
		return ""
	}
	i := 0
	for j := range s {
		if i == n {
			return s[:j]
		}
		i++
	}
	return s
}
