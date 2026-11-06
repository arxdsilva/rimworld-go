# internal/adapters/inbound/ebiten/ids

## Purpose
The click and widget ids that the client's screens, HUD regions and feature panels share, so a feature can name another's button without importing it, and the root's click router can be filled by registration (`.agents/specs/SOLID_GO.md` §4: one prefix dispatch per boundary). An id private to one feature stays a literal in that feature.

## Owns
- String constants only: the `Prefix*` values (`letter.`, `speed.`, `bar.pawn.`, `research.pick.`, `animals.`, `sched.brush.`, `sched.cell.`, `arch.cat.`, `arch.stuff.`, `arch.crop.`, `arch.tool.`, `float.`, `load.slot.`, and the rest) and the shared button ids.
- Prefix routing: a click id is dispatched to the feature that owns its prefix; the text after the prefix is that feature's own.

## Depends on
- Nothing: `ids` is a leaf with no imports.

## Must not depend on
- Every package. Adding an import here means it is no longer a leaf; reject it.

## Contracts
- Constants only. Features compare against them; the root's `clickRoutes` maps prefixes to features. A click route is one prefix comparison per click.
- Ids are UI routing names, never saved.

## Invariants
- Prefixes end with a dot, so a prefix match cannot cut into another prefix's text.
- An id used by two packages lives here; an id used by one package stays in that package.

## Tests
- None in the package. Click routing is covered by the root's journey tests.

## Notes
- New shared ids are added here before a second package uses them, so the router never needs a feature import.
