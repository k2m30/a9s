package resource

import (
	"fmt"

	"github.com/k2m30/a9s/v3/internal/domain"
)

// DetailEnricher is the function signature for on-demand detail enrichers.
// Declaration lives in internal/domain/contracts.go; this alias keeps
// existing consumers compiling. Deleted in PR-04n.
type DetailEnricher = domain.DetailEnricher

var detailEnricherRegistry = map[string]DetailEnricher{}

// RegisterDetailEnricher adds a detail enricher for the given resource short name.
// Panics on empty short name, nil function, or duplicate registration.
func RegisterDetailEnricher(shortName string, f DetailEnricher) {
	if shortName == "" {
		panic("RegisterDetailEnricher: empty short name")
	}
	if f == nil {
		panic(fmt.Sprintf("RegisterDetailEnricher: nil enricher func for short name %q", shortName))
	}
	if _, exists := detailEnricherRegistry[shortName]; exists {
		panic(fmt.Sprintf("RegisterDetailEnricher: duplicate registration for short name %q", shortName))
	}
	detailEnricherRegistry[shortName] = f
}

// GetDetailEnricher returns the detail enricher for the given resource short name, or nil.
func GetDetailEnricher(shortName string) DetailEnricher {
	return detailEnricherRegistry[shortName]
}

// HasDetailEnricher returns true if a detail enricher is registered for the given short name.
func HasDetailEnricher(shortName string) bool {
	_, ok := detailEnricherRegistry[shortName]
	return ok
}

// UnregisterDetailEnricher removes a detail enricher. Used only in tests for cleanup.
func UnregisterDetailEnricher(shortName string) {
	delete(detailEnricherRegistry, shortName)
}
