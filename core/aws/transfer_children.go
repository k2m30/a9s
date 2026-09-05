// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// transfer_children.go — Transfer Family Agreements child view. Agreements
// are the only server-scoped child (DescribedAgreement.ServerId;
// ListAgreements(ServerId)) — docs/resources/transfer.md §2.1. Profiles and
// certificates are account-scoped (ListedProfile/ListedCertificate carry no
// ServerId), so they are not separate child views; each agreement's
// LocalProfileId/PartnerProfileId and their certificates are resolved on
// demand by enrichTransferAgreement (the existing DetailEnrich hook), not
// eagerly at list-fetch time — keeps the child list itself a plain N+1
// (ListAgreements + DescribeAgreement), the same in-fetcher shape
// FetchTransferServersPage already uses (transfer.go).
package aws

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/transfer"
	transfertypes "github.com/aws/aws-sdk-go-v2/service/transfer/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// transfer.* agreement/certificate FindingCodes — docs/resources/transfer.md §3.2.
const (
	transferCodeAgreementInactive domain.FindingCode = "transfer.warn.agreement_inactive"
	transferCodeCertExpired       domain.FindingCode = "transfer.broken.cert_expired"
	transferCodeCertExpiring      domain.FindingCode = "transfer.warn.cert_expiring"
)

// transferCertExpiringWindow is the "expires soon" lookahead window
// (docs/resources/transfer.md §3.2: "within 30 days").
const transferCertExpiringWindow = 30 * 24 * time.Hour

// FetchTransferAgreements fetches one server's agreements: ListAgreements +
// DescribeAgreement N+1 (both RetryOnThrottle), the same in-fetcher N+1
// shape as FetchTransferServersPage. Only the agreement's own Status drives
// a Wave-1 finding here — LocalProfileId/PartnerProfileId and their
// certificates are account-scoped lookups with no server link, resolved on
// demand by enrichTransferAgreement instead of eagerly on every list fetch.
func FetchTransferAgreements(ctx context.Context, api TransferAPI, serverID string, continuationToken string) (resource.FetchResult, error) {
	if serverID == "" {
		return resource.FetchResult{}, nil
	}

	input := &transfer.ListAgreementsInput{
		ServerId:   aws.String(serverID),
		MaxResults: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.NextToken = aws.String(continuationToken)
	}

	listOutput, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*transfer.ListAgreementsOutput, error) {
		return api.ListAgreements(ctx, input)
	})
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("listing Transfer agreements for %s: %w", serverID, err)
	}

	total := len(listOutput.Agreements)
	var resources []resource.Resource
	var failures []string
	for i := range listOutput.Agreements {
		listed := listOutput.Agreements[i]
		id := aws.ToString(listed.AgreementId)

		describeOutput, describeErr := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*transfer.DescribeAgreementOutput, error) {
			return api.DescribeAgreement(ctx, &transfer.DescribeAgreementInput{
				AgreementId: aws.String(id),
				ServerId:    aws.String(serverID),
			})
		})
		if describeErr != nil || describeOutput.Agreement == nil {
			if describeErr == nil {
				describeErr = fmt.Errorf("nil agreement in DescribeAgreement response")
			}
			failures = append(failures, fmt.Sprintf("%s: %s", id, describeErr.Error()))
			continue
		}
		resources = append(resources, buildTransferAgreementResource(describeOutput.Agreement, serverID))
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: listOutput.NextToken != nil,
			NextToken:   aws.ToString(listOutput.NextToken),
			PageSize:    len(resources),
			TotalHint:   -1,
		},
	}, AggregateFailures("transfer: DescribeAgreement", failures, total)
}

// buildTransferAgreementResource constructs a Resource from a
// DescribeAgreement response. LocalProfileId/PartnerProfileId are stored
// raw (the bare profile id); enrichTransferAgreement resolves their As2Id
// fact and certificate expiry on demand (docs/resources/transfer.md
// §2.1/§3.2) — an ACTIVE agreement carries zero findings until then. serverID
// is threaded through to Fields["server_id"] so the console-link builder can
// deep-link to the parent server's page.
func buildTransferAgreementResource(agreement *transfertypes.DescribedAgreement, serverID string) resource.Resource {
	id := aws.ToString(agreement.AgreementId)

	var findings []domain.Finding
	if agreement.Status == transfertypes.AgreementStatusTypeInactive {
		findings = append(findings, domain.Finding{
			Code:     transferCodeAgreementInactive,
			Phrase:   "inactive: partner traffic rejected",
			Detail:   "Agreement is inactive; partner traffic is rejected.",
			Severity: domain.SevWarn,
			Source:   "wave1",
		})
	}

	return resource.Resource{
		ID:   id,
		Name: id,
		Fields: map[string]string{
			"agreement_id":    id,
			"description":     aws.ToString(agreement.Description),
			"status":          domain.StatusPhrase(findings),
			"local_profile":   aws.ToString(agreement.LocalProfileId),
			"partner_profile": aws.ToString(agreement.PartnerProfileId),
			"base_directory":  aws.ToString(agreement.BaseDirectory),
			"server_id":       serverID,
		},
		RawStruct: agreement,
		Findings:  findings,
	}
}

