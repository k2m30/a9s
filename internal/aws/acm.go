package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/acm"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.Register("acm", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchACMCertificates(ctx, c.ACM)
	})
	resource.RegisterFieldKeys("acm", []string{"domain_name", "status", "type", "not_after", "in_use"})
}

// FetchACMCertificates calls the ACM ListCertificates API and converts the
// response into a slice of generic Resource structs.
func FetchACMCertificates(ctx context.Context, api ACMListCertificatesAPI) ([]resource.Resource, error) {
	output, err := api.ListCertificates(ctx, &acm.ListCertificatesInput{})
	if err != nil {
		return nil, fmt.Errorf("fetching ACM certificates: %w", err)
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
			RawStruct:  cert,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
