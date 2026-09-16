package unit

// The ACTOR section's Principal row navigates to the IAM principal the ARN
// names, for every ARN form CloudTrail writes into userIdentity.arn — a direct
// role ARN is the same principal as the assumed-role ARN of the same role, so
// it must be as navigable. The navigation id is the catalogued name, which is
// the last path segment: IAM role and user names cannot contain "/", so
// everything between the type marker and the last "/" is the IAM path, which
// the catalogue does not key on. The displayed Value stays the full ARN.

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/semantics/ctevent"
)

// ct0916PrincipalRow returns the ACTOR section's Principal row for an event
// whose userIdentity carries arn and identity type uiType.
func ct0916PrincipalRow(t *testing.T, uiType, arn string) ctevent.Row {
	t.Helper()
	event := &ctevent.Event{
		EventID:            "evt-ct0916-0001",
		EventName:          "DescribeInstances",
		EventSource:        "ec2.amazonaws.com",
		EventCategory:      "Management",
		EventType:          "AwsApiCall",
		AWSRegion:          "eu-west-1",
		SourceIPAddress:    "203.0.113.24",
		AccountID:          "123456789012",
		RecipientAccountID: "123456789012",
		UserIdentity: ctevent.UserIdentity{
			Type:        uiType,
			PrincipalID: "AROAEXAMPLEID:session1",
			ARN:         arn,
			AccountID:   "123456789012",
		},
	}
	for _, sec := range ctevent.BuildSections(event) {
		if sec.Name != ctevent.SectionActor {
			continue
		}
		for _, row := range sec.Rows {
			if row.Key == "Principal" {
				return row
			}
		}
		t.Fatalf("ACTOR section has no Principal row for arn %q; rows = %+v", arn, sec.Rows)
	}
	t.Fatalf("no ACTOR section for arn %q", arn)
	return ctevent.Row{}
}

func TestCT_0916_PrincipalRow_NavigatesForEveryIAMPrincipalARNForm(t *testing.T) {
	cases := []struct {
		name      string
		uiType    string
		arn       string
		wantNav   bool
		wantType  string
		wantNavID string
		wantValue string
	}{
		{
			name:      "direct role ARN",
			uiType:    "Role",
			arn:       "arn:aws:iam::123456789012:role/Deployer",
			wantNav:   true,
			wantType:  "role",
			wantNavID: "Deployer",
			wantValue: "arn:aws:iam::123456789012:role/Deployer",
		},
		{
			name:      "direct role ARN with an IAM path",
			uiType:    "Role",
			arn:       "arn:aws:iam::123456789012:role/service/ops/Deployer",
			wantNav:   true,
			wantType:  "role",
			wantNavID: "Deployer",
			wantValue: "arn:aws:iam::123456789012:role/service/ops/Deployer",
		},
		{
			name:      "assumed-role ARN",
			uiType:    "AssumedRole",
			arn:       "arn:aws:sts::123456789012:assumed-role/Deployer/session1",
			wantNav:   true,
			wantType:  "role",
			wantNavID: "Deployer",
			wantValue: "arn:aws:sts::123456789012:assumed-role/Deployer/session1",
		},
		{
			name:      "user ARN with an IAM path",
			uiType:    "IAMUser",
			arn:       "arn:aws:iam::123456789012:user/team/alice",
			wantNav:   true,
			wantType:  "iam-user",
			wantNavID: "alice",
			wantValue: "arn:aws:iam::123456789012:user/team/alice",
		},
		{
			name:      "user ARN without a path",
			uiType:    "IAMUser",
			arn:       "arn:aws:iam::123456789012:user/alice",
			wantNav:   true,
			wantType:  "iam-user",
			wantNavID: "alice",
			wantValue: "arn:aws:iam::123456789012:user/alice",
		},
		{
			// The account root is not a catalogued principal: nothing to open.
			name:      "root ARN",
			uiType:    "Root",
			arn:       "arn:aws:iam::123456789012:root",
			wantNav:   false,
			wantType:  "",
			wantNavID: "",
			wantValue: "arn:aws:iam::123456789012:root",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			row := ct0916PrincipalRow(t, tc.uiType, tc.arn)
			if row.IsNavigable != tc.wantNav {
				t.Errorf("Principal.IsNavigable for %q = %v, want %v", tc.arn, row.IsNavigable, tc.wantNav)
			}
			if row.TargetType != tc.wantType {
				t.Errorf("Principal.TargetType for %q = %q, want %q", tc.arn, row.TargetType, tc.wantType)
			}
			if row.NavID != tc.wantNavID {
				t.Errorf("Principal.NavID for %q = %q, want %q", tc.arn, row.NavID, tc.wantNavID)
			}
			if row.Value != tc.wantValue {
				t.Errorf("Principal.Value for %q = %q, want the full ARN %q", tc.arn, row.Value, tc.wantValue)
			}
		})
	}
}
