// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/acm"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// acm canonical FindingCodes.
const (
	// acmCodeExpiresCritical — ISSUED cert with NotAfter - now() < 7 days
	// (including already expired). Read straight off ListCertificates'
	// CertificateSummary (NotAfter) — zero extra API calls.
	acmCodeExpiresCritical domain.FindingCode = "acm.expires-critical"
	// acmCodeExpiresSoon — ISSUED cert with 7d <= NotAfter - now() < 30d.
	acmCodeExpiresSoon domain.FindingCode = "acm.expires-soon"
	// acmCodeOrphan — ISSUED cert with InUse==false and NotAfter outside the
	// expiry windows above. Expiry takes priority over orphan.
	acmCodeOrphan domain.FindingCode = "acm.orphan"

	// acmCodeStatusPendingValidation — Status==PENDING_VALIDATION. The
	// certificate is awaiting DNS/email validation before ACM can issue it.
	acmCodeStatusPendingValidation domain.FindingCode = "acm.status.pending-validation"
	// acmCodeStatusFailed — Status is one of EXPIRED, REVOKED, FAILED, or
	// VALIDATION_TIMED_OUT. The certificate cannot be used for TLS.
	acmCodeStatusFailed domain.FindingCode = "acm.status.failed"
	// acmCodeStatusInactive — Status==INACTIVE. An imported certificate no
	// longer attached to any resource for TLS termination.
	acmCodeStatusInactive domain.FindingCode = "acm.status.inactive"

	// CodeACMWeakKey — KeyAlgorithm is RSA below 2048 bits. Elliptic-curve
	// keys and RSA 2048+ are fine.
	CodeACMWeakKey domain.FindingCode = "acm.weak-key"
)

// acmWeakKeyDetail is the S5 sentence for CodeACMWeakKey.
const acmWeakKeyDetail = "The certificate's key is short enough to be worth attacking, and browsers are withdrawing trust from keys this size. Reissue the certificate with a key of 2048 bits or more, or an elliptic-curve key."

// acmRSAMinimumBits is the shortest RSA key still considered sound.
const acmRSAMinimumBits = 2048

// acmKeyIsWeak reports whether the key algorithm is RSA below
// acmRSAMinimumBits. An empty algorithm is unresolved, not weak, and any
// elliptic-curve key AWS issues is sound.
func acmKeyIsWeak(alg string) bool {
	bits, ok := strings.CutPrefix(alg, "RSA_")
	if !ok {
		return false
	}
	n, err := strconv.Atoi(bits)
	return err == nil && n < acmRSAMinimumBits
}

// acmKeyAlgorithmWords renders the SDK enum as the words the console shows.
func acmKeyAlgorithmWords(alg string) string {
	return strings.ReplaceAll(alg, "_", " ")
}

// FetchACMCertificatesPage fetches a single page of ACM certificates.
func FetchACMCertificatesPage(ctx context.Context, api ACMListCertificatesAPI, continuationToken string) (resource.FetchResult, error) {
	input := &acm.ListCertificatesInput{
		MaxItems: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.ListCertificates(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching ACM certificates: %w", err)
	}

	var resources []resource.Resource
	now := time.Now()

	for _, cert := range output.CertificateSummaryList {
		domainName := ""
		if cert.DomainName != nil {
			domainName = *cert.DomainName
		}

		status := string(cert.Status)
		certType := string(cert.Type)

		notAfter := ""
		if cert.NotAfter != nil {
			notAfter = cert.NotAfter.Format("2006-01-02 15:04")
		}

		inUse := "false"
		if cert.InUse != nil && *cert.InUse {
			inUse = "true"
		}

		// Compute days_left until certificate expiry.
		// Format: "<N> days" for future expiry, "expired" for past expiry.
		// Check NotAfter against now directly so sub-day past expiry shows
		// "expired" rather than truncating to "0 days".
		daysLeft := ""
		if cert.NotAfter != nil {
			if !cert.NotAfter.After(now) {
				daysLeft = "expired"
			} else {
				d := int(cert.NotAfter.Sub(now).Hours() / 24)
				daysLeft = fmt.Sprintf("%d days", d)
			}
		}

		certARN := ""
		if cert.CertificateArn != nil {
			certARN = *cert.CertificateArn
		}

		// ID is the certificate ARN, not the domain name: two certs can share a
		// domain (an expired cert and its active renewal), and a domain-name ID
		// collides in the related-panel cache (keyed type:id) and detail nav —
		// the second same-domain cert would replay the first's related panel.
		// Name stays the domain for display. Fall back only if the ARN is absent.
		id := certARN
		if id == "" {
			id = domainName
		}

		r := resource.Resource{
			ID:   id,
			Name: domainName,
			Fields: map[string]string{
				"domain_name":     domainName,
				"certificate_arn": certARN,
				"status":          domain.HumanizeStatusPhrase(status),
				"type":            domain.HumanizeStatusPhrase(certType),
				"not_after":       notAfter,
				"in_use":          inUse,
				"days_left":       daysLeft,
			},
			// ISSUED certs get their expiry/orphan Wave 1 Finding
			// (acmIssuedFindings) read straight off this CertificateSummary;
			// every other status keeps its own status Finding
			// (acmStatusFindings), mirroring acmColor's own precedence.
			Findings:  acmFindings(status, cert.NotAfter, cert.InUse != nil && *cert.InUse, now),
			RawStruct: cert,
		}

		// Independent of status and expiry: a certificate can be issued, in
		// use, valid for a year, and still built on a key worth attacking.
		if alg := string(cert.KeyAlgorithm); acmKeyIsWeak(alg) {
			r.Findings = append(r.Findings, domain.Finding{
				Code: CodeACMWeakKey, Phrase: "weak key algorithm",
				Detail: acmWeakKeyDetail, Severity: domain.SevWarn, Source: "wave1",
			})
			addWave1Rows(&r, CodeACMWeakKey, domain.DetailRow{
				Label: "Key algorithm", Value: acmKeyAlgorithmWords(alg), Tier: "~",
			})
		}

		resources = append(resources, r)
	}

	nextToken := ""
	isTruncated := false
	if output.NextToken != nil {
		nextToken = *output.NextToken
		isTruncated = true
	}

	totalHint := len(resources)
	if isTruncated {
		totalHint = -1
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: isTruncated,
			NextToken:   nextToken,
			PageSize:    len(resources),
			TotalHint:   totalHint,
		},
	}, nil
}

