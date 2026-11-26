// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package dialogs

import "fmt"

// The fake speaks English: T is the source, Tf formats it.
// A test may set mark to see which strings went through T.
func (s *fakeShell) T(src string) string {
	if s.mark {
		return "~" + src
	}
	return src
}

func (s *fakeShell) Tf(src string, args ...any) string { return fmt.Sprintf(src, args...) }
