// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// vpc_issue_enrichment.go — Wave 2 issue enrichment for the vpc resource type.
package aws

import (
	"cmp"
	"context"
	"errors"
	"fmt"
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
// raises a finding when the VPC's traffic is not all logged (vpcFlowLogCoverage).
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
	resources = capAtEnrichmentCap(&result, resources, nil, resourceIDsOf)
	n := len(resources)
	ownAccount := accountIDFromClients(ctx, clients, clients.IdentityStore())
	var failures []Failure
	var mu sync.Mutex
	loopErr := ForEachRow(ctx, &result, resourceIDs(resources), EnrichmentParallelism, func(i int) {
		r := resources[i]
		vpcID := r.ID
		if vpcID == "" {
			return
		}
		// A flow log attaches to a VPC, a subnet or a network interface, and
		// all three write the same records, so asking only about the VPC's own
		// id reports a VPC whose subnets are fully covered as capturing
		// nothing. The subnet ids come off the row the fetcher built; a row
		// whose subnets were not read is asked about the VPC alone, which can
		// prove a flow log but never its absence.
		subnetsUnread := r.Fields["subnet_ids_unread"] == "true"
		scopes := []string{vpcID}
		if !subnetsUnread {
			scopes = append(scopes, splitCSV(r.Fields["subnet_ids"])...)
		}

		allFlowLogs, complete, flErr := PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]ec2types.FlowLog, *string, error) {
			out, err := clients.EC2.DescribeFlowLogs(ctx, &ec2svc.DescribeFlowLogsInput{
				Filter: []ec2types.Filter{
					{Name: aws.String("resource-id"), Values: scopes},
				},
				NextToken: token,
			})
			if err != nil {
				return nil, nil, err
			}
			return out.FlowLogs, out.NextToken, nil
		})

		mu.Lock()
		defer mu.Unlock()
		switch {
		case flErr != nil:
			MarkSkipped(&result, r.ID, &failures, flErr)
			return
		case !complete:
			markUninspected(&result, r.ID, CheckCap)
			return
		}
		covered, rows := vpcFlowLogCoverage(vpcID, splitCSV(r.Fields["subnet_ids"]), allFlowLogs)
		if !covered && subnetsUnread {
			markUninspected(&result, r.ID, checkListIncomplete("subnet"))
			return
		}
		flowLogsVal := "yes"
		if !covered {
			flowLogsVal = "no"
			if !vpcSharedFromAnotherAccount(r, ownAccount) {
				setWave2Finding(&result, vpcID, vpcCodeNoFlowLogs, rows)
			}
		}
		result.FieldUpdates[vpcID] = map[string]string{
			"flow_logs": flowLogsVal,
		}
	})
	return result, errors.Join(loopErr, AggregateFailures("DescribeFlowLogs", failures, n))
}

// vpcSharedFromAnotherAccount reports whether the VPC is owned by another
// account and shared into this one through AWS RAM: only the owner can create
// a flow log on it, so its flow logs are not this account's to fix. A VPC
// whose owner or whose session account is unknown is judged as the
// account's own.
func vpcSharedFromAnotherAccount(r resource.Resource, ownAccount string) bool {
	vpc, _ := assertStruct[ec2types.Vpc](r.RawStruct)
	owner := aws.ToString(vpc.OwnerId)
	return owner != "" && ownAccount != "" && owner != ownAccount
}

// vpcFlowLogCoverage reports whether the VPC's traffic is logged: by a
// delivering flow log on the VPC itself, or by one on every subnet. A flow
// log delivers when it is ACTIVE and its DeliverLogsStatus is not FAILED; an
// ACTIVE one whose delivery fails writes nothing. When not covered, rows name
// what is missing: the subnets without one, and each failing delivery's
// error.
func vpcFlowLogCoverage(vpcID string, subnetIDs []string, flowLogs []ec2types.FlowLog) (bool, []domain.DetailRow) {
	delivering := map[string]bool{}
	var rows []domain.DetailRow
	for _, fl := range flowLogs {
		if aws.ToString(fl.FlowLogStatus) != "ACTIVE" {
			continue
		}
		if aws.ToString(fl.DeliverLogsStatus) == "FAILED" {
			rows = append(rows, domain.DetailRow{Label: "Delivery failing", Value: aws.ToString(fl.ResourceId) + ": " + cmp.Or(aws.ToString(fl.DeliverLogsErrorMessage), "no error message"), Tier: tierOf(vpcCodeNoFlowLogs)})
			continue
		}
		delivering[aws.ToString(fl.ResourceId)] = true
	}
	if delivering[vpcID] {
		return true, nil
	}
	logged := 0
	for _, id := range subnetIDs {
		if delivering[id] {
			logged++
		}
	}
	if len(subnetIDs) > 0 && logged == len(subnetIDs) {
		return true, nil
	}
	if logged > 0 {
		rows = append(rows, domain.DetailRow{Label: "Subnets logged", Value: fmt.Sprintf("%d of %d", logged, len(subnetIDs)), Tier: tierOf(vpcCodeNoFlowLogs)})
	}
	return false, rows
}
