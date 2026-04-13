package resource

import "context"

// Enricher is the function signature for on-demand resource enrichers.
// It receives the current resource and returns an enriched copy with additional
// fields populated (e.g., RawStruct updated with fetched data).
type Enricher func(ctx context.Context, clients any, res Resource) (Resource, error)

var enricherRegistry = map[string]Enricher{}

// RegisterEnricher adds an enricher for the given resource short name.
func RegisterEnricher(shortName string, f Enricher) {
	enricherRegistry[shortName] = f
}

// GetEnricher returns the enricher for the given resource short name, or nil.
func GetEnricher(shortName string) Enricher {
	return enricherRegistry[shortName]
}

// HasEnricher returns true if an enricher is registered for the given short name.
func HasEnricher(shortName string) bool {
	_, ok := enricherRegistry[shortName]
	return ok
}

// UnregisterEnricher removes an enricher. Used only in tests for cleanup.
func UnregisterEnricher(shortName string) {
	delete(enricherRegistry, shortName)
}
