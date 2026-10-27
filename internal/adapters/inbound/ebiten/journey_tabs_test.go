package ebiten

// journey_tabs_test.go: main tab journeys (QA-001). The debts below are what this
// file still owes the guard (TestEveryInputActionHasAJourney); delete a
// name here once a journey in this file covers it.
var _ = owe("journey_tabs_test.go",
	[]string{"SetWorkPriority", "SetSchedule", "SetMedicalCare", "SetDefaultCare", "SetDrugPolicy", "SetResearch", "SetAnimalFollow", "SetTraining", "SetAnimalMaster", "ReleaseAnimal"},
	[]string{"tab.work", "tab.schedule", "tab.assign", "tab.animals", "tab.wildlife", "tab.research", "tab.world", "tab.history", "qty.x10", "qty.x1000"})
