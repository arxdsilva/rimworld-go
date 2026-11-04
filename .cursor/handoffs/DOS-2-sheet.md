# DOS-2: `sheet`, the colonist card's pure logic

Package `internal/adapters/inbound/ebiten/dossier/sheet` (no ebiten import). Tests pass with `-race`, coverage 94.6 percent, `go vet` clean, `go test ./internal/arch/` passes, `FuzzPresetDecode` ran 20 s clean (one found case kept in `testdata/fuzz`).

## API for DOS-3 to DOS-6

- Targets: `WhoOf(PawnView)`, `WhoOfAnimal(AnimalView)`, `Who.Letter()`, `WhoMatches`. Letter `p` is the player's creature (the doc's `C`), not a prisoner; prisoners, visitors, allies and raiders are `s`.
- Tree: `Prune`, `Measure`, `MeasureWith(empty)`, `Heights` (fill sharing), `Count`, `Validate`, `At`, `Move`, `Remove`, `Add`, `NewNode`, `CanHold`, `WhoSet`, `SetWho`, `MetricsFor(HUDText)`, `WidgetHeight`. Prune keeps an off-target widget inside a `line` as `gap` with `Def` = its key (a blank with that widget's height); elsewhere it is dropped.
- Catalog: `Catalog()`, `Lookup`, `Groups`, `Defs(key)`, `LookupDef`, `StatKey`/`StatByKey`, `TrainKey`/`TrainByKey`/`TrainLabel`/`TrainDesc`, `IsWidget`, `IsContainer`, `NeedsDef`, `IsBar`. `clothes_pick` is `Reserved`.
- Defaults and presets: `DefaultDocked`, `DefaultFloating`, `Default(docked, who)`, `BuiltIns`, `BuiltIn`, `BuiltInNames`, `EncodePreset`, `DecodePreset` (errors `ErrPresetTooBig/Newer/Invalid`), `IsOlder`, `ValidName`, `Slug`, `Capture(h, name, flags)`, `Apply(p, *HUD)`. `DockW` is the tab width.
- Lines: `StandingLine(p, bio, card)`, `IdentityLine(p, bio, card)`, `BodyLine(p, health)`, `MindLine(p, card)`, animal variants (`AnimalStandingLine`, `AnimalIdentityLine`, `AnimalBodyLine`, `AnimalMindLine`, `AnimalActivityLine`), `ActivityLine`, `QueuedLine`, `EquippedLine`, `CarryingLine`, `TraitsLine`, `CaptiveLine`, `KeeperLine`, `TallyLine`, `StatLine`, `NeedValue`, `FormatStat`.
- Cells: `SkillCell`, `SkillStateOf` (+ `SkillState.Key()` colour key), `SkillValue`, `TrainCell` (zero Cell when the species cannot learn it).
- Tips (`[]TipLine{Text, Tone}`; `JoinTip` for plain): `BioTip`, `HealthTip`, `MindTip`, `FoodTip`, `RestTip`, `FunTip`, `AnimalTip`, `TraitsTip`, `TrainTip`, `SkillTip(skill, card, weaponLine)`, `ActivityTip`, `WeaponLine`.
- Colour: `ToneKey`, `RelationKey`, `RelationTone`, `RelationOf`, `AnimalRelation`, `MoodTipTone`, `BarColor(style, frac, low, main)`, `Lerp`, `ParseColor`, `FormatColor`, `RGBToHSL`, `HSLToRGB`, `BarStyles`.
- `WarnsInWords(Line)`: the spec §59 guard (tone and words agree); tests use it on every ladder.

## Rows

Finished: every DOS-2 row except those below. `TestEveryRHUDRowIsClassified` parses the gap doc (order, statuses, five columns, package named, counts table, DOS-2's rows, widget key list).

Not done or best guess:
- RHUD-119 (tame pack label): no label; DOS-1 settles. RHUD-228, 235, 236, 245, 251, 255: come only through `Card.ExtraLines`; `SkillTip` prints whatever DOS-1 puts there.
- RHUD-244, 253: share the skill's speed line.
- RHUD-286: `stat.ID.Format` was not added to the domain; `FormatStat` in `sheet` does it.
- RHUD-441/457: thought stacks are merged by label in `MindTip`.

Deviations from the doc: `Catalog` is a function, not a map; tree edit functions take paths (`[]int`); `StandingLine`/`IdentityLine` also take `BioView` (for their tooltip); a `Line.Tip` is plain text, coloured tips use the `[]TipLine` builders.
