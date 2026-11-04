package uikit

import (
	"image/color"
	"slices"
)

// dossierTheme is the colonist card's default colour per key (RHUD family G2).
// The keys are what config.HUD.Colors stores: a player's colour overrides the
// value here for that key only. The tree_* keys colour the layout editor and
// are not player-editable.
var dossierTheme = map[string]color.RGBA{
	"text":          Text,
	"text_off":      DossierOff,
	"rule":          PanelBorder,
	"tone_critical": Danger,
	"tone_warning":  Warning,
	"tone_neutral":  TextDim,
	"tone_good":     Good,
	"tone_top":      DossierExcellent,
	"bar_back":      SliderTrack,
	"bar_main":      SliderFill,
	"bar_low":       Danger,
	"bar_tick":      DossierBarTick,
	"picker_text":   TextDim,
	"picker_back":   Field,
	"skill_like":    DossierSkillLike,
	"skill_love":    DossierSkillLove,
	"skill_full":    DossierSkillFull,
	"skill_busy":    DossierSkillBusy,
	"side_own":      Good,
	"side_ally":     DossierSideAlly,
	"side_free":     TextDim,
	"side_foe":      Danger,
	"side_wild":     DossierSideWild,
	"side_held":     Warning,
	"tree_stack":    DossierTreeStack,
	"tree_block":    DossierTreeBlock,
	"tree_line":     DossierTreeLine,
	"tree_widget":   Text,
	"tree_bad":      Danger,
	"tree_core":     TextDim,
	"tree_mine":     DossierTreeMine,
}

// dossierKeys is the theme's keys in the colour editor's order: card, skill,
// side, then the layout editor's.
var dossierKeys = []string{
	"text", "text_off", "rule", "tone_critical", "tone_warning", "tone_neutral", "tone_good", "tone_top",
	"bar_back", "bar_main", "bar_low", "bar_tick", "picker_text", "picker_back",
	"skill_like", "skill_love", "skill_full", "skill_busy",
	"side_own", "side_ally", "side_free", "side_foe", "side_wild", "side_held",
	"tree_stack", "tree_block", "tree_line", "tree_widget", "tree_bad", "tree_core", "tree_mine",
}

// DossierColor is the theme's default colour for a card colour key. It does not
// allocate, so a draw loop may call it per line.
func DossierColor(key string) (color.RGBA, bool) {
	c, ok := dossierTheme[key]
	return c, ok
}

// DossierColorKeys lists every key, in the colour editor's order, as a copy.
func DossierColorKeys() []string { return slices.Clone(dossierKeys) }
