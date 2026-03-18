package aws

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"

	"github.com/k2m30/a9s/internal/resource"
)

func init() {
	resource.Register("eks", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchEKSClusters(ctx, c.EKS, c.EKS)
	})
}

// FetchEKSClusters performs a two-step fetch: ListClusters to get cluster names,
// then DescribeCluster for each name to get full details.
func FetchEKSClusters(ctx context.Context, listAPI EKSListClustersAPI, describeAPI EKSDescribeClusterAPI) ([]resource.Resource, error) {
	listOutput, err := listAPI.ListClusters(ctx, &eks.ListClustersInput{})
	if err != nil {
		return nil, err
	}

	var resources []resource.Resource

	for _, clusterName := range listOutput.Clusters {
		descOutput, err := describeAPI.DescribeCluster(ctx, &eks.DescribeClusterInput{
			Name: aws.String(clusterName),
		})
		if err != nil {
			return nil, err
		}

		cluster := descOutput.Cluster

		name := ""
		if cluster.Name != nil {
			name = *cluster.Name
		}

		version := ""
		if cluster.Version != nil {
			version = *cluster.Version
		}

		status := string(cluster.Status)

		endpoint := ""
		if cluster.Endpoint != nil {
			endpoint = *cluster.Endpoint
		}

		platformVersion := ""
		if cluster.PlatformVersion != nil {
			platformVersion = *cluster.PlatformVersion
		}

		// Build DetailData
		detail := map[string]string{
			"Cluster Name":     name,
			"Version":          version,
			"Status":           status,
			"Endpoint":         endpoint,
			"Platform Version": platformVersion,
		}

		// ARN
		arn := ""
		if cluster.Arn != nil {
			arn = *cluster.Arn
		}
		detail["ARN"] = arn

		// Role ARN
		roleARN := ""
		if cluster.RoleArn != nil {
			roleARN = *cluster.RoleArn
		}
		detail["Role ARN"] = roleARN

		// Kubernetes Network Config
		if cluster.KubernetesNetworkConfig != nil && cluster.KubernetesNetworkConfig.ServiceIpv4Cidr != nil {
			detail["Kubernetes Network Config"] = *cluster.KubernetesNetworkConfig.ServiceIpv4Cidr
		} else {
			detail["Kubernetes Network Config"] = ""
		}

		// Build RawJSON
		rawJSON := ""
		if jsonBytes, err := json.MarshalIndent(cluster, "", "  "); err == nil {
			rawJSON = string(jsonBytes)
		}

		r := resource.Resource{
			ID:     name,
			Name:   name,
			Status: status,
			Fields: map[string]string{
				"cluster_name":     name,
				"version":          version,
				"status":           status,
				"endpoint":         endpoint,
				"platform_version": platformVersion,
			},
			DetailData: detail,
			RawJSON:    rawJSON,
			RawStruct:  cluster,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
