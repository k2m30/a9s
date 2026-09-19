package unit

import (
	"testing"

	_ "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// TestRegistry_AllChildTypesHaveParents verifies that every ChildViewDef.ChildType
// declared in AllResourceTypes() has a corresponding SetChildTypeForTest registration.
// A ChildViewDef referencing a short name that was never registered panics on
// a nil type when the child view is opened.
func TestRegistry_AllChildTypesHaveParents(t *testing.T) {
	declared := map[string]string{} // childType -> parentShortName
	for _, rt := range resource.AllResourceTypes() {
		for _, cv := range rt.Children {
			if cv.ChildType == "" {
				t.Errorf("resource type %q has a ChildViewDef with empty ChildType", rt.ShortName)
				continue
			}
			if _, seen := declared[cv.ChildType]; !seen {
				declared[cv.ChildType] = rt.ShortName
			}
		}
	}

	if len(declared) == 0 {
		t.Fatal("no child types found in AllResourceTypes() — AWS init() may not have been triggered")
	}

	for childShortName, parentShortName := range declared {
		def := resource.GetChildType(childShortName)
		if def == nil {
			t.Errorf(
				"child type %q declared in parent %q has no SetChildTypeForTest registration — GetChildType returned nil",
				childShortName, parentShortName,
			)
		}
	}

	// A ChildViewDef attached to a type whose ShortName was renamed or removed
	// leaves the parent reference dangling.
	topLevel := map[string]bool{}
	for _, rt := range resource.AllResourceTypes() {
		topLevel[rt.ShortName] = true
	}
	parentSet := map[string]bool{}
	for _, parentShortName := range declared {
		parentSet[parentShortName] = true
	}
	for parent := range parentSet {
		if !topLevel[parent] {
			t.Errorf(
				"parent resource type %q declares child views but is not present in AllResourceTypes()",
				parent,
			)
		}
	}
}

// TestRegistry_AllRegisteredTypesHaveFetcher verifies that every short name returned
// by AllShortNames() has at least one registered fetcher (paginated or filtered-paginated).
// A ResourceTypeDef without SetPaginatedForTest or SetFilteredPaginatedForTest
// in the aws/*.go init() opens a list that silently does nothing.
func TestRegistry_AllRegisteredTypesHaveFetcher(t *testing.T) {
	missing := []string{}

	for _, shortName := range resource.AllShortNames() {
		hasPaginated := resource.GetPaginatedFetcher(shortName) != nil
		hasFiltered := resource.GetFilteredPaginatedFetcher(shortName) != nil

		if !hasPaginated && !hasFiltered {
			missing = append(missing, shortName)
		}
	}

	if len(missing) > 0 {
		for _, name := range missing {
			t.Errorf(
				"resource type %q has no registered fetcher — "+
					"GetPaginatedFetcher and GetFilteredPaginatedFetcher both returned nil",
				name,
			)
		}
	}
}
