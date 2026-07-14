package unit_test

import (
	"time"

	acmtypes "github.com/aws/aws-sdk-go-v2/service/acm/types"
	autoscalingtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	sesv2types "github.com/aws/aws-sdk-go-v2/service/sesv2/types"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

var svcTestTime = time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)

// ---------------------------------------------------------------------------
// Realistic SDK struct builders for service types
// ---------------------------------------------------------------------------

func realisticLambdaFunction() lambdatypes.FunctionConfiguration {
	return lambdatypes.FunctionConfiguration{
		FunctionName: new("my-api-handler"),
		FunctionArn:  new("arn:aws:lambda:us-east-1:123456789012:function:my-api-handler"),
		Runtime:      lambdatypes.RuntimePython312,
		Handler:      new("index.handler"),
		MemorySize:   new(int32(256)),
		Timeout:      new(int32(30)),
		CodeSize:     5242880,
		Description:  new("API request handler"),
		Role:         new("arn:aws:iam::123456789012:role/lambda-exec-role"),
		State:        lambdatypes.StateActive,
		LastModified: new("2025-06-15T10:30:00.000+0000"),
	}
}

func realisticAlarm() cwtypes.MetricAlarm {
	return cwtypes.MetricAlarm{
		AlarmName:          new("HighCPUAlarm"),
		AlarmArn:           new("arn:aws:cloudwatch:us-east-1:123456789012:alarm:HighCPUAlarm"),
		StateValue:         cwtypes.StateValueAlarm,
		MetricName:         new("CPUUtilization"),
		Namespace:          new("AWS/EC2"),
		Statistic:          cwtypes.StatisticAverage,
		Period:             new(int32(300)),
		EvaluationPeriods:  new(int32(3)),
		Threshold:          new(80.0),
		ComparisonOperator: cwtypes.ComparisonOperatorGreaterThanOrEqualToThreshold,
	}
}

func realisticSNSTopic() snstypes.Topic {
	return snstypes.Topic{
		TopicArn: new("arn:aws:sns:us-east-1:123456789012:my-notifications"),
	}
}

func realisticELB() elbv2types.LoadBalancer {
	return elbv2types.LoadBalancer{
		LoadBalancerName: new("my-app-alb"),
		LoadBalancerArn:  new("arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/my-app-alb/50dc6c495c0c9188"),
		DNSName:          new("my-app-alb-123456789.us-east-1.elb.amazonaws.com"),
		Type:             elbv2types.LoadBalancerTypeEnumApplication,
		Scheme:           elbv2types.LoadBalancerSchemeEnumInternetFacing,
		State: &elbv2types.LoadBalancerState{
			Code: elbv2types.LoadBalancerStateEnumActive,
		},
		VpcId:       new("vpc-0abc1234"),
		CreatedTime: new(svcTestTime),
	}
}

func realisticTargetGroup() elbv2types.TargetGroup {
	return elbv2types.TargetGroup{
		TargetGroupName:    new("my-app-tg"),
		TargetGroupArn:     new("arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/my-app-tg/50dc6c495c0c9188"),
		Port:               new(int32(8080)),
		Protocol:           elbv2types.ProtocolEnumHttp,
		VpcId:              new("vpc-0abc1234"),
		TargetType:         elbv2types.TargetTypeEnumInstance,
		HealthCheckPath:    new("/health"),
		HealthCheckEnabled: new(true),
	}
}

func realisticECSClusterStruct() ecstypes.Cluster {
	return ecstypes.Cluster{
		ClusterName:         new("prod-cluster"),
		ClusterArn:          new("arn:aws:ecs:us-east-1:123456789012:cluster/prod-cluster"),
		Status:              new("ACTIVE"),
		RunningTasksCount:   5,
		PendingTasksCount:   1,
		ActiveServicesCount: 3,
	}
}

func realisticECSService() ecstypes.Service {
	return ecstypes.Service{
		ServiceName:    new("api-service"),
		ServiceArn:     new("arn:aws:ecs:us-east-1:123456789012:service/prod-cluster/api-service"),
		ClusterArn:     new("arn:aws:ecs:us-east-1:123456789012:cluster/prod-cluster"),
		Status:         new("ACTIVE"),
		DesiredCount:   3,
		RunningCount:   3,
		LaunchType:     ecstypes.LaunchTypeFargate,
		TaskDefinition: new("arn:aws:ecs:us-east-1:123456789012:task-definition/api-service:42"),
	}
}

func realisticECSTask() ecstypes.Task {
	return ecstypes.Task{
		TaskArn:           new("arn:aws:ecs:us-east-1:123456789012:task/prod-cluster/abc123def456"),
		ClusterArn:        new("arn:aws:ecs:us-east-1:123456789012:cluster/prod-cluster"),
		LastStatus:        new("RUNNING"),
		DesiredStatus:     new("RUNNING"),
		TaskDefinitionArn: new("arn:aws:ecs:us-east-1:123456789012:task-definition/api-service:42"),
		LaunchType:        ecstypes.LaunchTypeFargate,
		Cpu:               new("256"),
		Memory:            new("512"),
		StartedAt:         new(svcTestTime),
	}
}

