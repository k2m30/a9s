package unit

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	awsclient "github.com/k2m30/a9s/internal/aws"
	"github.com/k2m30/a9s/tests/testdata"
)

// ---------------------------------------------------------------------------
// T-SG-001 - Test Security Groups response parsing
// ---------------------------------------------------------------------------

func TestFetchSecurityGroups_ParsesMultipleGroups(t *testing.T) {
	mock := &mockEC2DescribeSecurityGroupsClient{
		output: &ec2.DescribeSecurityGroupsOutput{
			SecurityGroups: []ec2types.SecurityGroup{
				{
					GroupId:   aws.String("sg-0001"),
					GroupName: aws.String("web-sg"),
					VpcId:     aws.String("vpc-aaa"),
					Description: aws.String("Web server security group"),
					OwnerId:  aws.String("123456789012"),
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
					GroupId:   aws.String("sg-0002"),
					GroupName: aws.String("db-sg"),
					VpcId:     aws.String("vpc-bbb"),
					Description: aws.String("Database security group"),
					OwnerId:  aws.String("123456789012"),
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

	resources, err := awsclient.FetchSecurityGroups(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	// Verify first security group
	r0 := resources[0]
	if r0.ID != "sg-0001" {
		t.Errorf("resource[0].ID: expected %q, got %q", "sg-0001", r0.ID)
	}
	if r0.Name != "web-sg" {
		t.Errorf("resource[0].Name: expected %q, got %q", "web-sg", r0.Name)
	}
	if r0.Status != "" {
		t.Errorf("resource[0].Status: expected empty string, got %q", r0.Status)
	}

	// Verify second security group
	r1 := resources[1]
	if r1.ID != "sg-0002" {
		t.Errorf("resource[1].ID: expected %q, got %q", "sg-0002", r1.ID)
	}
	if r1.Name != "db-sg" {
		t.Errorf("resource[1].Name: expected %q, got %q", "db-sg", r1.Name)
	}
	if r1.Status != "" {
		t.Errorf("resource[1].Status: expected empty string, got %q", r1.Status)
	}

	// Verify Fields contain the expected keys
	requiredFields := []string{"group_id", "group_name", "vpc_id", "description"}
	for i, r := range resources {
		for _, key := range requiredFields {
			if _, ok := r.Fields[key]; !ok {
				t.Errorf("resource[%d].Fields missing key %q", i, key)
			}
		}
	}

	// Verify specific field values on the first SG
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

	// Second SG field values
	if r1.Fields["group_id"] != "sg-0002" {
		t.Errorf("resource[1].Fields[\"group_id\"]: expected %q, got %q", "sg-0002", r1.Fields["group_id"])
	}
	if r1.Fields["vpc_id"] != "vpc-bbb" {
		t.Errorf("resource[1].Fields[\"vpc_id\"]: expected %q, got %q", "vpc-bbb", r1.Fields["vpc_id"])
	}
}

// ---------------------------------------------------------------------------
// T-SG-002 - Test DetailData populated correctly
// ---------------------------------------------------------------------------

func TestFetchSecurityGroups_DetailDataPopulated(t *testing.T) {
	mock := &mockEC2DescribeSecurityGroupsClient{
		output: &ec2.DescribeSecurityGroupsOutput{
			SecurityGroups: []ec2types.SecurityGroup{
				{
					GroupId:          aws.String("sg-detail123"),
					GroupName:        aws.String("detail-test-sg"),
					VpcId:            aws.String("vpc-detail"),
					Description:      aws.String("A detailed test SG"),
					OwnerId:          aws.String("111222333444"),
					SecurityGroupArn: aws.String("arn:aws:ec2:us-east-1:111222333444:security-group/sg-detail123"),
					IpPermissions: []ec2types.IpPermission{
						{IpProtocol: aws.String("tcp"), FromPort: aws.Int32(80), ToPort: aws.Int32(80)},
						{IpProtocol: aws.String("tcp"), FromPort: aws.Int32(443), ToPort: aws.Int32(443)},
						{IpProtocol: aws.String("tcp"), FromPort: aws.Int32(22), ToPort: aws.Int32(22)},
					},
					IpPermissionsEgress: []ec2types.IpPermission{
						{IpProtocol: aws.String("-1")},
						{IpProtocol: aws.String("tcp"), FromPort: aws.Int32(443), ToPort: aws.Int32(443)},
					},
					Tags: []ec2types.Tag{
						{Key: aws.String("Name"), Value: aws.String("detail-test-sg")},
						{Key: aws.String("Environment"), Value: aws.String("staging")},
					},
				},
			},
		},
	}

	resources, err := awsclient.FetchSecurityGroups(context.Background(), mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1, got %d", len(resources))
	}

	r := resources[0]
	if r.DetailData == nil {
		t.Fatal("DetailData must not be nil")
	}
	if len(r.DetailData) == 0 {
		t.Fatal("DetailData must not be empty")
	}

	// Core fields
	if r.DetailData["Group ID"] != "sg-detail123" {
		t.Errorf("DetailData[Group ID] = %q, want %q", r.DetailData["Group ID"], "sg-detail123")
	}
	if r.DetailData["Group Name"] != "detail-test-sg" {
		t.Errorf("DetailData[Group Name] = %q, want %q", r.DetailData["Group Name"], "detail-test-sg")
	}
	if r.DetailData["VPC ID"] != "vpc-detail" {
		t.Errorf("DetailData[VPC ID] = %q, want %q", r.DetailData["VPC ID"], "vpc-detail")
	}
	if r.DetailData["Description"] != "A detailed test SG" {
		t.Errorf("DetailData[Description] = %q, want %q", r.DetailData["Description"], "A detailed test SG")
	}
	if r.DetailData["Owner ID"] != "111222333444" {
		t.Errorf("DetailData[Owner ID] = %q, want %q", r.DetailData["Owner ID"], "111222333444")
	}
	if r.DetailData["Security Group ARN"] != "arn:aws:ec2:us-east-1:111222333444:security-group/sg-detail123" {
		t.Errorf("DetailData[Security Group ARN] = %q, want full ARN", r.DetailData["Security Group ARN"])
	}

	// Rule counts
	if r.DetailData["Inbound Rules"] != "3 rules" {
		t.Errorf("DetailData[Inbound Rules] = %q, want %q", r.DetailData["Inbound Rules"], "3 rules")
	}
	if r.DetailData["Outbound Rules"] != "2 rules" {
		t.Errorf("DetailData[Outbound Rules] = %q, want %q", r.DetailData["Outbound Rules"], "2 rules")
	}

	// Tags
	if r.DetailData["Tag: Name"] != "detail-test-sg" {
		t.Errorf("DetailData[Tag: Name] = %q, want %q", r.DetailData["Tag: Name"], "detail-test-sg")
	}
	if r.DetailData["Tag: Environment"] != "staging" {
		t.Errorf("DetailData[Tag: Environment] = %q, want %q", r.DetailData["Tag: Environment"], "staging")
	}
}

// ---------------------------------------------------------------------------
// T-SG-003 - Test API error handling
// ---------------------------------------------------------------------------

func TestFetchSecurityGroups_ErrorResponse(t *testing.T) {
	mock := &mockEC2DescribeSecurityGroupsClient{
		output: nil,
		err:    fmt.Errorf("AWS API error: access denied"),
	}

	resources, err := awsclient.FetchSecurityGroups(context.Background(), mock)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d resources", len(resources))
	}
}

// ---------------------------------------------------------------------------
// T-SG-004 - Test empty response
// ---------------------------------------------------------------------------

func TestFetchSecurityGroups_EmptyResponse(t *testing.T) {
	mock := &mockEC2DescribeSecurityGroupsClient{
		output: &ec2.DescribeSecurityGroupsOutput{
			SecurityGroups: []ec2types.SecurityGroup{},
		},
	}

	resources, err := awsclient.FetchSecurityGroups(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}

// ---------------------------------------------------------------------------
// T-SG-005 - Test RawStruct populated for fieldpath
// ---------------------------------------------------------------------------

func TestFetchSecurityGroups_RawStructPopulated(t *testing.T) {
	mock := &mockEC2DescribeSecurityGroupsClient{
		output: &ec2.DescribeSecurityGroupsOutput{
			SecurityGroups: []ec2types.SecurityGroup{
				{
					GroupId:   aws.String("sg-raw123"),
					GroupName: aws.String("raw-test-sg"),
					VpcId:     aws.String("vpc-raw"),
					Description: aws.String("Raw struct test"),
				},
			},
		},
	}

	resources, err := awsclient.FetchSecurityGroups(context.Background(), mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1, got %d", len(resources))
	}

	r := resources[0]

	// RawStruct must be set
	if r.RawStruct == nil {
		t.Fatal("RawStruct must not be nil")
	}

	// RawStruct should be an ec2types.SecurityGroup
	sg, ok := r.RawStruct.(ec2types.SecurityGroup)
	if !ok {
		t.Fatalf("RawStruct should be ec2types.SecurityGroup, got %T", r.RawStruct)
	}
	if *sg.GroupId != "sg-raw123" {
		t.Errorf("RawStruct.GroupId = %q, want %q", *sg.GroupId, "sg-raw123")
	}

	// RawJSON must be non-empty
	if r.RawJSON == "" {
		t.Error("RawJSON must not be empty")
	}
}

// ---------------------------------------------------------------------------
// T-SG-006 - Test singular rule count ("1 rule" not "1 rules")
// ---------------------------------------------------------------------------

func TestFetchSecurityGroups_SingularRuleCount(t *testing.T) {
	mock := &mockEC2DescribeSecurityGroupsClient{
		output: &ec2.DescribeSecurityGroupsOutput{
			SecurityGroups: []ec2types.SecurityGroup{
				{
					GroupId:   aws.String("sg-singular"),
					GroupName: aws.String("singular-test"),
					VpcId:     aws.String("vpc-test"),
					IpPermissions: []ec2types.IpPermission{
						{IpProtocol: aws.String("tcp"), FromPort: aws.Int32(22), ToPort: aws.Int32(22)},
					},
					IpPermissionsEgress: []ec2types.IpPermission{},
				},
			},
		},
	}

	resources, err := awsclient.FetchSecurityGroups(context.Background(), mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1, got %d", len(resources))
	}

	r := resources[0]
	if r.DetailData["Inbound Rules"] != "1 rule" {
		t.Errorf("DetailData[Inbound Rules] = %q, want %q", r.DetailData["Inbound Rules"], "1 rule")
	}
	if r.DetailData["Outbound Rules"] != "0 rules" {
		t.Errorf("DetailData[Outbound Rules] = %q, want %q", r.DetailData["Outbound Rules"], "0 rules")
	}
}

// ---------------------------------------------------------------------------
// T-SG-007 - Test nil string fields handled gracefully
// ---------------------------------------------------------------------------

func TestFetchSecurityGroups_NilFieldsHandled(t *testing.T) {
	mock := &mockEC2DescribeSecurityGroupsClient{
		output: &ec2.DescribeSecurityGroupsOutput{
			SecurityGroups: []ec2types.SecurityGroup{
				{
					// All string pointer fields are nil
					GroupId:   nil,
					GroupName: nil,
					VpcId:     nil,
					Description: nil,
					OwnerId:  nil,
				},
			},
		},
	}

	resources, err := awsclient.FetchSecurityGroups(context.Background(), mock)
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

// ---------------------------------------------------------------------------
// T-SG-REAL - Test SG fetcher with all 21 sanitized security groups
// ---------------------------------------------------------------------------

func TestFetchSecurityGroups_RealAWSData(t *testing.T) {
	mock := &mockEC2DescribeSecurityGroupsClient{
		output: &ec2.DescribeSecurityGroupsOutput{
			SecurityGroups: testdata.RealSecurityGroups(),
		},
	}

	resources, err := awsclient.FetchSecurityGroups(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Real data has exactly 21 security groups
	if len(resources) != 21 {
		t.Fatalf("expected 21 resources from real data, got %d", len(resources))
	}

	// Build a lookup by group ID for targeted assertions
	byID := make(map[string]int)
	for i, r := range resources {
		byID[r.ID] = i
	}

	// Verify all 21 group IDs are present
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

	// --- Verify SG with most inbound rules: test-cluster-1-node (11 inbound, 1 outbound) ---
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
	if r.DetailData["Inbound Rules"] != "11 rules" {
		t.Errorf("eks node SG inbound rules: expected %q, got %q", "11 rules", r.DetailData["Inbound Rules"])
	}
	if r.DetailData["Outbound Rules"] != "1 rule" {
		t.Errorf("eks node SG outbound rules: expected %q, got %q", "1 rule", r.DetailData["Outbound Rules"])
	}
	if r.DetailData["Owner ID"] != "123456789012" {
		t.Errorf("eks node SG Owner ID: expected %q, got %q", "123456789012", r.DetailData["Owner ID"])
	}
	if r.DetailData["Security Group ARN"] != "arn:aws:ec2:us-east-1:123456789012:security-group/sg-0aa0000000000005e" {
		t.Errorf("eks node SG ARN mismatch: got %q", r.DetailData["Security Group ARN"])
	}
	// Verify tags on eks node SG (has 4 tags including kubernetes.io/cluster and karpenter.sh/discovery)
	if r.DetailData["Tag: kubernetes.io/cluster/test-cluster-1"] != "owned" {
		t.Errorf("eks node SG missing kubernetes tag, got %q", r.DetailData["Tag: kubernetes.io/cluster/test-cluster-1"])
	}
	if r.DetailData["Tag: karpenter.sh/discovery"] != "test-cluster-1" {
		t.Errorf("eks node SG missing karpenter tag, got %q", r.DetailData["Tag: karpenter.sh/discovery"])
	}

	// --- Verify SG with 0 inbound and 0 outbound rules: default dev-vpc SG ---
	idx = byID["sg-0aa0000000000014e"]
	rEmpty := resources[idx]
	if rEmpty.Name != "default" {
		t.Errorf("default dev-vpc SG Name: expected %q, got %q", "default", rEmpty.Name)
	}
	if rEmpty.DetailData["Inbound Rules"] != "0 rules" {
		t.Errorf("default dev-vpc SG inbound: expected %q, got %q", "0 rules", rEmpty.DetailData["Inbound Rules"])
	}
	if rEmpty.DetailData["Outbound Rules"] != "0 rules" {
		t.Errorf("default dev-vpc SG outbound: expected %q, got %q", "0 rules", rEmpty.DetailData["Outbound Rules"])
	}

	// --- Verify SG with 1 inbound rule (singular): DocumentDB SG ---
	idx = byID["sg-0aa0000000000001a"]
	rDocDB := resources[idx]
	if rDocDB.DetailData["Inbound Rules"] != "1 rule" {
		t.Errorf("docdb SG inbound: expected %q, got %q", "1 rule", rDocDB.DetailData["Inbound Rules"])
	}
	if rDocDB.DetailData["Outbound Rules"] != "1 rule" {
		t.Errorf("docdb SG outbound: expected %q, got %q", "1 rule", rDocDB.DetailData["Outbound Rules"])
	}
	if rDocDB.DetailData["Description"] != "Security group for DocumentDB" {
		t.Errorf("docdb SG description: expected %q, got %q", "Security group for DocumentDB", rDocDB.DetailData["Description"])
	}
	if rDocDB.DetailData["Tag: Name"] != "docdb-sg" {
		t.Errorf("docdb SG Tag: Name: expected %q, got %q", "docdb-sg", rDocDB.DetailData["Tag: Name"])
	}

	// --- Verify SG with no tags: msk-sg ---
	idx = byID["sg-0aa0000000000003c"]
	rMsk := resources[idx]
	if rMsk.Name != "msk-sg" {
		t.Errorf("msk SG Name: expected %q, got %q", "msk-sg", rMsk.Name)
	}
	if rMsk.DetailData["Inbound Rules"] != "0 rules" {
		t.Errorf("msk SG inbound: expected %q, got %q", "0 rules", rMsk.DetailData["Inbound Rules"])
	}
	if rMsk.DetailData["Outbound Rules"] != "1 rule" {
		t.Errorf("msk SG outbound: expected %q, got %q", "1 rule", rMsk.DetailData["Outbound Rules"])
	}

	// --- Verify SG with 0 inbound and 0 outbound (ci-runner) ---
	idx = byID["sg-0aa0000000000010a"]
	rCiRunner := resources[idx]
	if rCiRunner.DetailData["Inbound Rules"] != "0 rules" {
		t.Errorf("ci-runner SG inbound: expected %q, got %q", "0 rules", rCiRunner.DetailData["Inbound Rules"])
	}
	if rCiRunner.DetailData["Outbound Rules"] != "0 rules" {
		t.Errorf("ci-runner SG outbound: expected %q, got %q", "0 rules", rCiRunner.DetailData["Outbound Rules"])
	}

	// --- Verify SG spanning two VPCs ---
	// Count how many SGs belong to each VPC
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

	// --- Verify all SGs have Status="" (SGs don't have a status) ---
	for i, r := range resources {
		if r.Status != "" {
			t.Errorf("resource[%d].Status should be empty for SGs, got %q", i, r.Status)
		}
	}

	// --- Verify all SGs have non-empty RawJSON ---
	for i, r := range resources {
		if r.RawJSON == "" {
			t.Errorf("resource[%d].RawJSON must not be empty", i)
		}
	}

	// --- Verify all SGs have RawStruct of type ec2types.SecurityGroup ---
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
		// Verify GroupId in RawStruct matches resource ID
		if sg.GroupId == nil || *sg.GroupId != r.ID {
			t.Errorf("resource[%d].RawStruct.GroupId (%v) does not match ID (%q)", i, sg.GroupId, r.ID)
		}
	}

	// --- Verify SG with IPv6 egress rules (ci-runner-ubuntu) ---
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

	// --- Verify all required fields exist on every resource ---
	requiredFields := []string{"group_id", "group_name", "vpc_id", "description"}
	for i, r := range resources {
		for _, key := range requiredFields {
			if _, ok := r.Fields[key]; !ok {
				t.Errorf("resource[%d].Fields missing key %q", i, key)
			}
		}
	}

	// --- Verify RawJSON contains SG ID for a spot-check ---
	if !strings.Contains(resources[0].RawJSON, "sg-0aa0000000000001a") {
		t.Errorf("resource[0].RawJSON should contain the SG ID")
	}
}
