package aws

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// FetchNodeGroups performs a four-step fetch:
// 1. ListClusters to get cluster names (paginated)
// 2. ListNodegroups per cluster to get node group names (paginated)
// 3. DescribeNodegroup per node group to get full details
// 4. DescribeLaunchTemplateVersions for nodegroups with custom LaunchTemplates to resolve image_id
//
// The ltVersionsAPI parameter is optional (variadic). When omitted or nil, image_id is left empty.
func FetchNodeGroups(
	ctx context.Context,
	listClustersAPI EKSListClustersAPI,
	listNodegroupsAPI EKSListNodegroupsAPI,
	describeNodegroupAPI EKSDescribeNodegroupAPI,
	ltVersionsAPIs ...EC2DescribeLaunchTemplateVersionsAPI,
) ([]resource.Resource, error) {
	var ltVersionsAPI EC2DescribeLaunchTemplateVersionsAPI
	if len(ltVersionsAPIs) > 0 {
		ltVersionsAPI = ltVersionsAPIs[0]
	}
	// Step 1: List all clusters (paginated)
	var allClusters []string
	var clusterNextToken *string

	for {
		listOutput, err := listClustersAPI.ListClusters(ctx, &eks.ListClustersInput{
			NextToken: clusterNextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("listing EKS clusters: %w", err)
		}

		allClusters = append(allClusters, listOutput.Clusters...)

		if listOutput.NextToken == nil {
			break
		}
		clusterNextToken = listOutput.NextToken
	}

	var resources []resource.Resource

	// Step 2: For each cluster, list its node groups (paginated)
	for _, clusterName := range allClusters {
		var allNodegroups []string
		var ngNextToken *string

		for {
			ngListOutput, err := listNodegroupsAPI.ListNodegroups(ctx, &eks.ListNodegroupsInput{
				ClusterName: aws.String(clusterName),
				NextToken:   ngNextToken,
			})
			if err != nil {
				return nil, fmt.Errorf("listing node groups for cluster %s: %w", clusterName, err)
			}

			allNodegroups = append(allNodegroups, ngListOutput.Nodegroups...)

			if ngListOutput.NextToken == nil {
				break
			}
			ngNextToken = ngListOutput.NextToken
		}

		// Step 3: For each node group, describe it
		for _, ngName := range allNodegroups {
			descOutput, err := describeNodegroupAPI.DescribeNodegroup(ctx, &eks.DescribeNodegroupInput{
				ClusterName:   aws.String(clusterName),
				NodegroupName: aws.String(ngName),
			})
			if err != nil {
				return nil, fmt.Errorf("describing node group %s: %w", ngName, err)
			}
			if descOutput.Nodegroup == nil {
				continue
			}
			res := buildNodeGroupResource(clusterName, ngName, descOutput.Nodegroup)
			// Step 4: Resolve image_id from custom LaunchTemplate (non-fatal on error)
			if descOutput.Nodegroup.LaunchTemplate != nil && descOutput.Nodegroup.LaunchTemplate.Id != nil {
				imageID := resolveNGImageID(ctx, ltVersionsAPI, descOutput.Nodegroup.LaunchTemplate)
				res.Fields["image_id"] = imageID
			}
			resources = append(resources, res)
		}
	}

	return resources, nil
}

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
			issueCodes = append(issueCodes, string(issue.Code))
		}
	}

	// emit wave1 Findings for non-healthy lifecycle states.
	// ACTIVE → no Finding (healthy). Fields["status"] is still populated
	// so the existing structural Color path works for the wave2 fallback.
	var findings []domain.Finding
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
		findings = []domain.Finding{{Code: CodeNGStateDegraded, Phrase: "degraded", Severity: domain.SevBroken, Source: "wave1"}}
	}

	return resource.Resource{
		ID:   nodegroupName,
		Name: nodegroupName,
		Fields: map[string]string{
			"nodegroup_name":      nodegroupName,
			"cluster_name":        ngClusterName,
			"status":              status,
			"instance_types":      instanceTypes,
			"desired_size":        desiredSize,
			"health_issues_count": strconv.Itoa(healthIssuesCount),
			"health_issues":       strings.Join(issueCodes, ","),
		},
		Findings:  findings,
		RawStruct: ng,
	}
}
