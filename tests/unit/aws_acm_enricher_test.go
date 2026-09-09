package unit

// aws_acm_enricher_test.go — Structural contract test for acm's Wave 2
// registration.
//
// docs/attention-signals.md places acm's expiry/orphan signals (NotAfter <
// 30d/7d, InUse==false on a non-expired cert) in Wave 1: they are readable
// straight off ListCertificates' CertificateSummary (NotAfter, InUse) with
// zero extra API calls, so per-resource Describe* (Wave 2, by definition) is
// the wrong place for them. acm's ONLY spec'd Wave 2 signals
// (RenewalSummary.RenewalStatus, DomainValidationOptions[].ValidationStatus)
// are explicitly marked NOT IMPLEMENTED (backlog) in the golden contract.
//
// Consequently the acm catalog entry must carry NO Wave 2 IssueEnricher at
// all. The behavioral expiry/orphan tests are in aws_acm_test.go (Wave 1,
// driven off FetchACMCertificatesPage with an ACMListCertificatesAPI fake
// only — no DescribeCertificate fake needed).

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/catalog"
)

// TestACMCatalog_HasNoWave2IssueEnricher pins that the acm catalog entry
// carries no Wave 2 registration once its only spec'd Wave 2 signals are
// backlog/NOT IMPLEMENTED and its expiry/orphan checks live in Wave 1.
func TestACMCatalog_HasNoWave2IssueEnricher(t *testing.T) {
	td := catalog.FindAny("acm")
	if td == nil {
		t.Fatal(`catalog.FindAny("acm") returned nil`)
	}
	if td.Wave2 != nil {
		t.Errorf(`catalog.FindAny("acm").Wave2 = %v, want nil — acm has no spec'd Wave 2 signal (docs/attention-signals.md "acm" row: Wave 2 column is RenewalSummary/DomainValidationOptions checks marked NOT IMPLEMENTED)`, td.Wave2)
	}
}
