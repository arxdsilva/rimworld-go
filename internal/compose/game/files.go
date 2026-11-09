package game

import (
	"errors"
	"fmt"
	"log"

	uiadapter "rimworld-go/internal/adapters/inbound/ebiten"
	camerapkg "rimworld-go/internal/adapters/inbound/ebiten/camera"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/outbound/cardpresetfile"
	"rimworld-go/internal/adapters/outbound/filesave"
	"rimworld-go/internal/adapters/outbound/scenariofile"
	appgame "rimworld-go/internal/application/game"
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
// is the folder used, for the Options "Saves folder" row (SAV-023).
func saveHooks(dir string) uiadapter.SaveHooks {
	store, err := filesave.Default()
	if dir != "" {
		store, err = filesave.Store{Dir: dir}, nil
	}
	if err != nil {
		log.Printf("saving disabled: %v", err)
		return uiadapter.SaveHooks{}
	}
	queue := filesave.NewQueue(store)
	return uiadapter.SaveHooks{
		Queue: saveQueue{queue},
		Dir:   store.Dir,
		// Save writes the slot meta names (the store sanitises it; SAV-003
		// writes the rest of meta into the header).
		Save: func(sim uiadapter.Simulation, meta ports.SaveMeta) error {
			g, ok := sim.(*appgame.Game)
			if !ok {
				return fmt.Errorf("this simulation can't be saved")
			}
			return queue.Save(g, meta.Name)
		},
		Delete: func(string) error { return errors.New("not yet") }, // the load list's delete fills it
		List: func() ([]uiadapter.SaveSlot, error) {
			slots, err := store.List()
			out := make([]uiadapter.SaveSlot, 0, len(slots))
			for _, sl := range slots {
				s := uiadapter.SaveSlot{Name: sl.Name, Path: sl.Path, Date: sl.Info.Date,
					Colonists: sl.Info.Colonists, Modified: sl.Modified, Commitment: sl.Info.Commitment, Ending: sl.Info.Ending}
				if sl.Err != nil {
					s.Err = sl.Err.Error()
				}
				out = append(out, s)
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
			return g, ports.SaveMeta{Name: info.Name}, nil
		},
	}
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
