package ebiten

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"rimworld-go/internal/platform/assetroot"
)

// DefaultPackRel is the vendor character pack, relative to the repo root.
const DefaultPackRel = "assets/Free 2D Animated Vector Game Character Sprites"

// Pack is a filesystem inventory of the vendor sprite pack. Paths are
// absolute. Discovery does not decode PNGs.
type Pack struct {
	Root      string
	Colonists []Character
	Enemies   []Character
	Ground    []string
	Rocks     []string
	Weapons   []string
	Extras    []string
	Crosshair string
	Death     []string
	BodyParts []BodyPart
	PNGCount  int
}

// Character is one full-body animated figure.
type Character struct {
	ID    string
	Dir   string
	Anims map[string][]string
}

// BodyPart is one layered vendor part (hair, eyes, …). Catalogued for
// later compositional pawn work; the first-playable renderer does not
// load these 2048px sheets.
type BodyPart struct {
	Category string
	Variant  string
	Dir      string
	Anims    map[string][]string
}

// LocatePack finds DefaultPackRel above the working directory or the
// executable (assetroot).
func LocatePack() (string, error) {
	p, err := assetroot.Find(DefaultPackRel, true)
	if err != nil {
		return "", fmt.Errorf("sprite pack: %w", err)
	}
	return p, nil
}

// DiscoverPack inventories PNGs under root. It does not load textures.
func DiscoverPack(root string) (Pack, error) {
	st, err := os.Stat(root)
	if err != nil {
		return Pack{}, err
	}
	if !st.IsDir() {
		return Pack{}, fmt.Errorf("sprite pack %q is not a directory", root)
	}
	p := Pack{Root: root}

	full := filepath.Join(root, "Full body animated characters")
	entries, err := os.ReadDir(full)
	if err != nil {
		return Pack{}, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if name == "Enemies" {
			enemies, err := discoverCharacters(filepath.Join(full, name), "vendor_enemy")
			if err != nil {
				return Pack{}, err
			}
			p.Enemies = enemies
			continue
		}
		ch, err := discoverOneCharacter(filepath.Join(full, name), vendorID("vendor_char", name))
		if err != nil {
			return Pack{}, err
		}
		p.Colonists = append(p.Colonists, ch)
	}

	env := filepath.Join(root, "Environment")
	if err := walkPNGs(env, func(path string) {
		base := strings.ToLower(filepath.Base(path))
		if strings.Contains(base, "rock") {
			p.Rocks = append(p.Rocks, path)
			return
		}
		p.Ground = append(p.Ground, path)
	}); err != nil {
		return Pack{}, err
	}
	sort.Strings(p.Ground)
	sort.Strings(p.Rocks)

	if err := walkPNGs(filepath.Join(root, "Weapons"), func(path string) {
		p.Weapons = append(p.Weapons, path)
	}); err != nil {
		return Pack{}, err
	}
	sort.Strings(p.Weapons)

	if err := walkPNGs(filepath.Join(root, "Generic death animation"), func(path string) {
		p.Death = append(p.Death, path)
	}); err != nil {
		return Pack{}, err
	}
	sort.Strings(p.Death)

	extras := filepath.Join(root, "Extras")
	if err := walkPNGs(extras, func(path string) {
		p.Extras = append(p.Extras, path)
		if strings.EqualFold(filepath.Base(path), "crosshair.png") {
			p.Crosshair = path
		}
	}); err != nil {
		return Pack{}, err
	}
	sort.Strings(p.Extras)

	partsRoot := filepath.Join(root, "Animated body parts")
	cats, err := os.ReadDir(partsRoot)
	if err != nil {
		return Pack{}, err
	}
	sort.Slice(cats, func(i, j int) bool { return cats[i].Name() < cats[j].Name() })
	for _, cat := range cats {
		if !cat.IsDir() {
			continue
		}
		catDir := filepath.Join(partsRoot, cat.Name())
		vars, err := os.ReadDir(catDir)
		if err != nil {
			return Pack{}, err
		}
		sort.Slice(vars, func(i, j int) bool { return vars[i].Name() < vars[j].Name() })
		for _, v := range vars {
			if !v.IsDir() {
				continue
			}
			dir := filepath.Join(catDir, v.Name())
			anims, err := listAnims(dir)
			if err != nil {
				return Pack{}, err
			}
			p.BodyParts = append(p.BodyParts, BodyPart{
				Category: cat.Name(),
				Variant:  v.Name(),
				Dir:      dir,
				Anims:    anims,
			})
		}
	}

	if err := walkPNGs(root, func(string) { p.PNGCount++ }); err != nil {
		return Pack{}, err
	}
	return p, nil
}

func vendorID(prefix, folder string) string {
	n := strings.ToLower(strings.ReplaceAll(folder, " ", "_"))
	return prefix + "_" + n
}

func discoverCharacters(dir, prefix string) ([]Character, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	var out []Character
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		ch, err := discoverOneCharacter(filepath.Join(dir, e.Name()), vendorID(prefix, e.Name()))
		if err != nil {
			return nil, err
		}
		out = append(out, ch)
	}
	return out, nil
}

func discoverOneCharacter(dir, id string) (Character, error) {
	variant := filepath.Join(dir, "with hands")
	if st, err := os.Stat(variant); err != nil || !st.IsDir() {
		variant = filepath.Join(dir, "no hands")
	}
	if _, err := os.Stat(variant); err != nil {
		variant = dir
	}
	anims, err := listAnims(variant)
	if err != nil {
		return Character{}, err
	}
	return Character{ID: id, Dir: variant, Anims: anims}, nil
}

func listAnims(dir string) (map[string][]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := map[string][]string{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.EqualFold(filepath.Ext(name), ".png") {
			continue
		}
		anim := strings.TrimSuffix(name, filepath.Ext(name))
		if i := strings.LastIndex(anim, "_"); i >= 0 {
			anim = anim[:i]
		}
		out[anim] = append(out[anim], filepath.Join(dir, name))
	}
	for k := range out {
		sort.Strings(out[k])
	}
	return out, nil
}

func walkPNGs(dir string, fn func(path string)) error {
	st, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !st.IsDir() {
		return nil
	}
	return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(d.Name()), ".png") {
			fn(path)
		}
		return nil
	})
}
