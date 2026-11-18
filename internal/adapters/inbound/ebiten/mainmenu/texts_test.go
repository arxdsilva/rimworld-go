// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package mainmenu

import "fmt"

// T and Tf make the fake shells English catalogs (ports.Texts).
func (fakeShell) T(src string) string { return src }

func (fakeShell) Tf(src string, args ...any) string { return fmt.Sprintf(src, args...) }

func (fakeShellMode) T(src string) string { return src }

func (fakeShellMode) Tf(src string, args ...any) string { return fmt.Sprintf(src, args...) }
