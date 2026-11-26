// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package animalstate

import (
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/sim/component"
	"rimworld-go/internal/domain/sim/entity"
)

// Registry owns the active components. A Live view points into each group;
// detached State values are produced only for compatibility consumers.
type Registry struct {
	views             *component.Store[*animal.Live]
	identities        *component.Store[*animal.Identity]
	vitals            *component.Store[*animal.Vitals]
	intents           *component.Store[*animal.Intent]
	motions           *component.Store[*animal.Motion]
	threats           *component.Store[*animal.Threat]
	feeding           *component.Store[*animal.Feeding]
	handling          *component.Store[*animal.Handling]
	reproduction      *component.Store[*animal.Reproduction]
	medical           *component.Store[*animal.Medical]
	identityPages     groupPage[animal.Identity]
	vitalPages        groupPage[animal.Vitals]
	intentPages       groupPage[animal.Intent]
	motionPages       groupPage[animal.Motion]
	threatPages       groupPage[animal.Threat]
	feedingPages      groupPage[animal.Feeding]
	handlingPages     groupPage[animal.Handling]
	reproductionPages groupPage[animal.Reproduction]
	medicalPages      groupPage[animal.Medical]
}

func New() *Registry {
	return &Registry{
		views:        component.NewStore[*animal.Live](),
		identities:   component.NewStore[*animal.Identity](),
		vitals:       component.NewStore[*animal.Vitals](),
		intents:      component.NewStore[*animal.Intent](),
		motions:      component.NewStore[*animal.Motion](),
		threats:      component.NewStore[*animal.Threat](),
		feeding:      component.NewStore[*animal.Feeding](),
		handling:     component.NewStore[*animal.Handling](),
		reproduction: component.NewStore[*animal.Reproduction](),
		medical:      component.NewStore[*animal.Medical](),
	}
}

func (r *Registry) Has(id entity.EntityID) bool { return r.views.Has(id) }
func (r *Registry) Len() int                    { return r.views.Len() }

func (r *Registry) Ref(id entity.EntityID) *animal.Live {
	v, _ := r.views.Get(id)
	return v
}

func (r *Registry) Get(id entity.EntityID) (animal.State, bool) {
	view := r.Ref(id)
	if view == nil {
		return animal.State{}, false
	}
	return view.State(), true
}

func (r *Registry) Set(id entity.EntityID, state animal.State) {
	if view := r.Ref(id); view != nil {
		view.AssignState(state)
		return
	}
	view := &animal.Live{
		Identity: r.identityPages.next(), Vitals: r.vitalPages.next(),
		Intent: r.intentPages.next(), Motion: r.motionPages.next(),
		Threat: r.threatPages.next(), Feeding: r.feedingPages.next(),
		Handling: r.handlingPages.next(), Reproduction: r.reproductionPages.next(),
		Medical: r.medicalPages.next(),
	}
	view.AssignState(state)
	r.identities.Set(id, view.Identity)
	r.vitals.Set(id, view.Vitals)
	r.intents.Set(id, view.Intent)
	r.motions.Set(id, view.Motion)
	r.threats.Set(id, view.Threat)
	r.feeding.Set(id, view.Feeding)
	r.handling.Set(id, view.Handling)
	r.reproduction.Set(id, view.Reproduction)
	r.medical.Set(id, view.Medical)
	r.views.Set(id, view)
}

func (r *Registry) Delete(id entity.EntityID) {
	if view := r.Ref(id); view != nil {
		*view.Identity = animal.Identity{}
		*view.Vitals = animal.Vitals{}
		*view.Intent = animal.Intent{}
		*view.Motion = animal.Motion{}
		*view.Threat = animal.Threat{}
		*view.Feeding = animal.Feeding{}
		*view.Handling = animal.Handling{}
		*view.Reproduction = animal.Reproduction{}
		*view.Medical = animal.Medical{}
	}
	r.views.Delete(id)
	r.identities.Delete(id)
	r.vitals.Delete(id)
	r.intents.Delete(id)
	r.motions.Delete(id)
	r.threats.Delete(id)
	r.feeding.Delete(id)
	r.handling.Delete(id)
	r.reproduction.Delete(id)
	r.medical.Delete(id)
}

func (r *Registry) All(fn func(entity.EntityID, animal.State)) {
	r.views.All(func(id entity.EntityID, view *animal.Live) {
		fn(id, view.State())
	})
}

// AnyLive visits the authoritative active component groups without rebuilding
// legacy snapshots. The predicate is a pure owner-side read; it must not
// retain or mutate the live reference.
func (r *Registry) AnyLive(fn func(entity.EntityID, *animal.Live) bool) bool {
	return r.views.AnyRef(func(id entity.EntityID, view **animal.Live) bool { return fn(id, *view) })
}

// AnyTame materializes the compatibility record only for tame animals.
// The predicate must be a pure read and must not retain the scratch record.
func (r *Registry) AnyTame(fn func(entity.EntityID, *animal.State) bool) bool {
	var state *animal.State
	return r.AnyLive(func(id entity.EntityID, a *animal.Live) bool {
		if !a.Tame {
			return false
		}
		if state == nil {
			state = new(animal.State)
		}
		*state = a.State()
		return fn(id, state)
	})
}
