package animal

import "testing"

// TestSeasonWordsAndBands: the helpers moved from the game (FOOD-6) keep
// their words and numbers.
func TestSeasonWordsAndBands(t *testing.T) {
	t.Parallel()
	if Plural(TimberWolf) != "timber wolves" || Plural(Goose) != "geese" || Plural(Deer) != "deer" {
		t.Errorf("plurals %q %q %q", Plural(TimberWolf), Plural(Goose), Plural(Deer))
	}
	if ClimateBand(0) != HabCold || ClimateBand(150) != HabTemperate || ClimateBand(250) != HabHot {
		t.Error("climate bands")
	}
	if SeasonWeight(Caribou, "Winter") != weightFavoured || SeasonWeight(Rat, "Summer") != weightAnyHab {
		t.Error("season weights")
	}
	if got := YoungPhrase(Cow, 2); got != "2 cow calves" {
		t.Errorf("young phrase %q", got)
	}
	if got := YoungPhrase(Alpaca, 1); got[:3] != "an " {
		t.Errorf("young phrase %q", got)
	}
}
