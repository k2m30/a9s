// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// ec2_issue_enrichment.go — Wave 2 issue enrichment for the ec2 resource type.
package aws

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2svc "github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/secretscan"
)

// ec2 canonical FindingCodes. Each condition gets its OWN code so
// FindingsOverview (commit 06c1d646, groups findings by rule across types)
// reports them as four distinct rules instead of folding every status/event
// condition under one code.
const (
	ec2CodeInstanceStatusImpaired     domain.FindingCode = "ec2.instance-status-impaired"
	ec2CodeInstanceStatusInitializing domain.FindingCode = "ec2.instance-status.initializing"
	ec2CodeInstanceStatusInsufficient domain.FindingCode = "ec2.instance-status.insufficient-data"
	ec2CodeScheduledEvent             domain.FindingCode = "ec2.scheduled-event"
	ec2CodeInternetExposed            domain.FindingCode = "ec2.internet-exposed"
	//nolint:gosec // G101 false positive: a finding code, not a credential
	ec2CodeUserDataSecret domain.FindingCode = "ec2.user-data-secret"
)

// ec2StatusFinding is the (code, tier, phrase) triple for a
// single SystemStatus/InstanceStatus value, per docs/resources/ec2.md §4
// rows "impaired" / "initializing" / "insufficient-data" (lines 225-227).
type ec2StatusFinding struct {
	code   domain.FindingCode
	tier   string
	phrase string
}

// classifyEC2Status maps an AWS instance/system status-check value to its
// FindingCode, severity and list phrase (S4) mandated by
// docs/resources/ec2.md §4. Only "impaired" is Broken;
// "initializing" and "insufficient-data" are Warning and must never carry
// the "impaired" wording, and each gets its own code. "ok" and
// "not-applicable" produce no finding — "not-applicable" is explicitly out
// of scope per docs/resources/ec2.md §5 (line 244): AWS classifies it as
// Healthy/informational, not surfaced.
func classifyEC2Status(status ec2types.SummaryStatus) (ec2StatusFinding, bool) {
	switch status {
	case ec2types.SummaryStatusImpaired:
		return ec2StatusFinding{
			code:   ec2CodeInstanceStatusImpaired,
			tier:   "!",
			phrase: "impaired: system checks failing",
		}, true
	case ec2types.SummaryStatusInitializing:
		return ec2StatusFinding{
			code:   ec2CodeInstanceStatusInitializing,
			tier:   "~",
			phrase: "initializing: checks in progress",
		}, true
	case ec2types.SummaryStatusInsufficientData:
		return ec2StatusFinding{
			code:   ec2CodeInstanceStatusInsufficient,
			tier:   "~",
			phrase: "status unknown: AWS insufficient-data",
		}, true
	default:
		// "ok" and "not-applicable" — no finding.
		return ec2StatusFinding{}, false
	}
}

// EnrichEC2InstanceStatus calls DescribeInstanceStatus(IncludeAllInstances=true) (account-wide,
// paginated) and returns a Finding for every instance whose system or instance status is not "ok".
// Scheduled events with NotBeforeDeadline within the next 7 days also produce a Finding.
// Each condition (impaired / initializing / insufficient-data / scheduled-event) appends its
// OWN Finding under its own FindingCode — an instance with both an impaired status check and a
// scheduled event gets two Findings, not one merged Finding.
// Pagination uses NextToken; walks up to EnrichmentCap pages.
func EnrichEC2InstanceStatus(ctx context.Context, clients *ServiceClients, resources []resource.Resource, cache resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:         make(map[string][]domain.Finding),
		AttentionDetails: make(map[string]map[domain.FindingCode]domain.AttentionDetail),
		TruncatedIDs:     make(map[string]bool),
	}
	if clients.EC2 == nil {
		return result, nil
	}
	// Cache-only cross-ref: costs nothing and must survive a
	// DescribeInstanceStatus outage, so it runs before the API passes.
	ec2InternetExposure(&result, resources, cache)
	userDataErr := ec2UserDataSecrets(ctx, clients, resources, &result)
	statusResult, statusErr := ec2InstanceStatusFindings(ctx, clients, resources, &result)
	if statusErr != nil {
		return statusResult, errors.Join(statusErr, userDataErr)
	}
	return statusResult, userDataErr
}

