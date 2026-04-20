// ssm_issue_enrichment.go — Wave 2 = None for the ssm resource type.
package aws

func init() {
	registerIssueEnricher("ssm", NoOpIssueEnricher, 100)
}
