package lint

import "regexp"

// isIdentByte reports whether c can be part of a C-style identifier.
func isIdentByte(c byte) bool {
	return c == '_' ||
		(c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9')
}

// callMatch is one detected `name(` call site.
type callMatch struct {
	offset int    // byte offset of the first char of name in Code
	args   string // raw text between the outermost ( ) of the call, "" if unbalanced
}

// findCalls locates every invocation of a free function `name(` in src.Code,
// skipping member/scope accesses (`obj.name(`, `Class::name(`) so we don't
// mistake a legitimately-named method for the MQL4 free function.
func findCalls(src *Source, name string) []callMatch {
	re := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\s*\(`)
	code := src.Code
	var out []callMatch
	for _, loc := range re.FindAllStringIndex(code, -1) {
		start := loc[0]
		// Skip member (.name) and scope (::name) accesses.
		if p := prevNonSpace(code, start-1); p >= 0 {
			if code[p] == '.' {
				continue
			}
			if code[p] == ':' && p >= 1 && code[p-1] == ':' {
				continue
			}
		}
		out = append(out, callMatch{offset: start, args: extractArgs(code, loc[1]-1)})
	}
	return out
}

// prevNonSpace returns the index of the nearest non-space byte at or before i,
// or -1 if none.
func prevNonSpace(s string, i int) int {
	for ; i >= 0; i-- {
		if s[i] != ' ' && s[i] != '\t' && s[i] != '\n' && s[i] != '\r' {
			return i
		}
	}
	return -1
}

// extractArgs returns the text inside the parenthesis opened at openParen,
// respecting nesting. Returns "" if the parens are unbalanced.
func extractArgs(s string, openParen int) string {
	depth := 0
	for i := openParen; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return s[openParen+1 : i]
			}
		}
	}
	return ""
}
