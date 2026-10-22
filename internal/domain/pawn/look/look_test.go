package look

import "testing"

// refLook is a verbatim copy of the pre-NG-1 renderer's lookFor
// (internal/adapters/inbound/ebiten/art.go), kept as the reference the
// domain port must reproduce.
type refLook struct {
	Skin, Hair, HairStyle, Shirt, Pants, Vest string
}

var refHairStyles = []string{"hair_short", "hair_crop", "hair_bun"}

func refLookFor(id uint64, skins, hairs, cloths []string) refLook {
	pick := func(salt uint64, opts []string) string {
		if len(opts) == 0 {
			return ""
		}
		return opts[mix64(id*0x9E3779B97F4A7C15+salt)%uint64(len(opts))]
	}
	l := refLook{
		Skin:      pick(1, skins),
		Hair:      pick(2, hairs),
		HairStyle: pick(3, refHairStyles),
		Shirt:     pick(4, cloths),
		Pants:     pick(5, cloths),
	}
	if mix64(id+6)%2 == 0 {
		l.Vest = pick(7, cloths)
	}
	if l.Pants == l.Shirt && len(cloths) > 1 {
		for _, c := range cloths {
			if c != l.Shirt {
				l.Pants = c
				break
			}
		}
	}
	return l
}

// resolve turns a domain Look back into the renderer's names.
func resolve(l Look, skins, hairs, cloths []string) refLook {
	name := func(i uint8, opts []string) string {
		if len(opts) == 0 {
			return ""
		}
		return opts[int(i)%len(opts)]
	}
	r := refLook{
		Skin:      name(l.Skin, skins),
		Hair:      name(l.Hair, hairs),
		HairStyle: "hair_" + HairStyles[l.HairStyle],
		Shirt:     name(l.Shirt, cloths),
		Pants:     name(l.Pants, cloths),
	}
	if l.Vest {
		r.Vest = name(l.VestCloth, cloths)
	}
	return r
}

// The palettes as the renderer passes them today (sorted tint names,
// cloth without the raider tint).
var (
	skins  = []string{"brown", "dark", "light", "pale", "tan"}
	hairs  = []string{"auburn", "black", "blond", "brown", "grey"}
	cloths = []string{"cream", "navy", "olive", "rust", "sand", "slate"}
)

func TestLegacyLookMatchesRenderer(t *testing.T) {
	// Golden values printed by the renderer's lookFor before NG-1.
	golden := []struct {
		id                                       uint64
		skin, hair, style, shirt, pants, vestCol string
	}{
		{1, "brown", "auburn", "hair_short", "sand", "olive", "rust"},
		{2, "tan", "black", "hair_short", "sand", "cream", "cream"},
		{3, "brown", "black", "hair_short", "rust", "slate", ""},
		{4, "brown", "black", "hair_bun", "cream", "slate", ""},
		{5, "dark", "grey", "hair_short", "navy", "cream", ""},
		{7, "brown", "blond", "hair_short", "cream", "rust", ""},
		{12, "brown", "auburn", "hair_short", "rust", "sand", ""},
		{99, "brown", "auburn", "hair_short", "navy", "slate", ""},
		{1000, "dark", "black", "hair_short", "navy", "cream", "cream"},
		{123456789, "dark", "black", "hair_bun", "olive", "cream", ""},
	}
	for _, g := range golden {
		got := resolve(Legacy(g.id, len(skins), len(hairs), len(cloths)), skins, hairs, cloths)
		want := refLook{g.skin, g.hair, g.style, g.shirt, g.pants, g.vestCol}
		if got != want {
			t.Fatalf("id %d: got %+v want %+v", g.id, got, want)
		}
	}
	// Every id against the reference, over several palette shapes
	// (including empty and single-entry palettes).
	palettes := []struct{ s, h, c []string }{
		{skins, hairs, cloths},
		{skins[:2], hairs[:1], cloths[:2]},
		{skins[:1], hairs, cloths[:1]},
		{nil, nil, nil},
		{skins, hairs[:3], append(append([]string(nil), cloths...), "teal", "umber")},
	}
	for _, p := range palettes {
		for id := uint64(0); id < 60; id++ {
			l := Legacy(id, len(p.s), len(p.h), len(p.c))
			if l.Sex != Male || l.Build != Average || l.Beard != 0 || l.Grey || l.Bare || l.HairStyle >= 3 {
				t.Fatalf("id %d: legacy look not male/average/plain: %+v", id, l)
			}
			got, want := resolve(l, p.s, p.h, p.c), refLookFor(id, p.s, p.h, p.c)
			if got != want {
				t.Fatalf("id %d palettes %d/%d/%d: got %+v want %+v", id, len(p.s), len(p.h), len(p.c), got, want)
			}
		}
	}
}

func TestBuildKeyAndNames(t *testing.T) {
	cases := map[Look]string{
		{}:                            "m_avg",
		{Sex: Female, Build: Thin}:    "f_thin",
		{Sex: Male, Build: Heavy}:     "m_heavy",
		{Sex: Female, Build: Heavy}:   "f_heavy",
		{Sex: Female, Build: Average}: "f_avg",
	}
	for l, want := range cases {
		if l.BuildKey() != want {
			t.Fatalf("%+v: %q want %q", l, l.BuildKey(), want)
		}
	}
	if Male.String() != "male" || Female.String() != "female" ||
		Thin.String() != "thin" || Heavy.String() != "heavy" || Average.String() != "average" {
		t.Fatal("names")
	}
}
