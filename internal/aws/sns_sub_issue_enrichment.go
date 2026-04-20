// sns_sub_issue_enrichment.go — Wave 2 = None for the sns-sub resource type.
package aws

func init() {
	registerIssueEnricher("sns-sub", NoOpIssueEnricher, 100)
}
