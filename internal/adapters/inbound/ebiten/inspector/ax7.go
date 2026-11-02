package inspector

import (
	"fmt"
	"strings"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/book"
	"rimworld-go/internal/domain/economy/quality"
)

// AX-7 books and bookcases on the inspector, read from ItemView.AX,
// BuildingView.AX and PawnView.AX (game/axview): a book's title, what it
// teaches and what its quality does; a bookcase's books and its room's
// reading bonus; what a colonist is reading.

// Book kinds and bookcase kinds (item and construction names).
const (
	novelKind         = "novel"
	textbookKind      = "textbook"
	schematicKind     = "schematic"
	bookcaseKind      = "bookcase"
	smallBookcaseKind = "small bookcase"
)

// isBook reports a book stack.
func isBook(it game.ItemView) bool {
	return it.Kind == novelKind || it.Kind == textbookKind || it.Kind == schematicKind
}

// levelOf is the quality level a label names (Normal when none).
func levelOf(label string) quality.Level {
	for _, l := range quality.All() {
		if l.String() == label {
			return l
		}
	}
	return quality.Default
}

// BookLines are a book's lines: what reading it gives at its quality.
func BookLines(it game.ItemView) []string {
	if !isBook(it) {
		return nil
	}
	q := levelOf(it.Quality)
	out := []string{fmt.Sprintf("Recreation power %.2f", float64(it.AX.Power)/1000)}
	n := strings.Count(it.AX.Teaches, ",") + 1
	switch {
	case it.Kind == textbookKind && it.AX.Teaches != "":
		each := ""
		if n > 1 {
			each = " each"
		}
		out = append(out, fmt.Sprintf("Teaches %s: %d XP an hour%s, up to level %d", it.AX.Teaches, book.SkillXP(q, n), each, book.Cap(q)))
	case it.Kind == schematicKind && it.AX.Teaches != "":
		each := ""
		if n > 1 {
			each = " each"
		}
		out = append(out, fmt.Sprintf("Research %s: %d points an hour%s", it.AX.Teaches, book.Points(q, n), each))
	}
	return append(out, fmt.Sprintf("Any reader: %d Intellectual XP an hour", book.IntellectualXP),
		"A room's shelved books speed reading up")
}

// BookcaseLines are a bookcase's room bonus and the titles it holds.
func BookcaseLines(b game.BuildingView, items []game.ItemView) []string {
	if b.Kind != bookcaseKind && b.Kind != smallBookcaseKind {
		return nil
	}
	out := []string{ReadingBonusLine(b.AX.ReadingBonus)}
	for _, it := range items {
		if isBook(it) && !it.Carried && it.X >= b.X && it.X < b.X+max(b.W, 1) && it.Y >= b.Y && it.Y < b.Y+max(b.H, 1) {
			out = append(out, fmt.Sprintf("%s (%s, %s)", it.AX.Title, it.Kind, it.Quality))
		}
	}
	if len(out) == 1 {
		out = append(out, "No books yet: haulers shelve them here")
	}
	return out
}

// ReadingBonusLine is the room's reading bonus ("Reading bonus +6.0%:
// reading and research in this room").
func ReadingBonusLine(permille int) string {
	if permille <= 0 {
		return "Reading bonus: none (shelve books in a room)"
	}
	return fmt.Sprintf("Reading bonus +%.1f%%: reading and research in this room", float64(permille)/10)
}

// ReadingLine is "Reading: <title>" while a colonist reads ("" else).
func ReadingLine(p game.PawnView) string {
	if p.AX.Reading == "" {
		return ""
	}
	return "Reading: " + p.AX.Reading
}
