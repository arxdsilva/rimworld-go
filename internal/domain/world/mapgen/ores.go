package mapgen

// oresStep is the "ores" pass (MAPGEN-2). Stub: owned by its Phase 1 agent.
type oresStep struct{}

func (oresStep) Name() string { return "ores" }

func (oresStep) Run(c *Ctx) error { return nil }
