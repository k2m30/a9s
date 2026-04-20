// lambda_issue_enrichment.go — Wave 2 = None for the lambda resource type.
package aws

func init() {
	registerIssueEnricher("lambda", NoOpIssueEnricher, 100)
}
