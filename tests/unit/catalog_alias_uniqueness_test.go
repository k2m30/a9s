package unit

// catalog_alias_uniqueness_test.go — 023-events-alias-dedup: guards against a
// command-name registering on more than one catalog entry, which makes
// resolution order (registration order) the tiebreaker instead of an error.

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/catalog"
)

// TestCatalog_AliasUniqueness walks the full installed top-level catalog and
// asserts every alias and ShortName is a unique command name across types.
// A name registered on two types silently loses one of them to registration
// order in catalog.Find — this test names the colliding pair so the fix is
// obvious rather than discovered live via `-c <name>` opening the wrong view.
func TestCatalog_AliasUniqueness(t *testing.T) {
	owner := make(map[string]string) // lowercased name -> owning ShortName

	claim := func(name, ownerShortName string) {
		lower := strings.ToLower(name)
		if existing, ok := owner[lower]; ok && existing != ownerShortName {
			t.Errorf("command name %q is registered on both %q and %q", name, existing, ownerShortName)
			return
		}
		owner[lower] = ownerShortName
	}

	for _, td := range catalog.All() {
		claim(td.ShortName, td.ShortName)
		for _, alias := range td.Aliases {
			claim(alias, td.ShortName)
		}
	}
}
