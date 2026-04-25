package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/acm"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("acm", []string{"domain_name", "status", "type", "not_after", "in_use", "days_left"})

	resource.RegisterPaginated("acm", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchACMCertificatesPage(ctx, c.ACM, continuationToken)
	})

	resource.RegisterRelated("acm", []resource.RelatedDef{
		{TargetType: "cf", DisplayName: "CloudFront Distros", Checker: checkACMCF, NeedsTargetCache: true},
		{TargetType: "elb", DisplayName: "Load Balancers", Checker: checkACMELB},
		{TargetType: "apigw", DisplayName: "API Gateways", Checker: checkACMAPIGW},
		{TargetType: "r53", DisplayName: "Route 53 Zones", Checker: checkACMR53},
	})
	// No NavigableFields — CertificateSummary has no forward refs to other resource types
}

// FetchACMCertificates calls the ACM ListCertificates API and converts the
// response into a slice of generic Resource structs.
func FetchACMCertificates(ctx context.Context, api ACMListCertificatesAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchACMCertificatesPage(ctx, api, token)
		if err != nil {
			return nil, err
		}
		all = append(all, result.Resources...)
		if result.Pagination == nil || !result.Pagination.IsTruncated {
			break
		}
		token = result.Pagination.NextToken
	}
	return all, nil
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

		r := resource.Resource{
			ID:     domainName,
			Name:   domainName,
			Status: status,
			Fields: map[string]string{
				"domain_name":     domainName,
				"certificate_arn": certARN,
				"status":          status,
				"type":            certType,
				"not_after":       notAfter,
				"in_use":          inUse,
				"days_left":       daysLeft,
			},
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
