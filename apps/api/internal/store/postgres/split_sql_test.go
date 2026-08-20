package postgres

import (
	"strings"
	"testing"
)

func TestSplitSQLKeepsDollarQuotedFunctions(t *testing.T) {
	raw := `
CREATE TABLE IF NOT EXISTS t (id TEXT);
CREATE OR REPLACE FUNCTION f(x TEXT) RETURNS BOOLEAN AS $$
  SELECT x IS NULL
      OR x = 'ok';
$$ LANGUAGE sql STABLE;
ALTER TABLE t ENABLE ROW LEVEL SECURITY;
`
	got := splitSQL(raw)
	if len(got) != 3 {
		t.Fatalf("want 3 statements, got %d: %#v", len(got), got)
	}
	if !strings.Contains(got[1], "$$") || !strings.Contains(got[1], "LANGUAGE sql") {
		t.Fatalf("function body split incorrectly: %q", got[1])
	}
	if strings.Contains(got[1], "ENABLE ROW LEVEL") {
		t.Fatalf("merged next statement into function: %q", got[1])
	}
}
