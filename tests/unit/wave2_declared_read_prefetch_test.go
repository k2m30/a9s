package unit

import (
	"context"
	"errors"
	"testing"

	ec2svc "github.com/aws/aws-sdk-go-v2/service/ec2"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/session"
)

type prefetchDeniedSGEC2 struct{ awsclient.EC2API }

func (prefetchDeniedSGEC2) DescribeSecurityGroups(context.Context, *ec2svc.DescribeSecurityGroupsInput, ...func(*ec2svc.Options)) (*ec2svc.DescribeSecurityGroupsOutput, error) {
	return nil, errors.New("AccessDenied: not authorized to perform ec2:DescribeSecurityGroups")
}

// TestWave2DeclaredRead_UnobservedListIsFetchedDeniedOneStillMarks: the ec2
// list is opened before any sweep, so the sg list its exposure check declares
// was never observed. Enrichment reads that list itself and judges every row;
// when the read is refused, the rows the check could not judge keep their
// "sg list incomplete" mark.
func TestWave2DeclaredRead_UnobservedListIsFetchedDeniedOneStillMarks(t *testing.T) {
	for _, tc := range []struct {
		name      string
		denied    bool
		wantMarks bool
	}{
		{"sg readable", false, false},
		{"sg refused", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clients := demo.NewServiceClients()
			td := resource.FindResourceType("ec2")
			if td == nil {
				t.Fatal("ec2 is not registered")
			}
			rows, ok := DrainFixtures(t, *td, clients)
			if !ok {
				t.Fatal("the ec2 demo fixtures drained no rows")
			}
			if tc.denied {
				clients.EC2 = prefetchDeniedSGEC2{clients.EC2}
			}
			sess := session.New()
			sess.Clients = clients
			core := runtime.New(sess, catalog.All())
			core.Session().RowStore.Observe("ec2", rows, nil, session.OriginProbe, false)

			got := core.ProbeEnrichment(context.Background(), clients, "ec2")

			marked := 0
			for _, check := range got.TruncatedIDs {
				if check == "sg list incomplete" {
					marked++
				}
			}
			if tc.wantMarks && marked == 0 {
				t.Error("the sg read was refused, yet no instance is marked \"sg list incomplete\"")
			}
			if !tc.wantMarks && marked != 0 {
				t.Errorf("the sg list was readable, yet %d instance(s) are marked \"sg list incomplete\": %v", marked, got.TruncatedIDs)
			}
		})
	}
}
