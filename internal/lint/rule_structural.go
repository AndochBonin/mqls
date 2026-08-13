package lint 

import "github.com/AndochBonin/mqls/internal/finding"

// structuralRule checks that braces, brackets and parentheses are balanced.
// This is the cheapest possible sanity check and catches truncated or
// structurally garbled generations instantly.
type structuralRule struct{}

func (structuralRule) ID() string { return "struct" }

type openPos struct {
	ch     byte
	offset int
}

// openerFor maps a closer to the opener it must match.
var openerFor = map[byte]byte{')': '(', ']': '[', '}': '{'}

// closerFor maps an opener to its closer.
var closerFor = map[byte]byte{'(': ')', '[': ']', '{': '}'}

// delimName gives a human name for an opener/closer byte.
var delimName = map[byte]string{'(': "parenthesis", '[': "bracket", '{': "brace"}

func (r structuralRule) Check(src *Source) []finding.Finding {
	var out []finding.Finding
	var stack []openPos

	for i := 0; i < len(src.Code); i++ {
		c := src.Code[i]
		switch c {
		case '(', '[', '{':
			stack = append(stack, openPos{c, i})
		case ')', ']', '}':
			if len(stack) == 0 {
				line, col := src.posAt(i)
				out = append(out, finding.Finding{
					File: src.Path, Line: line, Column: col,
					Severity: finding.Error,
					RuleID:   "struct/unmatched-close",
					Message:  "closing '" + string(c) + "' with no matching opener",
					Pass:     finding.PassStatic,
				})
				continue
			}
			top := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if top.ch != openerFor[c] {
				line, col := src.posAt(i)
				out = append(out, finding.Finding{
					File: src.Path, Line: line, Column: col,
					Severity: finding.Error,
					RuleID:   "struct/mismatched-delimiter",
					Message:  "'" + string(c) + "' does not match the '" + string(top.ch) + "' opened earlier",
					Pass:     finding.PassStatic,
				})
			}
		}
	}

	for _, o := range stack {
		line, col := src.posAt(o.offset)
		out = append(out, finding.Finding{
			File: src.Path, Line: line, Column: col,
			Severity: finding.Error,
			RuleID:   "struct/unclosed-" + delimName[o.ch],
			Message:  "unclosed '" + string(o.ch) + "' " + delimName[o.ch],
			Suggest:  "add the matching '" + string(closerFor[o.ch]) + "'",
			Pass:     finding.PassStatic,
		})
	}
	return out
}
