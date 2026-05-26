// vpc_issue_enrichment.go — Wave 2 issue enrichment for the vpc resource type.
package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2svc "github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// vpc canonical FindingCodes.
const (
	vpcCodeNoFlowLogs domain.FindingCode = "vpc.no-flow-logs"
)

// EnrichVPCFlowLogs calls DescribeFlowLogs per VPC (capped at EnrichmentCap) and
// raises a finding when no ACTIVE flow log exists.
//
// Findings:
//   - No ACTIVE flow log for the VPC → "~" finding "no active VPC flow logs (CIS EC2.6)"
//
// IssueCount stays 0 (severity "~" only).
// Skip when clients.EC2 == nil.
func EnrichVPCFlowLogs(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string]domain.Finding),
		TruncatedIDs: make(map[string]bool),
		FieldUpdates: make(map[string]map[string]string),
	}
	if clients.EC2 == nil {
		return result, nil
	}
	truncated := len(resources) > EnrichmentCap
	for i, r := range resources {
		if i >= EnrichmentCap {
			break
		}
		vpcID := r.ID
		if vpcID == "" {
			continue
		}
		// Paginate through all flow logs for this VPC.
		var allFlowLogs []ec2types.FlowLog
		var flNextToken *string
		flPages := 0
		flTruncated := false
		for {
			if flPages >= PerParentPageCap {
				flTruncated = true
				truncated = true
				result.TruncatedIDs[r.ID] = true
				break
			}
			out, err := clients.EC2.DescribeFlowLogs(ctx, &ec2svc.DescribeFlowLogsInput{
				Filter: []ec2types.Filter{
					{Name: aws.String("resource-id"), Values: []string{vpcID}},
				},
				NextToken: flNextToken,
			})
			flPages++
			if err != nil {
				truncated = true
				result.TruncatedIDs[r.ID] = true
				flTruncated = true
				break
			}
			allFlowLogs = append(allFlowLogs, out.FlowLogs...)
			if out.NextToken == nil {
				break
			}
			flNextToken = out.NextToken
		}
		if flTruncated {
			continue
		}
		// No flow logs at all, or none with ACTIVE status → finding.
		hasActive := false
		for _, fl := range allFlowLogs {
			if fl.FlowLogStatus != nil && *fl.FlowLogStatus == "ACTIVE" {
				hasActive = true
				break
			}
		}
		flowLogsVal := "yes"
		if !hasActive {
			flowLogsVal = "no"
			setWave2Finding(&result, vpcID, vpcCodeNoFlowLogs, "no active VPC flow logs (CIS EC2.6)", "~", "vpc", nil)
		}
		result.FieldUpdates[vpcID] = map[string]string{
			"flow_logs": flowLogsVal,
		}
	}
	result.IssueCount = 0
	result.Truncated = truncated
	return result, nil
}
