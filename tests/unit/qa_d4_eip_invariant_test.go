package unit

// qa_d4_eip_invariant_test.go — one owner per Elastic IP allocation, across
// every fixture that names one.
//
// An allocation was a NAT gateway's address, an instance's Elastic IP, and two
// interface associations advertising two different public addresses, all at
// once. AWS produces none of those shapes, and each surface reading a different
// one is how the demo came to state three public addresses for one instance.
// Pinned as an invariant over the whole set rather than over the allocations
// that were wrong, because naming them would not stop the fourth.

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// d4Rows drains one type's demo rows through its own fetcher.
func d4Rows(t *testing.T, shortName string) []resource.Resource {
	t.Helper()
	td := resource.FindResourceType(shortName)
	if td == nil {
		t.Fatalf("%s not registered", shortName)
	}
	rows, ok := DrainFixtures(t, *td, demo.NewServiceClients())
	if !ok {
		t.Fatalf("%s has no Wave-1 Fetcher", shortName)
	}
	return rows
}

// TestD4Row27_EveryAllocationHasOneOwner sweeps every Elastic IP allocation in
// the demo set and fails on any that more than one thing claims.
func TestD4Row27_EveryAllocationHasOneOwner(t *testing.T) {
	eips := d4Rows(t, "eip")
	if len(eips) == 0 {
		t.Fatal("no demo Elastic IP rows, so this invariant would hold vacuously")
	}

	natAddress := d4NATAllocations(t)

	// Every allocation an interface advertises, and the address it advertises.
	type advert struct{ eni, publicIP string }
	onInterface := map[string][]advert{}
	for _, e := range d4Rows(t, "eni") {
		raw, ok := e.RawStruct.(ec2types.NetworkInterface)
		if !ok || raw.Association == nil {
			continue
		}
		id := aws.ToString(raw.Association.AllocationId)
		if id == "" {
			continue
		}
		onInterface[id] = append(onInterface[id], advert{e.ID, aws.ToString(raw.Association.PublicIp)})
	}

	byPublicIP := map[string][]string{}
	var problems []string

	for _, r := range eips {
		id := r.ID
		instance := r.Fields["instance_id"]
		publicIP := r.Fields["public_ip"]
		byPublicIP[publicIP] = append(byPublicIP[publicIP], id)

		nat, isNAT := natAddress[id]

		// A NAT gateway's address belongs to the gateway. AWS never lets an
		// instance hold it, and a row claiming both makes the eip list and the
		// nat list disagree about what the operator is looking at.
		if isNAT && instance != "" {
			problems = append(problems, fmt.Sprintf(
				"%s is %s's address and also claims instance %s", id, nat, instance))
		}
		// It is attached, so it must not read as an idle allocation.
		if isNAT && r.Fields["association_id"] == "" {
			problems = append(problems, fmt.Sprintf(
				"%s is %s's address but carries no association, so it reads idle", id, nat))
		}

		// One allocation is associated with at most one interface.
		if adverts := onInterface[id]; len(adverts) > 1 {
			var where []string
			for _, a := range adverts {
				where = append(where, a.eni+" advertising "+a.publicIP)
			}
			sort.Strings(where)
			problems = append(problems, fmt.Sprintf(
				"%s is associated with %d interfaces: %s", id, len(adverts), strings.Join(where, ", ")))
		}
		// And the interface advertises the address the allocation holds.
		for _, a := range onInterface[id] {
			if a.publicIP != "" && a.publicIP != publicIP {
				problems = append(problems, fmt.Sprintf(
					"%s holds %s but interface %s advertises %s for it", id, publicIP, a.eni, a.publicIP))
			}
		}
	}

	// No address belongs to two allocations.
	for ip, ids := range byPublicIP {
		if len(ids) > 1 {
			sort.Strings(ids)
			problems = append(problems, fmt.Sprintf("public address %s is held by %v", ip, ids))
		}
	}

	if len(problems) > 0 {
		sort.Strings(problems)
		t.Errorf("%d Elastic IP allocation(s) have more than one owner:\n  %s",
			len(problems), strings.Join(problems, "\n  "))
	}
}

