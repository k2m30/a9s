// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// r53_issue_enrichment.go — Wave 2 issue enrichment for the r53 resource type.
package aws

import (
	"context"
	"sync"

	"net/netip"

	"github.com/aws/aws-sdk-go-v2/aws"
	r53svc "github.com/aws/aws-sdk-go-v2/service/route53"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// r53 canonical FindingCodes.
const (
	r53CodeOrphanPrivateZone domain.FindingCode = "r53.orphan-private-zone"

	// CodeR53QueryLoggingOff — a public zone with no query-logging config.
	CodeR53QueryLoggingOff domain.FindingCode = "r53.query-logging-off"

	// CodeR53DanglingRecord — an A record in a public zone whose address is
	// an elastic IP this account holds with nothing attached to it.
	CodeR53DanglingRecord domain.FindingCode = "r53.dangling-record"
)

// The four answers R53AddressOwnership gives about one address. They are a
// closed vocabulary: a record is only dangling when the account can prove the
// address reaches nothing, and every other answer renders as something other
// than broken.
const (
	// R53AddrHeld — an instance in this account carries the address, or an
	// elastic IP this account holds is attached to something.
	R53AddrHeld = "held"
	// R53AddrUnattached — an elastic IP this account holds carries the
	// address and has nothing behind it. This is the only dangling case the
	// account's own inventory can prove.
	R53AddrUnattached = "unattached"
	// R53AddrOutside — nothing in this account's inventory carries the
	// address, so it belongs to another account, another provider or
	// somewhere off AWS entirely, and this account cannot judge it.
	R53AddrOutside = "outside"
	// R53AddrUnknown — an inventory the verdict depends on was absent or
	// truncated, so the address may simply be on a page nobody loaded.
	R53AddrUnknown = "unknown"
)

// R53AddressOwnership says what this account can prove about a public address
// a record points at. It reads the account's own inventory out of the cache,
// which is the only evidence a read-only session has.
//
// An address the account does not hold is not thereby released: absence from
// an inventory is absence of evidence, and treating it as proof turns a record
// pointing at a CDN, another account or on-premises into a broken finding.
func R53AddressOwnership(addr string, cache resource.ResourceCache) string {
	held := heldPublicAddresses(cache)
	if held == nil {
		return R53AddrUnknown
	}
	if word, ours := held[addr]; ours {
		return word
	}
	return R53AddrOutside
}

// heldPublicAddresses maps every public address this account holds to what the
// inventory says about it: R53AddrHeld when something is behind it,
// R53AddrUnattached for an elastic IP with nothing behind it. It returns nil
// when either cache is absent or truncated, in which case an address missing
// from them may simply be on a page nobody loaded.
//
// An instance address is always held: it exists because the instance does.
// Only an elastic IP can outlive what it pointed at, and eip.go writes that
// state into Fields["status"] rather than making every reader re-derive it.
// The ec2 cache is read last for that reason: should both inventories claim
// one address, being attached to a running instance is the stronger evidence
// and must not be overwritten by an idle-elastic-IP verdict.
//
// The eni cache is deliberately not consulted: the eni fetcher writes no
// public_ip, so requiring it voided the verdict without contributing a single
// address. An address carried only by a network interface therefore reads as
// outside the account, which under this rule emits nothing.
func heldPublicAddresses(cache resource.ResourceCache) map[string]string {
	held := make(map[string]string)
	for _, name := range []string{"eip", "ec2"} {
		entry, ok := cache[name]
		if !ok || entry.IsTruncated {
			return nil
		}
		for _, r := range entry.Resources {
			ip := r.Fields["public_ip"]
			if ip == "" {
				continue
			}
			word := R53AddrHeld
			if name == "eip" && r.Fields["status"] == eipStatusUnattached {
				word = R53AddrUnattached
			}
			held[ip] = word
		}
	}
	return held
}

// r53DanglingRecords returns the records pointing at an elastic IP this
// account holds with nothing behind it — the one unreachable target the
// inventory proves. Only plain A records carry a comparable address: an alias
// target names a resource rather than an address, AAAA and CNAME are outside
// this check, and a private address cannot be claimed.
func r53DanglingRecords(records []r53types.ResourceRecordSet, held map[string]string) []r53types.ResourceRecordSet {
	var out []r53types.ResourceRecordSet
	for _, rec := range records {
		if rec.Type != r53types.RRTypeA || rec.AliasTarget != nil {
			continue
		}
		for _, rr := range rec.ResourceRecords {
			ip, err := netip.ParseAddr(aws.ToString(rr.Value))
			if err != nil || !ip.Is4() || !ip.IsGlobalUnicast() || ip.IsPrivate() {
				continue
			}
			if held[ip.String()] == R53AddrUnattached {
				out = append(out, rec)
				break
			}
		}
	}
	return out
}

// EnrichRoute53Zone calls GetHostedZone per zone (cap EnrichmentCap) and raises a finding
// for private zones that have no VPC associations (orphaned private zone).
//
// Findings:
//   - HostedZone.Config.PrivateZone == true AND VPCs[] empty → "~" finding
//     "private zone with no VPC associations (orphan)"
//
// Skip if clients.Route53 == nil. Per-zone errors → Truncated.
func EnrichRoute53Zone(ctx context.Context, clients *ServiceClients, resources []resource.Resource, cache resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]bool),
	}
	if clients.Route53 == nil {
		return result, nil
	}
	held := heldPublicAddresses(cache)
	var failures []Failure
	total := 0
	resources = capAtEnrichmentCap(&result, resources, resourceIDsOf)
	n := len(resources)
	var mu sync.Mutex
	_ = ForEachParallel(ctx, n, EnrichmentParallelism, func(i int) {
		r := resources[i]
		zoneID := r.Fields["zone_id"]
		if zoneID == "" {
			zoneID = r.ID
		}
		if zoneID == "" {
			return
		}
		mu.Lock()
		total++
		mu.Unlock()
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*r53svc.GetHostedZoneOutput, error) {
			return clients.Route53.GetHostedZone(ctx, &r53svc.GetHostedZoneInput{
				Id: aws.String(zoneID),
			})
		})
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			// A zone deleted between ListHostedZones and this per-zone call
			// is an operational race, not a failure. See IsNotFoundErr.
			if IsNotFoundErr(err) {
				// The zone went away between the list call and this one: a
				// race, not a failure to log.
				result.TruncatedIDs[r.ID] = true
				return
			}
			MarkSkipped(&result, r.ID, &failures, err)
			return
		}
		if out.HostedZone == nil {
			return
		}
		// Only raise the orphan finding for private zones — public zones
		// cannot have VPC associations. The public-zone rows are evaluated
		// first, since they are the other half of this call's answer.
		if out.HostedZone.Config == nil || !out.HostedZone.Config.PrivateZone {
			r53PublicZoneFindings(ctx, clients, &result, &failures, r, zoneID, held)
			return
		}
		if len(out.VPCs) > 0 {
			return
		}
		// The zone identifier is the row's whole job: the phrase already says
		// what is wrong, so a second row repeating it prints one fact twice.
		setWave2Finding(&result, r.ID, r53CodeOrphanPrivateZone, "private zone with no VPC associations (orphan)", "~", "r53", []domain.DetailRow{
			{Label: "Zone ID", Value: zoneID, Tier: "~"},
		})
	})

	return result,
		AggregateFailures("r53-enrich: GetHostedZone", failures, total)
}

