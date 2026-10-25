package game

import (
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/animal/training"
	"rimworld-go/internal/domain/sim/entity"
)

// youngByMother maps each mother on the map to her young's names, in
// animalIDs order (one pass; nil when no animal knows its mother).
func (g *Game) youngByMother() map[entity.EntityID][]string {
	var out map[entity.EntityID][]string
	for _, id := range g.animalIDs {
		a := g.animals.Ref(id)
		if a == nil || a.Mother == 0 {
			continue
		}
		if out == nil {
			out = map[entity.EntityID][]string{}
		}
		m := entity.EntityID(a.Mother)
		out[m] = append(out[m], animalLabel(*a))
	}
	return out
}

// fillAnimalBonds fills an animal view's master, bond, training, mother
// and offspring (ENG-003, M10).
func (g *Game) fillAnimalBonds(av *AnimalView, id entity.EntityID, a animal.State, now uint64, young map[entity.EntityID][]string) {
	if a.Mother != 0 {
		if m, ok := g.animals.Get(entity.EntityID(a.Mother)); ok {
			av.Mother = animalLabel(m)
		}
	}
	av.Offspring = young[id]
	if !a.Tame {
		return
	}
	if a.Master != 0 {
		av.MasterID = entity.EntityID(a.Master)
		av.Master = g.name(av.MasterID)
	}
	av.CanFollow = trainLearned(a, training.Obedience, now) || trainLearned(a, training.Guard, now) ||
		trainLearned(a, training.Release, now)
	av.Follows = av.CanFollow && !a.NoFollow
	if a.Bond != 0 {
		av.BondID = entity.EntityID(a.Bond)
		av.Bond = g.name(av.BondID)
	}
	av.Trained, av.Training = trainViews(a, now) // train_animals.go
}
