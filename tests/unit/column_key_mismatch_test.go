package unit

import (
	"testing"

	_ "github.com/k2m30/a9s/internal/aws"
	"github.com/k2m30/a9s/internal/resource"
)

// Test that every column Key in every ResourceTypeDef has a corresponding
// Fields key registered by the fetcher. This catches mismatches between
// types.go column definitions and aws/*.go fetcher Fields keys.
func TestColumnKeys_MatchFetcherFieldKeys(t *testing.T) {
	for _, rt := range resource.AllResourceTypes() {
		validKeys := resource.GetFieldKeys(rt.ShortName)
		if validKeys == nil {
			t.Errorf("no field keys registered for resource type %q — add RegisterFieldKeys in fetcher init()", rt.ShortName)
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
