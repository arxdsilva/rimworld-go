package mapgen

// lakesStep is the "lakes" pass (MAPGEN-2). Stub: owned by its Phase 1 agent.
type lakesStep struct{}

func (lakesStep) Name() string { return "lakes" }

func (lakesStep) Run(c *Ctx) error { return nil }
