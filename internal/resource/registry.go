package resource

import "context"

// Fetcher is the function signature for resource fetchers.
// Each AWS resource type registers a fetcher that takes a context and
// a clients interface{} (which the fetcher type-asserts to the concrete
// client type it needs, e.g., *awsclient.ServiceClients).
type Fetcher func(ctx context.Context, clients interface{}) ([]Resource, error)

// registry maps resource short names to their fetcher functions.
var registry = map[string]Fetcher{}

// Register adds a fetcher for the given resource short name.
// Called from init() in each aws/*.go file.
func Register(shortName string, f Fetcher) {
	registry[shortName] = f
}

// Unregister removes a fetcher. Used only in tests for cleanup.
func Unregister(shortName string) {
	delete(registry, shortName)
}

// GetFetcher returns the fetcher for the given resource short name,
// or nil if no fetcher is registered.
func GetFetcher(shortName string) Fetcher {
	return registry[shortName]
}
