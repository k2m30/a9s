// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// ses_issue_enrichment.go — Wave 2 issue enrichment for the ses resource type.
package aws

import (
	"context"
	"strconv"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	sesv2types "github.com/aws/aws-sdk-go-v2/service/sesv2/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// ses canonical FindingCodes.
const (
	sesCodeShutdown  domain.FindingCode = "ses.account-shutdown"
	sesCodeProbation domain.FindingCode = "ses.account-probation"
	sesCodeQuota     domain.FindingCode = "ses.quota-high"
	sesCodeDKIMOff   domain.FindingCode = "ses.dkim-off"
)

// EnrichSESAccount calls sesv2:GetAccount once (account-wide) and replicates
// the single account-level finding onto every identity row in the input slice.
//
// §4 precedence:
//   - SHUTDOWN  → severity "!", Summary "sending paused by AWS (shutdown)"
//   - PROBATION → severity "!", Summary "account under review (probation)"
//   - quota > 80% → severity "~", Summary "quota 80%+ used"
//   - otherwise → no finding
//
// The enricher no longer writes FieldUpdates["status"]. The Wave-2
// phrase is sourced at render time from r.Findings via domain.StatusPhrase;
// row color is sourced from the Wave-2 finding's Severity via colorSES.
func EnrichSESAccount(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
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

	// Decide the single account-level finding using §4 precedence, then
	// replicate it onto every identity row.
	code, phrase, severityGlyph, rows, hasFinding := sesAccountFinding(out)
	if hasFinding {
		for _, res := range resources {
			setWave2Finding(&result, res.ID, code, phrase, severityGlyph, "ses", rows)
		}
	}

	// DKIM is per identity, not per account: one unsigned domain says
	// nothing about the others, so this runs alongside the replication
	// above rather than sharing its shape.
	sesIdentityDKIM(ctx, clients, &result, resources)

	result.Truncated = false
	return result, nil
}

// sesIdentityDKIM calls GetEmailIdentity per identity (cap EnrichmentCap) and
// reports a domain that does not sign its outbound mail. A single verified
// address cannot carry DKIM at all, so only domains are checked.
func sesIdentityDKIM(ctx context.Context, clients *ServiceClients, result *IssueEnricherResult, resources []resource.Resource) {
	n := min(len(resources), EnrichmentCap)
	var mu sync.Mutex
	_ = ForEachParallel(ctx, n, EnrichmentParallelism, func(i int) {
		r := resources[i]
		if r.ID == "" {
			return
		}
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*sesv2.GetEmailIdentityOutput, error) {
			return clients.SESv2.GetEmailIdentity(ctx, &sesv2.GetEmailIdentityInput{
				EmailIdentity: aws.String(r.ID),
			})
		})
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			result.TruncatedIDs[r.ID] = true
			return
		}
		if out.IdentityType != sesv2types.IdentityTypeDomain {
			return
		}
		if out.DkimAttributes != nil && out.DkimAttributes.SigningEnabled {
			return
		}
		setWave2Finding(result, r.ID, sesCodeDKIMOff, "DKIM not enabled", "~", "ses",
			[]domain.DetailRow{{Label: "DKIM signing", Value: "disabled", Tier: "~"}})
	})
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
		return sesCodeShutdown, "sending paused by AWS (shutdown)", "!", []domain.DetailRow{
			{Label: "Action", Value: "Open an AWS support case after fixing the underlying issue", Tier: "!"},
		}, true
	case "PROBATION":
		return sesCodeProbation, "account under review (probation)", "!", []domain.DetailRow{
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
