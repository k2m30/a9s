// ses_issue_enrichment.go — Wave 2 issue enrichment for the ses resource type.
package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/sesv2"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	registerIssueEnricher("ses", EnrichSESAccount, 100)
}

// EnrichSESAccount calls GetAccount once (account-wide) and returns a Finding
// keyed "account" when the account is shut down, on probation, or sending is disabled.
// Severity "!" for SHUTDOWN, "~" for PROBATION or sending disabled.
func EnrichSESAccount(ctx context.Context, clients *ServiceClients, _ []resource.Resource) (IssueEnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	truncatedIDs := make(map[string]bool)
	if clients.SESv2 == nil {
		return IssueEnricherResult{Findings: findings, TruncatedIDs: truncatedIDs}, nil
	}
	out, err := clients.SESv2.GetAccount(ctx, &sesv2.GetAccountInput{})
	if err != nil {
		return IssueEnricherResult{TruncatedIDs: truncatedIDs}, err
	}
	if out.EnforcementStatus != nil {
		switch *out.EnforcementStatus {
		case "SHUTDOWN":
			findings["account"] = resource.EnrichmentFinding{
				Severity: "!",
				Summary:  "SES account SHUTDOWN — sending blocked",
				Rows: []resource.FindingRow{
					{Label: "Enforcement Status", Value: "SHUTDOWN", Tier: "!"},
				},
			}
		case "PROBATION":
			findings["account"] = resource.EnrichmentFinding{
				Severity: "~",
				Summary:  "SES account on PROBATION",
				Rows: []resource.FindingRow{
					{Label: "Enforcement Status", Value: "PROBATION", Tier: "~"},
				},
			}
		}
	}
	// Only check sending-disabled if enforcement status didn't already produce a finding.
	if _, exists := findings["account"]; !exists && !out.SendingEnabled {
		findings["account"] = resource.EnrichmentFinding{
			Severity: "~",
			Summary:  "SES account sending disabled",
			Rows: []resource.FindingRow{
				{Label: "Sending Enabled", Value: "false", Tier: "~"},
			},
		}
	}
	issueCount := 0
	for _, f := range findings {
		if f.Severity == "!" {
			issueCount++
		}
	}
	return IssueEnricherResult{IssueCount: issueCount, Truncated: false, TruncatedIDs: truncatedIDs, Findings: findings}, nil
}
