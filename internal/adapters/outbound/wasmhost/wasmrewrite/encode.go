// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmrewrite

// encodeOrder is the binary order of the non-custom sections.
var encodeOrder = [...]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 12, 10, 11}

// Encode writes the module: sections that have a view (type, import,
// function, global, export, start, code) are re-encoded from it with
// recomputed sizes and the shortest LEBs, the others are copied raw, and
// custom sections are dropped. An empty view writes no section.
func (m *Module) Encode() []byte {
	out := []byte{0x00, 'a', 's', 'm', 1, 0, 0, 0}
	raw := map[byte][]byte{}
	for _, s := range m.Sections {
		if _, dup := raw[s.ID]; !dup {
			raw[s.ID] = s.Body
		}
	}
	emit := func(id byte, body []byte) {
		out = append(out, id)
		out = appendU32(out, uint32(len(body)))
		out = append(out, body...)
	}
	for _, id := range encodeOrder {
		switch id {
		case 1:
			if len(m.Types) > 0 {
				emit(id, m.encodeTypes())
			}
		case 2:
			if len(m.Imports) > 0 {
				emit(id, m.encodeImports())
			}
		case 3:
			if len(m.Funcs) > 0 {
				emit(id, m.encodeFuncs())
			}
		case 6:
			if len(m.Globals) > 0 {
				emit(id, m.encodeGlobals())
			}
		case 7:
			if len(m.Exports) > 0 {
				emit(id, m.encodeExports())
			}
		case 8:
			if m.HasStart {
				emit(id, appendU32(nil, m.Start))
			}
		case 10:
			if len(m.Bodies) > 0 {
				emit(id, m.encodeCode())
			}
		default:
			if body, ok := raw[id]; ok {
				emit(id, body)
			}
		}
	}
	return out
}

func appendName(b []byte, s string) []byte {
	b = appendU32(b, uint32(len(s)))
	return append(b, s...)
}

func (m *Module) encodeTypes() []byte {
	b := appendU32(nil, uint32(len(m.Types)))
	for _, t := range m.Types {
		b = append(b, 0x60)
		b = appendU32(b, uint32(len(t.Params)))
		b = append(b, t.Params...)
		b = appendU32(b, uint32(len(t.Results)))
		b = append(b, t.Results...)
	}
	return b
}

func (m *Module) encodeImports() []byte {
	b := appendU32(nil, uint32(len(m.Imports)))
	for _, im := range m.Imports {
		b = appendName(b, im.Module)
		b = appendName(b, im.Name)
		b = append(b, im.Kind)
		if im.Kind == 0 {
			b = appendU32(b, im.Index)
		} else {
			b = append(b, im.Desc...)
		}
	}
	return b
}

func (m *Module) encodeFuncs() []byte {
	b := appendU32(nil, uint32(len(m.Funcs)))
	for _, t := range m.Funcs {
		b = appendU32(b, t)
	}
	return b
}

func (m *Module) encodeGlobals() []byte {
	b := appendU32(nil, uint32(len(m.Globals)))
	for _, g := range m.Globals {
		b = append(b, g.Type)
		if g.Mutable {
			b = append(b, 1)
		} else {
			b = append(b, 0)
		}
		b = append(b, g.Init...)
	}
	return b
}

func (m *Module) encodeExports() []byte {
	b := appendU32(nil, uint32(len(m.Exports)))
	for _, e := range m.Exports {
		b = appendName(b, e.Name)
		b = append(b, e.Kind)
		b = appendU32(b, e.Index)
	}
	return b
}

func (m *Module) encodeCode() []byte {
	b := appendU32(nil, uint32(len(m.Bodies)))
	for _, body := range m.Bodies {
		e := appendU32(nil, uint32(len(body.Locals)))
		for _, r := range body.Locals {
			e = appendU32(e, r.Count)
			e = append(e, r.Type)
		}
		e = append(e, body.Code...)
		b = appendU32(b, uint32(len(e)))
		b = append(b, e...)
	}
	return b
}
