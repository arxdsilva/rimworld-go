package books

import (
	"rimworld-go/internal/application/game/axview"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/economy/book"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/quality"
	"rimworld-go/internal/domain/research"
)

// Where books come from (wiki: Books: traders, faction bases and quest
// rewards; never crafted in core 1.5). Every new book stack is minted
// where it is spawned (game SpawnStack): a quality and a seed hashed from
// the colony seed and the stack's id, no dice drawn. Raiders sometimes
// carry one (RaidLoot); traders stock them (domain trade pools) and
// ruins hold them (mapgen's ruin_books pass).

// Mint gives new book stack st (id, colony seed seed) its quality and
// seed; done reports a finished research project (a schematic covers
// open ones). Anything else is left alone.
func Mint(st *item.Stack, seed, id uint64, done func(research.Project) bool) {
	if st == nil || !st.Kind.IsBook() || st.Book != 0 {
		return
	}
	q, s := book.Mint(st.Kind, book.Hash(seed, id), func(p research.Project) bool { return !done(p) })
	st.Quality, st.Book = quality.Stored(q), s
}

// RaidBookPct is the chance a dead raider leaves a book (original
// tuning: the wiki names quest rewards and traders, not how often a
// pawn carries one).
const RaidBookPct = 4

// RaidLoot is the book dead raider id (colony seed seed) leaves, if any:
// a hash, no dice.
func RaidLoot(seed, id uint64) (item.Kind, bool) {
	h := book.Hash(seed^0x7261696462, id)
	if h%100 >= RaidBookPct {
		return 0, false
	}
	return []item.Kind{item.Novel, item.Novel, item.Textbook, item.Schematic}[h>>8%4], true
}

// FillItem sets a book's item view fields (ItemView.AX: Title, Teaches,
// Power); anything else is left as it is.
func FillItem(ax *axview.Item, st item.Stack) {
	if !st.Kind.IsBook() {
		return
	}
	q, _ := quality.FromStored(st.Quality)
	ax.Title, ax.Teaches, ax.Power = book.Title(st.Kind, st.Book), book.Teaches(st.Kind, st.Book), book.Power(st.Kind, q)
}

// Title is book st's title ("" for anything else).
func Title(st item.Stack) string { return book.Title(st.Kind, st.Book) }

// Reading is the title of the book a reader on job j reads (or goes to
// read), st the book (PawnView.AX.Reading; "" for any other job).
func Reading(j job.Record, st *item.Stack) string {
	if j.Kind != job.Read || st == nil {
		return ""
	}
	return book.Title(st.Kind, st.Book)
}
