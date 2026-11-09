package game

import (
	"errors"
	"fmt"
	"log"
	"strings"

	uiadapter "rimworld-go/internal/adapters/inbound/ebiten"
	camerapkg "rimworld-go/internal/adapters/inbound/ebiten/camera"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/outbound/cardpresetfile"
	"rimworld-go/internal/adapters/outbound/filesave"
	"rimworld-go/internal/adapters/outbound/scenariofile"
	appgame "rimworld-go/internal/application/game"
	"rimworld-go/internal/application/game/contentid"
	"rimworld-go/internal/application/game/savefmt"
	"rimworld-go/internal/platform/openfolder"
)

// cardFiles wires the colonist card's presets to <config>/rimworld-go/dossier.
// Captures get none, so they never read or write the player's folder.
func cardFiles(capturing bool) uiadapter.CardFiles {
	if capturing {
		return uiadapter.CardFiles{}
	}
	store, err := cardpresetfile.Default()
	if err != nil {
		log.Printf("card presets not kept: %v", err)
		return uiadapter.CardFiles{}
	}
	return uiadapter.CardFiles{Store: store, Open: openfolder.Open, Folder: store.Dir}
}

// saveHooks wires the UI's save/load to the per-user saves folder, or to
// dir when one is given (-save-dir). Saves and camera sidecars go through
// a filesave.Queue: the hook encodes on the simulation's goroutine and the
// queue's worker compresses and writes (CI-002); closeSaves drains it. Dir
// is the folder used, for the Options "Saves folder" row (SAV-023). build
// is the game's version, written into every save's header with the
// content fingerprint (SAV-003, SAV-025). List checks each new file's body
// (filesave.Verifier, SAV-022) and says how this build reads it (SAV-021).
func saveHooks(dir, build string) uiadapter.SaveHooks {
	store, err := filesave.Default()
	if dir != "" {
		store, err = filesave.Store{Dir: dir}, nil
	}
	if err != nil {
		log.Printf("saving disabled: %v", err)
		return uiadapter.SaveHooks{}
	}
	queue, verifier := filesave.NewQueue(store), filesave.NewVerifier()
	rt := saveRuntime(build, contentid.Current())
	return uiadapter.SaveHooks{
		Queue: saveQueue{queue},
		Dir:   store.Dir,
		// Save writes the slot meta names (the store sanitises it) with meta
		// and the build in the header (SAV-003).
		Save: func(sim uiadapter.Simulation, meta ports.SaveMeta) error {
			g, ok := sim.(*appgame.Game)
			if !ok {
				return fmt.Errorf("this simulation can't be saved")
			}
			return queue.SaveAs(g, savefmt.Stamp{Name: meta.Name, Colony: meta.Colony, ColonyName: meta.ColonyName,
				Base: meta.Base, Autosave: meta.Autosave, Season: meta.Season, Build: build, Content: rt.Content})
		},
		Delete: queue.Delete, // after the write in flight; the slot's waiting jobs are dropped (SAV-010)
		List: func() ([]uiadapter.SaveSlot, error) {
			slots, err := store.List()
			verifier.Verify(slots)
			out := make([]uiadapter.SaveSlot, 0, len(slots))
			for _, sl := range slots {
				out = append(out, slotOf(sl, rt))
			}
			return out, err
		},
		SaveView: func(slot string, views map[appgame.MapID]camerapkg.View) error {
			files := make(map[appgame.MapID]filesave.ViewFile, len(views))
			for id, v := range views {
				files[id] = filesave.ViewFile{CX: v.CX, CY: v.CY, Zoom: v.Zoom}
			}
			return queue.SaveView(slot, filesave.ViewsOf(files))
		},
		LoadView: func(path string) (map[appgame.MapID]camerapkg.View, bool) {
			f, ok := store.LoadView(path)
			if !ok {
				return nil, false
			}
			views := map[appgame.MapID]camerapkg.View{}
			for id, v := range f.PerMap() {
				views[id] = camerapkg.View{CX: v.CX, CY: v.CY, Zoom: v.Zoom}
			}
			return views, true
		},
		Load: func(path string) (uiadapter.Simulation, ports.SaveMeta, error) {
			g, info, err := store.Load(path)
			if err != nil {
				return nil, ports.SaveMeta{}, err
			}
			return g, ports.SaveMeta{Name: info.Name, Colony: info.Colony, ColonyName: info.ColonyName,
				Base: info.Base, Autosave: info.Autosave, Season: info.Season}, nil
		},
	}
}

