package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/acm"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
)

const (
	acmeCertARN = "arn:aws:acm:eu-west-1:123456789012:certificate/a1a1a1a1-1111-4111-8111-acmeacmeacme"
	ecCertARN   = "arn:aws:acm:eu-west-1:123456789012:certificate/b2b2b2b2-2222-4222-8222-ecdsaecdsaec"
)

// acmFilterTransport answers ListCertificates the way ACM does — an EC
// certificate only when Includes.keyTypes names its algorithm, an ACME-issued
// one only when CertificateKeyPairOrigins names ACME — and records each
// ListCertificates request's filters.
type acmFilterTransport struct {
	keyTypes [][]string
	origins  [][]string
}

func (tr *acmFilterTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	body, _ := io.ReadAll(req.Body) //nolint:errcheck // an unreadable body decodes as an empty request
	out := map[string]any{}
	if strings.HasSuffix(req.Header.Get("X-Amz-Target"), "ListCertificates") {
		var in struct {
			Includes struct {
				KeyTypes []string
			}
			CertificateKeyPairOrigins []string
		}
		_ = json.Unmarshal(body, &in) //nolint:errcheck // an empty body carries no filters
		tr.keyTypes = append(tr.keyTypes, in.Includes.KeyTypes)
		tr.origins = append(tr.origins, in.CertificateKeyPairOrigins)

		var list []map[string]any
		if slices.Contains(in.Includes.KeyTypes, "EC_prime256v1") {
			list = append(list, map[string]any{"CertificateArn": ecCertARN, "DomainName": "api.example.com", "Status": "ISSUED", "KeyAlgorithm": "EC_prime256v1", "CertificateKeyPairOrigin": "AWS_MANAGED"})
			if slices.Contains(in.CertificateKeyPairOrigins, "ACME") {
				list = append(list, map[string]any{"CertificateArn": acmeCertARN, "DomainName": "acme.example.com", "Status": "ISSUED", "KeyAlgorithm": "EC_prime256v1", "CertificateKeyPairOrigin": "ACME"})
			}
		}
		out["CertificateSummaryList"] = list
	}
	b, _ := json.Marshal(out) //nolint:errcheck // test-built maps always marshal
	return &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": []string{"application/x-amz-json-1.1"}},
		Body:       io.NopCloser(strings.NewReader(string(b))),
	}, nil
}

type acmInputRecorder struct{ in *acm.ListCertificatesInput }

func (r *acmInputRecorder) ListCertificates(_ context.Context, in *acm.ListCertificatesInput, _ ...func(*acm.Options)) (*acm.ListCertificatesOutput, error) {
	r.in = in
	return &acm.ListCertificatesOutput{}, nil
}

// The oracle is only a check on the app if it sees the certificates the app
// lists, so the collector's ListCertificates carries the same key-type and
// key-pair-origin filters as the ACM fetcher.
func TestCaptureACM_SendsTheFetchersKeyTypeAndKeyPairOriginFilters(t *testing.T) {
	rec := &acmInputRecorder{}
	if _, err := awsclient.FetchACMCertificatesPage(context.Background(), rec, ""); err != nil {
		t.Fatalf("FetchACMCertificatesPage: %v", err)
	}
	var wantKeyTypes, wantOrigins []string
	if rec.in.Includes != nil {
		for _, k := range rec.in.Includes.KeyTypes {
			wantKeyTypes = append(wantKeyTypes, string(k))
		}
	}
	for _, o := range rec.in.CertificateKeyPairOrigins {
		wantOrigins = append(wantOrigins, string(o))
	}
	slices.Sort(wantKeyTypes)
	slices.Sort(wantOrigins)

	tr := &acmFilterTransport{}
	cfg := aws.Config{
		Region:           "eu-west-1",
		Credentials:      credentials.NewStaticCredentialsProvider("AKIAIOSFODNN7EXAMPLE", "EXAMPLESECRETKEY", ""),
		HTTPClient:       &http.Client{Transport: tr},
		RetryMaxAttempts: 1,
	}
	got, err := captureACM(context.Background(), cfg)
	if err != nil {
		t.Fatalf("captureACM: %v", err)
	}
	data, ok := got.(acmData)
	if !ok {
		t.Fatalf("captureACM returned %T, want acmData", got)
	}

	if len(tr.keyTypes) == 0 {
		t.Fatal("captureACM sent no ListCertificates request")
	}
	for i := range tr.keyTypes {
		keyTypes, origins := slices.Sorted(slices.Values(tr.keyTypes[i])), slices.Sorted(slices.Values(tr.origins[i]))
		if !slices.Equal(keyTypes, wantKeyTypes) {
			t.Errorf("request %d Includes.keyTypes = %v, want the fetcher's %v", i, keyTypes, wantKeyTypes)
		}
		if !slices.Equal(origins, wantOrigins) {
			t.Errorf("request %d CertificateKeyPairOrigins = %v, want the fetcher's %v", i, origins, wantOrigins)
		}
	}

	var arns []string
	for _, c := range data.Certificates {
		arns = append(arns, c.CertificateArn)
	}
	slices.Sort(arns)
	if want := []string{acmeCertARN, ecCertARN}; !slices.Equal(arns, want) {
		t.Errorf("captured certificates %v, want the EC and the ACME-issued one %v", arns, want)
	}
}
