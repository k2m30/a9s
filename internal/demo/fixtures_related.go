package demo

import (
	"strings"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterRelatedDemo("acm", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "elb", Count: 1, ResourceIDs: []string{relatedACMELBID}},
			{TargetType: "cf", Count: 1, ResourceIDs: []string{relatedACMCFID}},
			{TargetType: "apigw", Count: 1, ResourceIDs: []string{relatedACMApigwID}},
			{TargetType: "r53", Count: 1, ResourceIDs: []string{relatedACMR53ID}},
		}
	})

	resource.RegisterRelatedDemo("ec2", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "tg", Count: 1, ResourceIDs: []string{relatedEC2TGID}},
			{TargetType: "asg", Count: 1, ResourceIDs: []string{relatedEC2ASGID}},
			{TargetType: "alarm", Count: 2, ResourceIDs: []string{relatedEC2AlarmID1, relatedEC2AlarmID2}},
			{TargetType: "cfn", Count: 0},
			{TargetType: "eip", Count: 1, ResourceIDs: []string{relatedEC2EIPID}},
			{TargetType: "ebs-snap", Count: 2, ResourceIDs: []string{relatedEC2SnapshotID1, relatedEC2SnapshotID2}},
			{TargetType: "ebs", Count: 2, ResourceIDs: []string{relatedEC2EBSVolID1, relatedEC2EBSVolID2}},
			{TargetType: "ng", Count: 1, ResourceIDs: []string{relatedEC2NGNodeGroupID}},
			{TargetType: "ct-events", Count: 1, ResourceIDs: []string{relatedEC2TrailEvent1}},
		}
	})

	resource.RegisterRelatedDemo("alarm", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "sns", Count: 1, ResourceIDs: []string{relatedAlarmSNSID}},
			{TargetType: "asg", Count: 1, ResourceIDs: []string{relatedAlarmASGID}},
		}
	})

	resource.RegisterRelatedDemo("ami", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "ec2", Count: 1, ResourceIDs: []string{relatedAMIEC2ID}},
			{TargetType: "ebs-snap", Count: 1, ResourceIDs: []string{relatedAMISnapID1}},
			{TargetType: "asg", Count: 1, ResourceIDs: []string{relatedAMIASGID}},
		}
	})

	resource.RegisterRelatedDemo("apigw", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "lambda", Count: 1, ResourceIDs: []string{relatedApigwLambdaID}},
			{TargetType: "logs", Count: 1, ResourceIDs: []string{relatedApigwLogsID}},
			{TargetType: "waf", Count: 1, ResourceIDs: []string{relatedApigwWAFID}},
		}
	})

	resource.RegisterRelatedDemo("athena", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "s3", Count: 1, ResourceIDs: []string{relatedAthenaS3ID}},
			{TargetType: "kms", Count: 1, ResourceIDs: []string{relatedAthenaKMSID}},
		}
	})

	resource.RegisterRelatedDemo("backup", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "role", Count: 1, ResourceIDs: []string{relatedBackupRoleID}},
		}
	})

	resource.RegisterRelatedDemo("asg", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "ec2", Count: 4, ResourceIDs: []string{relatedASGEC2ID1, relatedASGEC2ID2, relatedASGEC2ID3, relatedASGEC2ID4}},
			{TargetType: "tg", Count: 1, ResourceIDs: []string{relatedASGTGID}},
			{TargetType: "subnet", Count: 3, ResourceIDs: []string{relatedASGSubnetID1, relatedASGSubnetID2, relatedASGSubnetID3}},
			{TargetType: "alarm", Count: 1, ResourceIDs: []string{relatedASGAlarmID}},
			{TargetType: "ng", Count: 0},
		}
	})

	resource.RegisterRelatedDemo("cb", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "logs", Count: 1, ResourceIDs: []string{relatedCbLogsID}},
			{TargetType: "role", Count: 1, ResourceIDs: []string{relatedCbRoleID}},
			{TargetType: "pipeline", Count: 1, ResourceIDs: []string{relatedCbPipelineID}},
		}
	})

	resource.RegisterRelatedDemo("cf", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "s3", Count: 1, ResourceIDs: []string{relatedCfS3ID}},
			{TargetType: "elb", Count: 1, ResourceIDs: []string{relatedCfELBID}},
			{TargetType: "waf", Count: 1, ResourceIDs: []string{relatedCfWAFID}},
			{TargetType: "acm", Count: 1, ResourceIDs: []string{relatedCfACMID}},
			{TargetType: "r53", Count: 1, ResourceIDs: []string{relatedCfR53ID}},
		}
	})

	resource.RegisterRelatedDemo("cfn", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "role", Count: 1, ResourceIDs: []string{relatedCfnRoleID}},
		}
	})

	resource.RegisterRelatedDemo("codeartifact", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "cb", Count: 1, ResourceIDs: []string{relatedCodeartifactCbID}},
		}
	})

	resource.RegisterRelatedDemo("ct-events", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "role", Count: 1, ResourceIDs: []string{relatedCtEventsRoleID}},
			{TargetType: "iam-user", Count: 1, ResourceIDs: []string{relatedCtEventsUserID}},
		}
	})

	resource.RegisterRelatedDemo("dbc", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "sg", Count: 1, ResourceIDs: []string{relatedDbcSGID}},
			{TargetType: "alarm", Count: 1, ResourceIDs: []string{relatedDbcAlarmID}},
			{TargetType: "secrets", Count: 1, ResourceIDs: []string{relatedDbcSecretID}},
			{TargetType: "logs", Count: 1, ResourceIDs: []string{relatedDbcLogsID}},
		}
	})

	resource.RegisterRelatedDemo("dbi", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "sg", Count: 1, ResourceIDs: []string{relatedDbiSGID}},
			{TargetType: "kms", Count: 1, ResourceIDs: []string{relatedDbiKMSID}},
			{TargetType: "subnet", Count: 2, ResourceIDs: []string{relatedDbiSubnetID1, relatedDbiSubnetID2}},
			{TargetType: "alarm", Count: 1, ResourceIDs: []string{relatedDbiAlarmID}},
			{TargetType: "rds-snap", Count: 1, ResourceIDs: []string{relatedDbiRDSSnapID}},
			{TargetType: "secrets", Count: 1, ResourceIDs: []string{relatedDbiSecretID}},
		}
	})

	resource.RegisterRelatedDemo("ddb", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "kms",    Count: 1, ResourceIDs: []string{relatedDdbKMSID}},
			{TargetType: "lambda", Count: 1, ResourceIDs: []string{relatedDdbLambdaID}},
			{TargetType: "alarm",  Count: 1, ResourceIDs: []string{relatedDdbAlarmID}},
		}
	})

	resource.RegisterRelatedDemo("docdb-snap", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "dbc", Count: 1, ResourceIDs: []string{relatedDocdbSnapDbcID}},
			{TargetType: "kms", Count: 1, ResourceIDs: []string{relatedDocdbSnapKMSID}},
		}
	})

	resource.RegisterRelatedDemo("eb", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "cfn",  Count: 1, ResourceIDs: []string{relatedEbCFNID}},
			{TargetType: "logs", Count: 1, ResourceIDs: []string{relatedEbLogsID}},
			{TargetType: "asg",  Count: 1, ResourceIDs: []string{relatedEbASGID}},
		}
	})

	resource.RegisterRelatedDemo("eb-rule", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "role", Count: 1, ResourceIDs: []string{relatedEbRuleRoleID}},
		}
	})

	resource.RegisterRelatedDemo("ebs", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "ec2",      Count: 1, ResourceIDs: []string{relatedEBSEC2ID}},
			{TargetType: "ebs-snap", Count: 1, ResourceIDs: []string{relatedEBSSnapID}},
			{TargetType: "kms",      Count: 1, ResourceIDs: []string{relatedEBSKMSID}},
		}
	})

	resource.RegisterRelatedDemo("ebs-snap", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "ami", Count: 1, ResourceIDs: []string{relatedEBSSnapAMIID}},
			{TargetType: "ebs", Count: 1, ResourceIDs: []string{relatedEBSSnapEBSID}},
			{TargetType: "ec2", Count: 0},
			{TargetType: "kms", Count: 1, ResourceIDs: []string{relatedEBSSnapKMSID}},
		}
	})

	resource.RegisterRelatedDemo("ecr", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "lambda", Count: 1, ResourceIDs: []string{relatedECRLambdaID}},
			{TargetType: "cb", Count: 1, ResourceIDs: []string{relatedECRCbID}},
			{TargetType: "cfn", Count: 0},
		}
	})

	resource.RegisterRelatedDemo("ecs", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "ecs-svc", Count: 3, ResourceIDs: []string{relatedECSSvcID1, relatedECSSvcID2, relatedECSSvcID3}},
			{TargetType: "alarm", Count: 1, ResourceIDs: []string{relatedECSAlarmID}},
			{TargetType: "cfn", Count: 0},
		}
	})

	resource.RegisterRelatedDemo("ecs-svc", func(res resource.Resource) []resource.RelatedCheckResult {
		clusterName := res.Fields["cluster"]
		if clusterName == "" {
			clusterName = "acme-services"
		}
		return []resource.RelatedCheckResult{
			{TargetType: "ecs", Count: 1, ResourceIDs: []string{clusterName}},
			{TargetType: "tg", Count: 1, ResourceIDs: []string{relatedECSSvcTGID}},
			{TargetType: "alarm", Count: 1, ResourceIDs: []string{relatedECSSvcAlarmID}},
			{TargetType: "cfn", Count: 0},
		}
	})

	resource.RegisterRelatedDemo("ecs-task", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "ecs-svc", Count: 1, ResourceIDs: []string{"api-gateway"}},
			{TargetType: "ecs", Count: 1, ResourceIDs: []string{"acme-services"}},
		}
	})

	resource.RegisterRelatedDemo("efs", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "kms", Count: 1, ResourceIDs: []string{relatedEFSKMSID}},
			{TargetType: "cfn", Count: 0},
			{TargetType: "lambda", Count: 0},
		}
	})

	resource.RegisterRelatedDemo("eip", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "ec2", Count: 1, ResourceIDs: []string{"i-0a1b2c3d4e5f60001"}},
			{TargetType: "eni", Count: 1, ResourceIDs: []string{"eni-0aaa111111111111a"}},
			{TargetType: "nat", Count: 0},
		}
	})

	resource.RegisterRelatedDemo("eks", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "ng", Count: 2, ResourceIDs: []string{"general-pool", "gpu-pool"}},
			{TargetType: "alarm", Count: 0},
			{TargetType: "cfn", Count: 0},
		}
	})

	resource.RegisterRelatedDemo("elb", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "tg", Count: 2, ResourceIDs: []string{relatedELBTGID1, relatedELBTGID2}},
			{TargetType: "alarm", Count: 1, ResourceIDs: []string{relatedELBAlarmID1}},
			{TargetType: "cfn", Count: 0},
		}
	})

	resource.RegisterRelatedDemo("eni", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "ec2", Count: 1, ResourceIDs: []string{relatedENIEC2ID}},
			{TargetType: "sg", Count: 1, ResourceIDs: []string{relatedENISGID1}},
			{TargetType: "eip", Count: 1, ResourceIDs: []string{relatedENIEIPID}},
		}
	})

	resource.RegisterRelatedDemo("glue", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "role", Count: 1, ResourceIDs: []string{relatedGlueRoleID1}},
			{TargetType: "alarm", Count: 0},
			{TargetType: "cfn", Count: 0},
		}
	})

	resource.RegisterRelatedDemo("iam-group", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "iam-user", Count: 3},
			{TargetType: "policy", Count: 2},
		}
	})

	resource.RegisterRelatedDemo("iam-user", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "iam-group", Count: 2},
			{TargetType: "policy", Count: 3},
		}
	})

	resource.RegisterRelatedDemo("igw", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "vpc", Count: 1, ResourceIDs: []string{relatedIGWVPCID}},
			{TargetType: "rtb", Count: 2, ResourceIDs: []string{relatedIGWRTBID1, relatedIGWRTBID2}},
		}
	})

	resource.RegisterRelatedDemo("kinesis", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "lambda", Count: 1, ResourceIDs: []string{"data-pipeline-transform"}},
			{TargetType: "alarm", Count: 0},
			{TargetType: "cfn", Count: 0},
		}
	})

	resource.RegisterRelatedDemo("kms", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "ebs", Count: 2, ResourceIDs: []string{relatedKMSEBSID1, relatedKMSEBSID2}},
			{TargetType: "dbi", Count: 1, ResourceIDs: []string{relatedKMSDbiID}},
			{TargetType: "secrets", Count: 0},
		}
	})

	resource.RegisterRelatedDemo("lambda", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "role", Count: 1, ResourceIDs: []string{relatedLambdaRoleID}},
			{TargetType: "alarm", Count: 1, ResourceIDs: []string{relatedLambdaAlarmID}},
			{TargetType: "sqs", Count: 0},
			{TargetType: "cfn", Count: 0},
		}
	})

	resource.RegisterRelatedDemo("logs", func(res resource.Resource) []resource.RelatedCheckResult {
		lambdaCount := 0
		var lambdaIDs []string
		const lambdaPrefix = "/aws/lambda/"
		if strings.HasPrefix(res.ID, lambdaPrefix) {
			lambdaCount = 1
			lambdaIDs = []string{relatedApigwLambdaID}
		}
		return []resource.RelatedCheckResult{
			{TargetType: "lambda", Count: lambdaCount, ResourceIDs: lambdaIDs},
			{TargetType: "alarm", Count: 0},
		}
	})

	resource.RegisterRelatedDemo("msk", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "lambda", Count: 1, ResourceIDs: []string{"data-pipeline-transform"}},
			{TargetType: "alarm", Count: 0},
			{TargetType: "cfn", Count: 0},
		}
	})

	resource.RegisterRelatedDemo("nat", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "vpc", Count: 1, ResourceIDs: []string{relatedNATVPCID}},
			{TargetType: "subnet", Count: 1, ResourceIDs: []string{relatedNATSubnetID}},
			{TargetType: "rtb", Count: 1, ResourceIDs: []string{relatedNATRTBID}},
		}
	})

	resource.RegisterRelatedDemo("ng", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "eks", Count: 1, ResourceIDs: []string{relatedNGEKSID}},
			{TargetType: "role", Count: 1, ResourceIDs: []string{relatedNGRoleID}},
			{TargetType: "asg", Count: 1, ResourceIDs: []string{relatedNGASGID}},
		}
	})

	resource.RegisterRelatedDemo("opensearch", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "alarm", Count: 1, ResourceIDs: []string{"acme-opensearch-cluster-health"}},
			{TargetType: "cfn", Count: 0},
		}
	})

	resource.RegisterRelatedDemo("pipeline", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "cb", Count: 2, ResourceIDs: []string{"acme-api-build", "acme-frontend-build"}},
			{TargetType: "role", Count: 1, ResourceIDs: []string{"acme-ci-deploy-role"}},
		}
	})

	resource.RegisterRelatedDemo("policy", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "role", Count: 5},
			{TargetType: "iam-user", Count: 2},
			{TargetType: "iam-group", Count: 1},
		}
	})

	resource.RegisterRelatedDemo("r53", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "elb", Count: 1, ResourceIDs: []string{relatedCfELBID}},
			{TargetType: "cf", Count: 1, ResourceIDs: []string{relatedACMCFID}},
			{TargetType: "acm", Count: 0},
		}
	})

	resource.RegisterRelatedDemo("rds-snap", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "dbi", Count: 1, ResourceIDs: []string{relatedRDSSnapDbiID}},
			{TargetType: "kms", Count: 1, ResourceIDs: []string{relatedRDSSnapKMSID}},
		}
	})

	resource.RegisterRelatedDemo("redis", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "alarm", Count: 1, ResourceIDs: []string{relatedRedisAlarmID}},
			{TargetType: "cfn", Count: 0},
		}
	})

	resource.RegisterRelatedDemo("redshift", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "alarm", Count: 1, ResourceIDs: []string{relatedRedshiftAlarmID}},
			{TargetType: "cfn", Count: 0},
		}
	})

	resource.RegisterRelatedDemo("rtb", func(res resource.Resource) []resource.RelatedCheckResult {
		switch res.ID {
		case "rtb-0bbb222222222222b":
			return []resource.RelatedCheckResult{
				{TargetType: "subnet", Count: 2, ResourceIDs: []string{relatedRTBSubnetID1, relatedRTBSubnetID2}},
				{TargetType: "nat", Count: 0},
				{TargetType: "igw", Count: 1, ResourceIDs: []string{relatedRTBIGWID}},
				{TargetType: "cfn", Count: 0},
			}
		case "rtb-0aaa111111111111a":
			return []resource.RelatedCheckResult{
				{TargetType: "subnet", Count: 0},
				{TargetType: "nat", Count: 1, ResourceIDs: []string{relatedRTBNATID}},
				{TargetType: "igw", Count: 0},
				{TargetType: "cfn", Count: 0},
			}
		default:
			return []resource.RelatedCheckResult{
				{TargetType: "subnet", Count: 0},
				{TargetType: "nat", Count: 0},
				{TargetType: "igw", Count: 0},
				{TargetType: "cfn", Count: 0},
			}
		}
	})

	resource.RegisterRelatedDemo("s3", func(res resource.Resource) []resource.RelatedCheckResult {
		switch res.ID {
		case "data-pipeline-logs":
			return []resource.RelatedCheckResult{
				{TargetType: "trail", Count: 1, ResourceIDs: []string{relatedS3TrailID}},
				{TargetType: "cf", Count: 0},
				{TargetType: "lambda", Count: 0},
				{TargetType: "cfn", Count: 0},
			}
		case "webapp-assets-prod":
			return []resource.RelatedCheckResult{
				{TargetType: "trail", Count: 0},
				{TargetType: "cf", Count: 1, ResourceIDs: []string{relatedS3CFID}},
				{TargetType: "lambda", Count: 0},
				{TargetType: "cfn", Count: 0},
			}
		default:
			return []resource.RelatedCheckResult{
				{TargetType: "trail", Count: 0},
				{TargetType: "cf", Count: 0},
				{TargetType: "lambda", Count: 0},
				{TargetType: "cfn", Count: 0},
			}
		}
	})

	resource.RegisterRelatedDemo("role", func(res resource.Resource) []resource.RelatedCheckResult {
		switch res.ID {
		case "acme-eks-node-role":
			return []resource.RelatedCheckResult{
				{TargetType: "lambda", Count: 0},
				{TargetType: "glue", Count: 0},
				{TargetType: "ng", Count: 1, ResourceIDs: []string{relatedEC2NGNodeGroupID}},
				{TargetType: "policy", Count: 0},
			}
		case "acme-lambda-execution":
			return []resource.RelatedCheckResult{
				{TargetType: "lambda", Count: 4, ResourceIDs: []string{relatedRoleLambdaID1}},
				{TargetType: "glue", Count: 0},
				{TargetType: "ng", Count: 0},
				{TargetType: "policy", Count: 0},
			}
		case "acme-glue-role":
			return []resource.RelatedCheckResult{
				{TargetType: "lambda", Count: 0},
				{TargetType: "glue", Count: 3, ResourceIDs: []string{relatedRoleGlueID1}},
				{TargetType: "ng", Count: 0},
				{TargetType: "policy", Count: 0},
			}
		default:
			return []resource.RelatedCheckResult{
				{TargetType: "lambda", Count: 0},
				{TargetType: "glue", Count: 0},
				{TargetType: "ng", Count: 0},
				{TargetType: "policy", Count: 0},
			}
		}
	})

	resource.RegisterRelatedDemo("secrets", func(res resource.Resource) []resource.RelatedCheckResult {
		switch res.ID {
		case "prod/docdb/acme-docdb-prod":
			return []resource.RelatedCheckResult{
				{TargetType: "kms", Count: 1, ResourceIDs: []string{relatedSecretsKMSID}},
				{TargetType: "lambda", Count: 1, ResourceIDs: []string{relatedSecretsLambdaID}},
				{TargetType: "dbi", Count: 0},
				{TargetType: "cfn", Count: 0},
			}
		default:
			return []resource.RelatedCheckResult{
				{TargetType: "kms", Count: 0},
				{TargetType: "lambda", Count: 0},
				{TargetType: "dbi", Count: 0},
				{TargetType: "cfn", Count: 0},
			}
		}
	})

	resource.RegisterRelatedDemo("ses", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "r53", Count: 1, ResourceIDs: []string{relatedSESR53ID}},
			{TargetType: "cfn", Count: 0},
		}
	})

	resource.RegisterRelatedDemo("sfn", func(res resource.Resource) []resource.RelatedCheckResult {
		if res.ID == "order-fulfillment-workflow" {
			return []resource.RelatedCheckResult{
				{TargetType: "alarm", Count: 1, ResourceIDs: []string{relatedSFNAlarmID}},
				{TargetType: "logs", Count: 1, ResourceIDs: []string{relatedSFNLogsID}},
				{TargetType: "role", Count: 0},
				{TargetType: "cfn", Count: 0},
			}
		}
		return []resource.RelatedCheckResult{
			{TargetType: "alarm", Count: 0},
			{TargetType: "logs", Count: 0},
			{TargetType: "role", Count: 0},
			{TargetType: "cfn", Count: 0},
		}
	})

	resource.RegisterRelatedDemo("sns", func(res resource.Resource) []resource.RelatedCheckResult {
		if res.ID == "arn:aws:sns:us-east-1:123456789012:alarm-notifications" {
			return []resource.RelatedCheckResult{
				{TargetType: "alarm", Count: 2, ResourceIDs: []string{relatedSNSAlarmID1, relatedSNSAlarmID2}},
				{TargetType: "cfn", Count: 0},
			}
		}
		return []resource.RelatedCheckResult{
			{TargetType: "alarm", Count: 0},
			{TargetType: "cfn", Count: 0},
		}
	})

	resource.RegisterRelatedDemo("sns-sub", func(res resource.Resource) []resource.RelatedCheckResult {
		switch res.ID {
		case "arn:aws:sns:us-east-1:123456789012:alarm-notifications:a1b2c3d4-e5f6-7890-abcd-ef1234567890":
			// protocol=email: topic hit only
			return []resource.RelatedCheckResult{
				{TargetType: "sns", Count: 1, ResourceIDs: []string{relatedSNSSubTopicID}},
				{TargetType: "lambda", Count: 0},
				{TargetType: "sqs", Count: 0},
			}
		case "arn:aws:sns:us-east-1:123456789012:alarm-notifications:b2c3d4e5-f6a7-8901-bcde-f12345678901":
			// protocol=lambda: topic + lambda hit
			return []resource.RelatedCheckResult{
				{TargetType: "sns", Count: 1, ResourceIDs: []string{relatedSNSSubTopicID}},
				{TargetType: "lambda", Count: 1, ResourceIDs: []string{relatedSNSSubLambdaID}},
				{TargetType: "sqs", Count: 0},
			}
		case "arn:aws:sns:us-east-1:123456789012:order-events:c3d4e5f6-a7b8-9012-cdef-123456789012":
			// protocol=sqs: topic + sqs hit
			return []resource.RelatedCheckResult{
				{TargetType: "sns", Count: 1, ResourceIDs: []string{relatedSNSSubTopicID2}},
				{TargetType: "lambda", Count: 0},
				{TargetType: "sqs", Count: 1, ResourceIDs: []string{relatedSNSSubSQSID}},
			}
		}
		return []resource.RelatedCheckResult{
			{TargetType: "sns", Count: 0},
			{TargetType: "lambda", Count: 0},
			{TargetType: "sqs", Count: 0},
		}
	})

	resource.RegisterRelatedDemo("sqs", func(res resource.Resource) []resource.RelatedCheckResult {
		if res.ID == "order-processing-queue" {
			return []resource.RelatedCheckResult{
				{TargetType: "sns-sub", Count: 1, ResourceIDs: []string{relatedSQSSNSSubID}},
				{TargetType: "alarm", Count: 1, ResourceIDs: []string{relatedSQSAlarmID}},
				{TargetType: "lambda", Count: 0},
				{TargetType: "cfn", Count: 0},
			}
		}
		return []resource.RelatedCheckResult{
			{TargetType: "sns-sub", Count: 0},
			{TargetType: "alarm", Count: 0},
			{TargetType: "lambda", Count: 0},
			{TargetType: "cfn", Count: 0},
		}
	})

	resource.RegisterRelatedDemo("ssm", func(res resource.Resource) []resource.RelatedCheckResult {
		switch res.ID {
		case "/acme/prod/app/config", "/acme/prod/db/connection-string":
			return []resource.RelatedCheckResult{
				{TargetType: "kms", Count: 1, ResourceIDs: []string{relatedSSMKMSID}},
				{TargetType: "cfn", Count: 0},
			}
		}
		return []resource.RelatedCheckResult{
			{TargetType: "kms", Count: 0},
			{TargetType: "cfn", Count: 0},
		}
	})

	resource.RegisterRelatedDemo("subnet", func(res resource.Resource) []resource.RelatedCheckResult {
		switch res.ID {
		case prodPublicSubnetA:
			return []resource.RelatedCheckResult{
				{TargetType: "ec2", Count: 3, ResourceIDs: []string{relatedSubnetEC2ID1, relatedSubnetEC2ID2, relatedSubnetEC2ID3}},
				{TargetType: "eni", Count: 1, ResourceIDs: []string{relatedSubnetENIID1}},
				{TargetType: "nat", Count: 1, ResourceIDs: []string{relatedSubnetNATID1}},
				{TargetType: "elb", Count: 2, ResourceIDs: []string{relatedSubnetELBID1, relatedSubnetELBID2}},
				{TargetType: "rtb", Count: 0},
				{TargetType: "cfn", Count: 0},
			}
		}
		return []resource.RelatedCheckResult{
			{TargetType: "ec2", Count: 0},
			{TargetType: "eni", Count: 0},
			{TargetType: "nat", Count: 0},
			{TargetType: "elb", Count: 0},
			{TargetType: "rtb", Count: 0},
			{TargetType: "cfn", Count: 0},
		}
	})

	resource.RegisterRelatedDemo("tg", func(res resource.Resource) []resource.RelatedCheckResult {
		switch res.ID {
		case "acme-web-tg":
			return []resource.RelatedCheckResult{
				{TargetType: "elb", Count: 1, ResourceIDs: []string{prodELBName}},
				{TargetType: "ecs-svc", Count: 1, ResourceIDs: []string{"api-gateway"}},
				{TargetType: "asg", Count: 0},
				{TargetType: "alarm", Count: 0},
				{TargetType: "cfn", Count: 0},
			}
		case "acme-api-tg":
			return []resource.RelatedCheckResult{
				{TargetType: "elb", Count: 1, ResourceIDs: []string{"acme-internal-api"}},
				{TargetType: "ecs-svc", Count: 0},
				{TargetType: "asg", Count: 0},
				{TargetType: "alarm", Count: 0},
				{TargetType: "cfn", Count: 0},
			}
		}
		return []resource.RelatedCheckResult{
			{TargetType: "elb", Count: 0},
			{TargetType: "ecs-svc", Count: 0},
			{TargetType: "asg", Count: 0},
			{TargetType: "alarm", Count: 0},
			{TargetType: "cfn", Count: 0},
		}
	})
}
