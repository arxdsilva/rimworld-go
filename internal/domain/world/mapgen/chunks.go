package mapgen

// chunksStep is the "chunks" pass (MAPGEN-2). Stub: owned by its Phase 1 agent.
type chunksStep struct{}

func (chunksStep) Name() string { return "chunks" }

func (chunksStep) Run(c *Ctx) error { return nil }
