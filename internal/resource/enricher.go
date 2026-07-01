package resource

import (
	"fmt"

	"github.com/k2m30/a9s/v3/internal/catalog"
	"github.com/k2m30/a9s/v3/internal/domain"
)

// DetailEnricher is the function signature for on-demand detail enrichers.
// Declaration lives in internal/domain/contracts.go; this alias keeps
// existing consumers compiling.
type DetailEnricher = domain.DetailEnricher

var detailEnricherRegistry = map[string]DetailEnricher{}

// SetDetailEnricherForTest adds a detail enricher for the given resource short name.
// Panics on empty short name, nil function, or duplicate registration.
func SetDetailEnricherForTest(shortName string, f DetailEnricher) {
	if shortName == "" {
		panic("SetDetailEnricherForTest: empty short name")
	}
	if f == nil {
		panic(fmt.Sprintf("SetDetailEnricherForTest: nil enricher func for short name %q", shortName))
	}
	if _, exists := detailEnricherRegistry[shortName]; exists {
		panic(fmt.Sprintf("SetDetailEnricherForTest: duplicate registration for short name %q", shortName))
	}
	detailEnricherRegistry[shortName] = f
}

// GetDetailEnricher returns the detail enricher for the given resource short name.
// Catalog-backed: checks the catalog (both top-level and child types) first;
// falls through to the legacy map so test overrides via SetDetailEnricherForTest
// continue to work for synthetic short names.
func GetDetailEnricher(shortName string) DetailEnricher {
	if ct := catalog.Find(shortName); ct != nil && ct.DetailEnrich != nil {
		return ct.DetailEnrich
	}
	if ct := catalog.FindChild(shortName); ct != nil && ct.DetailEnrich != nil {
		return ct.DetailEnrich
	}
	return detailEnricherRegistry[shortName]
}

// HasDetailEnricher returns true if a detail enricher is registered for the given short name.
// Catalog-backed: checks the catalog (both top-level and child) first; falls
// through to the legacy map.
func HasDetailEnricher(shortName string) bool {
	if ct := catalog.Find(shortName); ct != nil && ct.DetailEnrich != nil {
		return true
	}
	if ct := catalog.FindChild(shortName); ct != nil && ct.DetailEnrich != nil {
		return true
	}
	_, ok := detailEnricherRegistry[shortName]
	return ok
}

// CleanupDetailEnricherForTest removes a detail enricher. Used only in tests for cleanup.
func CleanupDetailEnricherForTest(shortName string) {
	delete(detailEnricherRegistry, shortName)
}