func realisticCFNStack() cfntypes.Stack {
	return cfntypes.Stack{
		StackName:    new("my-app-stack"),
		StackId:      new("arn:aws:cloudformation:us-east-1:123456789012:stack/my-app-stack/guid-1234"),
		StackStatus:  cfntypes.StackStatusCreateComplete,
		CreationTime: new(svcTestTime),
		Description:  new("Application infrastructure stack"),
	}
}

func realisticIAMRole() iamtypes.Role {
	return iamtypes.Role{
		RoleName:           new("lambda-exec-role"),
		RoleId:             new("AROAEXAMPLEROLEID"),
		Arn:                new("arn:aws:iam::123456789012:role/lambda-exec-role"),
		Path:               new("/"),
		CreateDate:         new(svcTestTime),
		Description:        new("Execution role for Lambda functions"),
		MaxSessionDuration: new(int32(3600)),
	}
}

func realisticLogGroup() cwlogstypes.LogGroup {
	return cwlogstypes.LogGroup{
		LogGroupName:    new("/aws/lambda/my-api-handler"),
		LogGroupArn:     new("arn:aws:logs:us-east-1:123456789012:log-group:/aws/lambda/my-api-handler:*"),
		StoredBytes:     new(int64(1073741824)),
		RetentionInDays: new(int32(30)),
		CreationTime:    new(int64(1718444400000)),
	}
}

func realisticSSMParameter() ssmtypes.ParameterMetadata {
	return ssmtypes.ParameterMetadata{
		Name:             new("/app/config/db-host"),
		Type:             ssmtypes.ParameterTypeString,
		Version:          1,
		LastModifiedDate: new(svcTestTime),
		Description:      new("Database host parameter"),
	}
}

func realisticDDBTable() ddbtypes.TableDescription {
	return ddbtypes.TableDescription{
		TableName:        new("users-table"),
		TableArn:         new("arn:aws:dynamodb:us-east-1:123456789012:table/users-table"),
		TableStatus:      ddbtypes.TableStatusActive,
		ItemCount:        new(int64(50000)),
		TableSizeBytes:   new(int64(10485760)),
		CreationDateTime: new(svcTestTime),
	}
}

func realisticACMCertificate() acmtypes.CertificateSummary {
	return acmtypes.CertificateSummary{
		DomainName:     new("example.com"),
		CertificateArn: new("arn:aws:acm:us-east-1:123456789012:certificate/12345678-1234-1234-1234-123456789012"),
		Status:         acmtypes.CertificateStatusIssued,
		Type:           acmtypes.CertificateTypeAmazonIssued,
		CreatedAt:      new(svcTestTime),
	}
}

func realisticASG() autoscalingtypes.AutoScalingGroup {
	return autoscalingtypes.AutoScalingGroup{
		AutoScalingGroupName: new("my-app-asg"),
		AutoScalingGroupARN:  new("arn:aws:autoscaling:us-east-1:123456789012:autoScalingGroup:guid:autoScalingGroupName/my-app-asg"),
		MinSize:              new(int32(2)),
		MaxSize:              new(int32(10)),
		DesiredCapacity:      new(int32(4)),
		AvailabilityZones:    []string{"us-east-1a", "us-east-1b"},
		CreatedTime:          new(svcTestTime),
	}
}

func realisticIAMUser() iamtypes.User {
	return iamtypes.User{
		UserName:   new("deploy-user"),
		UserId:     new("AIDAEXAMPLEUSERID"),
		Arn:        new("arn:aws:iam::123456789012:user/deploy-user"),
		Path:       new("/"),
		CreateDate: new(svcTestTime),
	}
}

func realisticIAMGroup() iamtypes.Group {
	return iamtypes.Group{
		GroupName:  new("developers"),
		GroupId:    new("AGPAEXAMPLEGROUPID"),
		Arn:        new("arn:aws:iam::123456789012:group/developers"),
		Path:       new("/"),
		CreateDate: new(svcTestTime),
	}
}

// realisticSESIdentity returns an sesv2types.IdentityInfo matching the type
// produced by internal/aws/ses.go FetchSESIdentities.
func realisticSESIdentity() sesv2types.IdentityInfo {
	return sesv2types.IdentityInfo{
		IdentityName:       new("example.com"),
		IdentityType:       sesv2types.IdentityTypeDomain,
		SendingEnabled:     true,
		VerificationStatus: sesv2types.VerificationStatusSuccess,
	}
}

// ---------------------------------------------------------------------------
// Lambda
// ---------------------------------------------------------------------------
