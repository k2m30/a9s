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
	acmtypes "github.com/aws/aws-sdk-go-v2/service/acm/types"

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

	acmCodeStatusPendingValidation domain.FindingCode = "acm.status.pending-validation"
	acmCodeStatusFailed            domain.FindingCode = "acm.status.failed"
	acmCodeStatusInactive          domain.FindingCode = "acm.status.inactive"
	acmCodeStatusUnrecognised      domain.FindingCode = "acm.status.unrecognised"

	// CodeACMWeakKey — KeyAlgorithm is RSA below 2048 bits. Elliptic-curve
	// keys and RSA 2048+ are fine.
	CodeACMWeakKey domain.FindingCode = "acm.weak-key"
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
		// Unfiltered, ListCertificates returns only RSA_1024 and RSA_2048
		// certificates, so RSA-3072/4096 and every EC certificate would be
		// missing from the list entirely.
		Includes: &acmtypes.Filters{KeyTypes: acmtypes.KeyAlgorithm("").Values()},
		// Unfiltered, it also leaves out every ACME-issued certificate.
		CertificateKeyPairOrigins: acmtypes.CertificateKeyPairOrigin("").Values(),
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

		// NotAfter is compared against now directly so a sub-day past expiry reads
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

		keyAlgorithm := acmKeyAlgorithmWords(string(cert.KeyAlgorithm))

		r := withWave1(resource.Resource{
			ID:   id,
			Name: domainName,
			Fields: map[string]string{
				"domain_name":     domainName,
				"certificate_arn": certARN,
				"status":          status,
				"type":            domain.HumanizeStatusPhrase(certType),
				"not_after":       notAfter,
				"in_use":          inUse,
				"days_left":       daysLeft,
				"key_algorithm":   keyAlgorithm,
			},
			RawStruct: cert,
		}, acmFindings(status, notAfter, inUse, keyAlgorithm, now)...)

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

// acmFindings is the one predicate for a certificate. The expiry/orphan
// check is declared for an issued cert; every other status carries its own
// status finding. The key algorithm is independent of both — a cert can be
// issued, in use, valid for a year, and still built on a key worth attacking.
func acmFindings(status, notAfter, inUse, keyAlgorithmWords string, now time.Time) []domain.Finding {
	posture := acmIssuedFindings(notAfter, inUse, now)
	if acmKeyIsWeak(keyAlgorithmWords) {
		posture = append(posture, wave1Finding(CodeACMWeakKey))
	}
	return withState("acm", status, posture, strings.ToLower(domain.HumanizeStatusPhrase(status)))
}

// acmIssuedFindings returns the wave1 expiry/orphan Finding of a
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
