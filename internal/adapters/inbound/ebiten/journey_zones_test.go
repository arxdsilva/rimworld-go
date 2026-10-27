package ebiten

// journey_zones_test.go: zones, areas and pens journeys (QA-001). The debts below are what this
// file still owes the guard (TestEveryInputActionHasAJourney); delete a
// name here once a journey in this file covers it.
var _ = owe("journey_zones_test.go",
	[]string{"DesignateGrowZone", "DesignateStockpile", "DesignateHome", "DesignateRoof", "DesignateRemoveRoof", "DesignateSnowClear", "DesignatePen", "DeleteZone", "ExpandZone", "RemoveZoneCells", "RenameZone", "SetZoneCrop", "SetZoneCut", "SetZoneHidden", "SetZonePriority", "SetZoneSow", "PaintArea", "CreateArea", "DeleteArea", "InvertArea", "RenameArea", "AssignArea", "SetPenSettings", "SetTroughFilter"},
	[]string{})
