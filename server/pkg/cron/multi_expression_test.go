package cron

import (
	"strings"
	"testing"
)

func TestValidateExpressionsRejectsInvalidRuleWithIndex(t *testing.T) {
	err := ValidateExpressions("0 0 * * * *\ninvalid cron")
	if err == nil {
		t.Fatal("expected invalid expression list to return an error")
	}
	if got := err.Error(); !strings.HasPrefix(got, "第 2 条") {
		t.Fatalf("expected indexed validation error, got %q", got)
	}
}

func TestNextRunTimesForExpressionsReturnsEarliestMatches(t *testing.T) {
	times := NextRunTimesForExpressions("0 */30 * * * *\n0 0 */2 * * *", 3)
	if len(times) != 3 {
		t.Fatalf("expected three upcoming run times, got %d", len(times))
	}
	if !times[0].Before(times[1]) && !times[0].Equal(times[1]) {
		t.Fatalf("expected sorted run times, got %v then %v", times[0], times[1])
	}
}
