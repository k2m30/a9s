// ses_issue_enrichment.go — Wave 2 issue enrichment for the ses resource type.
package aws

import (
	"context"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/service/sesv2"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	registerIssueEnricher("ses", EnrichSESAccount, 100)
	resource.RegisterIssueEnricherFieldKeys("ses", []string{"status"})
}

// EnrichSESAccount calls sesv2:GetAccount once (account-wide) and replicates
// the single account-level finding onto every identity row in the input slice.
//
// §4 precedence:
//   - SHUTDOWN  → severity "!", Summary "account SHUTDOWN"
//   - PROBATION → severity "!", Summary "account PROBATION"
//   - quota > 80% → severity "~", Summary "quota 80%+ used"
//   - otherwise → no finding
//
// FieldUpdates["status"]: if the row is Healthy (Status==""), set to finding.Summary;
// if Wave-1 already set a Status, bump via resource.BumpFindingSuffix.
//
// IssueCount is 1 when severity is "!", else 0 — counted once for the whole
// account regardless of how many identity rows are in the list (spec §4
// "S1 counts the account-level finding once, not N times").
func EnrichSESAccount(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (IssueEnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	truncatedIDs := make(map[string]bool)
	fieldUpdates := make(map[string]map[string]string)

	if clients == nil || clients.SESv2 == nil {
		return IssueEnricherResult{Findings: findings, TruncatedIDs: truncatedIDs, FieldUpdates: fieldUpdates}, nil
	}

	out, err := clients.SESv2.GetAccount(ctx, &sesv2.GetAccountInput{})
	if err != nil {
		// Symmetric with the no-error paths below — always return fully
		// initialized maps so a caller that writes into result.Findings or
		// result.FieldUpdates cannot panic on a nil map. Range over nil is
		// safe, but writes would not be.
		return IssueEnricherResult{
			Findings:     findings,
			FieldUpdates: fieldUpdates,
			TruncatedIDs: truncatedIDs,
		}, err
	}

	// Decide the single account-level finding using §4 precedence.
	finding, hasFinding := sesAccountFinding(out)
	if !hasFinding {
		return IssueEnricherResult{
			IssueCount:   0,
			Truncated:    false,
			TruncatedIDs: truncatedIDs,
			Findings:     findings,
			FieldUpdates: fieldUpdates,
		}, nil
	}

	// Replicate the finding onto every identity row.
	for _, res := range resources {
		findings[res.ID] = finding

		// S4 FieldUpdate: Healthy row → set to summary; non-Healthy → bump suffix.
		var newStatus string
		if res.Status == "" {
			newStatus = finding.Summary
		} else {
			newStatus = resource.BumpFindingSuffix(res.Status)
		}
		fieldUpdates[res.ID] = map[string]string{"status": newStatus}
	}

	// IssueCount: 1 if "!" severity (account counted once), else 0.
	issueCount := 0
	if finding.Severity == "!" {
		issueCount = 1
	}

	return IssueEnricherResult{
		IssueCount:   issueCount,
		Truncated:    false,
		TruncatedIDs: truncatedIDs,
		Findings:     findings,
		FieldUpdates: fieldUpdates,
	}, nil
}

// sesAccountFinding derives the single account-level finding from GetAccount output.
// Returns (finding, true) when a finding exists, or (zero, false) when the account
// is healthy and below the quota threshold.
//
// U11 contract: Summary is the short S4 phrase only; per-account context lives in
// Rows (Enforcement Status / Sent Last 24h / Max 24h Send). strings.Contains(Summary, rowValue) == false.
func sesAccountFinding(out *sesv2.GetAccountOutput) (resource.EnrichmentFinding, bool) {
	if out == nil {
		return resource.EnrichmentFinding{}, false
	}

	enforcementStatus := ""
	if out.EnforcementStatus != nil {
		enforcementStatus = *out.EnforcementStatus
	}

	switch enforcementStatus {
	case "SHUTDOWN":
		return resource.EnrichmentFinding{
			Severity: "!",
			Summary:  "account SHUTDOWN",
			Rows: []resource.FindingRow{
				{Label: "Action", Value: "Open an AWS support case after fixing the underlying issue", Tier: "!"},
			},
		}, true
	case "PROBATION":
		return resource.EnrichmentFinding{
			Severity: "!",
			Summary:  "account PROBATION",
			Rows: []resource.FindingRow{
				{Label: "Action", Value: "Reduce bounce/complaint rate before AWS suspends sending", Tier: "!"},
			},
		}, true
	}

	// Check quota threshold (strict > 80%).
	if out.SendQuota != nil && out.SendQuota.Max24HourSend > 0 {
		sent := out.SendQuota.SentLast24Hours
		max := out.SendQuota.Max24HourSend
		if sent > 0.8*max {
			sentStr := strconv.FormatFloat(sent, 'f', -1, 64)
			maxStr := strconv.FormatFloat(max, 'f', -1, 64)
			return resource.EnrichmentFinding{
				Severity: "~",
				Summary:  "quota 80%+ used",
				Rows: []resource.FindingRow{
					{Label: "Sent Last 24h", Value: sentStr, Tier: "~"},
					{Label: "Max 24h Send", Value: maxStr},
				},
			}, true
		}
	}

	return resource.EnrichmentFinding{}, false
}
