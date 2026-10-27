package ebiten

// journey_select_test.go: selection and inspection journeys (QA-001). The debts below are what this
// file still owes the guard (TestEveryInputActionHasAJourney); delete a
// name here once a journey in this file covers it.
var _ = owe("journey_select_test.go",
	[]string{},
	[]string{"view.hide_ui", "dialog.cancel", "dialog.accept", "select.cycle_cell"})
