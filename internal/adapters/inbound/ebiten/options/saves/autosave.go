package saves

import (
	"fmt"
	"strings"
	"time"

	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/application/game/savefmt"
)

// TickAutosave saves at AutosaveHour every Options-chosen number of
// in-game days (0: never) to <slot>-autosave, or with General.AutosavesKept
// above one to the next of <slot>-autosave-1…N (autosaveSlot); a
// commitment colony saves its own slot every day, whatever Options say.
// slot is the colony's (SaveSlot), never the name of the player's last
// manual save, so the autosaves follow the colony (SAV-003).
func (b *Book) TickAutosave() {
	env := b.sh.SaveEnv()
	if !env.CanSave || env.Capture {
		return
	}
	n := b.sh.Settings().Gameplay.AutosaveDays
	if env.Commitment {
		n = 1 // min(Options, daily), never off
	}
	if n <= 0 {
		return
	}
	d := b.sh.Date()
	day := int(b.sh.Tick()) / 60000
	if d.Hour != AutosaveHour || day-b.AutosaveDay < n {
		return
	}
	b.AutosaveDay = day
	if env.Commitment {
		b.SaveNow(b.SaveSlot) // its one slot, keyed on the colony (session.Colony rides in the header)
		return
	}
	write(b, metaFor(b, autosaveSlot(b, day), true))
}

// autosaveSlot is the slot the next autosave of this colony is written
// to. One kept autosave is <slot>-autosave, as it always was. With more it
// is the first of <slot>-autosave-1…N that no save uses, else the one of
// this colony's autosaves written longest ago, so the oldest is
// overwritten. A name another colony's save holds is skipped; when the
// save list cannot be read, or every name is another colony's, it goes
// round by the in-game day.
func autosaveSlot(b *Book, day int) string {
	base := b.SaveSlot + "-autosave"
	kept := b.sh.Settings().General.AutosavesKept
	if kept <= 1 {
		return base
	}
	rotate := fmt.Sprintf("%s-%d", base, day%kept+1)
	slots, err := b.sh.List()
	if err != nil || !b.sh.SaveEnv().CanLoad {
		return rotate
	}
	var oldest time.Time
	pick := ""
	for i := 1; i <= kept; i++ {
		name := fmt.Sprintf("%s-%d", base, i)
		sl, found := slotKeyed(slots, name)
		switch {
		case !found:
			return name
		case !ownAutosave(b, sl):
			continue
		case pick == "" || sl.Modified.Before(oldest):
			pick, oldest = name, sl.Modified
		}
	}
	if pick == "" {
		return rotate
	}
	return pick
}

// slotKeyed is the save whose file name is name's (savefmt.SlotKey, case
// folded: the file systems players use most ignore case).
func slotKeyed(slots []ports.SaveSlot, name string) (ports.SaveSlot, bool) {
	key := savefmt.SlotKey(name)
	for _, sl := range slots {
		if strings.EqualFold(savefmt.SlotKey(sl.Name), key) {
			return sl, true
		}
	}
	return ports.SaveSlot{}, false
}

// ownAutosave reports sl is one of the running colony's autosaves: its
// header names this colony and says the game wrote it. A row whose header
// names no colony (a list that does not read headers) counts as the
// colony's, as names alone did before SAV-003.
func ownAutosave(b *Book, sl ports.SaveSlot) bool {
	if sl.Colony == "" {
		return true
	}
	return sl.Colony == b.session.Colony && sl.Autosave
}
