package unit

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/catalog"
)

// A name registered on two types silently loses one of them to registration
// order in catalog.Find.
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
