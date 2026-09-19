package unit

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/tests/testdata"
)

func TestFetchSecurityGroups_ParsesMultipleGroups(t *testing.T) {
	mock := &mockEC2DescribeSecurityGroupsClient{
		output: &ec2.DescribeSecurityGroupsOutput{
			SecurityGroups: []ec2types.SecurityGroup{
				{
					GroupId:     aws.String("sg-0001"),
					GroupName:   aws.String("web-sg"),
					VpcId:       aws.String("vpc-aaa"),
					Description: aws.String("Web server security group"),
					OwnerId:     aws.String("123456789012"),
					IpPermissions: []ec2types.IpPermission{
						{
							IpProtocol: aws.String("tcp"),
							FromPort:   aws.Int32(80),
							ToPort:     aws.Int32(80),
						},
						{
							IpProtocol: aws.String("tcp"),
							FromPort:   aws.Int32(443),
							ToPort:     aws.Int32(443),
						},
					},
					IpPermissionsEgress: []ec2types.IpPermission{
						{
							IpProtocol: aws.String("-1"),
						},
					},
					Tags: []ec2types.Tag{
						{Key: aws.String("Name"), Value: aws.String("web-sg")},
						{Key: aws.String("Env"), Value: aws.String("prod")},
					},
				},
				{
					GroupId:     aws.String("sg-0002"),
					GroupName:   aws.String("db-sg"),
					VpcId:       aws.String("vpc-bbb"),
					Description: aws.String("Database security group"),
					OwnerId:     aws.String("123456789012"),
					IpPermissions: []ec2types.IpPermission{
						{
							IpProtocol: aws.String("tcp"),
							FromPort:   aws.Int32(5432),
							ToPort:     aws.Int32(5432),
						},
					},
					IpPermissionsEgress: []ec2types.IpPermission{},
				},
			},
		},
	}

	resources, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchSecurityGroupsPage(context.Background(), mock, token)
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	r0 := resources[0]
	if r0.ID != "sg-0001" {
		t.Errorf("resource[0].ID: expected %q, got %q", "sg-0001", r0.ID)
	}
	if r0.Name != "web-sg" {
		t.Errorf("resource[0].Name: expected %q, got %q", "web-sg", r0.Name)
	}

	r1 := resources[1]
	if r1.ID != "sg-0002" {
		t.Errorf("resource[1].ID: expected %q, got %q", "sg-0002", r1.ID)
	}
	if r1.Name != "db-sg" {
		t.Errorf("resource[1].Name: expected %q, got %q", "db-sg", r1.Name)
	}

	requiredFields := []string{"group_id", "group_name", "vpc_id", "description"}
	for i, r := range resources {
		for _, key := range requiredFields {
			if _, ok := r.Fields[key]; !ok {
				t.Errorf("resource[%d].Fields missing key %q", i, key)
			}
		}
	}

	if r0.Fields["group_id"] != "sg-0001" {
		t.Errorf("resource[0].Fields[\"group_id\"]: expected %q, got %q", "sg-0001", r0.Fields["group_id"])
	}
	if r0.Fields["group_name"] != "web-sg" {
		t.Errorf("resource[0].Fields[\"group_name\"]: expected %q, got %q", "web-sg", r0.Fields["group_name"])
	}
	if r0.Fields["vpc_id"] != "vpc-aaa" {
		t.Errorf("resource[0].Fields[\"vpc_id\"]: expected %q, got %q", "vpc-aaa", r0.Fields["vpc_id"])
	}
	if r0.Fields["description"] != "Web server security group" {
		t.Errorf("resource[0].Fields[\"description\"]: expected %q, got %q", "Web server security group", r0.Fields["description"])
	}

	if r1.Fields["group_id"] != "sg-0002" {
		t.Errorf("resource[1].Fields[\"group_id\"]: expected %q, got %q", "sg-0002", r1.Fields["group_id"])
	}
	if r1.Fields["vpc_id"] != "vpc-bbb" {
		t.Errorf("resource[1].Fields[\"vpc_id\"]: expected %q, got %q", "vpc-bbb", r1.Fields["vpc_id"])
	}
}

func TestFetchSecurityGroups_ErrorResponse(t *testing.T) {
	mock := &mockEC2DescribeSecurityGroupsClient{
		output: nil,
		err:    fmt.Errorf("AWS API error: access denied"),
	}

	resources, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchSecurityGroupsPage(context.Background(), mock, token)
	})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d resources", len(resources))
	}
}

