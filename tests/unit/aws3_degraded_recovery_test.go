// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit

// The finding a Fields-only row recovers is the one whose CODE the row
// carries, never the one whose rendered status text matches a phrase constant.

import (
	"errors"
	"testing"

	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// A recovery keyed on the status phrase would retire silently once the wording
// changed; the phrase is not the key.
func TestDegradedRecoveryReadsTheCodeNotTheStatusText(t *testing.T) {
	denied := &smithy.GenericAPIError{Code: "AccessDenied", Message: "not authorized"}
	for _, tc := range []struct {
		name  string
		short string
		err   error
		code  domain.FindingCode
	}{
		{"ng denied", "ng", denied, awsclient.DetailsDeniedCode("ng")},
		{"ng unavailable", "ng", errors.New("boom"), awsclient.DetailsUnavailableCode("ng")},
		{"eks denied", "eks", denied, awsclient.DetailsDeniedCode("eks")},
		{"eks unavailable", "eks", errors.New("boom"), awsclient.DetailsUnavailableCode("eks")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			built := awsclient.DegradedDetails(tc.short, tc.short+"-1", tc.err)
			if len(built.Findings) != 1 || built.Findings[0].Code != tc.code {
				t.Fatalf("DegradedDetails findings = %+v, want one %s", built.Findings, tc.code)
			}

			// The row as a cache read or a colour fallback sees it: Fields only, no
			// Findings, and a reworded status cell.
			rebuilt := domain.Resource{ID: built.ID, Name: built.Name, Fields: map[string]string{}}
			for k, v := range built.Fields {
				rebuilt.Fields[k] = v
			}
			rebuilt.Fields["status"] = "access to the details was refused"

			td := resource.FindResourceType(tc.short)
			if td == nil {
				t.Fatalf("%s type not registered", tc.short)
			}
			if got := td.Color(rebuilt); got != resource.ColorWarning {
				t.Errorf("Color(degraded row, status reworded) = %v, want %v — the recovery must read the finding code the row carries",
					got, resource.ColorWarning)
			}
		})
	}
}
