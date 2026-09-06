// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// r53_issue_enrichment.go — Wave 2 issue enrichment for the r53 resource type.
package aws

import (
	"context"
	"fmt"
	"sort"
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
	// held by no elastic IP, instance or network interface in the account.
	CodeR53DanglingRecord domain.FindingCode = "r53.dangling-record"
)

// S5 detail sentences for the r53 wave-2 findings.
const (
	r53QueryLoggingOffDetail = "Nothing records who resolves names in this public zone, so a subdomain being probed or abused leaves no evidence. Create a query logging configuration for the zone."

	r53DanglingRecordDetail = "The record still answers with an address the account no longer holds, so whoever claims that address next receives traffic for this name. Delete the record or repoint it at an address you own."
)

// r53AddressCaches are the caches that together hold every public address the
// account controls. All three must be present and whole for an address to be
// called released.
var r53AddressCaches = []string{"eip", "ec2", "eni"} //nolint:gochecknoglobals // static list: intentional package-level var

// heldPublicAddresses returns every public address the account holds, or nil
// when any of the three caches is absent or truncated — in which case an
// address missing from them may simply be on a page nobody loaded.
func heldPublicAddresses(cache resource.ResourceCache) map[string]bool {
	held := make(map[string]bool)
	for _, name := range r53AddressCaches {
		entry, ok := cache[name]
		if !ok || entry.IsTruncated {
			return nil
		}
		for _, r := range entry.Resources {
			if ip := r.Fields["public_ip"]; ip != "" {
				held[ip] = true
			}
		}
	}
	return held
}

// r53DanglingRecords returns the records whose address no longer resolves to
// anything the account holds. Only plain A records carry a comparable
// address: an alias target names a resource rather than an address, AAAA and
// CNAME are outside this check, and a private address cannot be claimed.
func r53DanglingRecords(records []r53types.ResourceRecordSet, held map[string]bool) []r53types.ResourceRecordSet {
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
			if !held[ip.String()] {
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
	var failures []string
	total := 0
	n := min(len(resources), EnrichmentCap)
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
				result.TruncatedIDs[r.ID] = true
				return
			}
			failures = append(failures, fmt.Sprintf("%s: %v", r.ID, err))
			result.TruncatedIDs[r.ID] = true
			return
		}
		if out.HostedZone == nil {
			return
		}
		// Only raise the orphan finding for private zones — public zones
		// cannot have VPC associations. The public-zone rows are evaluated
		// first, since they are the other half of this call's answer.
		if out.HostedZone.Config == nil || !out.HostedZone.Config.PrivateZone {
			r53PublicZoneFindings(ctx, clients, &result, r, zoneID, held)
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
	sort.Strings(failures)
	// r53.dangling-record is "!", so the cap bounds the issue count and a
	// capped pass must say so rather than under-report the badge.
	result.Truncated = len(resources) > EnrichmentCap
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
func r53PublicZoneFindings(ctx context.Context, clients *ServiceClients, result *IssueEnricherResult, r resource.Resource, zoneID string, held map[string]bool) {
	logs, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*r53svc.ListQueryLoggingConfigsOutput, error) {
		return clients.Route53.ListQueryLoggingConfigs(ctx, &r53svc.ListQueryLoggingConfigsInput{
			HostedZoneId: aws.String(zoneID),
		})
	})
	switch {
	case err != nil:
		result.TruncatedIDs[r.ID] = true
	case len(logs.QueryLoggingConfigs) == 0:
		setWave2Finding(result, r.ID, CodeR53QueryLoggingOff, "query logging off", "~", "r53",
			[]domain.DetailRow{{Label: "Zone type", Value: "public", Tier: "~"}})
	}

	if held == nil {
		return
	}
	records, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*r53svc.ListResourceRecordSetsOutput, error) {
		return clients.Route53.ListResourceRecordSets(ctx, &r53svc.ListResourceRecordSetsInput{
			HostedZoneId: aws.String(zoneID),
		})
	})
	if err != nil {
		result.TruncatedIDs[r.ID] = true
		return
	}
	for _, rec := range r53DanglingRecords(records.ResourceRecordSets, held) {
		name := aws.ToString(rec.Name)
		target := ""
		if len(rec.ResourceRecords) > 0 {
			target = aws.ToString(rec.ResourceRecords[0].Value)
		}
		setWave2Finding(result, r.ID, CodeR53DanglingRecord, "record points at a released address", "!", "r53",
			[]domain.DetailRow{
				{Label: "Record", Value: name, Tier: "!"},
				{Label: "Target", Value: target, Tier: "!"},
			})
	}
}