// ec2InstanceStatusFindings is the DescribeInstanceStatus pass: status checks
// and imminent scheduled events, merged into the caller's result.
func ec2InstanceStatusFindings(ctx context.Context, clients *ServiceClients, resources []resource.Resource, resultp *IssueEnricherResult) (IssueEnricherResult, error) {
	result := *resultp
	// Build a set of known resource IDs so we can detect unmatched API returns.
	knownIDs := make(map[string]bool, len(resources))
	for _, r := range resources {
		if r.ID != "" {
			knownIDs[r.ID] = true
		}
	}
	var allInstanceStatuses []ec2types.InstanceStatus
	var nextToken *string
	truncated := false
	pages := 0
	for {
		if pages >= EnrichmentCap {
			truncated = true
			break
		}
		out, err := clients.EC2.DescribeInstanceStatus(ctx, &ec2svc.DescribeInstanceStatusInput{
			IncludeAllInstances: aws.Bool(true),
			NextToken:           nextToken,
		})
		pages++
		if err != nil {
			// One account-wide call answers for every row on screen, so its
			// failure leaves every row's status uninspected. Each input ID is
			// marked so the list renders "?" rather than inspected-and-healthy
			// — same contract as the ebs-snap public-share query. The findings
			// the cache-only and user-data passes already produced survive.
			markAllUninspected(&result, resources)
			return result, err
		}
		allInstanceStatuses = append(allInstanceStatuses, out.InstanceStatuses...)
		if out.NextToken == nil {
			break
		}
		nextToken = out.NextToken
	}

	now := time.Now()
	cutoff := now.Add(7 * 24 * time.Hour)

	for _, is := range allInstanceStatuses {
		if is.InstanceId == nil {
			continue
		}
		id := *is.InstanceId
		// Track unmatched: API returned an instance not in the input resources slice.
		if len(knownIDs) > 0 && !knownIDs[id] {
			continue
		}

		// Group rows by FindingCode so each distinct condition on this
		// instance (impaired / initializing / insufficient-data /
		// scheduled-event) becomes its own Finding, even when InstanceStatus
		// and SystemStatus both classify to the same code.
		type condition struct {
			tier   string
			phrase string
			rows   []domain.DetailRow
		}
		conditions := make(map[domain.FindingCode]*condition)
		addRow := func(code domain.FindingCode, tier, phrase string, row domain.DetailRow) {
			c, ok := conditions[code]
			if !ok {
				c = &condition{tier: tier, phrase: phrase}
				conditions[code] = c
			}
			c.rows = append(c.rows, row)
		}

		// Check instance status (docs/resources/ec2.md §4 lines 225-227).
		if is.InstanceStatus != nil {
			if sf, ok := classifyEC2Status(is.InstanceStatus.Status); ok {
				statusStr := domain.HumanizeStatusPhrase(string(is.InstanceStatus.Status))
				addRow(sf.code, sf.tier, sf.phrase, domain.DetailRow{Label: "Instance Status", Value: statusStr, Tier: sf.tier})
			}
		}

		// Check system status (docs/resources/ec2.md §4 lines 225-227).
		if is.SystemStatus != nil {
			if sf, ok := classifyEC2Status(is.SystemStatus.Status); ok {
				statusStr := domain.HumanizeStatusPhrase(string(is.SystemStatus.Status))
				addRow(sf.code, sf.tier, sf.phrase, domain.DetailRow{Label: "System Status", Value: statusStr, Tier: sf.tier})
			}
		}

		// Check scheduled events within 7 days.
		// NotBeforeDeadline is the hard deadline (forced retirement/reboot).
		// NotBefore is the earliest scheduled start — also within 7d is actionable.
		for _, ev := range is.Events {
			var eventDate *time.Time
			if ev.NotBeforeDeadline != nil && ev.NotBeforeDeadline.Before(cutoff) {
				eventDate = ev.NotBeforeDeadline
			} else if ev.NotBefore != nil && ev.NotBefore.Before(cutoff) {
				eventDate = ev.NotBefore
			}
			if eventDate == nil {
				continue
			}
			code := string(ev.Code)
			dateStr := eventDate.Format("2006-01-02")
			value := fmt.Sprintf("%s at %s", code, dateStr)
			addRow(ec2CodeScheduledEvent, "~", fmt.Sprintf("scheduled event: %s", value), domain.DetailRow{
				Label: "Scheduled Event",
				Value: value,
				Tier:  "~",
			})
		}

		if len(conditions) == 0 {
			continue
		}

		// Emit findings in a stable order so output is deterministic across runs.
		codes := make([]domain.FindingCode, 0, len(conditions))
		for code := range conditions {
			codes = append(codes, code)
		}
		slices.Sort(codes)
		for _, code := range codes {
			c := conditions[code]
			setWave2Finding(&result, id, code, c.phrase, c.tier, "ec2", c.rows)
		}
	}

	result.Truncated = result.Truncated || truncated
	return result, nil
}

