package migrations

import (
	"strings"
	"testing"
)

func TestUsageContentMemoryMigrationOnlyDropsTemporaryTables(t *testing.T) {
	raw, err := FS.ReadFile("343_usage_content_memory_window.sql")
	if err != nil {
		t.Fatal(err)
	}
	var statements []string
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "--") {
			statements = append(statements, line)
		}
	}
	want := "DROP TABLE IF EXISTS usage_request_contents;\nDROP TABLE IF EXISTS usage_content_budget;"
	if strings.Join(statements, "\n") != want {
		t.Fatal("migration must only remove the obsolete temporary tables")
	}
}
