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
	// acmCodeExpired — ISSUED cert whose NotAfter has already passed. Its own
	// code because "expired" and "expires in 3 days" are different things to
	// do, and one code cannot carry both wordings.
	acmCodeExpired domain.FindingCode = "acm.expired"
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

// The certificate statuses acmStatusFindings reports on, in the words
// HumanizeStatusPhrase renders and the fetcher writes into Fields. A status
// outside this vocabulary is reported on like an absent one: nothing.
const (
	acmStatusIssued             = "issued"
	acmStatusPendingValidation  = "pending validation"
	acmStatusExpired            = "expired"
	acmStatusRevoked            = "revoked"
	acmStatusFailed             = "failed"
	acmStatusValidationTimedOut = "validation timed out"
	acmStatusInactive           = "inactive"
)

// acmRSAMinimumBits is the shortest RSA key still considered sound.
const acmRSAMinimumBits = 2048

// acmKeyIsWeak reports whether the key algorithm is RSA below
// acmRSAMinimumBits. It takes the words acmKeyAlgorithmWords renders, which is
// what the fetcher writes into Fields, so the predicate and the row read one
// value. An empty algorithm is unresolved, not weak, and any elliptic-curve
// key AWS issues is sound.
func acmKeyIsWeak(algWords string) bool {
	bits, ok := strings.CutPrefix(algWords, "RSA ")
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

		statusWords := domain.HumanizeStatusPhrase(status)
		keyAlgorithm := acmKeyAlgorithmWords(string(cert.KeyAlgorithm))

		r := resource.Resource{
			ID:   id,
			Name: domainName,
			Fields: map[string]string{
				"domain_name":     domainName,
				"certificate_arn": certARN,
				"status":          statusWords,
				"type":            domain.HumanizeStatusPhrase(certType),
				"not_after":       notAfter,
				"in_use":          inUse,
				"days_left":       daysLeft,
				"key_algorithm":   keyAlgorithm,
			},
			Findings:  acmFindings(statusWords, notAfter, inUse, keyAlgorithm, now),
			RawStruct: cert,
		}

		if acmKeyIsWeak(keyAlgorithm) {
			addWave1Rows(&r, CodeACMWeakKey, domain.DetailRow{
				Label: "Key algorithm", Value: keyAlgorithm, Tier: "~",
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

// acmFindings is the one predicate for a certificate. An issued cert gets the
// expiry/orphan check; every other status carries its own status finding. The
// key algorithm is independent of both — a cert can be issued, in use, valid
// for a year, and still built on a key worth attacking.
//
// Every argument is a value the fetcher writes into Fields, so a row rebuilt
// from Fields alone reaches the same verdict. acm has no Wave 2 IssueEnricher:
// every signal it carries is readable straight off ListCertificates, zero
// extra API calls.
func acmFindings(statusWords, notAfter, inUse, keyAlgorithmWords string, now time.Time) []domain.Finding {
	var out []domain.Finding
	if statusWords == acmStatusIssued {
		out = acmIssuedFindings(notAfter, inUse, now)
	} else {
		out = acmStatusFindings(statusWords)
	}
	if acmKeyIsWeak(keyAlgorithmWords) {
		out = append(out, wave1Finding(CodeACMWeakKey))
	}
	return out
}

// acmIssuedFindings returns the wave1 expiry/orphan Finding for an ISSUED
// certificate, read straight off ListCertificates' CertificateSummary
// (NotAfter, InUse). Expiry takes priority over orphan: an expiring orphan
// cert is still primarily an expiry problem.
func acmIssuedFindings(notAfter, inUse string, now time.Time) []domain.Finding {
	if t, err := time.Parse("2006-01-02 15:04", notAfter); err == nil {
		remaining := t.Sub(now)
		switch {
		case remaining < 7*24*time.Hour:
			if remaining < 0 {
				return []domain.Finding{wave1Finding(acmCodeExpired)}
			}
			return []domain.Finding{wave1Finding(acmCodeExpiresCritical, strconv.Itoa(int(remaining.Hours()/24)))}
		case remaining < 30*24*time.Hour:
			return []domain.Finding{wave1Finding(acmCodeExpiresSoon, strconv.Itoa(int(remaining.Hours()/24)))}
		}
	}
	if inUse == "false" {
		return []domain.Finding{wave1Finding(acmCodeOrphan)}
	}
	return nil
}

// acmStatusFindings returns the wave1 Finding for a non-ISSUED certificate
// status, mirroring acmColor's (catalog_color_helpers.go) own precedence so
// the Findings list and the row color never disagree. ISSUED certs are
// routed to acmIssuedFindings by acmFindings instead.
func acmStatusFindings(statusWords string) []domain.Finding {
	switch statusWords {
	case acmStatusPendingValidation:
		return []domain.Finding{wave1Finding(acmCodeStatusPendingValidation)}
	case acmStatusExpired, acmStatusRevoked, acmStatusFailed, acmStatusValidationTimedOut:
		return []domain.Finding{wave1Finding(acmCodeStatusFailed, statusWords)}
	case acmStatusInactive:
		return []domain.Finding{wave1Finding(acmCodeStatusInactive)}
	}
	return nil
}
