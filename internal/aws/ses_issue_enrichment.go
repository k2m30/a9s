// ses_issue_enrichment.go — Wave 2 issue enrichment for the ses resource type.
package aws

import (
	"context"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/service/sesv2"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ses canonical FindingCodes.
const (
	sesCodeShutdown  domain.FindingCode = "ses.account-shutdown"
	sesCodeProbation domain.FindingCode = "ses.account-probation"
	sesCodeQuota     domain.FindingCode = "ses.quota-high"
)

// EnrichSESAccount calls sesv2:GetAccount once (account-wide) and replicates
// the single account-level finding onto every identity row in the input slice.
//
// §4 precedence:
//   - SHUTDOWN  → severity "!", Summary "account SHUTDOWN"
//   - PROBATION → severity "!", Summary "account PROBATION"
//   - quota > 80% → severity "~", Summary "quota 80%+ used"
//   - otherwise → no finding
//
// AS-1397: the enricher no longer writes FieldUpdates["status"]. The Wave-2
// phrase is sourced at render time from r.Findings via phraseFromFindings;
// row color is sourced from the Wave-2 finding's Severity via colorSES.
//
// IssueCount is 1 when severity is "!", else 0 — counted once for the whole
// account regardless of how many identity rows are in the list (spec §4
// "S1 counts the account-level finding once, not N times").
func EnrichSESAccount(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string]domain.Finding),
		TruncatedIDs: make(map[string]bool),
		FieldUpdates: make(map[string]map[string]string),
	}

	if clients == nil || clients.SESv2 == nil {
		return result, nil
	}

	out, err := clients.SESv2.GetAccount(ctx, &sesv2.GetAccountInput{})
	if err != nil {
		// Symmetric with the no-error paths below — always return fully
		// initialized maps so a caller that writes into result.Findings or
		// result.FieldUpdates cannot panic on a nil map. Range over nil is
		// safe, but writes would not be.
		return result, err
	}

	// Decide the single account-level finding using §4 precedence.
	code, phrase, severityGlyph, rows, hasFinding := sesAccountFinding(out)
	if !hasFinding {
		return result, nil
	}

	// Replicate the finding onto every identity row.
	for _, res := range resources {
		setWave2Finding(&result, res.ID, code, phrase, severityGlyph, "ses", rows)
	}

	// IssueCount: 1 if "!" severity (account counted once), else 0.
	issueCount := 0
	if severityGlyph == "!" {
		issueCount = 1
	}

	result.IssueCount = issueCount
	result.Truncated = false
	return result, nil
}

// sesAccountFinding derives the single account-level finding from GetAccount output.
// Returns (code, phrase, severityGlyph, rows, hasFinding) when a finding exists,
// or ("", "", "", nil, false) when the account is healthy and below the quota threshold.
//
// U11 contract: phrase is the short S4 phrase only; per-account context lives in
// rows (Enforcement Status / Sent Last 24h / Max 24h Send). strings.Contains(phrase, rowValue) == false.
func sesAccountFinding(out *sesv2.GetAccountOutput) (domain.FindingCode, string, string, []domain.DetailRow, bool) {
	if out == nil {
		return "", "", "", nil, false
	}

	enforcementStatus := ""
	if out.EnforcementStatus != nil {
		enforcementStatus = *out.EnforcementStatus
	}

	switch enforcementStatus {
	case "SHUTDOWN":
		return sesCodeShutdown, "account SHUTDOWN", "!", []domain.DetailRow{
			{Label: "Action", Value: "Open an AWS support case after fixing the underlying issue", Tier: "!"},
		}, true
	case "PROBATION":
		return sesCodeProbation, "account PROBATION", "!", []domain.DetailRow{
			{Label: "Action", Value: "Reduce bounce/complaint rate before AWS suspends sending", Tier: "!"},
		}, true
	}

	// Check quota threshold (strict > 80%).
	if out.SendQuota != nil && out.SendQuota.Max24HourSend > 0 {
		sent := out.SendQuota.SentLast24Hours
		max := out.SendQuota.Max24HourSend
		if sent > 0.8*max {
			sentStr := strconv.FormatFloat(sent, 'f', -1, 64)
			maxStr := strconv.FormatFloat(max, 'f', -1, 64)
			return sesCodeQuota, "quota 80%+ used", "~", []domain.DetailRow{
				{Label: "Sent Last 24h", Value: sentStr, Tier: "~"},
				{Label: "Max 24h Send", Value: maxStr},
			}, true
		}
	}

	return "", "", "", nil, false
}