func TestFetchSecurityGroups_EmptyResponse(t *testing.T) {
	mock := &mockEC2DescribeSecurityGroupsClient{
		output: &ec2.DescribeSecurityGroupsOutput{
			SecurityGroups: []ec2types.SecurityGroup{},
		},
	}

	resources, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchSecurityGroupsPage(context.Background(), mock, token)
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}

func TestFetchSecurityGroups_RawStructPopulated(t *testing.T) {
	mock := &mockEC2DescribeSecurityGroupsClient{
		output: &ec2.DescribeSecurityGroupsOutput{
			SecurityGroups: []ec2types.SecurityGroup{
				{
					GroupId:     aws.String("sg-raw123"),
					GroupName:   aws.String("raw-test-sg"),
					VpcId:       aws.String("vpc-raw"),
					Description: aws.String("Raw struct test"),
				},
			},
		},
	}

	resources, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchSecurityGroupsPage(context.Background(), mock, token)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1, got %d", len(resources))
	}

	r := resources[0]

	if r.RawStruct == nil {
		t.Fatal("RawStruct must not be nil")
	}

	sg, ok := r.RawStruct.(ec2types.SecurityGroup)
	if !ok {
		t.Fatalf("RawStruct should be ec2types.SecurityGroup, got %T", r.RawStruct)
	}
	if *sg.GroupId != "sg-raw123" {
		t.Errorf("RawStruct.GroupId = %q, want %q", *sg.GroupId, "sg-raw123")
	}

}

func TestFetchSecurityGroups_NilFieldsHandled(t *testing.T) {
	mock := &mockEC2DescribeSecurityGroupsClient{
		output: &ec2.DescribeSecurityGroupsOutput{
			SecurityGroups: []ec2types.SecurityGroup{
				{
					GroupId:     nil,
					GroupName:   nil,
					VpcId:       nil,
					Description: nil,
					OwnerId:     nil,
				},
			},
		},
	}

	resources, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchSecurityGroupsPage(context.Background(), mock, token)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1, got %d", len(resources))
	}

	r := resources[0]
	if r.ID != "" {
		t.Errorf("ID should be empty for nil GroupId, got %q", r.ID)
	}
	if r.Name != "" {
		t.Errorf("Name should be empty for nil GroupName, got %q", r.Name)
	}
	if r.Fields["group_id"] != "" {
		t.Errorf("Fields[group_id] should be empty, got %q", r.Fields["group_id"])
	}
	if r.Fields["group_name"] != "" {
		t.Errorf("Fields[group_name] should be empty, got %q", r.Fields["group_name"])
	}
	if r.Fields["vpc_id"] != "" {
		t.Errorf("Fields[vpc_id] should be empty, got %q", r.Fields["vpc_id"])
	}
	if r.Fields["description"] != "" {
		t.Errorf("Fields[description] should be empty, got %q", r.Fields["description"])
	}
}