// TestD4Row27_EveryInstanceStatesOnePublicAddress pins the surface the operator
// reads first. The instance list, the interface attached to it, and any Elastic
// IP naming it have to agree, or whichever they open first is the one they
// paste into a firewall rule.
func TestD4Row27_EveryInstanceStatesOnePublicAddress(t *testing.T) {
	claims := map[string]map[string][]string{} // instance → address → surfaces

	claim := func(instance, address, where string) {
		if instance == "" || address == "" {
			return
		}
		if claims[instance] == nil {
			claims[instance] = map[string][]string{}
		}
		claims[instance][address] = append(claims[instance][address], where)
	}

	for _, r := range d4Rows(t, "ec2") {
		claim(r.ID, r.Fields["public_ip"], "the instance list")
	}
	for _, r := range d4Rows(t, "eip") {
		claim(r.Fields["instance_id"], r.Fields["public_ip"], "Elastic IP "+r.ID)
	}
	for _, r := range d4Rows(t, "eni") {
		raw, ok := r.RawStruct.(ec2types.NetworkInterface)
		if !ok || raw.Association == nil || raw.Attachment == nil {
			continue
		}
		claim(aws.ToString(raw.Attachment.InstanceId), aws.ToString(raw.Association.PublicIp), "interface "+r.ID)
	}

	var problems []string
	for instance, addresses := range claims {
		if len(addresses) < 2 {
			continue
		}
		var lines []string
		for addr, where := range addresses {
			sort.Strings(where)
			lines = append(lines, fmt.Sprintf("%s per %s", addr, strings.Join(where, " and ")))
		}
		sort.Strings(lines)
		problems = append(problems, instance+": "+strings.Join(lines, "; "))
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		t.Errorf("%d instance(s) are given more than one public address:\n  %s",
			len(problems), strings.Join(problems, "\n  "))
	}
}

// TestD4Row27_EIPCountAndFindingsAreDerived pins the one count row 27 moved and
// every count it must not have.
//
// The list length is derived from the fixture, so it is stated here rather than
// pinned in a smoke script; the finding counts are the exactly-one-witness rules
// the batch spent its rounds establishing, and a fixture reshuffle is exactly
// the change that would move one without anyone noticing.
func TestD4Row27_EIPCountAndFindingsAreDerived(t *testing.T) {
	eips := d4Rows(t, "eip")
	if len(eips) != 9 {
		t.Errorf("demo Elastic IP list = %d rows, want 9", len(eips))
	}

	want := map[string]map[domain.FindingCode]int{
		"eip": {"eip.unassociated": 4},
		"ec2": {"ec2.public-ip": 2},
	}
	for short, codes := range want {
		got := map[domain.FindingCode]int{}
		for _, r := range d4Rows(t, short) {
			for _, f := range r.Findings {
				got[f.Code]++
			}
		}
		for code, n := range codes {
			if got[code] != n {
				t.Errorf("%s: %s fires on %d rows, want %d", short, code, got[code], n)
			}
		}
	}

	// Row counts on the sibling lists the reshuffle touched.
	for short, n := range map[string]int{"eni": 47, "ec2": 40, "nat": 6} {
		if got := len(d4Rows(t, short)); got != n {
			t.Errorf("demo %s list = %d rows, want %d", short, got, n)
		}
	}
}

// TestD4Row27_NATAddressColourComesFromTheFetcher pins that a NAT gateway's
// Elastic IP is coloured by the finding the fetcher emits, not by a classifier
// re-deriving attachment from two of the three fields the fetcher uses.
//
// That second derivation read an allocation with an interface and no instance
// as idle, which is exactly what an attached NAT address looks like.
func TestD4Row27_NATAddressColourComesFromTheFetcher(t *testing.T) {
	td := resource.FindResourceType("eip")
	if td == nil {
		t.Fatal("eip not registered")
	}

	natAddress := d4NATAllocations(t)

	seen := 0
	for _, r := range d4Rows(t, "eip") {
		hasFinding := len(r.Findings) > 0
		colour := td.ResolveColor(r)
		if !hasFinding && colour != domain.ColorHealthy {
			t.Errorf("%s carries no finding but colours %v, so something outside the fetcher decided it",
				r.ID, colour)
		}
		if hasFinding && colour == domain.ColorHealthy {
			t.Errorf("%s carries %s but colours healthy", r.ID, r.Findings[0].Code)
		}
		if _, isNAT := natAddress[r.ID]; isNAT {
			seen++
			if len(r.Findings) != 0 {
				t.Errorf("%s is an attached NAT gateway address but carries %s", r.ID, r.Findings[0].Code)
			}
		}
	}
	if seen == 0 {
		t.Error("no demo Elastic IP is a NAT gateway's address, so the shape that was misread is unwitnessed")
	}
}

// d4NATAllocations maps every allocation a NAT gateway holds to the gateway.
// The nat row's Fields carry only the public address, so the allocation id
// comes off the SDK struct the detail view renders from.
func d4NATAllocations(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, n := range d4Rows(t, "nat") {
		raw, ok := n.RawStruct.(ec2types.NatGateway)
		if !ok {
			continue
		}
		for _, addr := range raw.NatGatewayAddresses {
			if id := aws.ToString(addr.AllocationId); id != "" {
				out[id] = n.ID
			}
		}
	}
	if len(out) == 0 {
		t.Fatal("no demo NAT gateway names an allocation, so every NAT clause below would hold vacuously")
	}
	return out
}
