package unit

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/semantics/ctevent"
)

// sessionIssuer is a helper to build a *ctevent.SessionContext with a SessionIssuer.
func sessionIssuer(issuerUserName string) *ctevent.SessionContext {
	return &ctevent.SessionContext{
		SessionIssuer: &ctevent.SessionIssuer{
			UserName: issuerUserName,
		},
	}
}

// sessionIssuerWithIdentity builds a *ctevent.SessionContext with both
// a SessionIssuer and a SourceIdentity (used for IdentityCenter/SSO).
func sessionIssuerWithIdentity(issuerUserName, sourceIdentity string) *ctevent.SessionContext {
	return &ctevent.SessionContext{
		SessionIssuer: &ctevent.SessionIssuer{
			UserName: issuerUserName,
		},
		SourceIdentity: sourceIdentity,
	}
}

// sessionWithWebID builds a *ctevent.SessionContext with WebIDFederationData (used for IRSA).
func sessionWithWebID(federatedProvider string) *ctevent.SessionContext {
	return &ctevent.SessionContext{
		WebIDFederationData: &ctevent.WebIDFederationData{
			FederatedProvider: federatedProvider,
		},
	}
}

// eventWithIdentity is a convenience constructor: all other Event fields are zero values.
func eventWithIdentity(ui ctevent.UserIdentity) *ctevent.Event {
	return &ctevent.Event{UserIdentity: ui}
}

