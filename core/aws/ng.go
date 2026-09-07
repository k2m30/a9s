// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// resolveNGImageID calls DescribeLaunchTemplateVersions for the given LaunchTemplateSpecification
// and returns the ImageId from the first version found. Returns "" on any error or missing data.
func resolveNGImageID(ctx context.Context, api EC2DescribeLaunchTemplateVersionsAPI, lt *ekstypes.LaunchTemplateSpecification) string {
	if api == nil || lt == nil || lt.Id == nil {
		return ""
	}
	version := "$Default"
	if lt.Version != nil && *lt.Version != "" {
		version = *lt.Version
	}
	out, err := api.DescribeLaunchTemplateVersions(ctx, &ec2.DescribeLaunchTemplateVersionsInput{
		LaunchTemplateId: lt.Id,
		Versions:         []string{version},
	})
	if err != nil || out == nil || len(out.LaunchTemplateVersions) == 0 {
		return ""
	}
	data := out.LaunchTemplateVersions[0].LaunchTemplateData
	if data == nil || data.ImageId == nil {
		return ""
	}
	return *data.ImageId
}

// healthIssueFinding builds a Broken Finding for an unhealthy lifecycle
// state, folding the specific AWS health-issue text (e.g. "insufficient
// free addresses") into the Phrase instead of a generic lifecycle label —
// an operator scanning the Status column needs the cause, not just the
// state name. issueCodes is the already-humanized list from
// Health.Issues, one Attention row each.
func healthIssueFinding(code domain.FindingCode, issueCodes []string) (domain.Finding, []domain.DetailRow) {
	return healthIssueFindingSev(code, issueCodes, domain.SevBroken)
}

// healthIssueFindingSev returns the code's own finding plus one Attention row
// per reported issue code: a node group with three issues has three to read,
// and promoting the first into the phrase left the others unsayable.
func healthIssueFindingSev(code domain.FindingCode, issueCodes []string, sev domain.Severity) (domain.Finding, []domain.DetailRow) {
	f := wave1Finding(code, sev)
	tier := "~"
	if sev == domain.SevBroken {
		tier = "!"
	}
	rows := make([]domain.DetailRow, 0, len(issueCodes))
	for _, c := range issueCodes {
		rows = append(rows, domain.DetailRow{Label: "Issue", Value: c, Tier: tier})
	}
	return f, rows
}

// buildNodeGroupResource constructs a Resource from cluster name, nodegroup name, and EKS Nodegroup struct.
func buildNodeGroupResource(clusterName, ngName string, ng *ekstypes.Nodegroup) resource.Resource {
	nodegroupName := ngName
	if ng.NodegroupName != nil {
		nodegroupName = *ng.NodegroupName
	}

	ngClusterName := clusterName
	if ng.ClusterName != nil {
		ngClusterName = *ng.ClusterName
	}

	status := string(ng.Status)
	instanceTypes := strings.Join(ng.InstanceTypes, ", ")

	desiredSize := ""
	if ng.ScalingConfig != nil && ng.ScalingConfig.DesiredSize != nil {
		desiredSize = fmt.Sprintf("%d", *ng.ScalingConfig.DesiredSize)
	}

	// Wave 2: health.issues[] — populated by DescribeNodegroup (called per node group in fetcher).
	healthIssuesCount := 0
	var issueCodes []string
	if ng.Health != nil {
		for _, issue := range ng.Health.Issues {
			healthIssuesCount++
			issueCodes = append(issueCodes, domain.HumanizeStatusPhrase(string(issue.Code)))
		}
	}

	findings, issueRows := ngFindings(status, healthIssuesCount, issueCodes)

	r := resource.Resource{
		ID:   nodegroupName,
		Name: nodegroupName,
		Fields: map[string]string{
			"nodegroup_name":      nodegroupName,
			"cluster_name":        ngClusterName,
			"status":              status,
			"instance_types":      instanceTypes,
			"desired_size":        desiredSize,
			"health_issues_count": strconv.Itoa(healthIssuesCount),
			"health_issues":       strings.Join(issueCodes, ", "),
		},
		Findings:  findings,
		RawStruct: ng,
	}
	if len(findings) > 0 {
		// The rows belong to whichever finding built them — degraded folds
		// them into its state finding, ACTIVE-with-issues into the
		// health-issue one. A row filed under a code the resource does not
		// carry is never read.
		addWave1Rows(&r, findings[0].Code, issueRows...)
	}
	return r
}

// degradedNodeGroup is the name-only row for a node group DescribeNodegroup
// would not answer for. It writes the two identity fields buildNodeGroupResource
// writes, because those are what the related checkers filter on and, unlike
// RawStruct, they survive the disk cache: without them a warm row makes
// ng→eks, ng→ec2 and ng→ebs answer a confident zero instead of "not read".
func degradedNodeGroup(clusterName, ngName string, err error) resource.Resource {
	r := DegradedDetails("ng", ngName, err)
	r.Fields["cluster_name"] = clusterName
	r.Fields["nodegroup_name"] = ngName
	return r
}

// ngFindings is the one predicate for a node group: its lifecycle state, then
// a bare Health.Issues[] on a node group whose state says nothing — health is
// tracked independently of the state, the same way it is for the cluster.
// colorEKSNodeGroup runs it over Fields for rows built outside the fetcher,
// which have the issue count but not the codes.
func ngFindings(status string, healthIssuesCount int, issueCodes []string) ([]domain.Finding, []domain.DetailRow) {
	if f := degradedStatusFindings("ng", status); f != nil {
		return f, nil
	}
	switch status {
	case "CREATING":
		return []domain.Finding{wave1Finding(CodeNGStateCreating, domain.SevWarn)}, nil
	case "UPDATING":
		return []domain.Finding{wave1Finding(CodeNGStateUpdating, domain.SevWarn)}, nil
	case "DELETING":
		return []domain.Finding{wave1Finding(CodeNGStateDeleting, domain.SevWarn)}, nil
	case "CREATE_FAILED":
		return []domain.Finding{wave1Finding(CodeNGStateCreateFailed, domain.SevBroken)}, nil
	case "DELETE_FAILED":
		return []domain.Finding{wave1Finding(CodeNGStateDeleteFailed, domain.SevBroken)}, nil
	case "DEGRADED":
		f, rows := healthIssueFinding(CodeNGStateDegraded, issueCodes)
		return []domain.Finding{f}, rows
	}
	if healthIssuesCount > 0 {
		f, rows := healthIssueFindingSev(CodeNGHealthIssue, issueCodes, domain.SevWarn)
		return []domain.Finding{f}, rows
	}
	return nil, nil
}
