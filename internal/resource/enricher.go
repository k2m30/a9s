package resource

import "context"

// DetailEnricher is the function signature for on-demand detail enrichers.
// It receives the current resource and returns an enriched copy with additional
// fields populated (e.g., RawStruct updated with fetched data).
//
// Detail enrichment runs synchronously when the user opens a detail, YAML, or
// JSON view for a single resource. It is separate from Wave 2 issue enrichment
// (see internal/aws/enrichment.go), which scans the retained page in the
// background to surface attention signals.
type DetailEnricher func(ctx context.Context, clients any, res Resource) (Resource, error)

var detailEnricherRegistry = map[string]DetailEnricher{}

// RegisterDetailEnricher adds a detail enricher for the given resource short name.
func RegisterDetailEnricher(shortName string, f DetailEnricher) {
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