// acmFindings routes a certificate to its Wave 1 Finding by status: ISSUED
// certs get the expiry/orphan check (acmIssuedFindings); every other status
// keeps its own status Finding (acmStatusFindings). acm has no Wave 2
// IssueEnricher — every signal it carries is readable straight off
// ListCertificates, zero extra API calls.
func acmFindings(status string, notAfter *time.Time, inUse bool, now time.Time) []domain.Finding {
	if status == "ISSUED" {
		return acmIssuedFindings(notAfter, inUse, now)
	}
	return acmStatusFindings(status)
}

// acmIssuedFindings returns the wave1 expiry/orphan Finding for an ISSUED
// certificate, read straight off ListCertificates' CertificateSummary
// (NotAfter, InUse). Expiry takes priority over orphan: an expiring orphan
// cert is still primarily an expiry problem.
func acmIssuedFindings(notAfter *time.Time, inUse bool, now time.Time) []domain.Finding {
	if notAfter != nil {
		remaining := notAfter.Sub(now)
		switch {
		case remaining < 7*24*time.Hour:
			phrase := "expired"
			if remaining >= 0 {
				phrase = fmt.Sprintf("expires in %d days", int(remaining.Hours()/24))
			}
			return []domain.Finding{{
				Code: acmCodeExpiresCritical, Phrase: phrase,
				Severity: domain.SevBroken, Source: "wave1",
			}}
		case remaining < 30*24*time.Hour:
			return []domain.Finding{{
				Code: acmCodeExpiresSoon, Phrase: fmt.Sprintf("expires in %d days", int(remaining.Hours()/24)),
				Severity: domain.SevWarn, Source: "wave1",
			}}
		}
	}
	if !inUse {
		return []domain.Finding{{
			Code: acmCodeOrphan, Phrase: "certificate not in use (orphan)",
			Severity: domain.SevWarn, Source: "wave1",
		}}
	}
	return nil
}

// acmStatusFindings returns the wave1 Finding for a non-ISSUED certificate
// status, mirroring acmColor's (catalog_color_helpers.go) own precedence so
// the Findings list and the row color never disagree. ISSUED certs are
// routed to acmIssuedFindings by acmFindings instead.
func acmStatusFindings(status string) []domain.Finding {
	switch status {
	case "PENDING_VALIDATION":
		return []domain.Finding{{
			Code: acmCodeStatusPendingValidation, Phrase: "pending validation",
			Severity: domain.SevWarn, Source: "wave1",
		}}
	case "EXPIRED", "REVOKED", "FAILED", "VALIDATION_TIMED_OUT":
		return []domain.Finding{{
			Code: acmCodeStatusFailed, Phrase: strings.ToLower(status),
			Severity: domain.SevBroken, Source: "wave1",
		}}
	case "INACTIVE":
		return []domain.Finding{{
			Code: acmCodeStatusInactive, Phrase: "inactive",
			Severity: domain.SevDim, Source: "wave1",
		}}
	}
	return nil
}
