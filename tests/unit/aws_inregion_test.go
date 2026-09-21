package unit_test

import (
	"context"
	"reflect"
	"slices"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/acm"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
)

// InRegion is the one way to reach another region: the session region and ""
// are the receiver itself, any other region is a client set built once and
// reused, and its calls are signed for that region.
func TestInRegion_ReturnsOneCachedClientSetPerRegion(t *testing.T) {
	w := newRegionWorld()
	session := w.clients("eu-west-1")

	if got := session.InRegion(""); got != session {
		t.Error(`InRegion("") is not the receiver`)
	}
	if got := session.InRegion("eu-west-1"); got != session {
		t.Error(`InRegion(session region) is not the receiver`)
	}

	var wg sync.WaitGroup
	got := make([]*awsclient.ServiceClients, 16)
	for i := range got {
		wg.Go(func() { got[i] = session.InRegion("us-east-1") })
	}
	wg.Wait()
	use1 := got[0]
	if use1 == nil || use1 == session {
		t.Fatalf(`InRegion("us-east-1") = %p, want a client set distinct from the session's %p`, use1, session)
	}
	for i, c := range got {
		if c != use1 {
			t.Errorf("concurrent InRegion call %d returned %p, want the one cached %p", i, c, use1)
		}
	}
	if use1.Region != "us-east-1" {
		t.Errorf(`InRegion("us-east-1").Region = %q`, use1.Region)
	}
	if session.Region != "eu-west-1" {
		t.Errorf("session Region changed to %q", session.Region)
	}

	ctx := context.Background()
	if _, err := use1.ACM.ListCertificates(ctx, &acm.ListCertificatesInput{}); err != nil {
		t.Fatalf("us-east-1 ListCertificates: %v", err)
	}
	if _, err := session.ACM.ListCertificates(ctx, &acm.ListCertificatesInput{}); err != nil {
		t.Fatalf("eu-west-1 ListCertificates: %v", err)
	}
	var regions []string
	for _, c := range w.callsFor("acm", "") {
		regions = append(regions, c.Region)
	}
	if !slices.Equal(regions, []string{"us-east-1", "eu-west-1"}) {
		t.Errorf("ListCertificates signed for %v, want [us-east-1 eu-west-1]", regions)
	}
}

// One derivation answers for every Region, so a client field pinned to a single
// Region would be a second truth source for what that Region's client is.
func TestInRegion_NoDedicatedCloudFrontWAFClient(t *testing.T) {
	if _, ok := reflect.TypeFor[awsclient.ServiceClients]().FieldByName("WAFv2CloudFront"); ok {
		t.Error(`ServiceClients still has WAFv2CloudFront; CloudFront-scope WAF goes through InRegion("us-east-1").WAFv2`)
	}
}
