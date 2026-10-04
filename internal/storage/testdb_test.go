package storage

import "testing"

func TestCheckTestDatabaseName(t *testing.T) {
	for _, name := range []string{"acars_test", "ci_test"} {
		if err := checkTestDatabaseName(name); err != nil {
			t.Errorf("checkTestDatabaseName(%q) = %v, want nil", name, err)
		}
	}
	for _, name := range []string{"acars_state", "acars", "test", "acars_testing", ""} {
		if err := checkTestDatabaseName(name); err == nil {
			t.Errorf("checkTestDatabaseName(%q) = nil, want an error", name)
		}
	}
}
