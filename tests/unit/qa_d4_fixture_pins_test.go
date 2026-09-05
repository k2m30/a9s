package unit

// qa_d4_fixture_pins_test.go — demo fixture contracts this batch changes.
//
// A demo fixture set is the only bench any surface test has. When a filler
// pool leaves a posture field unset, the finding fires on the filler rather
// than on the witness planted for it; when every fixture is configured the
// same way, a pivot's empty state is never rendered and nothing notices when
// it breaks. Both make the bench agree with itself while disagreeing with
// what an operator would see.

import (
	"context"
	"strings"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	a9sruntime "github.com/k2m30/a9s/v3/core/runtime"
)

func d4DemoRows(t *testing.T, shortName string) []resource.Resource {
	t.Helper()
	td := resource.FindResourceType(shortName)
	if td == nil {
		t.Fatalf("%s not registered", shortName)
	}
	clients := demo.NewServiceClients()
	return DrainPages(t, shortName, func(token string) (resource.FetchResult, error) {
		return td.Fetcher(context.Background(), clients, token)
	})
}

func d4CarriersOf(rows []resource.Resource, code domain.FindingCode) []string {
	var ids []string
	for _, r := range rows {
		for _, f := range r.Findings {
			if f.Code == code {
				ids = append(ids, r.ID)
				break
			}
		}
	}
	return ids
}

// TestD4_DeletionProtectionHasOneWitness pins that the dbi filler pool no
// longer decides a security finding by omission.
//
// The bulk pool exists to give the list realistic length, and its posture pass
// forces every other predicate to the healthy value. DeletionProtection is
// left unset, so the finding fires across the filler and the row planted to
// demonstrate it is one of many — which means the demo cannot show what the
// signal looks like, and any count that moves cannot be attributed.
func TestD4_DeletionProtectionHasOneWitness(t *testing.T) {
	rows := d4DemoRows(t, "dbi")
	carriers := d4CarriersOf(rows, awsclient.CodeDBIDeletionProtectionOff)
	if len(carriers) != 1 {
		t.Errorf("%s fires on %d of %d demo dbi rows, want exactly 1 witness: %v",
			awsclient.CodeDBIDeletionProtectionOff, len(carriers), len(rows), carriers)
	}
}

// TestD4_S3AccessLogPivotShowsItsEmptyState pins that the demo bench renders
// both outcomes of the access-log pivot.
//
// Giving every bucket a logging config makes the pivot resolve everywhere, so
// the "no access logging configured" rendering never appears in the demo and
// no test can catch it regressing. A bench that only ever shows the populated
// branch proves half the behaviour.
func TestD4_S3AccessLogPivotShowsItsEmptyState(t *testing.T) {
	td := resource.FindResourceType("s3")
	if td == nil {
		t.Fatal("s3 not registered")
	}
	var checker resource.RelatedChecker
	for _, rel := range td.Related {
		if rel.DisplayName == "Access Log Bucket" {
			checker = rel.Checker
		}
	}
	if checker == nil {
		t.Fatal("s3 has no Access Log Bucket pivot")
	}

	rows := d4DemoRows(t, "s3")
	clients := demo.NewServiceClients()
	cache := resource.ResourceCache{"s3": resource.ResourceCacheEntry{Resources: rows}}

	var withLogs, withoutLogs []string
	for _, r := range rows {
		result := checker(context.Background(), clients, r, cache)
		if len(result.ResourceIDs()) > 0 {
			withLogs = append(withLogs, r.ID)
			continue
		}
		withoutLogs = append(withoutLogs, r.ID)
	}

	if len(withLogs) == 0 {
		t.Errorf("no demo bucket resolves an access-log target, so the populated pivot is unwitnessed")
	}
	if len(withoutLogs) == 0 {
		t.Errorf("all %d demo buckets resolve an access-log target, so the pivot's empty state is never "+
			"rendered; at least one bucket must have no logging configured", len(rows))
	}
}

// TestD4_ElasticIPAgreesWithItsInstance pins that the two places the demo
// states one instance's public address agree.
//
// The instance carries a public IP and the Elastic IP allocation associated
// with that instance carries another. Whichever the operator reads first is
// the one they will paste into a firewall rule, and one of them is wrong.
func TestD4_ElasticIPAgreesWithItsInstance(t *testing.T) {
	const instanceID = "i-0a1b2c3d4e5f60001"

	var instancePublicIP string
	for _, r := range d4DemoRows(t, "ec2") {
		if r.ID == instanceID {
			instancePublicIP = r.Fields["public_ip"]
		}
	}
	if instancePublicIP == "" {
		t.Fatalf("demo instance %s carries no public address, so there is nothing to agree with", instanceID)
	}

	var eipAddress, eipID string
	for _, r := range d4DemoRows(t, "eip") {
		if r.Fields["instance_id"] == instanceID {
			eipAddress = r.Fields["public_ip"]
			eipID = r.ID
		}
	}
	if eipAddress == "" {
		t.Fatalf("no demo Elastic IP is associated with %s", instanceID)
	}

	if eipAddress != instancePublicIP {
		t.Errorf("the demo states two public addresses for %s: the instance says %q and Elastic IP %s says %q",
			instanceID, instancePublicIP, eipID, eipAddress)
	}
}

// TestD4_PlainHTTPListenerWitnessedOnAnNLB pins the other half of the
// cleartext-listener rule.
//
// The finding covers an ALB serving HTTP and an NLB serving TCP, and the demo
// witnesses only the first. The NLB half is the one an operator is more likely
// to have by accident, and nothing on the bench shows it fires.
func TestD4_PlainHTTPListenerWitnessedOnAnNLB(t *testing.T) {
	td := resource.FindResourceType("elb")
	if td == nil {
		t.Fatal("elb not registered")
	}
	rows := d4DemoRows(t, "elb")
	clients := demo.NewServiceClients()

	enricher, ok := awsclient.Wave2EnricherFor("elb")
	if !ok || enricher.Fn == nil {
		t.Fatal("elb has no wave-2 enricher")
	}
	res, err := enricher.Fn(context.Background(), clients, rows, nil)
	if err != nil {
		t.Fatalf("elb wave-2: %v", err)
	}
	folded := append([]resource.Resource(nil), rows...)
	for i := range folded {
		a9sruntime.ApplyWave2ToRow(&folded[i], *td, res.Findings, res.AttentionDetails)
	}

	const code domain.FindingCode = "elb.plain-http-listener"
	var nlbCarriers, otherCarriers []string
	for _, r := range folded {
		carries := false
		for _, f := range r.Findings {
			if f.Code == code {
				carries = true
			}
		}
		if !carries {
			continue
		}
		if strings.EqualFold(r.Fields["type"], "network") {
			nlbCarriers = append(nlbCarriers, r.ID)
			continue
		}
		otherCarriers = append(otherCarriers, r.ID)
	}

	if len(otherCarriers) == 0 {
		t.Errorf("%s fires on no application load balancer, so the half that used to be witnessed is gone", code)
	}
	if len(nlbCarriers) == 0 {
		t.Errorf("%s fires on no network load balancer; the TCP-listener half of the rule has no demo witness "+
			"(balancers seen: %d)", code, len(folded))
	}
}
