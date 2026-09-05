// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// sgCodeWideOpen is the canonical FindingCode for a security group with an
// all-protocols (-1) ingress rule open to 0.0.0.0/0 or ::/0.
const sgCodeWideOpen domain.FindingCode = "sg.ingress.wide-open"

// sgCodeDangerousPorts is the canonical FindingCode for a security group
// exposing one or more sensitive ports (SSH, RDP, database, etc.) to the
// public internet.
const sgCodeDangerousPorts domain.FindingCode = "sg.ingress.dangerous-ports"

// sgCodeDefaultWithRules is the canonical FindingCode for a VPC's default
// security group that still carries rules. AWS attaches it to anything
// launched without an explicit group, so every rule on it applies to
// resources nobody chose to put there.
const sgCodeDefaultWithRules domain.FindingCode = "sg.default-with-rules"

// sgDefaultWithRulesPhrase is the S4 status phrase for sgCodeDefaultWithRules.
const sgDefaultWithRulesPhrase = "default group allows traffic"

// sgDefaultWithRulesDetail is the S5 operator sentence for sgCodeDefaultWithRules.
const sgDefaultWithRulesDetail = "The VPC's default security group still carries rules, and AWS attaches it to " +
	"any resource launched without an explicit group. Remove every ingress rule and every egress rule other than " +
	"the AWS-created allow-all, and give each workload its own group."

// sensitivePorts is the set of ports that are considered security-sensitive
// when exposed to the internet (0.0.0.0/0 or ::/0). Single source: both the
// finding and the risk_summary display phrase enumerate this map, so a port
// added here is exposed on every surface at once.
//
// 8080 and 8443 are deliberately absent: they front ordinary public HTTP(S)
// applications far more often than anything worth paging on, and adding them
// turns the signal into noise.
var sensitivePorts = map[int32]bool{
	20:    true, // FTP data
	21:    true, // FTP control
	22:    true, // SSH
	23:    true, // Telnet
	25:    true, // SMTP
	445:   true, // SMB
	1433:  true, // MSSQL
	1521:  true, // Oracle TNS listener
	2483:  true, // Oracle TTC
	3306:  true, // MySQL
	3389:  true, // RDP
	5432:  true, // PostgreSQL
	5601:  true, // Kibana
	6379:  true, // Redis
	7199:  true, // Cassandra JMX
	9092:  true, // Kafka
	9160:  true, // Cassandra Thrift
	9200:  true, // Elasticsearch
	8888:  true, // Cassandra OpsCenter
	11211: true, // Memcached
	27017: true, // MongoDB
}

// coversSensitivePort reports whether the given IpPermission covers any
// sensitive port when open to the public internet.
func coversSensitivePort(p ec2types.IpPermission) bool {
	// All-protocols rule: covers every port.
	if aws.ToString(p.IpProtocol) == "-1" {
		return true
	}
	proto := aws.ToString(p.IpProtocol)
	if proto != "tcp" && proto != "udp" {
		return false
	}
	from := p.FromPort
	to := p.ToPort
	if from == nil || to == nil {
		return false
	}
	for port := range sensitivePorts {
		if *from <= port && port <= *to {
			return true
		}
	}
	return false
}

// isInternetFacing reports whether the IpPermission is open to the public
// internet via 0.0.0.0/0 or ::/0.
func isInternetFacing(p ec2types.IpPermission) bool {
	for _, r := range p.IpRanges {
		if aws.ToString(r.CidrIp) == "0.0.0.0/0" {
			return true
		}
	}
	for _, r := range p.Ipv6Ranges {
		if aws.ToString(r.CidrIpv6) == "::/0" {
			return true
		}
	}
	return false
}

