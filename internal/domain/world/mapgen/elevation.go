package mapgen

// elevationStep is the "elevation" pass (MAPGEN-2). Stub: owned by its Phase 1 agent.
type elevationStep struct{}

func (elevationStep) Name() string { return "elevation" }

func (elevationStep) Run(c *Ctx) error { return nil }
