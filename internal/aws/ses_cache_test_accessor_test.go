// ses_cache_test_accessor_test.go — test-only accessors for SES rule-set cache state.
//
// These functions are intentionally in package aws (white-box) so that
// tests/unit/ can call the exported ClearAllSESRuleSetCaches and then
// verify via a companion test in this package that the map was fully wiped.
// They expose no production API — the file is compiled only during `go test`.
package aws

// SESRuleSetCachesLen returns the number of entries in the package-level
// sesRuleSetCaches map. Used by Pin 5 regression tests to assert that
// ClearAllSESRuleSetCaches wipes all entries.
func SESRuleSetCachesLen() int {
	sesRuleSetCacheMu.Lock()
	defer sesRuleSetCacheMu.Unlock()
	return len(sesRuleSetCaches)
}

// SeedSESRuleSetCache inserts a non-nil cache entry for the given clients
// pointer so tests can pre-populate state before calling ClearAllSESRuleSetCaches.
func SeedSESRuleSetCache(c *ServiceClients) {
	sesRuleSetCacheMu.Lock()
	defer sesRuleSetCacheMu.Unlock()
	sesRuleSetCaches[c] = &sesReceiptRuleSetCache{}
}
