// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// ses_issue_enrichment.go — Wave 2 issue enrichment for the ses resource type.
package aws

import (
	"context"
	"errors"
	"strconv"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"

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
func EnrichSESAccount(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]string),
		FieldUpdates: make(map[string]map[string]string),
	}

	if clients == nil || clients.SESv2 == nil {
		return result, nil
	}

	out, err := clients.SESv2.GetAccount(ctx, &sesv2.GetAccountInput{})
	// The account read is one answer every identity shares, so a refused one
	// is one failure, and the account signal is unread on every identity.
	code, rows, hasFinding := sesAccountFinding(out)
	for _, res := range resources {
		switch {
		case err != nil:
			markUninspected(&result, res.ID, checkOf(err))
		case hasFinding:
			setWave2Finding(&result, res.ID, code, rows)
		}
	}

	// DKIM is per identity, not per account: one unsigned domain says
	// nothing about the others, so this runs alongside the replication
	// above rather than sharing its shape.
	dkimErr := sesIdentityDKIM(ctx, clients, &result, resources)

	return result, errors.Join(err, dkimErr)
}

// sesIdentityDKIM calls GetEmailIdentity per identity (cap EnrichmentCap) and
// reports a domain that does not sign its outbound mail.
func sesIdentityDKIM(ctx context.Context, clients *ServiceClients, result *IssueEnricherResult, resources []resource.Resource) error {
	resources = capAtEnrichmentCap(result, resources, func(r resource.Resource) bool {
		return rowAppliesAny("ses", r, sesCodeDKIMOff)
	}, resourceIDsOf)
	n := len(resources)
	var failures []Failure
	var mu sync.Mutex
	loopErr := ForEachRow(ctx, result, resourceIDs(resources), EnrichmentParallelism, func(i int) {
		r := resources[i]
		if r.ID == "" {
			return
		}
		out, err := clients.SESv2.GetEmailIdentity(ctx, &sesv2.GetEmailIdentityInput{
			EmailIdentity: aws.String(r.ID),
		})
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			MarkSkipped(result, r.ID, &failures, err)
			return
		}
		if out.DkimAttributes != nil && out.DkimAttributes.SigningEnabled {
			return
		}
		setWave2Finding(result, r.ID, sesCodeDKIMOff, []domain.DetailRow{{Label: "DKIM signing", Value: "disabled", Tier: tierOf(sesCodeDKIMOff)}})
	})
	return errors.Join(loopErr, AggregateFailures("GetEmailIdentity", failures, n))
}

// sesAccountFinding derives the single account-level finding from GetAccount output.
// Returns (code, rows, true) when a finding exists, or ("", nil, false)
// when the account is healthy and below the quota threshold.
//
// Per-account context lives in rows (Enforcement Status / Sent Last 24h /
// Max 24h Send), never in the finding's phrase.
func sesAccountFinding(out *sesv2.GetAccountOutput) (domain.FindingCode, []domain.DetailRow, bool) {
	if out == nil {
		return "", nil, false
	}

	enforcementStatus := ""
	if out.EnforcementStatus != nil {
		enforcementStatus = *out.EnforcementStatus
	}

	switch enforcementStatus {
	case "SHUTDOWN":
		return sesCodeShutdown, []domain.DetailRow{
			{Label: "Action", Value: "Open an AWS support case after fixing the underlying issue", Tier: "!"},
		}, true
	case "PROBATION":
		return sesCodeProbation, []domain.DetailRow{
			{Label: "Action", Value: "Reduce bounce/complaint rate before AWS suspends sending", Tier: "!"},
		}, true
	}

	if out.SendQuota != nil && out.SendQuota.Max24HourSend > 0 {
		sent := out.SendQuota.SentLast24Hours
		max := out.SendQuota.Max24HourSend
		if sent > 0.8*max {
			sentStr := strconv.FormatFloat(sent, 'f', -1, 64)
			maxStr := strconv.FormatFloat(max, 'f', -1, 64)
			return sesCodeQuota, []domain.DetailRow{
				{Label: "Sent Last 24h", Value: sentStr, Tier: "~"},
				{Label: "Max 24h Send", Value: maxStr},
			}, true
		}
	}

	return "", nil, false
}
