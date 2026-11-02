// Package book is the books of vanilla 1.5 core (AX-7, ENG-003 wiki
// numbers, original names and prose): novels, textbooks and schematics,
// items with a quality and a generated title. A book's identity is one
// number, item.Stack.Book (Seed): the title's words, and the skills a
// textbook teaches or the research projects a schematic covers. The
// package is headless and deterministic: every choice is a hash of the
// seed, never a random draw.
//
// Seed layout: bits 0..15 pick the title words (never all zero, so a
// minted book's seed is never 0); bits 16..23 the first subject plus one
// (a skill.ID or a research.Project); bits 24..31 the second subject
// plus one (0: a single subject). Novels have no subject.
package book

import (
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/quality"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/research"
)

// Reading numbers by quality, Awful..Legendary (wiki: Novel, Textbook,
// Schematic, Books).
var (
	// NovelPower is a novel's recreation power, permille.
	NovelPower = [7]int{1200, 1400, 1600, 1800, 2000, 2250, 2500}
	// TextbookXP is a textbook's skill experience an hour, teaching one
	// skill; TextbookXP2 teaching two (each).
	TextbookXP  = [7]int{125, 188, 250, 312, 375, 438, 500}
	TextbookXP2 = [7]int{78, 117, 156, 195, 234, 273, 312}
	// TextbookCap is the highest skill level a textbook still teaches.
	TextbookCap = [7]int{2, 5, 8, 10, 12, 14, 16}
	// SchematicPoints is a schematic's research points an hour into one
	// project; SchematicPoints2 into each of two.
	SchematicPoints  = [7]int{20, 30, 40, 50, 60, 70, 80}
	SchematicPoints2 = [7]int{12, 18, 25, 31, 37, 43, 50}
)

const (
	// PlainPower is a textbook's or schematic's recreation power, permille
	// (wiki: 1.0, no quality factor).
	PlainPower = 1000
	// IntellectualXP is the Intellectual experience an hour any book gives
	// its reader, whatever its quality (wiki: Books, 250 an hour).
	IntellectualXP = 250
	// JoyPerHour is the recreation an hour of power 1 restores, in
	// recreation.Full (10000) units (wiki: Recreation, 36 % an hour at
	// power 1.0).
	JoyPerHour = 3600
	// TwoSubjectsPct is the chance a textbook teaches two skills, or a
	// schematic covers two projects (wiki: 25 %).
	TwoSubjectsPct = 25
)

// Reading bonus (wiki: Reading bonus): each book in a bookcase of the
// room adds, permille: the first five 8, the next five 4, the next five 2,
// every one after that 1, up to BonusCap (x1.20).
const BonusCap = 200

// Bonus is the reading bonus of a room holding n books in its bookcases,
// permille over 1000 (10 books: 60, 100 books: 155).
func Bonus(n int) int {
	b := 0
	for i := 1; i <= n && b < BonusCap; i++ {
		switch {
		case i <= 5:
			b += 8
		case i <= 10:
			b += 4
		case i <= 15:
			b += 2
		default:
			b++
		}
	}
	return min(b, BonusCap)
}

// foundWeights is how often a found or traded book has each quality,
// Awful..Legendary, percent (original tuning: the wiki gives none).
var foundWeights = [7]int{5, 15, 40, 25, 10, 4, 1}

// Mint is the quality and seed of a new book of kind k from hash h: open
// reports a research project not yet finished (a schematic covers those,
// any project once all are done; wiki: Schematic). The zero values for a
// kind that is no book.
func Mint(k item.Kind, h uint64, open func(research.Project) bool) (quality.Level, uint32) {
	if !k.IsBook() {
		return 0, 0
	}
	h = mix(h)
	roll, q := int(h%100), quality.Awful
	for l, w := range foundWeights {
		if roll < w {
			q = quality.Level(l)
			break
		}
		roll -= w
	}
	h = mix(h)
	seed := uint32(h&0xffff) | 1
	h = mix(h)
	two := int(h%100) < TwoSubjectsPct
	h = mix(h)
	switch k {
	case item.Textbook:
		a := skill.ID(h % uint64(skill.NumSkills))
		seed |= uint32(a+1) << 16
		if two {
			b := skill.ID((uint64(a) + 1 + (h>>8)%uint64(skill.NumSkills-1)) % uint64(skill.NumSkills))
			seed |= uint32(b+1) << 24
		}
	case item.Schematic:
		pool := projects(open)
		if len(pool) == 0 {
			break
		}
		i := int(h % uint64(len(pool)))
		seed |= uint32(pool[i]+1) << 16
		if two && len(pool) > 1 {
			j := (i + 1 + int((h>>8)%uint64(len(pool)-1))) % len(pool)
			seed |= uint32(pool[j]+1) << 24
		}
	}
	return q, seed
}

// projects is the projects a schematic may cover: the open ones (all of
// them when none is open), each with a cost.
func projects(open func(research.Project) bool) []research.Project {
	var all, live []research.Project
	for p := research.Project(0); p < research.NumProjects && p < 255; p++ {
		if research.DefOf(p).Cost <= 0 {
			continue
		}
		all = append(all, p)
		if open == nil || open(p) {
			live = append(live, p)
		}
	}
	if len(live) == 0 {
		return all
	}
	return live
}

// mix is a 64-bit finaliser (splitmix64's): a seed spread over all bits.
func mix(h uint64) uint64 {
	h += 0x9e3779b97f4a7c15
	h = (h ^ h>>30) * 0xbf58476d1ce4e5b9
	h = (h ^ h>>27) * 0x94d049bb133111eb
	return h ^ h>>31
}

// Hash is a book's mint hash from the colony seed and the new stack's id.
func Hash(seed, id uint64) uint64 { return mix(seed ^ mix(id)) }

// Skills is the skills a textbook with seed s teaches (none for other
// books).
func Skills(k item.Kind, s uint32) []skill.ID {
	if k != item.Textbook {
		return nil
	}
	var out []skill.ID
	for _, b := range [2]uint32{s >> 16 & 0xff, s >> 24} {
		if b > 0 && skill.ID(b-1) < skill.NumSkills {
			out = append(out, skill.ID(b-1))
		}
	}
	return out
}

// Projects is the research projects a schematic with seed s covers.
func Projects(k item.Kind, s uint32) []research.Project {
	if k != item.Schematic {
		return nil
	}
	var out []research.Project
	for _, b := range [2]uint32{s >> 16 & 0xff, s >> 24} {
		if b > 0 && research.Project(b-1) < research.NumProjects {
			out = append(out, research.Project(b-1))
		}
	}
	return out
}

// Power is a book's recreation power, permille.
func Power(k item.Kind, q quality.Level) int {
	if k == item.Novel {
		return NovelPower[idx(q)]
	}
	if k.IsBook() {
		return PlainPower
	}
	return 0
}

// SkillXP is the experience an hour a textbook of quality q gives each of
// its n skills.
func SkillXP(q quality.Level, n int) int {
	if n > 1 {
		return TextbookXP2[idx(q)]
	}
	return TextbookXP[idx(q)]
}

// Points is the research points an hour a schematic of quality q puts
// into each of its n projects.
func Points(q quality.Level, n int) int {
	if n > 1 {
		return SchematicPoints2[idx(q)]
	}
	return SchematicPoints[idx(q)]
}

// Cap is the highest skill level a textbook of quality q teaches.
func Cap(q quality.Level) int { return TextbookCap[idx(q)] }

func idx(q quality.Level) int { return min(int(q), 6) }
