// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package windows

import "fmt"

// T and Tf make fakeShell the English catalog (ports.Texts): the source
// text is the key, so English draws it unchanged.
func (s *fakeShell) T(src string) string { return src }

func (s *fakeShell) Tf(src string, args ...any) string { return fmt.Sprintf(src, args...) }
