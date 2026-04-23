// redis_issue_enrichment.go — spec §3.2 = "No Wave 2 signals".
//
// Registers NoOpIssueEnricher so TestConformance_EveryResourceTypeHasWave2Registration
// sees an explicit entry for redis. The fetcher (redis.go) computes all Wave 1
// signals from the list response without additional API calls.
package aws

func init() {
	registerIssueEnricher("redis", NoOpIssueEnricher, 100)
}