// enrichTransferAgreement is the on-demand DetailEnrich for the Agreements
// child type (registered as DetailEnrich on the transfer_agreements catalog
// entry, catalog_networking.go): resolves LocalProfileId/PartnerProfileId
// to their As2Id fact (DescribeProfile) and evaluates each profile's
// certificates for expiry (DescribeCertificate) — both account-scoped
// lookups with no server link, so this detail-open enrichment is the only
// place they can be resolved (docs/resources/transfer.md §2.1). clients must
// be the session's *DetailEnrichmentCtx, the production shape the runtime
// always passes.
func enrichTransferAgreement(ctx context.Context, clients any, res resource.Resource) (resource.Resource, error) {
	dctx, ok := clients.(*DetailEnrichmentCtx)
	if !ok || dctx == nil {
		return res, fmt.Errorf("invalid detail-enrichment context")
	}

	// Same contract and same check ORDER as the generic engine's guard
	// (detail_enrich_engine.go): a disk-cache-seeded row carries Fields only,
	// no RawStruct, until the live refetch lands, and pre-connect the session
	// caches exist while Clients is still nil — so the thin-row skip must run
	// before requiring Clients, or a pre-connect open of a seeded row flashes
	// an error for a transient, self-healing state.
	if res.RawStruct == nil {
		return res, nil
	}
	if dctx.Clients == nil {
		return res, fmt.Errorf("invalid detail-enrichment context")
	}
	c := dctx.Clients

	agreement, ok := assertStruct[transfertypes.DescribedAgreement](res.RawStruct)
	if !ok {
		return res, fmt.Errorf("unexpected RawStruct type: %T", res.RawStruct)
	}

	localProfileID := aws.ToString(agreement.LocalProfileId)
	partnerProfileID := aws.ToString(agreement.PartnerProfileId)
	localAs2ID, localCertFindings := resolveTransferProfile(ctx, c.Transfer, localProfileID)
	partnerAs2ID, partnerCertFindings := resolveTransferProfile(ctx, c.Transfer, partnerProfileID)

	enriched := res
	enriched.Fields = make(map[string]string, len(res.Fields))
	maps.Copy(enriched.Fields, res.Fields)
	// fieldpath.ExtractFieldList resolves {Path: "LocalProfileId"} /
	// {Path: "PartnerProfileId"} via its designed override lookup —
	// Fields[ToSnakeCase(path)] — BEFORE falling back to RawStruct, so the
	// "_id"-suffixed keys below (not the list-column "local_profile" /
	// "partner_profile" keys above) are what the rendered detail actually
	// picks up.
	if localAs2ID != "" {
		enriched.Fields["local_profile"] = localAs2ID
		enriched.Fields["local_profile_id"] = localAs2ID
	}
	if partnerAs2ID != "" {
		enriched.Fields["partner_profile"] = partnerAs2ID
		enriched.Fields["partner_profile_id"] = partnerAs2ID
	}

	enriched.Findings = slices.Concat(res.Findings, localCertFindings, partnerCertFindings)

	return enriched, nil
}

// resolveTransferProfile resolves a profile id to its As2Id fact
// (DescribeProfile) and evaluates each of its certificates for expiry
// (DescribeCertificate). Returns ("", nil) for an empty profileID or a
// failed/empty lookup — a resolution failure must not break the agreement
// detail it is enriching.
func resolveTransferProfile(ctx context.Context, api TransferAPI, profileID string) (string, []domain.Finding) {
	if profileID == "" {
		return "", nil
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*transfer.DescribeProfileOutput, error) {
		return api.DescribeProfile(ctx, &transfer.DescribeProfileInput{ProfileId: aws.String(profileID)})
	})
	if err != nil || out.Profile == nil {
		return "", nil
	}
	as2ID := aws.ToString(out.Profile.As2Id)

	var findings []domain.Finding
	for _, certID := range out.Profile.CertificateIds {
		if f, ok := transferCertificateFinding(ctx, api, certID); ok {
			findings = append(findings, f)
		}
	}
	return as2ID, findings
}

// transferCertificateFinding evaluates one certificate for expiry via
// DescribeCertificate: InactiveDate past or Status INACTIVE is Broken
// "expired"; within 30 days is Warning "expires in <N>d" — the child-row
// signal (docs/resources/transfer.md §3.2), never bubbled to the server
// row. Returns (zero, false) for an empty certID or a failed lookup.
func transferCertificateFinding(ctx context.Context, api TransferAPI, certID string) (domain.Finding, bool) {
	if certID == "" {
		return domain.Finding{}, false
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*transfer.DescribeCertificateOutput, error) {
		return api.DescribeCertificate(ctx, &transfer.DescribeCertificateInput{CertificateId: aws.String(certID)})
	})
	if err != nil || out.Certificate == nil {
		return domain.Finding{}, false
	}
	cert := out.Certificate

	expired := cert.Status == transfertypes.CertificateStatusTypeInactive
	if !expired && cert.InactiveDate != nil && cert.InactiveDate.Before(time.Now()) {
		expired = true
	}
	if expired {
		return domain.Finding{
			Code:     transferCodeCertExpired,
			Phrase:   "expired",
			Detail:   fmt.Sprintf("Certificate %s has expired.", certID),
			Severity: domain.SevBroken,
			Source:   "wave1",
		}, true
	}

	if cert.InactiveDate != nil {
		remaining := time.Until(*cert.InactiveDate)
		if remaining > 0 && remaining <= transferCertExpiringWindow {
			days := int(remaining.Hours() / 24)
			return domain.Finding{
				Code:     transferCodeCertExpiring,
				Phrase:   fmt.Sprintf("expires in %dd", days),
				Detail:   fmt.Sprintf("Certificate %s expires in %d day(s).", certID, days),
				Severity: domain.SevWarn,
				Source:   "wave1",
			}, true
		}
	}
	return domain.Finding{}, false
}
