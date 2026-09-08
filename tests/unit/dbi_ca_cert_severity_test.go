// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit

// dbi_ca_cert_severity_test.go — a finding's severity belongs to its code.
//
// The CA-certificate row was one code emitted at two severities: warning
// beyond 30 days, broken inside it. The catalog declares one severity per
// code, so half the emissions contradicted the declaration — and the demo
// fixtures never reach the inner window, so the standing severity gate had
// nothing to compare. This drives a certificate through the real fetcher at
// each side of the boundary and reads the severity back off the row and its
// detail sentence.

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
)

func TestDBICACertSeverityIsTheCodes(t *testing.T) {
	cases := []struct {
		name string
		days int
		code string
	}{
		{"inside the urgent window", 15, "dbi.ca-cert-expiring-urgent"},
		{"on the urgent boundary", 30, "dbi.ca-cert-expiring-urgent"},
		{"one day outside it", 31, "dbi.ca-cert-expiring"},
		{"on the reporting boundary", 90, "dbi.ca-cert-expiring"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := w2DBIInstance("acme-orders-db")
			db.CertificateDetails = &rdstypes.CertificateDetails{
				CAIdentifier: aws.String("rds-ca-2019"),
				ValidTill:    aws.Time(w2DBINow.AddDate(0, 0, tc.days)),
			}

			got := w2DBIFetch(t, w2DBINow, db)["acme-orders-db"]
			f, ok := w2Find(got.Findings, tc.code)
			if !ok {
				t.Fatalf("no finding with code %q at %d days; got %s", tc.code, tc.days, w2Codes(got.Findings))
			}

			def := findingDefFor(t, domain.FindingCode(tc.code))
			if f.Severity != def.Severity {
				t.Errorf("%s: emitted severity %v, catalog declares %v — the severity is the code's, "+
					"not the call site's", tc.code, f.Severity, def.Severity)
			}
			if f.Detail != def.Detail {
				t.Errorf("%s: emitted detail %q, catalog declares %q", tc.code, f.Detail, def.Detail)
			}
			// Only one of the two codes may speak for one certificate.
			other := "dbi.ca-cert-expiring"
			if tc.code == other {
				other = "dbi.ca-cert-expiring-urgent"
			}
			w2AssertNoCode(t, got.Findings, other)
		})
	}
}

// findingDefFor returns the installed declaration for code.
func findingDefFor(t *testing.T, code domain.FindingCode) catalog.FindingDef {
	t.Helper()
	for _, td := range append(catalog.All(), catalog.AllChildren()...) {
		for _, f := range td.Findings {
			if f.Code == code {
				return f
			}
		}
	}
	t.Fatalf("finding code %q is not declared on any catalog entry", code)
	return catalog.FindingDef{}
}