// ec2InternetExposure is the ec2 ↔ sg cross-reference: an instance holding a
// public IP whose security groups already carry sg.go's risk verdict is
// reachable from the internet on the ports that verdict names. It makes NO
// API call and re-derives nothing — the sensitive-port set and the exposure
// rules stay owned by sg.go, this pass only joins them to the instance.
// An unloaded "sg" cache is silence, not a clean bill of health.
func ec2InternetExposure(result *IssueEnricherResult, resources []resource.Resource, cache resource.ResourceCache) {
	sgEntry, ok := cache["sg"]
	if !ok {
		return
	}
	type sgRisk struct {
		wideOpen bool
		ports    string
	}
	risk := make(map[string]sgRisk, len(sgEntry.Resources))
	for _, sg := range sgEntry.Resources {
		risk[sg.ID] = sgRisk{
			wideOpen: sg.Fields["wide_open"] == "true",
			ports:    sgPortsFromRiskSummary(sg.Fields["risk_summary"]),
		}
	}

	for _, r := range resources {
		publicIP := r.Fields["public_ip"]
		if publicIP == "" || r.Fields["state"] != "running" {
			continue
		}
		inst, ok := assertStruct[ec2types.Instance](r.RawStruct)
		if !ok {
			continue
		}
		var groupIDs []string
		wideOpen := false
		portSet := map[string]bool{}
		for _, g := range inst.SecurityGroups {
			id := aws.ToString(g.GroupId)
			ri, known := risk[id]
			if !known {
				continue
			}
			groupIDs = append(groupIDs, id)
			if ri.wideOpen {
				wideOpen = true
			}
			for p := range strings.SplitSeq(ri.ports, ", ") {
				if p != "" {
					portSet[p] = true
				}
			}
		}
		portList := "all"
		if !wideOpen {
			if len(portSet) == 0 {
				continue
			}
			ports := make([]string, 0, len(portSet))
			for p := range portSet {
				ports = append(ports, p)
			}
			sort.Slice(ports, func(i, j int) bool {
				a, _ := strconv.Atoi(ports[i])
				b, _ := strconv.Atoi(ports[j])
				return a < b
			})
			portList = strings.Join(ports, ", ")
		}
		sort.Strings(groupIDs)
		setWave2Finding(result, r.ID, ec2CodeInternetExposed,
			"port(s) "+portList+" reachable from the internet", "!", "ec2",
			[]domain.DetailRow{
				{Label: "Public address", Value: publicIP, Tier: "!"},
				{Label: "Security groups", Value: strings.Join(groupIDs, ", "), Tier: "!"},
				{Label: "Ports", Value: portList, Tier: "!"},
			})

	}
}

// ec2UserDataSecrets reads each instance's user-data script via
// DescribeInstanceAttribute and reports plaintext credentials found in it.
// Terminated and shutting-down instances are skipped: their user data can no
// longer be changed and the host is gone. The call is not part of the EC2API
// aggregate (see ec2_interfaces.go), so a client that cannot serve it — every
// narrow test double — yields no findings rather than an error.
func ec2UserDataSecrets(ctx context.Context, clients *ServiceClients, resources []resource.Resource, result *IssueEnricherResult) error {
	api, ok := clients.EC2.(EC2DescribeInstanceAttributeAPI)
	if !ok {
		return nil
	}
	var targets []resource.Resource
	for _, r := range resources {
		if r.ID == "" {
			continue
		}
		if ec2InstanceGone(r.Fields["state"]) {
			continue
		}
		targets = append(targets, r)
	}
	if len(targets) > EnrichmentCap {
		result.Truncated = true
		targets = targets[:EnrichmentCap]
	}

	const op = "ec2-enrich: DescribeInstanceAttribute(userData)"
	var mu sync.Mutex
	var failures []string
	_ = ForEachParallel(ctx, len(targets), EnrichmentParallelism, func(i int) {
		r := targets[i]
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*ec2svc.DescribeInstanceAttributeOutput, error) {
			return api.DescribeInstanceAttribute(ctx, &ec2svc.DescribeInstanceAttributeInput{
				InstanceId: aws.String(r.ID),
				Attribute:  ec2types.InstanceAttributeNameUserData,
			})
		})
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			if IsNotFoundErr(err) {
				result.TruncatedIDs[r.ID] = true
				return
			}
			MarkSkipped(result, r.ID, &failures, op, err)
			return
		}
		if out == nil || out.UserData == nil || aws.ToString(out.UserData.Value) == "" {
			return
		}
		hits := secretscan.ScanText(decodeUserData(*out.UserData.Value))
		if len(hits) == 0 {
			return
		}
		rows := make([]domain.DetailRow, 0, len(hits))
		for _, h := range hits {
			rows = append(rows, domain.DetailRow{Label: h.Where, Value: h.Kind, Tier: "!"})
		}
		setWave2Finding(result, r.ID, ec2CodeUserDataSecret, "credential in user data", "!", "ec2",
			rows)

	})
	sort.Strings(failures)
	return Finish(result, failures, len(targets), op)
}
