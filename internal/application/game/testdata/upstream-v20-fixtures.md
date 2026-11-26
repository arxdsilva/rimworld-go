# Historical upstream v20 fixtures

The three binary fixtures were generated with the unmodified public Save
implementation from upstream commit
`4e28e30ba6d23b8f17c5a5d7487e9006fe8769e4` (SaveVersion 20).

Generation: create a broken_hull setup with map side 60 and world seed
text `comparison-v20`; add one live generated pawn named Migration.
Set that pawn and all three setup candidates to the existing Scrapper
and LightFooted traits. Save the same state three times with
Difficulty.FriendlyFirePct set to 0, 37 and 100, using the native
Game.Save method and save name comparison. No historical wire records
from the new loader were used to create these files.

TestUpstreamV20SaveFixtures checks the original version, the exact
friendly-fire value, live and setup trait identity, and the migrated
game's StateHash after saving and loading again.
