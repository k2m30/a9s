// secrets_issue_enrichment.go — Wave 2 = None for the secrets resource type.
package aws

func init() {
	registerIssueEnricher("secrets", NoOpIssueEnricher, 100)
}
