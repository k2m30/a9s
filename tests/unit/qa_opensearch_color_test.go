package unit

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/opensearch"
	ostypes "github.com/aws/aws-sdk-go-v2/service/opensearch/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// TestOpenSearchColor pins the domain state → colour mapping for OpenSearch.
//
// Two rows read differently from the phrase-matching era, and both follow from
// colour being the worst finding's severity. An available software update is
// only reported when AWS has already passed the automated-install date, so an
// update flag on its own leaves the row green. Encryption at rest being off is
// a Warn finding, so it colours the row: the old table kept it green on the
// grounds that a background signal shows as a glyph instead, and that
// distinction does not survive a severity comparison.
func TestOpenSearchColor(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*ostypes.DomainStatus)
		want   resource.Color
	}{
		{name: "healthy", want: resource.ColorHealthy},
		{
			name: "update_available_without_a_forced_date",
			mutate: func(d *ostypes.DomainStatus) {
				d.ServiceSoftwareOptions = &ostypes.ServiceSoftwareOptions{UpdateAvailable: aws.Bool(true)}
			},
			want: resource.ColorHealthy,
		},

		{
			name: "update_forced_date_passed",
			mutate: func(d *ostypes.DomainStatus) {
				d.ServiceSoftwareOptions = &ostypes.ServiceSoftwareOptions{
					UpdateAvailable:     aws.Bool(true),
					AutomatedUpdateDate: aws.Time(time.Now().Add(-24 * time.Hour)),
				}
			},
			want: resource.ColorWarning,
		},
		{
			name: "encryption_at_rest_off",
			mutate: func(d *ostypes.DomainStatus) {
				d.EncryptionAtRestOptions = &ostypes.EncryptionAtRestOptions{Enabled: aws.Bool(false)}
			},
			want: resource.ColorWarning,
		},
		{
			name: "https_not_enforced",
			mutate: func(d *ostypes.DomainStatus) {
				d.DomainEndpointOptions = &ostypes.DomainEndpointOptions{EnforceHTTPS: aws.Bool(false)}
			},
			want: resource.ColorWarning,
		},
		{
			name: "node_to_node_encryption_off",
			mutate: func(d *ostypes.DomainStatus) {
				d.NodeToNodeEncryptionOptions = &ostypes.NodeToNodeEncryptionOptions{Enabled: aws.Bool(false)}
			},
			want: resource.ColorWarning,
		},
		{
			name:   "processing",
			mutate: func(d *ostypes.DomainStatus) { d.Processing = aws.Bool(true) },
			want:   resource.ColorWarning,
		},
		{
			name:   "upgrade_processing",
			mutate: func(d *ostypes.DomainStatus) { d.UpgradeProcessing = aws.Bool(true) },
			want:   resource.ColorWarning,
		},

		{
			name: "isolated",
			mutate: func(d *ostypes.DomainStatus) {
				d.DomainProcessingStatus = ostypes.DomainProcessingStatusTypeIsolated
			},
			want: resource.ColorBroken,
		},
		{
			name: "reachable_outside_a_vpc",
			mutate: func(d *ostypes.DomainStatus) {
				d.AccessPolicies = aws.String(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":"*","Action":"es:*","Resource":"*"}]}`)
			},
			want: resource.ColorBroken,
		},
		{
			name: "isolated_outranks_encryption_off",
			mutate: func(d *ostypes.DomainStatus) {
				d.DomainProcessingStatus = ostypes.DomainProcessingStatusTypeIsolated
				d.EncryptionAtRestOptions = &ostypes.EncryptionAtRestOptions{Enabled: aws.Bool(false)}
			},
			want: resource.ColorBroken,
		},

		// A domain being torn down reports one dim finding and nothing else,
		// so no posture signal can promote its colour.
		{
			name: "deleted",
			mutate: func(d *ostypes.DomainStatus) {
				d.Deleted = aws.Bool(true)
				d.EncryptionAtRestOptions = &ostypes.EncryptionAtRestOptions{Enabled: aws.Bool(false)}
			},
			want: resource.ColorDim,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := osTestBaseDomain("acme-search")
			if tc.mutate != nil {
				tc.mutate(&d)
			}
			listMock := &mockOSListDomainNamesAPI{
				output: &opensearch.ListDomainNamesOutput{
					DomainNames: []ostypes.DomainInfo{{DomainName: d.DomainName}},
				},
			}
			describeMock := &mockOSDescribeDomainsAPI{
				output: &opensearch.DescribeDomainsOutput{
					DomainStatusList: []ostypes.DomainStatus{d},
				},
			}
			resources, err := awsclient.FetchOpenSearchDomains(context.Background(), listMock, describeMock)
			if err != nil {
				t.Fatalf("FetchOpenSearchDomains: %v", err)
			}
			if len(resources) != 1 {
				t.Fatalf("expected 1 resource, got %d", len(resources))
			}
			d1AssertColor(t, resources[0], tc.want)
		})
	}
}
