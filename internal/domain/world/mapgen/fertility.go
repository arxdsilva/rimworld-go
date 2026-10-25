package mapgen

// fertilityStep is the "fertility" pass (MAPGEN-2). Stub: owned by its Phase 1 agent.
type fertilityStep struct{}

func (fertilityStep) Name() string { return "fertility" }

func (fertilityStep) Run(c *Ctx) error { return nil }
