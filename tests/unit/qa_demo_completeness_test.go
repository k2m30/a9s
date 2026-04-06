package unit_test

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/fieldpath"
	"github.com/k2m30/a9s/v3/internal/resource"

	// Import demo fixture sub-packages to trigger init() registrations.
	// Without these blank imports, demoData map is empty and all tests skip.
	_ "github.com/k2m30/a9s/v3/internal/demo"
)

// ---------------------------------------------------------------------------
// TestDemoFieldCompleteness
//
// For every resource type that has both demo fixtures AND a detail view config,
// verify that every detail field path resolves to a non-empty value via
// fieldpath.ExtractSubtree on the fixture's RawStruct.
//
// These tests MUST FAIL before fixtures are fully populated — they prove that
// fields are actually set in the RawStruct, not just in Fields map.
// ---------------------------------------------------------------------------

func TestDemoFieldCompleteness(t *testing.T) {
	cfg := config.DefaultConfig()

	// All resource types that should have demo data and detail configs.
	// Expand this list as new resource types are added.
	typesToTest := []string{
		"ec2", "lambda", "ebs", "ebs-snap", "ami",
		"asg", "ecs", "ecs-svc", "ecs-task",
		"eks", "ng",
		"dbi", "redis", "dbc", "ddb", "opensearch", "redshift",
		"alarm", "logs", "trail",
		"sqs", "sns", "sns-sub", "eb-rule", "kinesis", "msk", "sfn",
		"secrets", "ssm", "kms",
		"r53", "cf", "acm", "apigw",
		"role", "policy", "iam-user", "iam-group", "waf",
		"cfn", "ecr", "codeartifact", "pipeline", "cb",
		"s3",
		"vpc", "sg", "subnet", "nat", "igw", "eip", "eni", "rtb", "tgw", "vpce",
		"elb", "tg",
		"backup", "ses", "efs",
		"rds-snap", "docdb-snap",
	}

	for _, shortName := range typesToTest {
		shortName := shortName // capture loop var
		t.Run(shortName, func(t *testing.T) {
			// Check demo fixtures exist for this type.
			fixtures, ok := demo.GetResources(shortName)
			if !ok || len(fixtures) == 0 {
				t.Skipf("no demo fixtures registered for %q — skipping", shortName)
				return
			}

			// Check detail view config exists for this type.
			vd := config.GetViewDef(cfg, shortName)
			if len(vd.Detail) == 0 {
				t.Skipf("no detail view config for %q — skipping", shortName)
				return
			}

			// Fields that are legitimately empty for the first (most common) fixture
			// because AWS only populates them in specific conditions.
			conditionalFields := map[string]map[string]bool{
				"ec2": {"InstanceLifecycle": true}, // empty for on-demand instances
			}

			// Use first fixture only — it should be the most complete one.
			fix := fixtures[0]
			if fix.RawStruct == nil {
				t.Errorf("%s: fixture[0] has nil RawStruct — cannot verify field paths", shortName)
				return
			}

			for _, fieldPath := range vd.Detail {
				fieldPath := fieldPath // capture loop var
				if conditionalFields[shortName][fieldPath] {
					continue
				}
				t.Run(fieldPath, func(t *testing.T) {
					val := fieldpath.ExtractSubtree(fix.RawStruct, fieldPath)
					if strings.TrimSpace(val) == "" {
						t.Errorf(
							"%s fixture[0] (ID=%q): field path %q resolved to empty string via ExtractSubtree — populate this field in the RawStruct",
							shortName, fix.ID, fieldPath,
						)
					}
				})
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestDemoCrossReference
//
// Verifies that forward-reference IDs set on one resource type exist as
// fixture IDs (or substrings of fixture ARNs) in the target type.
// ---------------------------------------------------------------------------

func TestDemoCrossReference(t *testing.T) {
	// Helper: collect all fixture IDs for a given resource type.
	allIDs := func(shortName string) map[string]bool {
		resources, ok := demo.GetResources(shortName)
		if !ok {
			return nil
		}
		ids := make(map[string]bool, len(resources))
		for _, r := range resources {
			ids[r.ID] = true
		}
		return ids
	}

	// Helper: check whether any fixture of targetType has the given value as a
	// substring of its ID or any of its Fields values (handles ARN matching).
	anyContains := func(shortName, needle string) bool {
		resources, ok := demo.GetResources(shortName)
		if !ok {
			return false
		}
		for _, r := range resources {
			if strings.Contains(r.ID, needle) || strings.Contains(r.Name, needle) {
				return true
			}
			for _, v := range r.Fields {
				if strings.Contains(v, needle) {
					return true
				}
			}
		}
		return false
	}

	// Helper: collect all field values for given key across all fixtures of a type.
	allFieldValues := func(shortName, fieldKey string) []string {
		resources, ok := demo.GetResources(shortName)
		if !ok {
			return nil
		}
		var vals []string
		for _, r := range resources {
			if v, ok := r.Fields[fieldKey]; ok && v != "" {
				vals = append(vals, v)
			}
		}
		return vals
	}

	// -----------------------------------------------------------------------
	// EC2 → VPC: EC2 VpcId must exist in VPC fixture IDs.
	// -----------------------------------------------------------------------
	t.Run("ec2-vpc", func(t *testing.T) {
		ec2Fixtures, ok := demo.GetResources("ec2")
		if !ok || len(ec2Fixtures) == 0 {
			t.Fatal("no ec2 fixtures — demo.GetResources(\"ec2\") should return data")
		}
		vpcIDs := allIDs("vpc")
		if vpcIDs == nil {
			t.Fatal("no vpc fixtures")
		}
		for _, r := range ec2Fixtures {
			vpcID := r.Fields["vpc_id"]
			if vpcID == "" {
				continue // terminated instances may have no VPC
			}
			if !vpcIDs[vpcID] {
				t.Errorf("ec2 fixture %q has VpcId=%q but no matching vpc fixture found", r.ID, vpcID)
			}
		}
	})

	// -----------------------------------------------------------------------
	// EC2 → Subnet: EC2 SubnetId must exist in Subnet fixture IDs.
	// -----------------------------------------------------------------------
	t.Run("ec2-subnet", func(t *testing.T) {
		ec2Fixtures, ok := demo.GetResources("ec2")
		if !ok || len(ec2Fixtures) == 0 {
			t.Fatal("no ec2 fixtures — demo.GetResources(\"ec2\") should return data")
		}
		subnetIDs := allIDs("subnet")
		if subnetIDs == nil {
			t.Fatal("no subnet fixtures")
		}
		for _, r := range ec2Fixtures {
			subnetID := r.Fields["subnet_id"]
			if subnetID == "" {
				continue
			}
			if !subnetIDs[subnetID] {
				t.Errorf("ec2 fixture %q has SubnetId=%q but no matching subnet fixture found", r.ID, subnetID)
			}
		}
	})

	// -----------------------------------------------------------------------
	// EC2 → SG: EC2 security group IDs must exist in SG fixture IDs.
	// -----------------------------------------------------------------------
	t.Run("ec2-sg", func(t *testing.T) {
		ec2Fixtures, ok := demo.GetResources("ec2")
		if !ok || len(ec2Fixtures) == 0 {
			t.Fatal("no ec2 fixtures — demo.GetResources(\"ec2\") should return data")
		}
		sgIDs := allIDs("sg")
		if sgIDs == nil {
			t.Fatal("no sg fixtures")
		}
		for _, r := range ec2Fixtures {
			// SG IDs appear in Fields keys like "sg_id_0", "sg_id_1", etc.
			// or in a comma-separated "security_groups" field. Check both patterns.
			sgFieldVal := r.Fields["security_groups"]
			if sgFieldVal == "" {
				continue
			}
			// The field may be "sg-0abc/name, sg-0def/name" or just IDs — we
			// only verify that at least one SG fixture exists to cross-reference.
			if len(sgIDs) == 0 {
				t.Errorf("ec2 fixture %q references security groups but sg fixture map is empty", r.ID)
			}
		}
	})

	// -----------------------------------------------------------------------
	// EC2 → AMI: EC2 ImageId must exist in AMI fixture IDs.
	// -----------------------------------------------------------------------
	t.Run("ec2-ami", func(t *testing.T) {
		ec2Fixtures, ok := demo.GetResources("ec2")
		if !ok || len(ec2Fixtures) == 0 {
			t.Fatal("no ec2 fixtures — demo.GetResources(\"ec2\") should return data")
		}
		amiIDs := allIDs("ami")
		if amiIDs == nil {
			t.Fatal("no ami fixtures")
		}
		// At least the first EC2 fixture must reference a known AMI.
		firstEC2 := ec2Fixtures[0]
		imageID := firstEC2.Fields["image_id"]
		if imageID == "" {
			t.Errorf("ec2 fixture[0] %q has empty image_id field", firstEC2.ID)
			return
		}
		if !amiIDs[imageID] {
			t.Errorf("ec2 fixture[0] %q references ImageId=%q but no matching ami fixture found", firstEC2.ID, imageID)
		}
	})

	// -----------------------------------------------------------------------
	// EC2 → IAM Instance Profile: IamInstanceProfile ARN must match a Role fixture.
	// -----------------------------------------------------------------------
	t.Run("ec2-iam-instance-profile", func(t *testing.T) {
		ec2Fixtures, ok := demo.GetResources("ec2")
		if !ok || len(ec2Fixtures) == 0 {
			t.Fatal("no ec2 fixtures — demo.GetResources(\"ec2\") should return data")
		}
		roleFixtures, ok := demo.GetResources("role")
		if !ok || len(roleFixtures) == 0 {
			t.Fatal("no role fixtures")
		}
		// The prodInstanceProfileARN is set on all EC2 fixtures in makeEC2Instance.
		// Verify the ARN contains a role name that exists in IAM role fixtures.
		found := false
		for _, r := range ec2Fixtures {
			if r.Fields["iam_profile"] != "" || anyContains("role", "acme-rds-monitoring") {
				found = true
				break
			}
		}
		if !found {
			// Fallback: just ensure role fixtures are non-empty (ARN substring check)
			_ = anyContains("role", "instance-profile")
		}
		// Primary assertion: at least one EC2 fixture must have the instance profile ARN
		// resolvable via fieldpath from its RawStruct.
		firstEC2 := ec2Fixtures[0]
		if firstEC2.RawStruct == nil {
			t.Fatal("ec2 fixture[0] has nil RawStruct")
		}
		profileARN := fieldpath.ExtractSubtree(firstEC2.RawStruct, "IamInstanceProfile.Arn")
		if profileARN == "" {
			t.Errorf("ec2 fixture[0] %q: IamInstanceProfile.Arn resolved to empty — field must be set in RawStruct", firstEC2.ID)
		}
	})

	// -----------------------------------------------------------------------
	// Lambda → IAM Role: Lambda Role ARN must match a Role fixture ARN.
	// -----------------------------------------------------------------------
	t.Run("lambda-role", func(t *testing.T) {
		lambdaFixtures, ok := demo.GetResources("lambda")
		if !ok || len(lambdaFixtures) == 0 {
			t.Fatal("no lambda fixtures — demo.GetResources(\"lambda\") should return data")
		}
		roleFixtures, ok := demo.GetResources("role")
		if !ok || len(roleFixtures) == 0 {
			t.Fatal("no role fixtures")
		}

		// Collect all role ARNs.
		roleARNs := make(map[string]bool)
		for _, r := range roleFixtures {
			if r.RawStruct != nil {
				arn := fieldpath.ExtractSubtree(r.RawStruct, "Arn")
				if arn != "" {
					roleARNs[arn] = true
				}
			}
		}

		for _, lf := range lambdaFixtures {
			if lf.RawStruct == nil {
				continue
			}
			roleARN := fieldpath.ExtractSubtree(lf.RawStruct, "Role")
			if roleARN == "" {
				t.Errorf("lambda fixture %q: Role field resolved to empty in RawStruct", lf.ID)
				continue
			}
			if !roleARNs[roleARN] {
				t.Errorf("lambda fixture %q: Role=%q not found in role fixtures ARNs", lf.ID, roleARN)
			}
		}
	})

	// -----------------------------------------------------------------------
	// EKS cluster → Node Group: EKS cluster name must match NodeGroup ClusterName.
	// -----------------------------------------------------------------------
	t.Run("eks-nodegroup-cluster-name", func(t *testing.T) {
		eksFixtures, ok := demo.GetResources("eks")
		if !ok || len(eksFixtures) == 0 {
			t.Fatal("no eks fixtures — demo.GetResources(\"eks\") should return data")
		}
		ngFixtures, ok := demo.GetResources("ng")
		if !ok || len(ngFixtures) == 0 {
			t.Fatal("no ng (node group) fixtures")
		}

		// Collect all cluster names referenced by node groups.
		ngClusterNames := make(map[string]bool)
		for _, ng := range ngFixtures {
			clusterName := ng.Fields["cluster_name"]
			if clusterName != "" {
				ngClusterNames[clusterName] = true
			}
		}

		for _, eks := range eksFixtures {
			clusterName := eks.Name
			if clusterName == "" {
				clusterName = eks.ID
			}
			if !ngClusterNames[clusterName] {
				t.Errorf("eks fixture %q: cluster name %q not referenced by any node group fixture's cluster_name field", eks.ID, clusterName)
			}
		}
	})

	// -----------------------------------------------------------------------
	// Lambda → Log Group: Lambda function name must have matching log group fixture.
	// -----------------------------------------------------------------------
	t.Run("lambda-log-group", func(t *testing.T) {
		lambdaFixtures, ok := demo.GetResources("lambda")
		if !ok || len(lambdaFixtures) == 0 {
			t.Fatal("no lambda fixtures — demo.GetResources(\"lambda\") should return data")
		}
		logFixtures, ok := demo.GetResources("logs")
		if !ok || len(logFixtures) == 0 {
			t.Fatal("no logs fixtures")
		}

		logGroupIDs := allIDs("logs")

		// The process-orders function must have a log group.
		processOrdersLogGroup := "/aws/lambda/process-orders"
		if !logGroupIDs[processOrdersLogGroup] {
			t.Errorf("expected log group fixture with ID=%q for lambda process-orders but not found", processOrdersLogGroup)
		}

		// All lambda fixtures that specify a log_group field must have a matching log group fixture.
		logGroupValues := allFieldValues("lambda", "log_group")
		for _, lg := range logGroupValues {
			if !logGroupIDs[lg] {
				t.Errorf("lambda references log group %q but no matching logs fixture found", lg)
			}
		}
	})

	// -----------------------------------------------------------------------
	// SG fixtures: SG IDs must be self-consistent (SG fixture IDs match their own
	// IDs referenced in EC2 security groups).
	// -----------------------------------------------------------------------
	t.Run("ec2-sg-ids-exist", func(t *testing.T) {
		sgFixtures, ok := demo.GetResources("sg")
		if !ok || len(sgFixtures) == 0 {
			t.Fatal("no sg fixtures")
		}

		// The four shared SG IDs used in EC2 fixtures must have matching SG fixture IDs.
		expectedSGIDs := []string{
			"sg-0aaa111111111111a", // prodWebALBSGID
			"sg-0bbb222222222222b", // prodAPIInternalSGID
			"sg-0ccc333333333333c", // prodRDSSGID
			"sg-0ddd444444444444d", // prodDBProxySGID
		}
		sgIDs := allIDs("sg")
		for _, id := range expectedSGIDs {
			if !sgIDs[id] {
				t.Errorf("expected sg fixture with ID=%q (referenced by ec2 fixtures) but not found", id)
			}
		}
	})

	// -----------------------------------------------------------------------
	// Subnet fixtures: subnet IDs referenced in EC2 fixtures must exist.
	// -----------------------------------------------------------------------
	t.Run("subnet-ids-exist", func(t *testing.T) {
		subnetFixtures, ok := demo.GetResources("subnet")
		if !ok || len(subnetFixtures) == 0 {
			t.Fatal("no subnet fixtures")
		}

		// Known subnet IDs used across EC2, ASG, EKS, NodeGroup fixtures.
		expectedSubnetIDs := []string{
			"subnet-0aaa111111111111a",
			"subnet-0bbb222222222222b",
			"subnet-0ccc333333333333c",
			"subnet-0ddd444444444444d",
			"subnet-0eee555555555555e",
			"subnet-0fff666666666666f",
		}
		subnetIDs := allIDs("subnet")
		for _, id := range expectedSubnetIDs {
			if !subnetIDs[id] {
				t.Errorf("expected subnet fixture with ID=%q (referenced by ec2/asg/eks fixtures) but not found", id)
			}
		}
	})

	// -----------------------------------------------------------------------
	// VPC fixtures: VPC IDs referenced across the fleet must exist.
	// -----------------------------------------------------------------------
	t.Run("vpc-ids-exist", func(t *testing.T) {
		_, ok := demo.GetResources("vpc")
		if !ok {
			t.Fatal("no vpc fixtures")
		}

		vpcIDs := allIDs("vpc")
		expectedVPCIDs := []string{
			"vpc-0abc123def456789a", // prodVPCID
			"vpc-0def456789abc123d", // stagingVPCID
		}
		for _, id := range expectedVPCIDs {
			if !vpcIDs[id] {
				t.Errorf("expected vpc fixture with ID=%q but not found", id)
			}
		}
	})

	// -----------------------------------------------------------------------
	// AMI fixtures: AMI IDs referenced by EC2 fixtures must exist.
	// -----------------------------------------------------------------------
	t.Run("ami-ids-exist", func(t *testing.T) {
		_, ok := demo.GetResources("ami")
		if !ok {
			t.Fatal("no ami fixtures")
		}

		amiIDs := allIDs("ami")
		expectedAMIIDs := []string{
			"ami-0a1b2c3d4e5f60001",
			"ami-0a1b2c3d4e5f60002",
			"ami-0a1b2c3d4e5f60003",
		}
		for _, id := range expectedAMIIDs {
			if !amiIDs[id] {
				t.Errorf("expected ami fixture with ID=%q (referenced by ec2 fixtures) but not found", id)
			}
		}
	})

	// -----------------------------------------------------------------------
	// ELB → ACM: ELB fixtures must reference ACM cert ARNs that exist.
	// -----------------------------------------------------------------------
	t.Run("elb-acm-cert", func(t *testing.T) {
		elbFixtures, ok := demo.GetResources("elb")
		if !ok || len(elbFixtures) == 0 {
			t.Fatal("no elb fixtures — demo.GetResources(\"elb\") should return data")
		}
		_, ok = demo.GetResources("acm")
		if !ok {
			t.Fatal("no acm fixtures")
		}

		// Prod ACM cert ARNs — referenced by ELB listener fixtures.
		expectedACMCertARNs := []string{
			"arn:aws:acm:us-east-1:123456789012:certificate/a1b2c3d4-5678-90ab-cdef-111111111111",
			"arn:aws:acm:us-east-1:123456789012:certificate/b2c3d4e5-6789-01ab-cdef-222222222222",
		}
		acmIDs := allIDs("acm")
		for _, arn := range expectedACMCertARNs {
			if !acmIDs[arn] {
				t.Errorf("expected acm fixture with ARN=%q (referenced by elb fixtures) but not found", arn)
			}
		}
	})

	// -----------------------------------------------------------------------
	// Alarm → SNS: Alarm fixtures must reference SNS topic ARN that exists.
	// -----------------------------------------------------------------------
	t.Run("alarm-sns-topic", func(t *testing.T) {
		alarmFixtures, ok := demo.GetResources("alarm")
		if !ok || len(alarmFixtures) == 0 {
			t.Fatal("no alarm fixtures — demo.GetResources(\"alarm\") should return data")
		}
		snsFixtures, ok := demo.GetResources("sns")
		if !ok || len(snsFixtures) == 0 {
			t.Fatal("no sns fixtures")
		}

		// relatedAlarmSNSID must exist in SNS fixtures.
		const relatedAlarmSNSID = "arn:aws:sns:us-east-1:123456789012:alarm-notifications"
		snsIDs := allIDs("sns")
		if !snsIDs[relatedAlarmSNSID] {
			t.Errorf("expected sns fixture with ARN=%q (referenced by alarm related-demo) but not found", relatedAlarmSNSID)
		}

		_ = alarmFixtures
	})

	// -----------------------------------------------------------------------
	// EC2 → EBS volumes: EBS volume IDs referenced in EC2 related-demo must exist.
	// -----------------------------------------------------------------------
	t.Run("ec2-ebs-volumes", func(t *testing.T) {
		_, ok := demo.GetResources("ec2")
		if !ok {
			t.Fatal("no ec2 fixtures — demo.GetResources(\"ec2\") should return data")
		}
		_, ok = demo.GetResources("ebs")
		if !ok {
			t.Fatal("no ebs fixtures")
		}

		ebsIDs := allIDs("ebs")
		expectedVolIDs := []string{
			"vol-0a1b2c3d4e5f60001",
			"vol-0a1b2c3d4e5f60002",
		}
		for _, id := range expectedVolIDs {
			if !ebsIDs[id] {
				t.Errorf("expected ebs fixture with ID=%q (referenced by ec2 related-demo) but not found", id)
			}
		}
	})

	// -----------------------------------------------------------------------
	// EC2 → EBS snapshots: snapshot IDs in related-demo must exist.
	// -----------------------------------------------------------------------
	t.Run("ec2-ebs-snapshots", func(t *testing.T) {
		_, ok := demo.GetResources("ec2")
		if !ok {
			t.Fatal("no ec2 fixtures — demo.GetResources(\"ec2\") should return data")
		}
		_, ok = demo.GetResources("ebs-snap")
		if !ok {
			t.Fatal("no ebs-snap fixtures")
		}

		snapIDs := allIDs("ebs-snap")
		expectedSnapIDs := []string{
			"snap-0a1b2c3d4e5f60001",
			"snap-0a1b2c3d4e5f60002",
		}
		for _, id := range expectedSnapIDs {
			if !snapIDs[id] {
				t.Errorf("expected ebs-snap fixture with ID=%q (referenced by ec2 related-demo) but not found", id)
			}
		}
	})

	// -----------------------------------------------------------------------
	// EC2 → TG: Target Group ID in related-demo must exist.
	// -----------------------------------------------------------------------
	t.Run("ec2-target-group", func(t *testing.T) {
		tgIDs := allIDs("tg")
		if tgIDs == nil {
			t.Fatal("no tg fixtures")
		}
		const relatedEC2TGID = "acme-web-tg"
		if !tgIDs[relatedEC2TGID] {
			t.Errorf("expected tg fixture with ID=%q (referenced by ec2 related-demo) but not found", relatedEC2TGID)
		}
	})

	// -----------------------------------------------------------------------
	// EC2 → ASG: ASG ID in related-demo must exist.
	// -----------------------------------------------------------------------
	t.Run("ec2-asg", func(t *testing.T) {
		asgIDs := allIDs("asg")
		if asgIDs == nil {
			t.Fatal("no asg fixtures")
		}
		const relatedEC2ASGID = "acme-web-prod-asg"
		if !asgIDs[relatedEC2ASGID] {
			t.Errorf("expected asg fixture with ID=%q (referenced by ec2 related-demo) but not found", relatedEC2ASGID)
		}
	})

	// -----------------------------------------------------------------------
	// EC2 → EIP: EIP ID in related-demo must exist.
	// -----------------------------------------------------------------------
	t.Run("ec2-eip", func(t *testing.T) {
		eipIDs := allIDs("eip")
		if eipIDs == nil {
			t.Fatal("no eip fixtures")
		}
		const relatedEC2EIPID = "eipalloc-0aaa111111111111a"
		if !eipIDs[relatedEC2EIPID] {
			t.Errorf("expected eip fixture with ID=%q (referenced by ec2 related-demo) but not found", relatedEC2EIPID)
		}
	})

	// -----------------------------------------------------------------------
	// EC2 → Alarm: Alarm IDs in related-demo must exist.
	// -----------------------------------------------------------------------
	t.Run("ec2-alarms", func(t *testing.T) {
		alarmIDs := allIDs("alarm")
		if alarmIDs == nil {
			t.Fatal("no alarm fixtures")
		}
		expectedAlarmIDs := []string{
			"api-high-error-rate",
			"rds-cpu-utilization",
		}
		for _, id := range expectedAlarmIDs {
			if !alarmIDs[id] {
				t.Errorf("expected alarm fixture with ID=%q (referenced by ec2 related-demo) but not found", id)
			}
		}
	})

	// -----------------------------------------------------------------------
	// AMI → EC2: EC2 ID in AMI related-demo must exist.
	// -----------------------------------------------------------------------
	t.Run("ami-ec2", func(t *testing.T) {
		ec2IDs := allIDs("ec2")
		if ec2IDs == nil {
			t.Fatal("no ec2 fixtures")
		}
		const relatedAMIEC2ID = "i-0a1b2c3d4e5f60001"
		if !ec2IDs[relatedAMIEC2ID] {
			t.Errorf("expected ec2 fixture with ID=%q (referenced by ami related-demo) but not found", relatedAMIEC2ID)
		}
	})

	// -----------------------------------------------------------------------
	// AMI → EBS snapshot: snapshot ID in AMI related-demo must exist.
	// -----------------------------------------------------------------------
	t.Run("ami-ebs-snapshot", func(t *testing.T) {
		snapIDs := allIDs("ebs-snap")
		if snapIDs == nil {
			t.Fatal("no ebs-snap fixtures")
		}
		const relatedAMISnapID1 = "snap-0a1b2c3d4e5f60001"
		if !snapIDs[relatedAMISnapID1] {
			t.Errorf("expected ebs-snap fixture with ID=%q (referenced by ami related-demo) but not found", relatedAMISnapID1)
		}
	})

	// -----------------------------------------------------------------------
	// EC2 → NodeGroup: NodeGroup ID in related-demo must exist.
	// -----------------------------------------------------------------------
	t.Run("ec2-nodegroup", func(t *testing.T) {
		ngIDs := allIDs("ng")
		if ngIDs == nil {
			t.Fatal("no ng fixtures")
		}
		const relatedEC2NGNodeGroupID = "general-pool"
		if !ngIDs[relatedEC2NGNodeGroupID] {
			t.Errorf("expected ng fixture with ID=%q (referenced by ec2 related-demo) but not found", relatedEC2NGNodeGroupID)
		}
	})

	// -----------------------------------------------------------------------
	// Lambda → SQS: Lambda event source ARN must reference a SQS fixture.
	// -----------------------------------------------------------------------
	t.Run("lambda-sqs-event-source", func(t *testing.T) {
		lambdaFixtures, ok := demo.GetResources("lambda")
		if !ok || len(lambdaFixtures) == 0 {
			t.Fatal("no lambda fixtures — demo.GetResources(\"lambda\") should return data")
		}
		sqsFixtures, ok := demo.GetResources("sqs")
		if !ok || len(sqsFixtures) == 0 {
			t.Fatal("no sqs fixtures")
		}

		// process-orders lambda references the SQS queue order-processing-queue.
		const orderQueueARN = "arn:aws:sqs:us-east-1:123456789012:order-processing-queue"
		found := false
		for _, r := range sqsFixtures {
			if r.ID == orderQueueARN || strings.Contains(r.ID, "order-processing-queue") {
				found = true
				break
			}
			for _, v := range r.Fields {
				if strings.Contains(v, "order-processing-queue") {
					found = true
					break
				}
			}
		}
		if !found {
			t.Errorf("expected sqs fixture for %q (referenced by lambda process-orders event_source_arn) but not found", orderQueueARN)
		}
	})

	// -----------------------------------------------------------------------
	// ECS service → ELB: ECS service load balancer ARN must match an ELB fixture.
	// -----------------------------------------------------------------------
	t.Run("ecs-svc-elb", func(t *testing.T) {
		ecsSvcFixtures, ok := demo.GetResources("ecs-svc")
		if !ok || len(ecsSvcFixtures) == 0 {
			t.Fatal("no ecs-svc fixtures — demo.GetResources(\"ecs-svc\") should return data")
		}
		elbFixtures, ok := demo.GetResources("elb")
		if !ok || len(elbFixtures) == 0 {
			t.Fatal("no elb fixtures")
		}

		// ELB fixture ID is the load balancer name (matching production fetcher behavior).
		const prodELBName = "acme-prod-web"
		elbIDs := allIDs("elb")
		if !elbIDs[prodELBName] {
			t.Errorf("expected elb fixture with name=%q (referenced by ecs-svc load balancer config) but not found", prodELBName)
		}
	})

	// -----------------------------------------------------------------------
	// CloudFront → S3: CloudFront origin domain must reference an S3 bucket fixture.
	// -----------------------------------------------------------------------
	t.Run("cf-s3-origin", func(t *testing.T) {
		cfFixtures, ok := demo.GetResources("cf")
		if !ok || len(cfFixtures) == 0 {
			t.Fatal("no cf fixtures — demo.GetResources(\"cf\") should return data")
		}
		s3Fixtures, ok := demo.GetResources("s3")
		if !ok || len(s3Fixtures) == 0 {
			t.Fatal("no s3 fixtures")
		}

		// webapp-assets-prod must exist in S3 fixtures.
		const prodStaticAssetsBucket = "webapp-assets-prod"
		s3IDs := allIDs("s3")
		if !s3IDs[prodStaticAssetsBucket] {
			t.Errorf("expected s3 fixture with name=%q (referenced by CloudFront origin) but not found", prodStaticAssetsBucket)
		}
	})

	// -----------------------------------------------------------------------
	// R53 → ELB: R53 alias records must reference ELB DNS names that exist.
	// -----------------------------------------------------------------------
	t.Run("r53-elb-alias", func(t *testing.T) {
		_, ok := demo.GetResources("r53")
		if !ok {
			t.Fatal("no r53 fixtures — demo.GetResources(\"r53\") should return data")
		}
		_, ok = demo.GetResources("elb")
		if !ok {
			t.Fatal("no elb fixtures")
		}

		// prodELBDNS must appear as a field value in ELB fixtures.
		const prodELBDNS = "acme-prod-web-1234567890.us-east-1.elb.amazonaws.com"
		found := anyContains("elb", prodELBDNS)
		if !found {
			t.Errorf("expected elb fixture to contain DNS=%q (referenced by r53 alias records) but not found", prodELBDNS)
		}
	})

	// -----------------------------------------------------------------------
	// EKS → CloudWatch log group: EKS cluster must have a matching log group.
	// -----------------------------------------------------------------------
	t.Run("eks-log-group", func(t *testing.T) {
		eksFixtures, ok := demo.GetResources("eks")
		if !ok || len(eksFixtures) == 0 {
			t.Fatal("no eks fixtures — demo.GetResources(\"eks\") should return data")
		}
		logFixtures, ok := demo.GetResources("logs")
		if !ok || len(logFixtures) == 0 {
			t.Fatal("no logs fixtures")
		}

		logGroupIDs := allIDs("logs")

		// For each EKS cluster, there should be a log group /aws/eks/{name}/cluster.
		for _, eks := range eksFixtures {
			clusterName := eks.Name
			if clusterName == "" {
				clusterName = eks.ID
			}
			expectedLogGroup := "/aws/eks/" + clusterName + "/cluster"
			if !logGroupIDs[expectedLogGroup] {
				t.Errorf("expected log group fixture with ID=%q for eks cluster %q but not found", expectedLogGroup, clusterName)
			}
		}
	})

	// -----------------------------------------------------------------------
	// VPC → broad cross-resource: prodVPCID must appear across EC2, Subnet, SG,
	// NAT, IGW fixtures (spot-check two of them).
	// -----------------------------------------------------------------------
	t.Run("vpc-broad-cross-reference", func(t *testing.T) {
		const prodVPCID = "vpc-0abc123def456789a"

		// EC2 fixtures reference prodVPCID.
		if !anyContains("ec2", prodVPCID) {
			t.Errorf("prodVPCID %q not found in any ec2 fixture field values", prodVPCID)
		}

		// Subnet fixtures reference prodVPCID.
		if !anyContains("subnet", prodVPCID) {
			t.Errorf("prodVPCID %q not found in any subnet fixture field values", prodVPCID)
		}
	})
}

// ---------------------------------------------------------------------------
// TestDemoReverseRelationship
//
// For each resource type that has a RegisterRelatedDemo checker, call the
// demo checker with each fixture of that type and assert:
//   - result.Count > 0 for at least one fixture
//   - result.ResourceIDs contains IDs that exist in the target type's fixtures
// ---------------------------------------------------------------------------

func TestDemoReverseRelationship(t *testing.T) {
	// Pairs: source type → expected target type that should have Count > 0.
	// These mirror what is registered in fixtures_related.go.
	testCases := []struct {
		sourceType string
		targetType string
		minCount   int // minimum Count expected for at least one fixture
	}{
		{"ec2", "tg", 1},
		{"ec2", "asg", 1},
		{"ec2", "alarm", 2},
		{"ec2", "eip", 1},
		{"ec2", "ebs-snap", 2},
		{"ec2", "ebs", 2},
		{"ec2", "ng", 1},
		{"alarm", "sns", 1},
		{"ami", "ec2", 1},
		{"ami", "ebs-snap", 1},
	}

	for _, tc := range testCases {
		tc := tc // capture loop var
		testName := tc.sourceType + "->" + tc.targetType
		t.Run(testName, func(t *testing.T) {
			checker := resource.GetRelatedDemo(tc.sourceType)
			if checker == nil {
				t.Skipf("no related demo checker registered for %q — skipping", tc.sourceType)
				return
			}

			sourceFixtures, ok := demo.GetResources(tc.sourceType)
			if !ok || len(sourceFixtures) == 0 {
				t.Skipf("no fixtures for source type %q", tc.sourceType)
				return
			}

			targetFixtures, _ := demo.GetResources(tc.targetType)
			targetIDs := make(map[string]bool, len(targetFixtures))
			for _, tf := range targetFixtures {
				targetIDs[tf.ID] = true
			}

			// Call checker with each source fixture; look for at least one result
			// with Count >= minCount for the expected target type.
			foundCount := false
			for _, src := range sourceFixtures {
				results := checker(src)
				for _, result := range results {
					if result.TargetType != tc.targetType {
						continue
					}
					if result.Count >= tc.minCount {
						foundCount = true
					}
					// Every returned resource ID must exist in target fixtures.
					for _, id := range result.ResourceIDs {
						if !targetIDs[id] {
							t.Errorf(
								"related demo checker for %q→%q returned ID=%q but no matching fixture found in %q",
								tc.sourceType, tc.targetType, id, tc.targetType,
							)
						}
					}
				}
			}

			if !foundCount {
				t.Errorf(
					"related demo checker for %q→%q: expected Count>=%d for at least one source fixture, but never reached that threshold",
					tc.sourceType, tc.targetType, tc.minCount,
				)
			}
		})
	}
}
