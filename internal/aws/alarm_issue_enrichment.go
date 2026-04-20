// alarm_issue_enrichment.go — Wave 2 = None for the alarm resource type.
package aws

func init() {
	registerIssueEnricher("alarm", NoOpIssueEnricher, 100)
}