// saveRuntime is this build as savefmt.CompatOf sees it: the format it
// writes, the oldest it reads, its build and its content.
func saveRuntime(build string, content savefmt.Content) savefmt.Runtime {
	low := appgame.SaveVersion
	for appgame.SupportedSaveVersion(low - 1) {
		low--
	}
	return savefmt.Runtime{Version: appgame.SaveVersion, Min: low, Build: build, Content: content}
}

// slotOf is one save as the load list shows it. Name and Path are the
// file; Title is the name the player typed (the header's). A save of
// another version keeps its header and says which; a damaged body is
// "damaged file".
func slotOf(sl filesave.Slot, rt savefmt.Runtime) uiadapter.SaveSlot {
	i := sl.Info // upgraded: an older header names its colony from its name
	s := uiadapter.SaveSlot{Name: sl.Name, Path: sl.Path, Title: i.Name, Date: i.Date,
		Colonists: i.Colonists, Modified: sl.Modified, Commitment: i.Commitment, Ending: i.Ending,
		Colony: i.Colony, ColonyName: i.ColonyName, Base: i.Base, Autosave: i.Autosave, Day: i.Day, Season: i.Season,
		Storyteller: i.Storyteller, Difficulty: i.Difficulty, Biome: i.Biome, MapSize: i.MapSize,
		Build: i.Build, Version: i.Version}
	switch {
	case sl.Err == nil || errors.Is(sl.Err, appgame.ErrSaveVersion):
		c := savefmt.CompatOf(i, rt)
		s.Compat = int(c)
		switch c {
		case savefmt.CompatNewer:
			s.Err = "Saved by a newer version of the game"
		case savefmt.CompatTooOld:
			s.Err = "Saved by a version too old to load"
		default:
			s.Notice = slotNotice(i, rt, c)
		}
	case errors.Is(sl.Err, savefmt.ErrCorrupt):
		s.Err = "damaged file"
	default:
		s.Err = sl.Err.Error()
	}
	return s
}

// slotNotice is the warning a loadable save shows ("": none): plugins it
// was played with that this build lacks (SAV-025), or content that differs
// from this build's.
func slotNotice(i savefmt.Info, rt savefmt.Runtime, c savefmt.Compat) string {
	if missing := savefmt.MissingPlugins(i, rt); len(missing) > 0 {
		return "Played with plugins this game does not have: " + strings.Join(missing, ", ")
	}
	if c == savefmt.CompatContent {
		return "Played with other game content: it may not load as it was"
	}
	return ""
}

// saveQueue is the background save writer as the UI sees it: the saves'
// results; a camera sidecar that failed is only logged, as before.
type saveQueue struct{ *filesave.Queue }

func (q saveQueue) Done() []uiadapter.SaveResult {
	var out []uiadapter.SaveResult
	for _, r := range q.Queue.Done() {
		if !r.View {
			out = append(out, uiadapter.SaveResult{Slot: r.Slot, Err: r.Err})
		} else if r.Err != nil {
			log.Printf("save camera view: %v", r.Err)
		}
	}
	return out
}

// closeSaves writes what the save queue still holds and stops its worker,
// once the window has closed.
func closeSaves(h uiadapter.SaveHooks) {
	if q, ok := h.Queue.(saveQueue); ok {
		q.Close()
	}
}

// scenarioFiles adapts the scenario file store to the UI's port (NG-1 §5.7).
type scenarioFiles struct{ scenariofile.Store }

func (s scenarioFiles) List() ([]uiadapter.ScenarioFile, error) {
	fs, err := s.Store.List()
	out := make([]uiadapter.ScenarioFile, 0, len(fs))
	for _, f := range fs {
		out = append(out, uiadapter.ScenarioFile{Name: f.Name, Modified: f.Modified.Format("2006-01-02 15:04")})
	}
	return out, err
}

// scenarioStore is where custom scenarios live; captures get none so they
// never read the player's files.
func scenarioStore(capturing bool) uiadapter.ScenarioStore {
	if capturing {
		return nil
	}
	s, err := scenariofile.Default()
	if err != nil {
		log.Printf("custom scenarios unavailable: %v", err)
		return nil
	}
	return scenarioFiles{s}
}
