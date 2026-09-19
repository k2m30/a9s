package unit

// acm's expiry/orphan signals (NotAfter < 30d/7d, InUse==false on a
// non-expired cert) are readable straight off ListCertificates'
// CertificateSummary with zero extra API calls, so they are Wave 1 and the acm
// catalog entry carries no Wave 2 IssueEnricher.

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/catalog"
)

func TestACMCatalog_HasNoWave2IssueEnricher(t *testing.T) {
	td := catalog.FindAny("acm")
	if td == nil {
		t.Fatal(`catalog.FindAny("acm") returned nil`)
	}
	if td.Wave2 != nil {
		t.Errorf(`catalog.FindAny("acm").Wave2 = %v, want nil — acm has no spec'd Wave 2 signal (docs/attention-signals.md "acm" row: Wave 2 column is RenewalSummary/DomainValidationOptions checks marked NOT IMPLEMENTED)`, td.Wave2)
	}
}
