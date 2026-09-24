package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
)

// A log group whose name carries no API Gateway execution-log prefix was
// read and names no API: a proven 0.
func TestT570Dev_LogsAPIGW_UnnamedGroupIsAProvenZero(t *testing.T) {
	c := t570Demo()
	group := t570Row(t, c, "logs", fixtures.WAFProdAPILogGroup)
	t570Exact(t, t570Pivot(t, c, group, "logs", "apigw"))
}

// Every demo group names its launch source, so the pivots that read it
// answer exactly.
func TestT570Dev_ASGLaunchSourcesAreRead(t *testing.T) {
	c := t570Demo()
	asg := t570Row(t, c, "asg", "eks-acme-prod-ng-general")
	for _, target := range []string{"ami", "role", "sg"} {
		r := t570Pivot(t, c, asg, "asg", target)
		if r.State() != domain.RelatedResolved || r.Truncated() {
			t.Errorf("asg → %s: state %v truncated %v (err %v), want an exact answer", target, r.State(), r.Truncated(), r.Err())
		}
	}
	ami := t570Row(t, c, "ami", "ami-0a1b2c3d4e5f60003")
	if r := t570Pivot(t, c, ami, "ami", "asg"); r.State() != domain.RelatedResolved || r.Truncated() || r.Count() != 1 {
		t.Errorf("ami → asg: state %v truncated %v count %d, want exactly 1", r.State(), r.Truncated(), r.Count())
	}
}
