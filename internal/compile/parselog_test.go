package compile

import (
	"testing"
	"unicode/utf16"

	"github.com/AndochBonin/mqls/internal/finding"
)

// utf16le encodes s as UTF-16LE with a BOM, matching MetaEditor's log format.
func utf16le(s string) []byte {
	out := []byte{0xFF, 0xFE}
	for _, u := range utf16.Encode([]rune(s)) {
		out = append(out, byte(u), byte(u>>8))
	}
	return out
}

func TestParseLog(t *testing.T) {
	log := "\\\\Mac\\Home\\mqls\\strategy.mq5(22,7) : error 145: 'foo' - undeclared identifier\r\n" +
		"\\\\Mac\\Home\\mqls\\strategy.mq5(30,3) : warning 43: possible loss of data\r\n" +
		"strategy.mq5(1,1) : information 62: something advisory\r\n" +
		"Result: 1 errors, 1 warnings, 0 information\r\n" +
		"\r\n"

	got := parseLog("strategy.mq5", utf16le(log))
	if len(got) != 3 {
		t.Fatalf("want 3 findings, got %d: %+v", len(got), got)
	}

	want := []finding.Finding{
		{File: "strategy.mq5", Line: 22, Column: 7, Severity: finding.Error, RuleID: "compile/145", Message: "'foo' - undeclared identifier", Pass: finding.PassCompile},
		{File: "strategy.mq5", Line: 30, Column: 3, Severity: finding.Warning, RuleID: "compile/43", Message: "possible loss of data", Pass: finding.PassCompile},
		{File: "strategy.mq5", Line: 1, Column: 1, Severity: finding.Info, RuleID: "compile/62", Message: "something advisory", Pass: finding.PassCompile},
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("finding %d:\n got %+v\nwant %+v", i, got[i], w)
		}
	}
}

func TestParseLogEmpty(t *testing.T) {
	if got := parseLog("x.mq5", utf16le("Result: 0 errors, 0 warnings\r\n")); len(got) != 0 {
		t.Fatalf("want no findings, got %+v", got)
	}
	if got := parseLog("x.mq5", nil); len(got) != 0 {
		t.Fatalf("want no findings for empty log, got %+v", got)
	}
}
