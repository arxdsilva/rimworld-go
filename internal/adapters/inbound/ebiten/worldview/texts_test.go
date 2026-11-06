package worldview

import "fmt"

// T and Tf make the fake shell an English catalog (ports.Texts).
func (*fakeShell) T(src string) string { return src }

func (*fakeShell) Tf(src string, args ...any) string { return fmt.Sprintf(src, args...) }
