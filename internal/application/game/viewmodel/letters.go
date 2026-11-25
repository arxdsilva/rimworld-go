// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package viewmodel

import (
	"rimworld-go/internal/application/game/letters"
	"rimworld-go/pkg/letterdata"
)

// Letters detaches the saved letter stack for presentation. Older saves get
// their stable event codes derived from their original message format.
func Letters(list []letters.Letter) []LetterView {
	if len(list) == 0 {
		return nil
	}
	out := make([]LetterView, len(list))
	for i, l := range list {
		code, fields := letterdata.Defaults(l.Code, &l.Fields, l.Title, l.Msg.Format, l.Msg.Args())
		out[i] = LetterView{ID: l.ID, Tick: l.Tick, Kind: l.Kind, Code: code, Fields: fields,
			Title: l.Title, Text: l.Text, Msg: l.Msg,
			X: l.X, Y: l.Y, Map: MapID(l.Map), Focus: l.Focus, Intro: l.Intro,
			JoinOffer: l.Join != 0 || l.Wanderer != 0, Demand: l.Demand,
			Quest: l.Quest, Until: l.Until, Choices: l.Choices}
	}
	return out
}
