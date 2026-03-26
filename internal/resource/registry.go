package resource

import "context"

// Fetcher is the function signature for resource fetchers.
// Each AWS resource type registers a fetcher that takes a context and
// a clients interface{} (which the fetcher type-asserts to the concrete
// client type it needs, e.g., *awsclient.ServiceClients).
type Fetcher func(ctx context.Context, clients interface{}) ([]Resource, error)

// ParentContext holds key-value pairs passed from a parent view to a child
// fetcher. For example, {"bucket": "my-bucket", "prefix": "data/"}.
type ParentContext map[string]string

// ChildFetcher is the function signature for child resource fetchers.
// Unlike top-level Fetcher, it receives a ParentContext with parameters
// from the parent view (e.g., bucket name, zone ID).
type ChildFetcher func(ctx context.Context, clients interface{}, parentCtx ParentContext) ([]Resource, error)

// registry maps resource short names to their fetcher functions.
var registry = map[string]Fetcher{}

// fieldKeyRegistry maps resource short names to their valid Fields keys.
// Populated by RegisterFieldKeys calls in each aws/*.go init().
var fieldKeyRegistry = map[string][]string{}

// childTypes maps child type short names to their type definitions.
var childTypes = map[string]*ResourceTypeDef{}

// childFetcherRegistry maps child type short names to their fetcher functions.
var childFetcherRegistry = map[string]ChildFetcher{}

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

// RegisterFieldKeys records the valid Fields keys for a resource type.
// Called from init() in each aws/*.go file alongside Register.
func RegisterFieldKeys(shortName string, keys []string) {
	fieldKeyRegistry[shortName] = keys
}

// GetFieldKeys returns the registered Fields keys for the given resource type,
// or nil if none are registered.
func GetFieldKeys(shortName string) []string {
	return fieldKeyRegistry[shortName]
}

// RegisterChildType stores a child type definition in the child types registry.
// Called from init() in each aws/*.go file for sub-resource types.
func RegisterChildType(def ResourceTypeDef) {
	copy := def
	childTypes[def.ShortName] = &copy
}

// GetChildType returns the child type definition for the given short name,
// or nil if no child type is registered.
func GetChildType(shortName string) *ResourceTypeDef {
	return childTypes[shortName]
}

// UnregisterChildType removes a child type. Used only in tests for cleanup.
func UnregisterChildType(shortName string) {
	delete(childTypes, shortName)
}

// RegisterChildFetcher stores a child fetcher function for the given short name.
// Called from init() in each aws/*.go file for sub-resource types.
func RegisterChildFetcher(shortName string, f ChildFetcher) {
	childFetcherRegistry[shortName] = f
}

// GetChildFetcher returns the child fetcher for the given short name,
// or nil if no child fetcher is registered.
func GetChildFetcher(shortName string) ChildFetcher {
	return childFetcherRegistry[shortName]
}

// UnregisterChildFetcher removes a child fetcher. Used only in tests for cleanup.
func UnregisterChildFetcher(shortName string) {
	delete(childFetcherRegistry, shortName)
}

// PaginatedFetcher returns a single page of resources.
type PaginatedFetcher func(ctx context.Context, clients interface{}, continuationToken string) (FetchResult, error)

// PaginatedChildFetcher returns a single page of child resources.
type PaginatedChildFetcher func(ctx context.Context, clients interface{}, parentCtx ParentContext, continuationToken string) (FetchResult, error)

// paginatedRegistry maps resource short names to their paginated fetcher functions.
var paginatedRegistry = map[string]PaginatedFetcher{}

// paginatedChildRegistry maps child type short names to their paginated child fetcher functions.
var paginatedChildRegistry = map[string]PaginatedChildFetcher{}

// RegisterPaginated adds a paginated fetcher for the given resource short name.
// Called from init() in each aws/*.go file for resources that support pagination.
func RegisterPaginated(shortName string, f PaginatedFetcher) {
	paginatedRegistry[shortName] = f
}

// GetPaginatedFetcher returns the paginated fetcher for the given resource short name,
// or nil if no paginated fetcher is registered.
func GetPaginatedFetcher(shortName string) PaginatedFetcher {
	return paginatedRegistry[shortName]
}

// UnregisterPaginated removes a paginated fetcher. Used only in tests for cleanup.
func UnregisterPaginated(shortName string) {
	delete(paginatedRegistry, shortName)
}

// RegisterPaginatedChild adds a paginated child fetcher for the given short name.
// Called from init() in each aws/*.go file for child resources that support pagination.
func RegisterPaginatedChild(shortName string, f PaginatedChildFetcher) {
	paginatedChildRegistry[shortName] = f
}

// GetPaginatedChildFetcher returns the paginated child fetcher for the given short name,
// or nil if no paginated child fetcher is registered.
func GetPaginatedChildFetcher(shortName string) PaginatedChildFetcher {
	return paginatedChildRegistry[shortName]
}

// UnregisterPaginatedChild removes a paginated child fetcher. Used only in tests for cleanup.
func UnregisterPaginatedChild(shortName string) {
	delete(paginatedChildRegistry, shortName)
}
