# OPT-4 (lang, client side): messages and letters drawn through the catalog

OPT-001, Wave 4, client half. W4-SIMTEXT fills `Msg` on the game side, and the contract is `pkg/msgfmt` (0cb6b271f). English output is unchanged: `lang.Msg` returns `Text` when a view has no `Msg`, and English joins the same format and arguments that `fmt.Sprintf` used. Client only, so the state hash doesn't change.

## What changed
- `lang.Msg(tr, msgfmt.Text, text)` returns `text` when `Format` is empty. Otherwise it returns the translated format joined with each argument, and each argument goes through `tr.T` by exact key. As a result, a kind name or a reason that is a key gets translated, and a number or a name is left as it is.
- HUD changes:
  - The message line uses `lang.Msg`, and a letter notice reads "Letter: %s" with the title translated.
  - The letter row's label translates the title.
  - `LetterTextIn` gives the letter window's body and the row's hover card (`right.go`).
  - The window title is `T(title)`. The window id keeps the English title.
  - `OpenLetter` copies `Msg` across.
- The gear tab translates its Drop and Eat refusal reasons (`p.Gear.NoDrop`, `Pack.Why`) with `T`. These are the `game/sense` `Why*` constants.
- Extractor (`lang/simkeys_test.go`): `TestCatalogIsCurrent` also scans `internal/application/game/**`.
  - Seed posters: `message`, `threat`, `worldNote`, `letter` and `letterf`. Every host or helper that forwards a parameter to one of them is found by itself (for example `h.Note`, `h.Letter`, `raidEvents.Threat`).
  - Keys come from:
    - posters: literal or same-package constant formats (as `msgfmt.Key`), titles, plain letter texts (as `msgfmt.Plain`) and literal arguments;
    - `msgfmt.Of`/`Plain` calls;
    - `MessageView`/`LetterView` `Title:` literals;
    - `sense.Why*`.
  - en.json has about 220 simulation keys.
- Tests:
  - `lang`: `TestMsgIsTheTextInEnglishAndTranslatesFormatAndArgs`, `TestSimKeysSeeEveryForm`.
  - `hud`: `TestMessagesAndLettersGoThroughTheCatalog`, `TestLetterWindowAndCardTranslate`. They use hand-built views with and without `Msg`, in English and in a fake French.
  - `inspector/gear`: `TestRefusalReasonsAreTranslated`.

## Gaps
- A letter whose text is put together outside a poster (for example `fmt.Sprintf` handed to `h.Letter`) is keyed on its finished sentence, which no template can hold. W4-SIMTEXT is converting those to `letterf`.
- The developer panels (`devtools/panels.go`) stay in English, as `exempt.txt` says.
- Item and species names inside arguments are translated only when they are keys. Nothing extracts them yet.