// TestCTDetailActor covers all 12 userIdentity variants (taxonomy §4) plus edge cases.
// Actor() must never return an empty string — that is the primary contract under test.
func TestCTDetailActor(t *testing.T) {
	type tc struct {
		name     string
		identity ctevent.UserIdentity
		// mustContain lists substrings that MUST appear in the result.
		mustContain []string
	}

	cases := []tc{
		// Case 1 — IAMUser
		// ARN format: arn:aws:iam::<account>:user/<username>
		{
			name: "IAMUser",
			identity: ctevent.UserIdentity{
				Type:     "IAMUser",
				ARN:      "arn:aws:iam::333333333333:user/bob",
				UserName: "bob",
			},
			mustContain: []string{"bob"},
		},

		// Case 2 — AssumedRole (Karpenter node role)
		// ARN format: arn:aws:sts::<account>:assumed-role/<role-name>/<session-label>
		{
			name: "AssumedRole",
			identity: ctevent.UserIdentity{
				Type:           "AssumedRole",
				ARN:            "arn:aws:sts::111111111111:assumed-role/KarpenterNodeRole/karpenter-1759",
				SessionContext: sessionIssuer("KarpenterNodeRole"),
			},
			mustContain: []string{"KarpenterNodeRole", "karpenter-1759"},
		},

		// Case 3 — IdentityCenterUser (SSO)
		// SourceIdentity is the human email; the role name encodes the permission set.
		{
			name: "IdentityCenterUser_SSO",
			identity: ctevent.UserIdentity{
				Type: "AssumedRole",
				ARN:  "arn:aws:sts::222222222222:assumed-role/AWSReservedSSO_AdminAccess_3c4d5e6f7a8b9c0d/alice@corp",
				SessionContext: sessionIssuerWithIdentity(
					"AWSReservedSSO_AdminAccess_3c4d5e6f7a8b9c0d",
					"alice@corp",
				),
			},
			mustContain: []string{"alice@corp", "AWSReservedSSO_AdminAccess"},
		},

		// Case 4 — Root
		{
			name: "Root",
			identity: ctevent.UserIdentity{
				Type: "Root",
				ARN:  "arn:aws:iam::555555555555:root",
			},
			mustContain: []string{"oot"}, // covers "Root" and "root"
		},

		// Case 5 — AWSService (e.g. KMS calling on behalf of a resource)
		// There is no ARN for service events; InvokedBy carries the service FQDN.
		{
			name: "AWSService",
			identity: ctevent.UserIdentity{
				Type:      "AWSService",
				InvokedBy: "kms.amazonaws.com",
			},
			mustContain: []string{"kms.amazonaws.com"},
		},

		// Case 6 — WebIdentityUser / IRSA (Kubernetes service account via OIDC)
		{
			name: "WebIdentityUser_IRSA",
			identity: ctevent.UserIdentity{
				Type: "AssumedRole",
				ARN:  "arn:aws:sts::666666666666:assumed-role/eks-checkout-svc-sa/1717156821993453824",
				SessionContext: sessionWithWebID(
					"oidc.eks.eu-west-1.amazonaws.com/id/EXAMPLE0D8C",
				),
			},
			mustContain: []string{"eks-checkout-svc-sa"},
		},

		// Case 7 — FederatedUser
		{
			name: "FederatedUser",
			identity: ctevent.UserIdentity{
				Type:     "FederatedUser",
				UserName: "alice",
			},
			mustContain: []string{"alice"},
		},

		// Case 8 — SAMLUser
		{
			name: "SAMLUser",
			identity: ctevent.UserIdentity{
				Type:     "SAMLUser",
				UserName: "alice@example.com",
			},
			mustContain: []string{"alice@example.com"},
		},

		// Case 9 — AWSAccount (cross-account delegation, no role assumption)
		{
			name: "AWSAccount",
			identity: ctevent.UserIdentity{
				Type:      "AWSAccount",
				AccountID: "999988887777",
			},
			mustContain: []string{"999988887777"},
		},

		// Case 10 — Unknown / Directory variant (representative unknown type)
		// Must not panic and must return a non-empty string.
		{
			name: "UnknownType_Directory",
			identity: ctevent.UserIdentity{
				Type:      "Directory",
				AccountID: "444444444444",
			},
			mustContain: []string{}, // only non-empty is asserted below
		},

		// Case 11 — Empty userIdentity (degenerate / missing block)
		// The function MUST NOT return "".
		{
			name:        "EmptyUserIdentity",
			identity:    ctevent.UserIdentity{},
			mustContain: []string{}, // only non-empty is asserted below
		},

		// Case 12 — Cross-account: same userIdentity as Case 1 (IAMUser) but with
		// recipientAccountId != accountId.  Actor is an ACTOR concern only, not a
		// CONTEXT concern (design §2.4), so the result must contain "bob" regardless.
		{
			name: "CrossAccount_DoesNotChangeActor",
			identity: ctevent.UserIdentity{
				Type:     "IAMUser",
				ARN:      "arn:aws:iam::333333333333:user/bob",
				UserName: "bob",
			},
			mustContain: []string{"bob"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			event := eventWithIdentity(c.identity)
			got := ctevent.Actor(event)

			// Primary contract: never return an empty string.
			if got == "" {
				t.Errorf("Actor() returned empty string for type %q — must never be empty", c.identity.Type)
			}

			// Secondary: all expected substrings must be present.
			for _, sub := range c.mustContain {
				if !strings.Contains(got, sub) {
					t.Errorf("Actor() = %q; want it to contain %q", got, sub)
				}
			}
		})
	}
}

// TestCTDetailActor_CrossAccountIdenticalToSameAccount verifies that the
// cross-account flag (recipientAccountId != accountId) does not alter the
// Actor string — per design doc §2.4, cross-account is a CONTEXT concern.
func TestCTDetailActor_CrossAccountIdenticalToSameAccount(t *testing.T) {
	identity := ctevent.UserIdentity{
		Type:      "IAMUser",
		ARN:       "arn:aws:iam::333333333333:user/bob",
		UserName:  "bob",
		AccountID: "333333333333",
	}

	sameAccount := &ctevent.Event{
		UserIdentity:       identity,
		AccountID:          "333333333333",
		RecipientAccountID: "333333333333",
	}
	crossAccount := &ctevent.Event{
		UserIdentity:       identity,
		AccountID:          "333333333333",
		RecipientAccountID: "777777777777",
	}

	same := ctevent.Actor(sameAccount)
	cross := ctevent.Actor(crossAccount)

	if same != cross {
		t.Errorf("Actor() differs for cross-account vs same-account:\n  same-account = %q\n  cross-account = %q\n  want identical", same, cross)
	}
}
