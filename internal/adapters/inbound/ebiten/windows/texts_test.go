package windows

import "fmt"

// T and Tf make fakeShell the English catalog (ports.Texts): the source
// text is the key, so English draws it unchanged.
func (s *fakeShell) T(src string) string { return src }

func (s *fakeShell) Tf(src string, args ...any) string { return fmt.Sprintf(src, args...) }
