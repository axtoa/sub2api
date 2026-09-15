package migrations

import (
	"strings"
	"testing"
)

func TestMiniMaxPlatformConstraintsMigration(t *testing.T) {
	content, err := FS.ReadFile("237_minimax_platform_constraints.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, constraint := range []string{
		"user_platform_quotas_platform_check",
		"composite_model_routes_target_platform_check",
	} {
		if !strings.Contains(string(content), constraint) {
			t.Errorf("migration is missing %s", constraint)
		}
	}
	if got := strings.Count(string(content), "'minimax'"); got != 2 {
		t.Errorf("migration must allow minimax in both constraints; got %d occurrences", got)
	}
}