// r53PublicZoneFindings evaluates the two public-zone rows: query logging, and
// records pointing at addresses the account has released.
//
// The dangling check is skipped when held is nil — an absent or truncated
// address cache cannot distinguish a released address from an unloaded page,
// and calling that a takeover sends an operator chasing a healthy record. A
// zone whose records cannot be listed is marked truncated rather than reported
// clean.
func r53PublicZoneFindings(ctx context.Context, clients *ServiceClients, result *IssueEnricherResult, failures *[]Failure, r resource.Resource, zoneID string, held map[string]string) {
	r53QueryLoggingFinding(ctx, clients, result, failures, r, zoneID)

	if held == nil {
		return
	}
	records, recordsErr := listAllR53Records(ctx, clients.Route53, zoneID)
	if recordsErr != nil {
		MarkSkipped(result, r.ID, failures, recordsErr)
	}
	for _, rec := range r53DanglingRecords(records, held) {
		name := aws.ToString(rec.Name)
		target := ""
		if len(rec.ResourceRecords) > 0 {
			target = aws.ToString(rec.ResourceRecords[0].Value)
		}
		setWave2Finding(result, r.ID, CodeR53DanglingRecord, "record points at an unassociated elastic IP", "!", "r53",
			[]domain.DetailRow{
				{Label: "Record", Value: name, Tier: "!"},
				{Label: "Target", Value: target, Tier: "!"},
			})
	}
}

// r53QueryLoggingFinding evaluates the query-logging row for one public zone.
// A zone with no config on any page has none at all, so the walk stops at the
// first config it sees rather than counting them.
func r53QueryLoggingFinding(ctx context.Context, clients *ServiceClients, result *IssueEnricherResult, failures *[]Failure, r resource.Resource, zoneID string) {
	input := &r53svc.ListQueryLoggingConfigsInput{HostedZoneId: aws.String(zoneID)}
	for {
		logs, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*r53svc.ListQueryLoggingConfigsOutput, error) {
			return clients.Route53.ListQueryLoggingConfigs(ctx, input)
		})
		switch {
		case err != nil:
			MarkSkipped(result, r.ID, failures, err)
			return
		case len(logs.QueryLoggingConfigs) > 0:
			return
		case logs.NextToken == nil:
			setWave2Finding(result, r.ID, CodeR53QueryLoggingOff, "query logging off", "~", "r53",
				[]domain.DetailRow{{Label: "Zone type", Value: "public", Tier: "~"}})
			return
		}
		input.NextToken = logs.NextToken
	}
}
