// kinesis_issue_enrichment.go — Wave 2 = None for the kinesis resource type.
package aws

func init() {
	registerIssueEnricher("kinesis", NoOpIssueEnricher, 100)
}
