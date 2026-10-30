package care

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// TestForceAgreesWithTheMenu: Force refuses exactly when the option is
// hidden or disabled, with the option's reason; an enabled capture or
// arrest answers what StartCapture/StartArrest answer on the same scene.
func TestForceAgreesWithTheMenu(t *testing.T) {
	for _, tc := range orderCases() {
		c, h, pr := orderScene()
		tc.set(c, h, pr)
		got := c.Force(tc.doer, tc.o, tc.target)
		want := tc.why
		switch {
		case tc.hidden:
			if got == "" {
				t.Errorf("%s: a hidden option must not start", tc.name)
			}
			continue
		case tc.why == "" && (tc.o == command.OrderCapture || tc.o == command.OrderArrest):
			c2, h2, pr2 := orderScene()
			tc.set(c2, h2, pr2)
			if tc.o == command.OrderCapture {
				want = c2.StartCapture(tc.doer, tc.target, true)
			} else {
				want = c2.StartArrest(tc.doer, tc.target, true)
			}
		}
		if got != want {
			t.Errorf("%s: Force %q, want %q", tc.name, got, want)
		}
	}
}

func TestForceRescue(t *testing.T) {
	c, h, _ := orderScene()
	h.drafted[doc] = true
	h.doctors[doc] = false // Doctor priority never gates a forced order
	h.jobs[doc] = job.Record{Kind: job.Move, State: job.Goto}
	if why := c.Force(doc, command.OrderRescue, patient); why != "" {
		t.Fatalf("rescue refused: %s", why)
	}
	j := h.jobs[doc]
	if j.Kind != job.Rescue || j.Target != patient || !j.Forced || j.TX != 6 || j.TY != 5 {
		t.Fatalf("a forced rescue from beside the patient: %+v", j)
	}
	if h.log[0] != "interrupt" || h.owners[patient] != doc {
		t.Fatalf("the old job dropped, the patient reserved: log %v owners %v", h.log, h.owners)
	}
	h.doctors[other] = true
	if c.Doctor.Assign(other) && h.jobs[other].Target == patient {
		t.Fatal("an autonomous doctor must not race the forced rescue")
	}
	j.State = job.Do // beside the patient
	c.Doctor.DoRescue(doc, j)
	j = h.jobs[doc]
	j.State = job.Do // at the bed
	c.Doctor.DoRescue(doc, j)
	if h.jobs[doc].State != job.Complete || h.pos[patient] != [2]int{8, 8} || !h.drafted[doc] {
		t.Fatalf("laid in bed, the doer still drafted: job %+v pos %v", h.jobs[doc], h.pos[patient])
	}
}

func TestForceTendFetchesMedicine(t *testing.T) {
	c, h, _ := orderScene()
	h.stacks[herbal] = item.Stack{Kind: item.HerbalMedicine, Qty: 3, X: 2, Y: 0}
	if why := c.Force(doc, command.OrderTendPatient, patient); why != "" {
		t.Fatalf("tend refused: %s", why)
	}
	j := h.jobs[doc]
	if j.Kind != job.Tend || j.Target != patient || !j.Forced || j.WorkLeft != TendWork ||
		j.Dest != herbal || j.TX != 2 || j.DestX != 6 || j.DestY != 5 {
		t.Fatalf("a forced tend fetching the herbal medicine first: %+v", j)
	}
	if h.owners[patient] != doc { // RES-1: the game's job start holds the dose (a count claim)
		t.Fatalf("the patient reserved: %v", h.owners)
	}
	h.pos[doc] = [2]int{6, 5} // walked to the patient with the dose
	c.Doctor.DoTend(doc, j)
	if h.jobs[doc].State != job.Complete || h.tended[patient] != 800 {
		t.Fatalf("tended with the medicine: job %+v quality %d", h.jobs[doc], h.tended[patient])
	}
}

func TestForceLaterOrdersRefused(t *testing.T) {
	c, _, _ := orderScene()
	for _, o := range []command.Order{command.OrderBuild, command.OrderNone} { // a tile order is no pawn order; feed, strip and take-to-bed have their own tests
		if why := c.Force(doc, o, patient); why != ReasonIncapable {
			t.Errorf("order %d: %q, want %q", o, why, ReasonIncapable)
		}
	}
}

// refusingHost is a fakeHost whose StartJob refuses every job, storing
// stored (when it has a Kind) the way the game stores a failed start.
type refusingHost struct {
	*fakeHost
	stored job.Record
}

func (h refusingHost) StartJob(id entity.EntityID, rec job.Record) bool {
	if h.stored.Kind != job.None {
		h.jobs[id] = h.stored
	}
	return false
}

// TestForceStartFails: a job the host refuses to start reports the
// failure the host stored, else ReasonIncapable.
func TestForceStartFails(t *testing.T) {
	for _, tc := range []struct {
		o      command.Order
		stored job.Record
		want   string
	}{
		{command.OrderRescue, job.Record{}, ReasonIncapable},
		{command.OrderTendPatient, job.Record{Kind: job.Tend, State: job.Fail, Fail: job.Unreachable}, job.Unreachable.String()},
	} {
		_, h, pr := orderScene()
		c := New(refusingHost{h, tc.stored}, Roles{Prison: pr})
		if why := c.Force(doc, tc.o, patient); why != tc.want {
			t.Errorf("order %d: %q, want %q", tc.o, why, tc.want)
		}
		if _, held := h.owners[patient]; held {
			t.Errorf("order %d: nothing started, nothing reserved", tc.o)
		}
	}
}
