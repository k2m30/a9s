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
)

// ec2 canonical FindingCodes. Each condition gets its OWN code so it carries
// its own FindingDef phrase, severity and Attention entry, instead of folding
// every status/event condition under one code and one phrase.
const (
	ec2CodeInstanceStatusImpaired     domain.FindingCode = "ec2.instance-status-impaired"
	ec2CodeInstanceStatusInitializing domain.FindingCode = "ec2.instance-status.initializing"
	ec2CodeInstanceStatusInsufficient domain.FindingCode = "ec2.instance-status.insufficient-data"
	ec2CodeScheduledEvent             domain.FindingCode = "ec2.scheduled-event"
	ec2CodeInternetExposed            domain.FindingCode = "ec2.internet-exposed"
	ec2CodeInternetExposedAll         domain.FindingCode = "ec2.internet-exposed-all"
	//nolint:gosec // G101 false positive: a finding code, not a credential
	ec2CodeUserDataSecret domain.FindingCode = "ec2.user-data-secret"
)

// classifyEC2Status maps an AWS instance/system status-check value to its
// FindingCode and severity per docs/resources/ec2.md. Only "impaired" is Broken;
// "initializing" and "insufficient-data" are Warning and must never carry
// the "impaired" wording, and each gets its own code. "ok" and
// "not-applicable" produce no finding — AWS classifies "not-applicable" as
// Healthy/informational.
func classifyEC2Status(status ec2types.SummaryStatus) (domain.FindingCode, bool) {
	switch status {
	case ec2types.SummaryStatusImpaired:
		return ec2CodeInstanceStatusImpaired, true
	case ec2types.SummaryStatusInitializing:
		return ec2CodeInstanceStatusInitializing, true
	case ec2types.SummaryStatusInsufficientData:
		return ec2CodeInstanceStatusInsufficient, true
	default:
		return "", false
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
		TruncatedIDs:     make(map[string]string),
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
	// One account-wide call answers for every row on screen, so a walk that
	// stops early leaves the rows it never named uninspected — the list then
	// renders "?" rather than inspected-and-healthy, the same contract as the
	// ebs-snap public-share query. The findings the cache-only and user-data
	// passes already produced survive.
	allInstanceStatuses, _, cut, walkErr := walkAccountPages(&result, resources, oneItemPerRow,
		func(is ec2types.InstanceStatus) string { return aws.ToString(is.InstanceId) },
		func(token *string) ([]ec2types.InstanceStatus, *string, error) {
			out, err := clients.EC2.DescribeInstanceStatus(ctx, &ec2svc.DescribeInstanceStatusInput{
				IncludeAllInstances: aws.Bool(true),
				NextToken:           token,
			})
			if err != nil {
				return nil, nil, err
			}
			return out.InstanceStatuses, out.NextToken, nil
		})

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
			tier string
			rows []domain.DetailRow
		}
		conditions := make(map[domain.FindingCode]*condition)
		addRow := func(code domain.FindingCode, tier string, row domain.DetailRow) {
			c, ok := conditions[code]
			if !ok {
				c = &condition{tier: tier}
				conditions[code] = c
			}
			c.rows = append(c.rows, row)
		}

		if is.InstanceStatus != nil {
			if code, ok := classifyEC2Status(is.InstanceStatus.Status); ok {
				statusStr := domain.HumanizeStatusPhrase(string(is.InstanceStatus.Status))
				addRow(code, tierOf(code), domain.DetailRow{Label: "Instance Status", Value: statusStr, Tier: tierOf(code)})
			}
		}

		if is.SystemStatus != nil {
			if code, ok := classifyEC2Status(is.SystemStatus.Status); ok {
				statusStr := domain.HumanizeStatusPhrase(string(is.SystemStatus.Status))
				addRow(code, tierOf(code), domain.DetailRow{Label: "System Status", Value: statusStr, Tier: tierOf(code)})
			}
		}

		// NotBeforeDeadline is the hard deadline (forced retirement/reboot).
		// NotBefore is the earliest scheduled start — also within 7d is actionable.
		for _, ev := range is.Events {
			// AWS keeps a finished event describable for a week and marks it
			// by prefixing the description with "[Completed]"; there is
			// nothing left for the operator to plan around.
			if strings.HasPrefix(aws.ToString(ev.Description), "[Completed]") {
				continue
			}
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
			addRow(ec2CodeScheduledEvent, "~", domain.DetailRow{
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
			setWave2Finding(&result, id, code, c.rows)
		}
	}

	SetTruncated(&result, cut)
	return result, walkErr
}

// ec2InternetExposure is the ec2 ↔ sg ↔ rtb cross-reference: a public
// address whose interface's security groups open a sensitive port, or every
// port, to everyone on that address's family, in a subnet whose route table
// sends that family's default route to an internet gateway, is reachable from
// the internet. No API call; the exposure rule per address family is owned by
// sg.go (sgFamilyExposure), and this pass only joins it to each interface.
//
// A NAT gateway or an egress-only gateway admits nothing inbound, so an
// address behind one is not exposed. A group or a route table the loaded
// lists cannot answer for marks the row not inspected — beside the finding
// when one was found, since the unread part could widen it.
func ec2InternetExposure(result *IssueEnricherResult, resources []resource.Resource, cache resource.ResourceCache) {
	sgEntry, sgLoaded := cache["sg"]
	rtbEntry, rtbLoaded := cache["rtb"]
	groups := make(map[string]ec2types.SecurityGroup, len(sgEntry.Resources))
	for _, sg := range sgEntry.Resources {
		if raw, ok := assertStruct[ec2types.SecurityGroup](sg.RawStruct); ok {
			groups[sg.ID] = raw
		}
	}

	for _, r := range resources {
		if r.Fields["state"] != "running" {
			continue
		}
		inst, ok := assertStruct[ec2types.Instance](r.RawStruct)
		if !ok {
			continue
		}
		ifaces := ec2Interfaces(inst)
		if !slices.ContainsFunc(ifaces, ec2Interface.public) {
			continue
		}
		if !sgLoaded {
			markUninspected(result, r.ID, checkListIncomplete("sg"))
			continue
		}
		var all ec2Exposure
		var addresses, groupIDs []string
		groupUnread, routeUnread := false, ""
		for _, ifc := range ifaces {
			for _, fam := range []struct {
				ipv6  bool
				addrs []string
				dest  string
			}{{false, ifc.ipv4, "0.0.0.0/0"}, {true, ifc.ipv6, "::/0"}} {
				if len(fam.addrs) == 0 {
					continue
				}
				var e ec2Exposure
				for _, g := range ifc.groups {
					id := aws.ToString(g.GroupId)
					sg, known := groups[id]
					// A live instance's group cannot be absent from the
					// account's groups; missing from any list, the list is
					// short or stale, and says nothing about its rules.
					if !known {
						groupUnread = true
						continue
					}
					if !slices.Contains(groupIDs, id) {
						groupIDs = append(groupIDs, id)
					}
					e.add(sgFamilyExposure(sg, fam.ipv6))
				}
				if !e.exposed() {
					continue
				}
				routed, unread := ec2RoutedToInternet(aws.ToString(inst.VpcId), ifc.subnet, fam.dest, rtbEntry, rtbLoaded)
				if routeUnread == "" || unread == checkInspectionEndpointRoute {
					routeUnread = unread
				}
				if !routed {
					continue
				}
				all.add(e.wideOpen, strings.Join(e.ports, ", "))
				addresses = append(addresses, fam.addrs...)
			}
		}
		switch {
		case groupUnread:
			markUninspected(result, r.ID, checkListIncomplete("sg"))
		case routeUnread != "":
			markUninspected(result, r.ID, routeUnread)
		}
		if !all.exposed() {
			continue
		}
		portList := "all"
		if !all.wideOpen {
			sort.Slice(all.ports, func(i, j int) bool {
				a, _ := strconv.Atoi(all.ports[i])
				b, _ := strconv.Atoi(all.ports[j])
				return a < b
			})
			portList = strings.Join(all.ports, ", ")
		}
		sort.Strings(groupIDs)
		rows := []domain.DetailRow{
			{Label: "Public address", Value: strings.Join(addresses, ", "), Tier: "!"},
			{Label: "Security groups", Value: strings.Join(groupIDs, ", "), Tier: "!"},
			{Label: "Ports", Value: portList, Tier: "!"},
		}
		// A group admitting every protocol is a different statement from a
		// list of ports, so it carries its own code and its own sentence
		// rather than a list whose only member is the word "all".
		if all.wideOpen {
			setWave2Finding(result, r.ID, ec2CodeInternetExposedAll, rows)
			continue
		}
		setWave2Finding(result, r.ID, ec2CodeInternetExposed, rows, portList)
	}
}

// ec2Exposure accumulates sgFamilyExposure verdicts across groups and
// interfaces: wide open when any is, else the union of their ports.
type ec2Exposure struct {
	wideOpen bool
	ports    []string
}

func (e *ec2Exposure) add(wideOpen bool, ports string) {
	e.wideOpen = e.wideOpen || wideOpen
	for p := range strings.SplitSeq(ports, ", ") {
		if p != "" && !slices.Contains(e.ports, p) {
			e.ports = append(e.ports, p)
		}
	}
}

func (e ec2Exposure) exposed() bool { return e.wideOpen || len(e.ports) > 0 }

// ec2Interface is one network interface's public addresses, security groups
// and subnet: AWS applies groups and routes per interface, not per instance.
type ec2Interface struct {
	subnet string
	groups []ec2types.GroupIdentifier
	ipv4   []string
	ipv6   []string
}

func (i ec2Interface) public() bool { return len(i.ipv4)+len(i.ipv6) > 0 }

// ec2Interfaces reads the instance's interfaces: each one's public IPv4
// addresses (the association of each of its private addresses, which is
// where an Elastic IP on a secondary interface lands) and its IPv6 addresses.
// The instance-level public address, IPv6 address, subnet and groups are the
// primary interface's (device index 0); they fill in whatever the reported
// interfaces leave out, and stand alone for an instance reported without any.
func ec2Interfaces(inst ec2types.Instance) []ec2Interface {
	out := make([]ec2Interface, 0, len(inst.NetworkInterfaces)+1)
	primary := 0
	for _, eni := range inst.NetworkInterfaces {
		i := ec2Interface{subnet: aws.ToString(eni.SubnetId), groups: eni.Groups}
		if eni.Association != nil {
			i.ipv4 = appendNew(i.ipv4, aws.ToString(eni.Association.PublicIp))
		}
		for _, p := range eni.PrivateIpAddresses {
			if p.Association != nil {
				i.ipv4 = appendNew(i.ipv4, aws.ToString(p.Association.PublicIp))
			}
		}
		for _, a := range eni.Ipv6Addresses {
			i.ipv6 = appendNew(i.ipv6, aws.ToString(a.Ipv6Address))
		}
		if eni.Attachment != nil && aws.ToInt32(eni.Attachment.DeviceIndex) == 0 {
			primary = len(out)
		}
		out = append(out, i)
	}
	if len(out) == 0 {
		out = append(out, ec2Interface{})
	}
	carried := func(addr string, v6 bool) bool {
		return slices.ContainsFunc(out, func(i ec2Interface) bool {
			if v6 {
				return slices.Contains(i.ipv6, addr)
			}
			return slices.Contains(i.ipv4, addr)
		})
	}
	p := &out[primary]
	if v4 := aws.ToString(inst.PublicIpAddress); !carried(v4, false) {
		p.ipv4 = appendNew(p.ipv4, v4)
	}
	if v6 := aws.ToString(inst.Ipv6Address); !carried(v6, true) {
		p.ipv6 = appendNew(p.ipv6, v6)
	}
	if p.subnet == "" {
		p.subnet = aws.ToString(inst.SubnetId)
	}
	if len(p.groups) == 0 {
		p.groups = inst.SecurityGroups
	}
	return out
}

// appendNew appends s to list unless it is empty or already there.
func appendNew(list []string, s string) []string {
	if s == "" || slices.Contains(list, s) {
		return list
	}
	return append(list, s)
}

// checkInspectionEndpointRoute is what an instance records when its subnet
// sends a default route to a VPC endpoint: a Gateway Load Balancer endpoint
// in front of an inspection appliance, or a Network Firewall endpoint
// (docs.aws.amazon.com/elasticloadbalancing/latest/gateway/getting-started-cli.html#configure-routing-aws-cli,
// docs.aws.amazon.com/network-firewall/latest/developerguide/vpc-config-route-tables.html).
// The appliance's rules decide what reaches the instance, and neither the
// security groups nor the route tables say what they are. DescribeRouteTables
// reports the endpoint ID in the route's GatewayId
// (docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Route.html). A gateway
// endpoint (S3, DynamoDB) carries a vpce- GatewayId too, on a prefix-list
// destination, so only the default route's target is read this way.
const checkInspectionEndpointRoute = "default route through an inspection endpoint"

// ec2RoutedToInternet reports whether subnet's route table sends dest (an
// address family's default route, "0.0.0.0/0" or "::/0") to an internet
// gateway. unread is "" when the loaded rtb list tells, and otherwise the
// check the route could not decide: the rtb list, or a default route to an
// inspection endpoint. subnetRouteTableIDs owns which table serves the
// subnet, falling back to the VPC's main table only on a complete list.
func ec2RoutedToInternet(vpc, subnet, dest string, rtb resource.ResourceCacheEntry, loaded bool) (routed bool, unread string) {
	unread = checkListIncomplete("rtb")
	if !loaded {
		return false, unread
	}
	ids := subnetRouteTableIDs(subnet, vpc, rtb.Resources, !rtb.IsTruncated)
	for _, row := range rtb.Resources {
		if !slices.Contains(ids, row.ID) {
			continue
		}
		table, ok := assertStruct[ec2types.RouteTable](row.RawStruct)
		if !ok {
			continue
		}
		unread = ""
		for _, route := range table.Routes {
			to := aws.ToString(route.DestinationCidrBlock)
			if dest == "::/0" {
				to = aws.ToString(route.DestinationIpv6CidrBlock)
			}
			switch {
			case to != dest || route.State == ec2types.RouteStateBlackhole:
			case routeGatewayTarget(route, "igw") != "":
				return true, ""
			case strings.HasPrefix(aws.ToString(route.GatewayId), "vpce-"):
				return false, checkInspectionEndpointRoute
			}
		}
	}
	return false, unread
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
	targets := capAtEnrichmentCap(result, resources, func(r resource.Resource) bool {
		return r.ID != "" && !ec2InstanceGone(r.Fields["state"])
	}, resourceIDsOf)

	const op = "DescribeInstanceAttribute(userData)"
	var mu sync.Mutex
	var failures []Failure
	loopErr := ForEachRow(ctx, result, resourceIDs(targets), EnrichmentParallelism, func(i int) {
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
			MarkSkipped(result, r.ID, &failures, err)
			return
		}
		if out == nil || out.UserData == nil || aws.ToString(out.UserData.Value) == "" {
			return
		}
		rows := secretScanTextRows(decodeUserData(*out.UserData.Value))
		if len(rows) == 0 {
			return
		}
		setWave2Finding(result, r.ID, ec2CodeUserDataSecret, rows)

	})

	return errors.Join(loopErr, Finish(result, failures, len(targets), op))
}
