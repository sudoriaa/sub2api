package migrations

import (
	"strings"
	"testing"
)

func TestUsageRequestContentRetentionSchema(t *testing.T) {
	raw, err := FS.ReadFile("338_usage_request_contents.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(raw)
	for _, required := range []string{"usage_request_contents", "expires_at = created_at + INTERVAL '24 hours'", "idx_usage_request_contents_expiry", "idx_usage_request_contents_billing", "idx_usage_request_contents_client", "request_body BYTEA", "response_body BYTEA", "toast.autovacuum_vacuum_scale_factor"} {
		if !strings.Contains(sql, required) {
			t.Fatalf("missing schema invariant: %s", required)
		}
	}
	if strings.Contains(sql, "ALTER TABLE usage_logs") {
		t.Fatal("conversation payloads must not enlarge usage rows")
	}
}
