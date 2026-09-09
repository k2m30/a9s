// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// vpc_issue_enrichment.go — Wave 2 issue enrichment for the vpc resource type.
package aws

import (
	"context"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2svc "github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// vpc canonical FindingCodes.
const (
	vpcCodeNoFlowLogs domain.FindingCode = "vpc.no-flow-logs"
)

// EnrichVPCFlowLogs calls DescribeFlowLogs per VPC (capped at EnrichmentCap) and
// raises a finding when no ACTIVE flow log exists.
//
// Findings:
//   - No ACTIVE flow log for the VPC → "~" finding "no active VPC flow logs"
//
// Skip when clients.EC2 == nil.
func EnrichVPCFlowLogs(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]string),
		FieldUpdates: make(map[string]map[string]string),
	}
	if clients.EC2 == nil {
		return result, nil
	}
	resources = capAtEnrichmentCap(&result, resources, resourceIDsOf)
	n := len(resources)
	var failures []Failure
	var mu sync.Mutex
	_ = ForEachParallel(ctx, n, EnrichmentParallelism, func(i int) {
		r := resources[i]
		vpcID := r.ID
		if vpcID == "" {
			return
		}
		// Paginate through all flow logs for this VPC.
		var allFlowLogs []ec2types.FlowLog
		var flNextToken *string
		flPages := 0
		flTruncated := false
		var flErr error
		for {
			if flPages >= PerParentPageCap {
				flTruncated = true
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
				flErr = err
				break
			}
			allFlowLogs = append(allFlowLogs, out.FlowLogs...)
			if out.NextToken == nil {
				break
			}
			flNextToken = out.NextToken
		}

		mu.Lock()
		defer mu.Unlock()
		switch {
		case flErr != nil:
			MarkSkipped(&result, r.ID, &failures, flErr)
			return
		case flTruncated:
			markUninspected(&result, r.ID, checkCap)
			return
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
			setWave2Finding(&result, vpcID, vpcCodeNoFlowLogs, nil)
		}
		result.FieldUpdates[vpcID] = map[string]string{
			"flow_logs": flowLogsVal,
		}
	})
	MarkInformationalOnly(&result)
	return result, AggregateFailures("DescribeFlowLogs", failures, n)
}
