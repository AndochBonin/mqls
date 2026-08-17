package lint

import "strings"

// Source is the parsed view of one MQL5 file handed to every rule.
//
// Code is the crucial field: it is the raw text with the *contents* of
// comments, string literals and char literals replaced by spaces, while every
// newline and every other byte position is preserved. This means a rule can
// pattern-match against Code and trust that a hit is real source code, not text
// inside a // comment or a "string literal" — which is the single biggest
// source of false positives in pattern-based linting. - that might be a lie (claude wrote it)
type Source struct {
	Path  string
	Raw   string   // original, untouched bytes
	Code  string   // comments + literal contents blanked to spaces, positions preserved
	Lines []string // Raw split by \n, for building human-readable context
}

// NewSource builds a Source, running the blanking preprocessor.
func NewSource(path, raw string) *Source {
	return &Source{
		Path:  path,
		Raw:   raw,
		Code:  blankCommentsAndStrings(raw),
		Lines: strings.Split(raw, "\n"),
	}
}

// posAt converts a byte offset in Raw into a 1-based (line, column).
func (s *Source) posAt(offset int) (line, col int) {
	line, col = 1, 1
	for i := 0; i < offset && i < len(s.Raw); i++ {
		if s.Raw[i] == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return line, col
}

// blankCommentsAndStrings replaces comment bodies and string/char literal
// bodies with spaces. Delimiters are also blanked so a rule can't match a stray
// quote. Newlines are always preserved so line numbers stay exact.
func blankCommentsAndStrings(src string) string {
	out := []byte(src)
	n := len(out)

	const (
		normal = iota
		lineComment
		blockComment
		str  // "..."
		char // '.'
	)
	state := normal

	blank := func(i int) {
		if out[i] != '\n' {
			out[i] = ' '
		}
	}

	for i := 0; i < n; i++ {
		c := out[i]
		switch state {
		case normal:
			switch {
			case c == '/' && i+1 < n && out[i+1] == '/':
				state = lineComment
				blank(i)
				blank(i + 1)
				i++
			case c == '/' && i+1 < n && out[i+1] == '*':
				state = blockComment
				blank(i)
				blank(i + 1)
				i++
			case c == '"':
				state = str
				blank(i)
			case c == '\'':
				state = char
				blank(i)
			}
		case lineComment:
			if c == '\n' {
				state = normal
			} else {
				blank(i)
			}
		case blockComment:
			if c == '*' && i+1 < n && out[i+1] == '/' {
				blank(i)
				blank(i + 1)
				i++
				state = normal
			} else {
				blank(i)
			}
		case str:
			if c == '\\' && i+1 < n {
				blank(i)
				blank(i + 1)
				i++
			} else if c == '"' {
				blank(i)
				state = normal
			} else {
				blank(i)
			}
		case char:
			if c == '\\' && i+1 < n {
				blank(i)
				blank(i + 1)
				i++
			} else if c == '\'' {
				blank(i)
				state = normal
			} else {
				blank(i)
			}
		}
	}
	return string(out)
}
