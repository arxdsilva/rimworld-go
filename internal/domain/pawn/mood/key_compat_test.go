// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Hand-maintained fixture of the thought keys plugins patch by. The pairs
// are written out literally so a rename of a Kind or a key fails here.
package mood

import "testing"

func TestThoughtKeysArePinned(t *testing.T) {
	t.Parallel()
	pinned := [...]struct {
		kind Kind
		key  string
	}{
		{SleptOutside, "core.slept_outside"},
		{SleptOnGround, "core.slept_on_ground"},
		{SleptInBed, "core.slept_in_bed"},
		{SleptInAwfulRoom, "core.slept_in_awful_room"},
		{SleptInNiceRoom, "core.slept_in_nice_room"},
		{AteMeal, "core.ate_meal"},
		{AteRaw, "core.ate_raw"},
		{AteAtTable, "core.ate_at_table"},
		{AteWithoutTable, "core.ate_without_table"},
		{Hungry, "core.hungry"},
		{Tired, "core.tired"},
		{Cold, "core.cold"},
		{Freezing, "core.freezing"},
		{Hot, "core.hot"},
		{Sweltering, "core.sweltering"},
		{RecreationStarved, "core.recreation_starved"},
		{Bored, "core.bored"},
		{Entertained, "core.entertained"},
		{Catharsis, "core.catharsis"},
		{Sick, "core.sick"},
		{InPain, "core.in_pain"},
		{ColonistDied, "core.colonist_died"},
		{UnburiedCorpse, "core.unburied_corpse"},
		{LaidToRest, "core.laid_to_rest"},
		{Soaked, "core.soaked"},
		{SnowDay, "core.snow_day"},
		{Landed, "core.landed"},
		{LovesWork, "core.loves_work"},
		{Difficulty, "core.difficulty"},
		{Insulted, "core.insulted"},
		{KindWords, "core.kind_words"},
		{RebuffedMood, "core.rebuffed_mood"},
		{HadToRebuff, "core.had_to_rebuff"},
		{CheatedOn, "core.cheated_on"},
		{ProposalRejected, "core.proposal_rejected"},
		{GotMarried, "core.got_married"},
		{AttendedWedding, "core.attended_wedding"},
		{AttendedParty, "core.attended_party"},
		{BrokenUpWith, "core.broken_up_with"},
		{Divorced, "core.divorced"},
		{Lovin, "core.lovin"},
		{PartnerOpinion, "core.partner_opinion"},
		{SleptAlone, "core.slept_alone"},
		{Hungover, "core.hungover"},
		{Drunk, "core.drunk"},
		{Withdrawal, "core.withdrawal"},
		{HighOnDrug, "core.high_on_drug"},
		{BondedMaster, "core.bonded_master"},
		{BondedNotMaster, "core.bonded_not_master"},
		{BondedDied, "core.bonded_died"},
		{BondedLost, "core.bonded_lost"},
		{BondedGivenAway, "core.bonded_given_away"},
		{BondedReleased, "core.bonded_released"},
		{Imprisoned, "core.imprisoned"},
		{ColonistLost, "core.colonist_lost"},
		{DisfiguredSelf, "core.disfigured_self"},
		{MissingPart, "core.missing_part"},
		{Surgery, "core.surgery"},
		{Grief, "core.grief"},
		{OrganHarvested, "core.organ_harvested"},
		{KnowOrganHarvested, "core.know_organ_harvested"},
		{KnowColonistOrganHarvested, "core.know_colonist_organ_harvested"},
		{SleptInBedroom, "core.slept_in_bedroom"},
		{SleptInBarracks, "core.slept_in_barracks"},
		{SleptInHospital, "core.slept_in_hospital"},
		{AteInDiningRoom, "core.ate_in_dining_room"},
		{EnjoyedRecRoom, "core.enjoyed_rec_room"},
		{SharedBed, "core.shared_bed"},
		{RattyApparel, "core.ratty_apparel"},
		{TatteredApparel, "core.tattered_apparel"},
		{TaintedApparel, "core.tainted_apparel"},
		{ComfortMood, "core.comfort_mood"},
		{BeautyMood, "core.beauty_mood"},
		{OutdoorsMood, "core.outdoors_mood"},
		{SleptCold, "core.slept_cold"},
		{SleptHot, "core.slept_hot"},
		{AtePaste, "core.ate_paste"},
		{AteFineMeal, "core.ate_fine_meal"},
		{AteLavishMeal, "core.ate_lavish_meal"},
		{AteKibble, "core.ate_kibble"},
		{AteHumanMeat, "core.ate_human_meat"},
		{AteInsectMeat, "core.ate_insect_meat"},
		{ButcheredHuman, "core.butchered_human"},
		{ObservedButchery, "core.observed_butchery"},
		{Ravenous, "core.ravenous"},
		{Malnourished, "core.malnourished"},
		{AteHumanMeatCooked, "core.ate_human_meat_cooked"},
		{AteInsectMeatCooked, "core.ate_insect_meat_cooked"},
		{ToxicFallout, "core.toxic_fallout"},
		{Chilly, "core.chilly"},
		{Shivering, "core.shivering"},
		{NumbingCold, "core.numbing_cold"},
		{AbsolutelyFreezing, "core.absolutely_freezing"},
		{Sweaty, "core.sweaty"},
		{Overheated, "core.overheated"},
		{Baking, "core.baking"},
		{BlisteringHot, "core.blistering_hot"},
		{Naked, "core.naked"},
		{TurnedAwayWanderer, "core.turned_away_wanderer"},
		{RescuedByColony, "core.rescued_by_colony"},
		{BrawlerRanged, "core.brawler_ranged"},
		{ExpectationsExtremelyLow, "core.expectations_extremely_low"},
		{ExpectationsVeryLow, "core.expectations_very_low"},
		{ExpectationsLow, "core.expectations_low"},
		{ExpectationsModerate, "core.expectations_moderate"},
		{ExpectationsHigh, "core.expectations_high"},
		{ExpectationsSkyHigh, "core.expectations_sky_high"},
		{BlissCircuit, "core.bliss_circuit"},
		{AugmentFan, "core.augment_fan"},
		{NaturalPurist, "core.natural_purist"},
		{AnestheticWoozy, "core.anesthetic_woozy"},
		{RecreationDeprived, "core.recreation_deprived"},
		{RecreationSatisfied, "core.recreation_satisfied"},
		{FatalOrganHarvest, "core.fatal_organ_harvest"},
	}
	if len(pinned) != int(numKinds) {
		t.Fatalf("%d pinned keys for %d kinds", len(pinned), numKinds)
	}
	for id, p := range pinned {
		if p.kind != Kind(id) {
			t.Fatalf("pinned entry %d is kind %d: list is out of Kind order", id, p.kind)
		}
		if got := cb.Key(p.kind); got != p.key {
			t.Fatalf("kind %d key %q, want %q", p.kind, got, p.key)
		}
	}
}
