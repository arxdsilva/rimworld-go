package modding

import "rimworld-go/internal/domain/pawn/backstory"

// BackstoryClass is the class name in defs files and patch targets.
const BackstoryClass = "backstory"

// EncodeBackstories turns a book's defs into trees under their keys, with
// Plugin "core" (PLG-021). Owner: W1-backstory.
func EncodeBackstories(b backstory.Book) []Def { return nil }

// DecodeBackstories types the backstory defs (abstract ones already
// dropped) in order and freezes them into a book (PLG-024, PLG-042).
// Owner: W1-backstory.
func DecodeBackstories(defs []Def) (backstory.Book, []Problem) { return backstory.Core(), nil }
