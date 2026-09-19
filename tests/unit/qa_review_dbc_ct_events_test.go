package unit

import (
	"testing"

	_ "github.com/k2m30/a9s/v3/core/aws" // trigger init chain
	"github.com/k2m30/a9s/v3/core/resource"
)

// ct-events is in dbc's related-def list, declared in dbc's own catalog
// Related list (core/aws/catalog_databases.go) like every top-level type.
func TestReview_DBC_RegistersCTEvents(t *testing.T) {
	defs := resource.GetRelated("dbc")
	found := false
	for _, d := range defs {
		if d.TargetType == "ct-events" {
			found = true
			break
		}
	}
	if !found {
		names := make([]string, 0, len(defs))
		for _, d := range defs {
			names = append(names, d.TargetType)
		}
		t.Errorf("dbc must register a ct-events related def; got %d defs: %v", len(defs), names)
	}
}
