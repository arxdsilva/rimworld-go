// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package dialogs

import "fmt"

// The fake speaks English: T is the source, Tf formats it.
func (s *fakeShell) T(src string) string { return src }

func (s *fakeShell) Tf(src string, args ...any) string { return fmt.Sprintf(src, args...) }
