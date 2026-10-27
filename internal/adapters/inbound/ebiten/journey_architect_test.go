package ebiten

// journey_architect_test.go: Architect journeys (QA-001). The debts below are what this
// file still owes the guard (TestEveryInputActionHasAJourney); delete a
// name here once a journey in this file covers it.
var _ = owe("journey_architect_test.go",
	[]string{"DesignateBuild", "DesignateCancel", "DesignateDeconstruct", "DesignateHarvest", "DesignateCut", "DesignateMine", "DesignateSmooth", "DesignatePlan", "DesignateForbid", "DesignateHaul", "DesignateHunt", "DesignateTame", "CancelFrame", "MoveBuilding", "Undesignate"},
	[]string{"tab.architect", "arch.cancel_tool", "arch.deconstruct_tool", "arch.rotate_left", "arch.rotate_right", "arch.constrain"})
