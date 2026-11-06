# DOS-7: letters and the floating card

- `hudlayout/layout.go` letters block: `Inputs.LetterGap` (0: `HudGap`) spaces rows; `Layout.LetterGap` carries it to `hud`. `LetterAvoid` (set by `colony_compose.go` only while the card floats and `CompressLetters` is on) makes the stack take the larger free band below or above the card when the card overlaps the column; empty or off the column: layout identical to before.
- `hud/letters.go`: the "+N older" row is a button (`letter.<id>` of the newest folded letter, `LetterRow.Opens`), hover drawn; rows use `Layout.LetterGap`.
- Tests: `hud/letters_test.go`, `TestLettersAvoidTheCard` (hudlayout), `TestJourneyLetterOlderOpens`.
- Deviation: root `letters_test.go` `TestLettersPlusNOlder` asserted the older row had no button; changed to require the button opening letter 3 (spec change RHUD-474).
- Not done: the alerts above a card-avoiding stack are not moved off the card.
