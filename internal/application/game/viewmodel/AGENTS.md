# internal/application/game/viewmodel

## Purpose
Read-only snapshot DTOs, value comparisons and letter projection for native game readers and presentation (LOG-002, PLAY structured letters). The simulation supplies data; these helpers never mutate it.

## Owns
PawnView, ItemView, FrameView, BuildingView, LetterView and related records. EqualBuilding, EqualFrame and EqualItem compare visible fields including slices and optional coordinates.

## Contracts
Append stable fields with matching equality checks. Clone or project mutable collections for independently owned views; shared frame views are read-only. Preserve structured letter arguments. Do not import root game, renderer or input handlers.

## Hot paths
Comparisons and projections run on frequent reads; preserve memoization keys and avoid unnecessary copies. Never traverse the simulation from a comparison.

## Tests
Root TestBuildingViewEqual, TestFrameViewEqual and TestSameItemViewSeesEveryField cover field comparisons. Letter and UI journey tests cover display contracts.
