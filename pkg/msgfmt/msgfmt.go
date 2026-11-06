// Package msgfmt splits a printf call into a format a translator can key on
// and its arguments as text (OPT-001, Wave 4). The simulation posts English
// messages; the client translates the format by its key, translates each
// argument that is a known name, and joins them again. For any call,
// Join(Split(format, args...)) is fmt.Sprintf(format, args...) byte for byte,
// so English output never changes.
package msgfmt

import (
	"fmt"
	"strings"
)

// Split is format with every verb written as %s, and each argument already
// formatted by its own verb ("%.1f" of 3.25 is "3.2"). A call Split cannot
// take apart (explicit argument indexes, a * width, too many or too few
// arguments) comes back whole: the printed text as a Plain format, no args.
func Split(format string, args ...any) (string, []string) {
	var b strings.Builder
	out := make([]string, 0, len(args))
	for i := 0; i < len(format); i++ {
		c := format[i]
		if c != '%' {
			b.WriteByte(c)
			continue
		}
		end, ok := verbEnd(format, i)
		if !ok {
			return whole(format, args)
		}
		spec := format[i : end+1]
		i = end
		if spec == "%%" {
			b.WriteString("%%")
			continue
		}
		if len(out) == len(args) {
			return whole(format, args) // missing argument
		}
		out = append(out, fmt.Sprintf(spec, args[len(out)]))
		b.WriteString("%s")
	}
	if len(out) != len(args) {
		return whole(format, args) // extra arguments
	}
	return b.String(), out
}

// Key is the catalog key a literal format posts under: Split's format for
// any call with the right number of arguments. ok is false for a format Split
// would not take apart (it posts its printed text instead).
func Key(format string) (key string, ok bool) {
	var b strings.Builder
	for i := 0; i < len(format); i++ {
		if format[i] != '%' {
			b.WriteByte(format[i])
			continue
		}
		end, fine := verbEnd(format, i)
		if !fine {
			return "", false
		}
		if format[i:end+1] == "%%" {
			b.WriteString("%%")
		} else {
			b.WriteString("%s")
		}
		i = end
	}
	return b.String(), true
}

// Join is fmt.Sprintf(format, args...) for a format Split made (or its
// translation, which may reorder with %[2]s).
func Join(format string, args []string) string {
	vals := make([]any, len(args))
	for i, a := range args {
		vals[i] = a
	}
	return fmt.Sprintf(format, vals...)
}

// Text is a message as its key and arguments: what a read model carries
// beside the printed English. The zero Text is "no format" (show the
// English). It is comparable, so read-model memos can compare views, and
// saves as two strings: Packed is the arguments, each one led by sep (read
// them with Args).
type Text struct {
	Format string
	Packed string
}

// sep leads each argument in Text.Packed (the ASCII unit separator, which no
// message prints).
const sep = "\x1f"

// Of is Split as a Text.
func Of(format string, args ...any) Text {
	f, a := Split(format, args...)
	return Make(f, a)
}

// Make is the Text of a split format and its arguments.
func Make(format string, args []string) Text {
	t := Text{Format: format}
	for _, a := range args {
		t.Packed += sep + a
	}
	return t
}

// Args are t's arguments, as Split gave them (nil when none).
func (t Text) Args() []string {
	if t.Packed == "" {
		return nil
	}
	return strings.Split(t.Packed[len(sep):], sep)
}

// String is the message printed, as fmt.Sprintf(format, args...) would.
func (t Text) String() string { return Join(t.Format, t.Args()) }

// Plain is text as a format with no verbs: every % doubled, so Join prints
// text unchanged. A letter whose text is built elsewhere is keyed on it.
func Plain(text string) string { return strings.ReplaceAll(text, "%", "%%") }

// whole is the printed call as a Plain format, for a call Split can't take apart.
func whole(format string, args []any) (string, []string) {
	return Plain(fmt.Sprintf(format, args...)), nil
}

// verbEnd is the index of the verb letter of the directive at format[i]
// ('%'): flags, width and precision as digits only. ok is false for an
// argument index or * width, and for a directive cut off at the end.
func verbEnd(format string, i int) (int, bool) {
	for j := i + 1; j < len(format); j++ {
		switch c := format[j]; {
		case c == '[' || c == '*':
			return 0, false
		case strings.IndexByte("+-# 0123456789.", c) >= 0:
		default:
			return j, true
		}
	}
	return 0, false
}
