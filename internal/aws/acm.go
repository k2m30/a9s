package aws

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/acm"

	"github.com/k2m30/a9s/internal/resource"
)

func init() {
	resource.Register("acm", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchACMCertificates(ctx, c.ACM)
	})
}

// FetchACMCertificates calls the ACM ListCertificates API and converts the
// response into a slice of generic Resource structs.
func FetchACMCertificates(ctx context.Context, api ACMListCertificatesAPI) ([]resource.Resource, error) {
	output, err := api.ListCertificates(ctx, &acm.ListCertificatesInput{})
	if err != nil {
		return nil, err
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
			notAfter = cert.NotAfter.Format("2006-01-02T15:04:05Z07:00")
		}

		inUse := "false"
		if cert.InUse != nil && *cert.InUse {
			inUse = "true"
		}

		detail := map[string]string{
			"Domain Name": domainName,
			"Status":      status,
			"Type":        certType,
			"Not After":   notAfter,
			"In Use":      inUse,
		}

		if cert.CertificateArn != nil {
			detail["ARN"] = *cert.CertificateArn
		}

		if cert.NotBefore != nil {
			detail["Not Before"] = cert.NotBefore.Format("2006-01-02T15:04:05Z07:00")
		}

		if cert.CreatedAt != nil {
			detail["Created At"] = cert.CreatedAt.Format("2006-01-02T15:04:05Z07:00")
		}

		if cert.RenewalEligibility != "" {
			detail["Renewal Eligibility"] = string(cert.RenewalEligibility)
		}

		detail["Key Algorithm"] = string(cert.KeyAlgorithm)

		rawJSON := ""
		if jsonBytes, err := json.MarshalIndent(cert, "", "  "); err == nil {
			rawJSON = string(jsonBytes)
		}

		r := resource.Resource{
			ID:     domainName,
			Name:   domainName,
			Status: status,
			Fields: map[string]string{
				"domain_name": domainName,
				"status":      status,
				"type":        certType,
				"not_after":   notAfter,
				"in_use":      inUse,
			},
			DetailData: detail,
			RawJSON:    rawJSON,
			RawStruct:  cert,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
