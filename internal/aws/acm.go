package aws

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/acm"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

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
			now := time.Now()
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
			// emit canonical Findings for every non-ISSUED status branch
			// acmColor reads, mirroring acmColor's own precedence — ISSUED
			// certs are covered by EnrichACMCertificate's expiry/orphan Wave-2
			// findings instead, so their status never reaches this switch.
			Findings:  acmStatusFindings(status),
			RawStruct: cert,
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

// acmStatusFindings returns the wave1 Finding for a non-ISSUED certificate
// status, mirroring acmColor's (catalog_color_helpers.go) own precedence so
// the Findings list and the row color never disagree. ISSUED certs return no
// finding here — their color (and any "in use" / expiry signal) comes from
// EnrichACMCertificate's Wave-2 findings instead.
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
