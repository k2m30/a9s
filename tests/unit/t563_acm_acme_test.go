package unit

import (
	"context"
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	acmtypes "github.com/aws/aws-sdk-go-v2/service/acm/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// acmFilteringAPI answers ListCertificates the way ACM does: without
// Includes.KeyTypes only RSA_1024/RSA_2048 certificates come back, and
// without CertificateKeyPairOrigins only AWS_MANAGED and CUSTOMER_PROVIDED
// ones — an ACME-issued certificate is left out. Matches are served one per
// page so the filters must ride on every page request.
type acmFilteringAPI struct {
	certs  []acmtypes.CertificateSummary
	inputs []*acm.ListCertificatesInput
}

func (f *acmFilteringAPI) ListCertificates(_ context.Context, in *acm.ListCertificatesInput, _ ...func(*acm.Options)) (*acm.ListCertificatesOutput, error) {
	f.inputs = append(f.inputs, in)
	keyTypes := []acmtypes.KeyAlgorithm{acmtypes.KeyAlgorithmRsa1024, acmtypes.KeyAlgorithmRsa2048}
	if in.Includes != nil && len(in.Includes.KeyTypes) > 0 {
		keyTypes = in.Includes.KeyTypes
	}
	origins := []acmtypes.CertificateKeyPairOrigin{acmtypes.CertificateKeyPairOriginAwsManaged, acmtypes.CertificateKeyPairOriginCustomerProvided}
	if len(in.CertificateKeyPairOrigins) > 0 {
		origins = in.CertificateKeyPairOrigins
	}
	var match []acmtypes.CertificateSummary
	for _, c := range f.certs {
		if slices.Contains(keyTypes, c.KeyAlgorithm) && slices.Contains(origins, c.CertificateKeyPairOrigin) {
			match = append(match, c)
		}
	}
	page := 0
	if in.NextToken != nil {
		page = int(aws.ToString(in.NextToken)[0] - '0')
	}
	if page >= len(match) {
		return &acm.ListCertificatesOutput{}, nil
	}
	out := &acm.ListCertificatesOutput{CertificateSummaryList: match[page : page+1]}
	if page+1 < len(match) {
		out.NextToken = aws.String(string(rune('0' + page + 1)))
	}
	return out, nil
}

const (
	t563ACMEArn     = "arn:aws:acm:us-east-1:123456789012:certificate/a1a1a1a1-1111-4111-8111-acmeacmeacme"
	t563AmazonArn   = "arn:aws:acm:us-east-1:123456789012:certificate/b2b2b2b2-2222-4222-8222-amazonamazon"
	t563ImportedArn = "arn:aws:acm:us-east-1:123456789012:certificate/c3c3c3c3-3333-4333-8333-importimport"
)

func t563ACMAccount(now time.Time) []acmtypes.CertificateSummary {
	year := now.Add(300 * 24 * time.Hour)
	soon := now.Add(20 * 24 * time.Hour)
	return []acmtypes.CertificateSummary{
		{
			CertificateArn:           aws.String(t563AmazonArn),
			DomainName:               aws.String("www.example.com"),
			Status:                   acmtypes.CertificateStatusIssued,
			Type:                     acmtypes.CertificateTypeAmazonIssued,
			KeyAlgorithm:             acmtypes.KeyAlgorithmRsa2048,
			CertificateKeyPairOrigin: acmtypes.CertificateKeyPairOriginAwsManaged,
			NotAfter:                 &year,
			InUse:                    aws.Bool(true),
		},
		{
			CertificateArn:           aws.String(t563ImportedArn),
			DomainName:               aws.String("legacy.example.com"),
			Status:                   acmtypes.CertificateStatusIssued,
			Type:                     acmtypes.CertificateTypeImported,
			KeyAlgorithm:             acmtypes.KeyAlgorithmEcPrime256v1,
			CertificateKeyPairOrigin: acmtypes.CertificateKeyPairOriginCustomerProvided,
			NotAfter:                 &year,
			InUse:                    aws.Bool(true),
		},
		{
			CertificateArn:           aws.String(t563ACMEArn),
			DomainName:               aws.String("acme.example.com"),
			Status:                   acmtypes.CertificateStatusIssued,
			Type:                     acmtypes.CertificateTypeAmazonIssued,
			KeyAlgorithm:             acmtypes.KeyAlgorithmEcPrime256v1,
			CertificateKeyPairOrigin: acmtypes.CertificateKeyPairOriginAcme,
			NotAfter:                 &soon,
			InUse:                    aws.Bool(true),
		},
	}
}

// An ACME-issued certificate is excluded by ListCertificates unless the
// request names ACME among CertificateKeyPairOrigins, so the list must ask for
// every key-pair origin as well as every key type to show the whole account.
func TestT563_ACMListIncludesACMEIssuedCertificateWithItsExpirySignal(t *testing.T) {
	api := &acmFilteringAPI{certs: t563ACMAccount(time.Now())}

	rows, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchACMCertificatesPage(context.Background(), api, token)
	})
	if err != nil {
		t.Fatalf("FetchACMCertificatesPage: %v", err)
	}

	byID := map[string]resource.Resource{}
	for _, r := range rows {
		byID[r.ID] = r
	}
	if len(byID) != 3 {
		t.Errorf("listed %d certificates, want 3 (AWS-issued, imported, ACME); ids=%v", len(byID), slices.Collect(maps.Keys(byID)))
	}

	acme, ok := byID[t563ACMEArn]
	if !ok {
		t.Fatalf("the ACME-issued certificate is missing from the list")
	}
	if len(acme.Findings) != 1 || acme.Findings[0].Code != domain.FindingCode("acm.expires-soon") {
		t.Errorf("ACME certificate expiring in 20 days: findings = %+v, want exactly acm.expires-soon", acme.Findings)
	}
	for _, id := range []string{t563AmazonArn, t563ImportedArn} {
		if r, ok := byID[id]; ok && len(r.Findings) != 0 {
			t.Errorf("%s is issued, in use and valid for 300 days: findings = %+v, want none", id, r.Findings)
		}
	}

	wantOrigins := acmtypes.CertificateKeyPairOrigin("").Values()
	wantKeyTypes := acmtypes.KeyAlgorithm("").Values()
	for i, in := range api.inputs {
		if !t563SameElements(in.CertificateKeyPairOrigins, wantOrigins) {
			t.Errorf("request %d CertificateKeyPairOrigins = %v, want every SDK value %v", i, in.CertificateKeyPairOrigins, wantOrigins)
		}
		if in.Includes == nil || !t563SameElements(in.Includes.KeyTypes, wantKeyTypes) {
			t.Errorf("request %d Includes = %+v, want KeyTypes = every SDK value %v", i, in.Includes, wantKeyTypes)
		}
	}
}

func t563SameElements[T comparable](got, want []T) bool {
	if len(got) != len(want) {
		return false
	}
	for _, w := range want {
		if !slices.Contains(got, w) {
			return false
		}
	}
	return true
}
