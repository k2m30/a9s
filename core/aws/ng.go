// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"

	"github.com/k2m30/a9s/v3/core/catalog"
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
// Health.Issues; when empty (AWS reported the state without an issues[]
// entry, which happens transiently) fallbackPhrase is used instead. A
// second and later issue is folded into Detail as "+N more" rather than
// widening the Phrase, keeping the Status cell short.
func healthIssueFinding(code domain.FindingCode, fallbackPhrase string, issueCodes []string) (domain.Finding, []domain.DetailRow) {
	return healthIssueFindingSev(code, fallbackPhrase, issueCodes, domain.SevBroken)
}

// healthIssueWarnFinding is healthIssueFinding at SevWarn instead of
// SevBroken — for a Health.Issues[] signal that fires on an otherwise-healthy
// lifecycle state (docs/resources/eks.md §3.2: Health is tracked
// independently of lifecycle; colorEKSCluster's own precedence puts a bare
// health issue below FAILED/CREATING/UPDATING). issueCodes is always non-empty
// here (callers only invoke this when health_issues_count > 0), so there is
// no fallbackPhrase parameter.
func healthIssueWarnFinding(code domain.FindingCode, issueCodes []string) (domain.Finding, []domain.DetailRow) {
	return healthIssueFindingSev(code, "health issue", issueCodes, domain.SevWarn)
}

// healthIssueFindingSev returns the finding plus one Attention row per
// reported issue code, so the phrase names the first and the rows the rest.
func healthIssueFindingSev(code domain.FindingCode, fallbackPhrase string, issueCodes []string, sev domain.Severity) (domain.Finding, []domain.DetailRow) {
	f := domain.Finding{Code: code, Phrase: fallbackPhrase, Detail: catalog.Detail(code), Severity: sev, Source: "wave1"}
	if len(issueCodes) == 0 {
		return f, nil
	}
	f.Phrase = issueCodes[0]
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

	// emit wave1 Findings for non-healthy lifecycle states.
	// ACTIVE → no Finding (healthy). Fields["status"] is still populated
	// so the existing structural Color path works for the wave2 fallback.
	var findings []domain.Finding
	var issueRows []domain.DetailRow
	switch status {
	case "CREATING":
		findings = []domain.Finding{{Code: CodeNGStateCreating, Phrase: "creating", Severity: domain.SevWarn, Source: "wave1"}}
	case "UPDATING":
		findings = []domain.Finding{{Code: CodeNGStateUpdating, Phrase: "updating", Severity: domain.SevWarn, Source: "wave1"}}
	case "DELETING":
		findings = []domain.Finding{{Code: CodeNGStateDeleting, Phrase: "deleting", Severity: domain.SevWarn, Source: "wave1"}}
	case "CREATE_FAILED":
		findings = []domain.Finding{{Code: CodeNGStateCreateFailed, Phrase: "create failed", Severity: domain.SevBroken, Source: "wave1"}}
	case "DELETE_FAILED":
		findings = []domain.Finding{{Code: CodeNGStateDeleteFailed, Phrase: "delete failed", Severity: domain.SevBroken, Source: "wave1"}}
	case "DEGRADED":
		f, rows := healthIssueFinding(CodeNGStateDegraded, "degraded", issueCodes)
		findings = []domain.Finding{f}
		issueRows = rows
	}

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
	addWave1Rows(&r, CodeNGStateDegraded, issueRows...)
	return r
}