// computeSGRiskFields inspects the ingress rules of a security group and
// returns (dangerous_open_count, wide_open, risk_summary).
//
//	dangerous_open_count and wide_open are the machine fields the ec2
//	internet-exposure signal reads through the sg cache; they are never
//	rewritten by the humanizer.
//
//	risk_summary is a display-only, owner-worded phrase for the list Risk
//	column and the detail "Risk Summary" row:
//	  ""                                  — no internet exposure on dangerous ports
//	  "all ports open to 0.0.0.0/0"        — at least one rule with all-protocols (-1) open to 0.0.0.0/0
//	  "ports 22, 3306 open to 0.0.0.0/0"   — specific dangerous ports open to 0.0.0.0/0
//	When both wide-open and specific ports are present, the wide-open phrase
//	wins (it's the more severe signal) — mirrors sgRiskFindings' precedence.
func computeSGRiskFields(perms []ec2types.IpPermission) (string, string, string) {
	dangerousCount := 0
	wideOpen := false
	portSet := make(map[int32]struct{})
	for _, p := range perms {
		if !isInternetFacing(p) {
			continue
		}
		if aws.ToString(p.IpProtocol) == "-1" {
			wideOpen = true
			continue
		}
		if coversSensitivePort(p) {
			dangerousCount++
			// Capture the specific port(s) covered.
			if p.FromPort != nil && p.ToPort != nil {
				from, to := *p.FromPort, *p.ToPort
				if to < from {
					from, to = to, from
				}
				// Always enumerate the dangerous ports that fall inside [from, to].
				// We iterate the sensitive-port set (small, constant) rather than the
				// range itself, so a 1-65535 rule stays O(|sensitivePorts|) and
				// correctly surfaces every dangerous port the rule actually exposes.
				for sp := range sensitivePorts {
					if sp >= from && sp <= to {
						portSet[sp] = struct{}{}
					}
				}
			}
		}
	}
	wideOpenStr := "false"
	if wideOpen {
		wideOpenStr = "true"
	}
	riskSummary := ""
	switch {
	case wideOpen:
		riskSummary = sgWideOpenPhrase
	case dangerousCount > 0:
		ports := make([]int, 0, len(portSet))
		for p := range portSet {
			ports = append(ports, int(p))
		}
		sort.Ints(ports)
		parts := make([]string, len(ports))
		for i, p := range ports {
			parts[i] = strconv.Itoa(p)
		}
		if len(parts) > 0 {
			riskSummary = sgDangerousPortsPhrase(strings.Join(parts, ", "))
		} else {
			// dangerousCount > 0 but no specific port captured (large-range case).
			riskSummary = sgDangerousPortsPhrase("unspecified")
		}
	}
	return strconv.Itoa(dangerousCount), wideOpenStr, riskSummary
}

// sgWideOpenPhrase is the single owner-worded source for the all-protocols
// exposure phrase, shared by risk_summary (display) and sgRiskFindings
// (the Broken-color explanation) so the two never drift.
const sgWideOpenPhrase = "all ports open to 0.0.0.0/0"

// sgDangerousPortsPhrase is the single owner-worded source for the
// specific-ports exposure phrase, shared by risk_summary (display) and
// sgRiskFindings (the Broken-color explanation) so the two never drift.
func sgDangerousPortsPhrase(ports string) string {
	return "ports " + ports + " open to 0.0.0.0/0"
}

// sgPortsFromRiskSummary is the inverse of sgDangerousPortsPhrase: it recovers
// the comma-separated port list a security group's risk_summary field encodes.
// It lives beside the formatter so the two can never drift, and it is the only
// way a consumer outside sg.go (the ec2 internet-exposure cross-ref) learns
// which ports a group leaves open — the sensitive-port set stays owned here.
// Returns "" for a summary that names no specific ports.
func sgPortsFromRiskSummary(summary string) string {
	if !strings.HasPrefix(summary, "ports ") {
		return ""
	}
	ports := strings.TrimSuffix(strings.TrimPrefix(summary, "ports "), " open to 0.0.0.0/0")
	if ports == summary || ports == "unspecified" {
		return ""
	}
	return ports
}

// sgRiskFindings ranks wide-open ahead of dangerous ports so the list Status
// cell / detail Attention block explain the Broken color with the same
// owner-worded phrase risk_summary carries for display. The row color derives
// from these findings alone (colorAnyFindingOrHealthy) — wide_open and
// dangerous_open_count are consumed by other types, never by the classifier.
func sgRiskFindings(wideOpen, dangerousOpenCount, riskSummary string) []domain.Finding {
	switch {
	case wideOpen == "true":
		return []domain.Finding{{
			Code: sgCodeWideOpen, Phrase: sgWideOpenPhrase,
			Severity: domain.SevBroken, Source: "wave1",
		}}
	case dangerousOpenCount != "" && dangerousOpenCount != "0":
		return []domain.Finding{{
			Code: sgCodeDangerousPorts, Phrase: riskSummary,
			Severity: domain.SevBroken, Source: "wave1",
		}}
	}
	return nil
}

