// rds_snap_issue_enrichment.go — Wave 2 = None for the rds-snap resource type.
package aws

func init() {
	registerIssueEnricher("rds-snap", NoOpIssueEnricher, 100)
}
