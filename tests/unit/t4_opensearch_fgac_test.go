package unit

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ostypes "github.com/aws/aws-sdk-go-v2/service/opensearch/types"
)

// "There are some situations, however, where it's acceptable to use an open
// access policy, such as when you enable fine-grained access control."
// (OpenSearch Service, Operational best practices)
func TestT4_OpenSearch_FineGrainedAccessControlGuardsAPublicDomain(t *testing.T) {
	fgac := w2OSDomain("acme-search-fgac")
	fgac.VPCOptions = nil
	fgac.Endpoint = aws.String("search-acme-search-fgac.eu-central-1.es.amazonaws.com")
	fgac.AccessPolicies = aws.String(w2OSPublicPolicy)
	fgac.AdvancedSecurityOptions = &ostypes.AdvancedSecurityOptions{
		Enabled:                     aws.Bool(true),
		InternalUserDatabaseEnabled: aws.Bool(true),
		AnonymousAuthEnabled:        aws.Bool(false),
	}

	open := w2OSDomain("acme-search-open")
	open.VPCOptions = nil
	open.Endpoint = aws.String("search-acme-search-open.eu-central-1.es.amazonaws.com")
	open.AccessPolicies = aws.String(w2OSPublicPolicy)
	open.AdvancedSecurityOptions = &ostypes.AdvancedSecurityOptions{Enabled: aws.Bool(false)}

	unset := w2OSDomain("acme-search-unset")
	unset.VPCOptions = nil
	unset.AccessPolicies = aws.String(w2OSPublicPolicy)

	_, res := w2OSRun(t, fgac, open, unset)
	w2AssertNoCode(t, res.Findings["acme-search-fgac"], w2OSCodePublic)
	for _, id := range []string{"acme-search-open", "acme-search-unset"} {
		if !hasCode(res.Findings[id], w2OSCodePublic) {
			t.Errorf("%s findings = %v, want %s: no fine-grained access control in front of an open policy", id, w2Codes(res.Findings[id]), w2OSCodePublic)
		}
	}
}