// sgDefaultAllowsTraffic reports whether sg is a VPC's default security
// group that still carries rules. Every group is created with an allow-all
// egress rule, so that one rule alone is the untouched state, not a signal.
func sgDefaultAllowsTraffic(sg ec2types.SecurityGroup) bool {
	if aws.ToString(sg.GroupName) != "default" {
		return false
	}
	return len(sg.IpPermissions) > 0 || !isAWSDefaultEgress(sg.IpPermissionsEgress)
}

// isAWSDefaultEgress reports whether perms is exactly the egress rule AWS
// creates with every security group: all protocols to 0.0.0.0/0, nothing else.
func isAWSDefaultEgress(perms []ec2types.IpPermission) bool {
	if len(perms) == 0 {
		return true
	}
	if len(perms) != 1 {
		return false
	}
	p := perms[0]
	return aws.ToString(p.IpProtocol) == "-1" &&
		len(p.IpRanges) == 1 && aws.ToString(p.IpRanges[0].CidrIp) == "0.0.0.0/0" &&
		len(p.Ipv6Ranges) == 0 && len(p.UserIdGroupPairs) == 0 && len(p.PrefixListIds) == 0
}

// FetchSecurityGroupsPage calls the EC2 DescribeSecurityGroups API and returns
// a single page of security groups. Pass an empty continuationToken for the first page.
func FetchSecurityGroupsPage(ctx context.Context, api EC2DescribeSecurityGroupsAPI, continuationToken string) (resource.FetchResult, error) {
	input := &ec2.DescribeSecurityGroupsInput{
		MaxResults: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*ec2.DescribeSecurityGroupsOutput, error) {
		return api.DescribeSecurityGroups(ctx, input)
	})
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching security groups: %w", err)
	}

	var resources []resource.Resource
	for _, sg := range output.SecurityGroups {
		// Extract GroupId
		groupID := ""
		if sg.GroupId != nil {
			groupID = *sg.GroupId
		}

		// Extract GroupName
		groupName := ""
		if sg.GroupName != nil {
			groupName = *sg.GroupName
		}

		// Extract VpcId
		vpcID := ""
		if sg.VpcId != nil {
			vpcID = *sg.VpcId
		}

		// Extract Description
		description := ""
		if sg.Description != nil {
			description = *sg.Description
		}

		dangerousCount, wideOpen, riskSummary := computeSGRiskFields(sg.IpPermissions)

		findings := sgRiskFindings(wideOpen, dangerousCount, riskSummary)
		var attentionDetails map[domain.FindingCode]domain.AttentionDetail
		if sgDefaultAllowsTraffic(sg) {
			findings = append(findings, domain.Finding{
				Code:     sgCodeDefaultWithRules,
				Phrase:   sgDefaultWithRulesPhrase,
				Detail:   sgDefaultWithRulesDetail,
				Severity: domain.SevWarn,
				Source:   "wave1",
			})
			attentionDetails = map[domain.FindingCode]domain.AttentionDetail{
				sgCodeDefaultWithRules: {Rows: []domain.DetailRow{
					{Label: "Ingress rules", Value: strconv.Itoa(len(sg.IpPermissions)), Tier: "~"},
					{Label: "Egress rules", Value: strconv.Itoa(len(sg.IpPermissionsEgress)), Tier: "~"},
				}},
			}
		}

		r := resource.Resource{
			ID:   groupID,
			Name: groupName,
			Fields: map[string]string{
				"group_id":             groupID,
				"group_name":           groupName,
				"vpc_id":               vpcID,
				"description":          description,
				"dangerous_open_count": dangerousCount,
				"wide_open":            wideOpen,
				"risk_summary":         riskSummary,
			},
			Findings:         findings,
			AttentionDetails: attentionDetails,
			RawStruct:        sg,
		}

		resources = append(resources, r)
	}

	// Build pagination metadata
	nextToken := ""
	isTruncated := false
	if output.NextToken != nil {
		nextToken = *output.NextToken
		isTruncated = true
	}

	totalHint := len(resources)
	if isTruncated {
		totalHint = -1
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: isTruncated,
			NextToken:   nextToken,
			PageSize:    len(resources),
			TotalHint:   totalHint,
		},
	}, nil
}
