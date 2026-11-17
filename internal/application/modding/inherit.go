package modding

import "strings"

// Inherit fills each def from its parent chain and drops abstract defs
// (PLG-022). defs is core's, then each plugin's, in load order:
//
//   - a class/key defined twice is an error on the later one, which is
//     dropped (PLG-023): changing someone else's def is a replace patch;
//   - a parent is a def of the same class (abstract or not); a missing
//     parent or a cycle is an error, and drops the def and its heirs;
//   - a child's body merges over its resolved parent's: objects merge,
//     lists and scalars replace, and a null deletes the inherited field.
//
// The output is the non-abstract defs in input order, Parent kept for
// provenance; bodies are fresh copies, so the input is never changed.
// Problems come in input order.
func Inherit(defs []Def) ([]Def, []Problem) {
	in := inheritance{defs: defs, byRef: make(map[string]int, len(defs)),
		state: make([]inheritState, len(defs)), body: make([]map[string]any, len(defs)),
		probs: make([][]Problem, len(defs))}
	for i, d := range defs {
		if first, dup := in.byRef[d.Ref()]; dup {
			in.fail(i, "", "already defined by "+defs[first].Plugin+"; use a replace patch to change it")
			continue
		}
		in.byRef[d.Ref()] = i
	}
	for i := range defs {
		in.resolve(i)
	}
	var out []Def
	var probs []Problem
	for i, d := range defs {
		probs = append(probs, in.probs[i]...)
		if in.state[i] != inheritDone || d.Abstract {
			continue
		}
		d.Body = in.body[i]
		d.Touched = append([]Touch(nil), d.Touched...)
		out = append(out, d)
	}
	return out, probs
}

type inheritState uint8

const (
	inheritNew inheritState = iota
	inheritVisiting
	inheritDone
	inheritFailed
)

// inheritance is one Inherit run: per def index, its state, its resolved
// body and its problems.
type inheritance struct {
	defs  []Def
	byRef map[string]int
	state []inheritState
	body  []map[string]any
	probs [][]Problem
}

func (in *inheritance) fail(i int, field, msg string) {
	d := in.defs[i]
	in.state[i] = inheritFailed
	in.probs[i] = append(in.probs[i], Problem{Plugin: d.Plugin, File: d.File, Def: d.Ref(), Field: field, Msg: msg})
}

// resolve walks i's parent chain up to a def already settled, a root, a
// missing parent or a cycle, then settles the chain from the top down.
func (in *inheritance) resolve(i int) {
	var chain []int
	for cur := i; ; {
		switch in.state[cur] {
		case inheritDone, inheritFailed:
		case inheritVisiting:
			in.failCycle(chain, cur)
		default:
			in.state[cur] = inheritVisiting
			chain = append(chain, cur)
			d := in.defs[cur]
			if d.Parent == "" {
				in.body[cur] = treeCopyMap(d.Body)
				if in.body[cur] == nil {
					in.body[cur] = map[string]any{}
				}
				in.state[cur] = inheritDone
				break
			}
			p, ok := in.byRef[d.Class+"/"+d.Parent]
			if !ok {
				in.fail(cur, "parent", "parent "+d.Parent+" is not a "+d.Class+" definition")
				break
			}
			cur = p
			continue
		}
		break
	}
	for k := len(chain) - 1; k >= 0; k-- {
		c := chain[k]
		if in.state[c] != inheritVisiting {
			continue
		}
		d := in.defs[c]
		p := in.byRef[d.Class+"/"+d.Parent]
		if in.state[p] == inheritFailed {
			in.fail(c, "parent", "parent "+d.Parent+" could not be built")
			continue
		}
		body := treeCopyMap(in.body[p])
		treeMerge(body, d.Body)
		in.body[c] = body
		in.state[c] = inheritDone
	}
}

// failCycle fails every def of the cycle that starts where chain first
// reached start.
func (in *inheritance) failCycle(chain []int, start int) {
	from := 0
	for chain[from] != start {
		from++
	}
	cycle := chain[from:]
	keys := make([]string, 0, len(cycle)+1)
	for _, c := range cycle {
		keys = append(keys, in.defs[c].Key)
	}
	keys = append(keys, in.defs[start].Key)
	msg := "parent cycle: " + strings.Join(keys, " -> ")
	for _, c := range cycle {
		in.fail(c, "parent", msg)
	}
}
