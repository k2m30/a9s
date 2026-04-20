// ebs_snap_issue_enrichment.go — Wave 2 = None for the ebs-snap resource type.
package aws

func init() {
	registerIssueEnricher("ebs-snap", NoOpIssueEnricher, 100)
}
