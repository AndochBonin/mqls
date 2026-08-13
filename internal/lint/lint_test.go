package lint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AndochBonin/mqls/internal/finding"
)

// hasRule reports whether any finding has the given rule id.
func hasRule(fs []finding.Finding, id string) bool {
	for _, f := range fs {
		if f.RuleID == id {
			return true
		}
	}
	return false
}

func TestFootgunFixture(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "footguns.mq5")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	rep := Run(path, string(raw))

	want := []string{
		"mql4-api/order-select",
		"mql4-api/order-lots",
		"mql4-api/order-close",
		"mql4-api/order-ticket",
		"mql4-api/ordersend-signature",
		"event-handler/oninit-params",
		"event-handler/ondeinit-signature",
		"predefined-var/bid",
		"predefined-var/ask",
	}
	for _, id := range want {
		if !hasRule(rep.Findings, id) {
			t.Errorf("expected a finding with rule_id %q, none found", id)
		}
	}
	if rep.OK {
		t.Error("report should not be OK: fixture contains error-severity findings")
	}
}

func TestNoFalsePositivesInCommentsAndStrings(t *testing.T) {
	src := `
void OnTick()
{
   // OrderClose OrderSelect OrderModify — all in a comment
   string s = "OrderDelete(1); OrderClose(2);";
   /* OrderLots() block comment */
}
`
	rep := Run("x.mq5", src)
	for _, f := range rep.Findings {
		if strings.HasPrefix(f.RuleID, "mql4-api/") {
			t.Errorf("false positive in comment/string: %s at %d:%d", f.RuleID, f.Line, f.Column)
		}
	}
}

func TestCleanFileHasNoFindings(t *testing.T) {
	src := `
int OnInit()          { return(INIT_SUCCEEDED); }
void OnDeinit(const int reason) {}
void OnTick()
{
   double bid = SymbolInfoDouble(_Symbol, SYMBOL_BID);
   MqlTradeRequest req; MqlTradeResult res;
   OrderSend(req, res);
}
`
	rep := Run("clean.mq5", src)
	if len(rep.Findings) != 0 {
		t.Errorf("clean file produced %d finding(s): %+v", len(rep.Findings), rep.Findings)
	}
	if !rep.OK {
		t.Error("clean file should be OK")
	}
}

func TestStructuralBalance(t *testing.T) {
	rep := Run("b.mq5", "void OnTick() { if(x) { ")
	if !hasRule(rep.Findings, "struct/unclosed-brace") {
		t.Errorf("expected unclosed-brace finding, got %+v", rep.Findings)
	}
}
