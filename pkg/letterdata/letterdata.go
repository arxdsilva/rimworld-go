// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Package letterdata is an upstreamable, game-agnostic envelope for durable
// event letters. Domains supply semantic field kinds; the generic fallback
// deliberately preserves values rather than guessing a person/place from prose.
package letterdata

import (
	"fmt"
	"hash/fnv"
	"strconv"
	"strings"
)

type Field struct {
	Name  string // who, what, where, why, duration, action, or argN
	Kind  string // person, animal, faction, item, place, reason, duration, count, text
	Value string
}

// Code is an immutable source-identity for one title/format combination.
// This is NOT a translated title and cannot change with the user's language.
func Code(title, format string) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(title))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(format))
	return fmt.Sprintf("letter.%016x", h.Sum64())
}

// FromArgs retains every value of a msgfmt record as an explicit typed field.
// Domain mods can replace the generic argN names with semantic field names.
func FromArgs(args []string) []Field {
	if len(args) == 0 {
		return nil
	}
	out := make([]Field, 0, len(args))
	for i, v := range args {
		kind := "text"
		if strings.TrimSpace(v) != "" {
			if _, err := strconv.ParseFloat(v, 64); err == nil {
				kind = "number"
			}
		}
		out = append(out, Field{Name: fmt.Sprintf("arg%d", i+1), Kind: kind, Value: v})
	}
	return out
}

// Defaults fills a letter's identity: code or Code(title, format) when it has
// none, fields (nil: none given) or FromArgs(args); both detached.
func Defaults(code string, fields *[]Field, title, format string, args []string) (string, []Field) {
	var out []Field
	if fields != nil {
		out = Clone(*fields)
	}
	if code == "" {
		code = Code(title, format)
	}
	if out == nil {
		out = FromArgs(args)
	}
	return code, out
}

func Clone(fields []Field) []Field {
	if fields == nil {
		return nil
	}
	return append([]Field(nil), fields...)
}