func TestFetchSecurityGroups_RealAWSData(t *testing.T) {
	mock := &mockEC2DescribeSecurityGroupsClient{
		output: &ec2.DescribeSecurityGroupsOutput{
			SecurityGroups: testdata.RealSecurityGroups(),
		},
	}

	resources, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchSecurityGroupsPage(context.Background(), mock, token)
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 21 {
		t.Fatalf("expected 21 resources from real data, got %d", len(resources))
	}

	byID := make(map[string]int)
	for i, r := range resources {
		byID[r.ID] = i
	}

	expectedIDs := []string{
		"sg-0aa0000000000001a", // migration-sg (docdb-sg)
		"sg-0aa0000000000002b", // node-to-node-traffic
		"sg-0aa0000000000003c", // msk-sg
		"sg-0aa0000000000004d", // app-efs
		"sg-0aa0000000000005e", // test-cluster-1-node
		"sg-0aa0000000000006f", // ci-runner-ubuntu-sg
		"sg-0aa0000000000007a", // elasticache
		"sg-0aa0000000000008b", // vpn-sg (allow-http-https-ssh)
		"sg-0aa0000000000009c", // eks-cluster-sg
		"sg-0aa000000000000ad", // vpc-endpoints
		"sg-0aa000000000000be", // media-efs
		"sg-0aa000000000000cf", // k8s-ingress-external
		"sg-0aa000000000000d0", // k8s-traffic-shared
		"sg-0aa000000000000e1", // k8s-ingress-internal
		"sg-0aa000000000000f2", // vpn-sg
		"sg-0aa0000000000010a", // ci-runner
		"sg-0aa0000000000011b", // default (default VPC)
		"sg-0aa0000000000012c", // test-cluster-1-cluster
		"sg-0aa0000000000013d", // launch-wizard-1
		"sg-0aa0000000000014e", // default (dev-vpc)
		"sg-0aa0000000000015f", // rds
	}
	for _, id := range expectedIDs {
		if _, ok := byID[id]; !ok {
			t.Errorf("missing expected SG ID %q in resources", id)
		}
	}

	// test-cluster-1-node: the SG with the most inbound rules.
	idx := byID["sg-0aa0000000000005e"]
	r := resources[idx]
	if r.Name != "test-cluster-1-node" {
		t.Errorf("eks node SG Name: expected %q, got %q",
			"test-cluster-1-node", r.Name)
	}
	if r.Fields["group_name"] != "test-cluster-1-node" {
		t.Errorf("eks node SG Fields[group_name]: expected node SG name, got %q", r.Fields["group_name"])
	}
	if r.Fields["vpc_id"] != "vpc-0aaa1111bbb2222cc" {
		t.Errorf("eks node SG Fields[vpc_id]: expected %q, got %q", "vpc-0aaa1111bbb2222cc", r.Fields["vpc_id"])
	}
	if r.Fields["description"] != "EKS node shared security group" {
		t.Errorf("eks node SG Fields[description]: expected %q, got %q", "EKS node shared security group", r.Fields["description"])
	}

	// default dev-vpc SG: no rules.
	idx = byID["sg-0aa0000000000014e"]
	rEmpty := resources[idx]
	if rEmpty.Name != "default" {
		t.Errorf("default dev-vpc SG Name: expected %q, got %q", "default", rEmpty.Name)
	}

	// msk-sg: no tags.
	idx = byID["sg-0aa0000000000003c"]
	rMsk := resources[idx]
	if rMsk.Name != "msk-sg" {
		t.Errorf("msk SG Name: expected %q, got %q", "msk-sg", rMsk.Name)
	}

	vpcCounts := make(map[string]int)
	for _, r := range resources {
		vpcCounts[r.Fields["vpc_id"]]++
	}
	if vpcCounts["vpc-0aaa1111bbb2222cc"] != 19 {
		t.Errorf("expected 19 SGs in dev-vpc, got %d", vpcCounts["vpc-0aaa1111bbb2222cc"])
	}
	if vpcCounts["vpc-0ddd3333eee4444ff"] != 2 {
		t.Errorf("expected 2 SGs in default VPC, got %d", vpcCounts["vpc-0ddd3333eee4444ff"])
	}

	for i, r := range resources {
		if r.RawStruct == nil {
			t.Errorf("resource[%d].RawStruct must not be nil", i)
			continue
		}
		sg, ok := r.RawStruct.(ec2types.SecurityGroup)
		if !ok {
			t.Errorf("resource[%d].RawStruct should be ec2types.SecurityGroup, got %T", i, r.RawStruct)
			continue
		}
		if sg.GroupId == nil || *sg.GroupId != r.ID {
			t.Errorf("resource[%d].RawStruct.GroupId (%v) does not match ID (%q)", i, sg.GroupId, r.ID)
		}
	}

	// ci-runner-ubuntu: IPv6 egress rules.
	idx = byID["sg-0aa0000000000006f"]
	rCiUbuntu := resources[idx]
	sgRaw, ok := rCiUbuntu.RawStruct.(ec2types.SecurityGroup)
	if !ok {
		t.Fatalf("ci-runner-ubuntu RawStruct should be ec2types.SecurityGroup, got %T", rCiUbuntu.RawStruct)
	}
	if len(sgRaw.IpPermissionsEgress) != 1 {
		t.Fatalf("ci-runner-ubuntu expected 1 egress rule, got %d", len(sgRaw.IpPermissionsEgress))
	}
	if len(sgRaw.IpPermissionsEgress[0].Ipv6Ranges) != 1 {
		t.Errorf("ci-runner-ubuntu egress rule expected 1 IPv6 range, got %d", len(sgRaw.IpPermissionsEgress[0].Ipv6Ranges))
	}

	requiredFields := []string{"group_id", "group_name", "vpc_id", "description"}
	for i, r := range resources {
		for _, key := range requiredFields {
			if _, ok := r.Fields[key]; !ok {
				t.Errorf("resource[%d].Fields missing key %q", i, key)
			}
		}
	}

}
