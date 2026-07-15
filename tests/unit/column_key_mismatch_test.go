package unit

import (
	"testing"

	_ "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// Test that every column Key in every ResourceTypeDef has a corresponding
// Fields key registered by the fetcher or a Wave 2 issue enricher. This
// catches mismatches between types.go column definitions and aws/*.go
// fetcher/enricher Fields keys.
//
// Uses resource.GetAllFieldKeys (fetcher FieldKeys unioned with
// IssueEnricherFieldKeys) rather than resource.GetFieldKeys alone: several
// ResourceTypeDef.Columns entries (e.g. tg's health_summary, policy's risk,
// iam-user's mfa/risk, pipeline's last_status) are populated exclusively by
// a Wave 2 issue enricher, never by the raw fetcher.
func TestColumnKeys_MatchFetcherFieldKeys(t *testing.T) {
	for _, rt := range resource.AllResourceTypes() {
		validKeys := resource.GetAllFieldKeys(rt.ShortName)
		if validKeys == nil {
			t.Errorf("no field keys registered for resource type %q — add SetFieldKeysForTest in fetcher init()", rt.ShortName)
			continue
		}

		validSet := make(map[string]bool)
		for _, k := range validKeys {
			validSet[k] = true
		}

		for _, col := range rt.Columns {
			if !validSet[col.Key] {
				t.Errorf("resource type %q: column Key %q does not match any fetcher Fields key. Valid keys: %v",
					rt.ShortName, col.Key, validKeys)
			}
		}
	}
}
