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
// internet. Both address families are one question, answered by
// CIDROpenToEveryone: a rule reaching every IPv6 client is internet-facing
// whether or not it also names an IPv4 range.
func isInternetFacing(p ec2types.IpPermission) bool {
	cidrs := make([]string, 0, len(p.IpRanges)+len(p.Ipv6Ranges))
	for _, r := range p.IpRanges {
		cidrs = append(cidrs, aws.ToString(r.CidrIp))
	}
	for _, r := range p.Ipv6Ranges {
		cidrs = append(cidrs, aws.ToString(r.CidrIpv6))
	}
	return CIDROpenToEveryone(cidrs)
}

// computeSGRiskFields inspects the ingress rules of a security group and
// returns (dangerous_open_count, wide_open, open_ports). All three are machine
// fields: the ec2 internet-exposure signal reads wide_open and open_ports
// through the sg cache, and no consumer reads a rendered sentence.
//
// open_ports is the sorted, comma-separated list of sensitive ports the group
// leaves open to the internet, empty when it leaves none. A wide-open group
// reports no list: every port is open, which wide_open already says.
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
	openPorts := ""
	if !wideOpen && len(portSet) > 0 {
		ports := make([]int, 0, len(portSet))
		for p := range portSet {
			ports = append(ports, int(p))
		}
		sort.Ints(ports)
		parts := make([]string, len(ports))
		for i, p := range ports {
			parts[i] = strconv.Itoa(p)
		}
		openPorts = strings.Join(parts, ", ")
	}
	return strconv.Itoa(dangerousCount), wideOpenStr, openPorts
}

// sgRiskFindings ranks wide-open ahead of dangerous ports: a rule opening every
// port says everything the specific list would. The row colour derives from
// these findings alone (colorAnyFindingOrHealthy), and risk_summary is the
// phrase they carry, so the Status cell and the Attention block cannot word one
// verdict two ways.
//
// The ports arrive as the list, never as a sentence to take apart: an empty one
// beside a non-zero count reaches the slot filler, which refuses it, rather than
// quietly rendering a group with an open port as clean.
func sgRiskFindings(wideOpen, dangerousOpenCount, openPorts string) []domain.Finding {
	switch {
	case wideOpen == "true":
		return []domain.Finding{wave1Finding(sgCodeWideOpen)}
	case dangerousOpenCount != "" && dangerousOpenCount != "0":
		return []domain.Finding{wave1Finding(sgCodeDangerousPorts, openPorts)}
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

		dangerousCount, wideOpen, openPorts := computeSGRiskFields(sg.IpPermissions)

		findings := sgRiskFindings(wideOpen, dangerousCount, openPorts)
		// The Status cell is the risk verdict's own phrase, read back off the
		// finding rather than assembled a second time. Taken before the
		// default-group finding is appended, which is a separate fact and does
		// not belong in the risk cell.
		riskSummary := domain.StatusPhrase(findings)
		var attentionDetails map[domain.FindingCode]domain.AttentionDetail
		if sgDefaultAllowsTraffic(sg) {
			findings = append(findings, wave1Finding(sgCodeDefaultWithRules))
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
				"open_ports":           openPorts,
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
