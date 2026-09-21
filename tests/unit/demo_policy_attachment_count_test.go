// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
)

// A policy's attachment count and the principals its detail view lists are one
// fact. A count stated on the policy and a principal list built from the
// attachments are two, and the row then advertises attachments the drill into
// it cannot show.
func TestDemoPolicyAttachmentCountMatchesItsPrincipals(t *testing.T) {
	f := fixtures.NewIAMFixtures()
	if len(f.Policies) == 0 {
		t.Fatal("no policy fixtures")
	}

	orphans := 0
	for _, p := range f.Policies {
		arn := aws.ToString(p.Arn)
		principals := 0
		if e := f.EntitiesForPolicy[arn]; e != nil {
			principals = len(e.Roles) + len(e.Users) + len(e.Groups)
		}
		if got := int(aws.ToInt32(p.AttachmentCount)); got != principals {
			t.Errorf("%s: AttachmentCount=%d, but %d principals attach it",
				aws.ToString(p.PolicyName), got, principals)
		}
		if principals == 0 && p.IsAttachable {
			orphans++
			if name := aws.ToString(p.PolicyName); name != "orphan-unattached-policy" {
				t.Errorf("%s is attachable and nothing attaches it, so it carries "+
					"iam-policy.orphan-unattached beside the row that exists to carry it", name)
			}
		}
	}
	if orphans != 1 {
		t.Errorf("%d demo policies raise iam-policy.orphan-unattached, want exactly the one witness", orphans)
	}
}
