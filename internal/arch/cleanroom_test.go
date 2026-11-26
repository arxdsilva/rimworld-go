// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package arch

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// proprietaryRefs matches names from the commercial game's implementation:
// its C# class families (JobGiver_X, WorkGiver_X ...), engine types and Def
// fields the wiki does not publish, and words that say code was read
// (CLEAN_ROOM.md Policy; review procedure in
// .agents/skills/rimworld-go-clean-room/SKILL.md). Wiki page names in
// `// wiki: <page>` are fine; these are not.
func proprietaryRefs() *regexp.Regexp {
	families := `JobGiver|JobDriver|WorkGiver|ThinkNode|ThoughtWorker|IncidentWorker|InteractionWorker|StorytellerComp|QuestNode|Toils|ITab|MainTabWindow|CompProperties|Designator|MentalState|SitePart|GenStep|PawnRenderNode|HediffComp`
	engine := `WorldPawns|DefDatabase|DefOf\.[A-Z]|DefPreventing[A-Z]\w*|PassThroughOnly|CoexistsWithFloors|ThingDef|PawnKindDef|HediffDef|ThoughtDef|WorkTypeDef|ResearchProjectDef|IncidentDef|Verse\.[A-Z]|RimWorld\.[A-Z]|Assembly-CSharp`
	read := `[Dd]ecompil\w*|ILSpy|dnSpy`
	return regexp.MustCompile(`\b(?:(?:` + families + `)_[A-Z]\w*|` + engine + `|` + read + `)`)
}

// cleanRoomScanned are the trees that ship: code, definitions, locale,
// generator scripts and package docs. The policy files under .agents name
// these words on purpose and are not scanned.
var cleanRoomScanned = []string{"cmd", "internal", "pkg", "scripts", "assets/metadata", "assets/locale"}

func cleanRoomText(name string) bool {
	switch filepath.Ext(name) {
	case ".go", ".json", ".toml", ".po", ".pot", ".py", ".md", ".txt", ".yaml", ".yml", ".csv":
		return true
	}
	return false
}

// TestNoProprietaryImplementationNames: nothing that ships names the
// commercial game's classes, methods or Def fields, or says its code was
// decompiled (CLEAN_ROOM.md). Reword the rule in our terms and cite a public
// wiki page with `// wiki: <page>`, or mark it `// original tuning`.
func TestNoProprietaryImplementationNames(t *testing.T) {
	t.Parallel()
	root := moduleRoot(t)
	re := proprietaryRefs()
	self := filepath.ToSlash(filepath.Join("internal", "arch", "cleanroom_test.go"))
	var hits []string
	for _, top := range cleanRoomScanned {
		dir := filepath.Join(root, top)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}
		err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(root, path)
			rel = filepath.ToSlash(rel)
			if d.IsDir() {
				if d.Name() == "testdata" || strings.HasPrefix(d.Name(), ".") {
					return filepath.SkipDir
				}
				return nil
			}
			if rel == self || !cleanRoomText(d.Name()) {
				return nil
			}
			src, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			hits = append(hits, scanProprietary(re, rel, src)...)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(hits) > 0 {
		sort.Strings(hits)
		t.Fatalf("names from the commercial game's implementation (CLEAN_ROOM.md; reword in our terms, cite `// wiki: <page>` or mark `// original tuning`):\n%s", strings.Join(hits, "\n"))
	}
}

func scanProprietary(re *regexp.Regexp, rel string, src []byte) []string {
	var hits []string
	sc := bufio.NewScanner(bytes.NewReader(src))
	sc.Buffer(make([]byte, 0, 64*1024), 4<<20)
	for n := 1; sc.Scan(); n++ {
		if m := re.FindString(sc.Text()); m != "" {
			hits = append(hits, fmt.Sprintf("%s:%d: %s", rel, n, m))
		}
	}
	return hits
}

// TestProprietaryRefsSeesEach: the scan catches each kind of reference and
// leaves our own vocabulary and wiki citations alone.
func TestProprietaryRefsSeesEach(t *testing.T) {
	t.Parallel()
	re := proprietaryRefs()
	for _, bad := range []string{
		"// Vanilla JobGiver_ConfigurableHostilityResponse.TryGetAttackNearbyEnemyJob:",
		"// the core equivalent of WorldPawns.ShouldMothball plus",
		"// DefPreventingMothball. Non-caravan stashed pawns",
		"PassThroughOnly bool",
		"// FloorBarrier is !CoexistsWithFloors",
		"// read from the decompiled source",
		"x := ThingDefOf.Steel",
		"using Verse.AI;",
	} {
		if re.FindString(bad) == "" {
			t.Errorf("not caught: %q", bad)
		}
	}
	for _, good := range []string{
		"// wiki: Mental inspiration (Go frenzy)",
		"NoStop bool",
		"func cellStandable(g *Game, x, y int) bool",
		"rimworld-go is inspired by RimWorld. It is not affiliated with",
		"construction.DefOf(b.Kind)",
		"biome.DefOf(b).AnimalDensity100",
		"type Mothballed map[entity.EntityID]bool",
	} {
		if m := re.FindString(good); m != "" {
			t.Errorf("false positive %q in %q", m, good)
		}
	}
}
