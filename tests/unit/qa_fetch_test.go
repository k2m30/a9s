package unit

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/backup"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/codeartifact"
	"github.com/aws/aws-sdk-go-v2/service/codebuild"
	"github.com/aws/aws-sdk-go-v2/service/codepipeline"
	"github.com/aws/aws-sdk-go-v2/service/docdb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/efs"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	"github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/kafka"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/opensearch"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/redshift"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/wafv2"

	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

// ---------------------------------------------------------------------------
// mockRoundTripper: returns canned HTTP responses per service
// ---------------------------------------------------------------------------

type mockRoundTripper struct {
	// responseFunc is called for each request, returning the status code and body.
	responseFunc func(req *http.Request) (int, string)
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	statusCode, body := m.responseFunc(req)
	return &http.Response{
		StatusCode: statusCode,
		Header:     http.Header{"Content-Type": []string{"application/xml"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}, nil
}

// serviceRouter returns a canned empty-but-valid response for each AWS service.
func serviceRouter(req *http.Request) (int, string) {
	host := req.URL.Host

	// S3 ListBuckets (REST-XML)
	if strings.Contains(host, "s3.") || strings.Contains(host, "s3-") {
		// Check if this is a ListObjectsV2 call (has bucket in path or list-type=2 param)
		if req.URL.Query().Get("list-type") == "2" {
			return 200, `<?xml version="1.0" encoding="UTF-8"?>
<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Name>test-bucket</Name>
  <IsTruncated>false</IsTruncated>
  <Contents>
    <Key>test-file.txt</Key>
    <Size>1024</Size>
    <StorageClass>STANDARD</StorageClass>
  </Contents>
</ListBucketResult>`
		}
		return 200, `<?xml version="1.0" encoding="UTF-8"?>
<ListAllMyBucketsResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Buckets>
    <Bucket>
      <Name>mock-bucket-1</Name>
      <CreationDate>2025-01-01T00:00:00.000Z</CreationDate>
    </Bucket>
  </Buckets>
</ListAllMyBucketsResult>`
	}

	// Read the body to determine the action for query-based services
	var bodyStr string
	if req.Body != nil {
		bodyBytes, _ := io.ReadAll(req.Body)
		bodyStr = string(bodyBytes)
		// Restore the body for any downstream use
		req.Body = io.NopCloser(strings.NewReader(bodyStr))
	}

	// EC2 (EC2 query protocol)
	if strings.Contains(host, "ec2.") {
		return 200, `<?xml version="1.0" encoding="UTF-8"?>
<DescribeInstancesResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <reservationSet>
    <item>
      <instancesSet>
        <item>
          <instanceId>i-mock123</instanceId>
          <instanceState><name>running</name></instanceState>
          <instanceType>t3.micro</instanceType>
        </item>
      </instancesSet>
    </item>
  </reservationSet>
</DescribeInstancesResponse>`
	}

	// DocumentDB uses the RDS endpoint — must check BEFORE RDS handler.
	// DocumentDB SDK sends DescribeDBClusters and DescribeDBClusterSnapshots to rds.* endpoint.
	if strings.Contains(host, "rds.") && strings.Contains(bodyStr, "DescribeDBClusterSnapshots") {
		return 200, `<?xml version="1.0" encoding="UTF-8"?>
<DescribeDBClusterSnapshotsResponse xmlns="http://rds.amazonaws.com/doc/2014-10-31/">
  <DescribeDBClusterSnapshotsResult>
    <DBClusterSnapshots/>
  </DescribeDBClusterSnapshotsResult>
</DescribeDBClusterSnapshotsResponse>`
	}
	if strings.Contains(host, "rds.") && strings.Contains(bodyStr, "DescribeDBClusters") {
		return 200, `<?xml version="1.0" encoding="UTF-8"?>
<DescribeDBClustersResponse xmlns="http://rds.amazonaws.com/doc/2014-10-31/">
  <DescribeDBClustersResult>
    <DBClusters>
      <DBCluster>
        <DBClusterIdentifier>mock-docdb-1</DBClusterIdentifier>
        <Engine>docdb</Engine>
        <Status>available</Status>
      </DBCluster>
    </DBClusters>
  </DescribeDBClustersResult>
</DescribeDBClustersResponse>`
	}

	// RDS (AWS Query) — must come AFTER DocumentDB check above
	if strings.Contains(host, "rds.") && strings.Contains(bodyStr, "DescribeDBSnapshots") {
		return 200, `<?xml version="1.0" encoding="UTF-8"?>
<DescribeDBSnapshotsResponse xmlns="http://rds.amazonaws.com/doc/2014-10-31/">
  <DescribeDBSnapshotsResult>
    <DBSnapshots/>
  </DescribeDBSnapshotsResult>
</DescribeDBSnapshotsResponse>`
	}
	if strings.Contains(host, "rds.") {
		return 200, `<?xml version="1.0" encoding="UTF-8"?>
<DescribeDBInstancesResponse xmlns="http://rds.amazonaws.com/doc/2014-10-31/">
  <DescribeDBInstancesResult>
    <DBInstances>
      <DBInstance>
        <DBInstanceIdentifier>mock-rds-1</DBInstanceIdentifier>
        <Engine>mysql</Engine>
        <DBInstanceStatus>available</DBInstanceStatus>
        <DBInstanceClass>db.t3.micro</DBInstanceClass>
      </DBInstance>
    </DBInstances>
  </DescribeDBInstancesResult>
</DescribeDBInstancesResponse>`
	}

	// ElastiCache (AWS Query)
	if strings.Contains(host, "elasticache.") {
		return 200, `<?xml version="1.0" encoding="UTF-8"?>
<DescribeCacheClustersResponse xmlns="http://elasticache.amazonaws.com/doc/2015-02-02/">
  <DescribeCacheClustersResult>
    <CacheClusters>
      <CacheCluster>
        <CacheClusterId>mock-redis-1</CacheClusterId>
        <Engine>redis</Engine>
        <EngineVersion>7.0</EngineVersion>
        <CacheClusterStatus>available</CacheClusterStatus>
        <CacheNodeType>cache.t3.micro</CacheNodeType>
        <NumCacheNodes>1</NumCacheNodes>
      </CacheCluster>
    </CacheClusters>
  </DescribeCacheClustersResult>
</DescribeCacheClustersResponse>`
	}

	// EKS (REST-JSON)
	if strings.Contains(host, "eks.") {
		path := req.URL.Path
		// EKS DescribeCluster: /clusters/{name} (path has more segments)
		if strings.Contains(path, "/clusters/") {
			return 200, `{"cluster":{"name":"mock-eks-1","status":"ACTIVE","version":"1.28","endpoint":"https://mock.eks.endpoint","arn":"arn:aws:eks:us-east-1:123456789012:cluster/mock-eks-1"}}`
		}
		// EKS ListClusters: /clusters (exact path)
		if strings.HasSuffix(path, "/clusters") {
			return 200, `{"clusters":["mock-eks-1"]}`
		}
	}

	// SecretsManager (AWS JSON 1.1)
	if strings.Contains(host, "secretsmanager.") {
		// Check the target header for action
		target := req.Header.Get("X-Amz-Target")
		if strings.Contains(target, "ListSecrets") {
			return 200, `{"SecretList":[{"Name":"mock-secret-1","Description":"A test secret"}]}`
		}
	}

	// Lambda (REST-JSON)
	if strings.Contains(host, "lambda.") {
		return 200, `{"Functions":[]}`
	}

	// CloudWatch (AWS Query XML)
	if strings.Contains(host, "monitoring.") {
		return 200, `<?xml version="1.0" encoding="UTF-8"?>
<DescribeAlarmsResponse xmlns="http://monitoring.amazonaws.com/doc/2010-08-01/">
  <DescribeAlarmsResult>
    <MetricAlarms/>
  </DescribeAlarmsResult>
</DescribeAlarmsResponse>`
	}

	// SNS (AWS Query XML)
	if strings.Contains(host, "sns.") {
		if strings.Contains(bodyStr, "Action=ListSubscriptions") {
			return 200, `<?xml version="1.0" encoding="UTF-8"?>
<ListSubscriptionsResponse xmlns="http://sns.amazonaws.com/doc/2010-03-31/">
  <ListSubscriptionsResult>
    <Subscriptions/>
  </ListSubscriptionsResult>
</ListSubscriptionsResponse>`
		}
		return 200, `<?xml version="1.0" encoding="UTF-8"?>
<ListTopicsResponse xmlns="http://sns.amazonaws.com/doc/2010-03-31/">
  <ListTopicsResult>
    <Topics/>
  </ListTopicsResult>
</ListTopicsResponse>`
	}

	// SQS (AWS JSON)
	if strings.Contains(host, "sqs.") {
		return 200, `{"QueueUrls":[]}`
	}

	// ELBv2 (AWS Query XML)
	if strings.Contains(host, "elasticloadbalancing.") {
		if strings.Contains(bodyStr, "DescribeTargetGroups") {
			return 200, `<?xml version="1.0" encoding="UTF-8"?>
<DescribeTargetGroupsResponse xmlns="http://elasticloadbalancing.amazonaws.com/doc/2015-12-01/">
  <DescribeTargetGroupsResult>
    <TargetGroups/>
  </DescribeTargetGroupsResult>
</DescribeTargetGroupsResponse>`
		}
		return 200, `<?xml version="1.0" encoding="UTF-8"?>
<DescribeLoadBalancersResponse xmlns="http://elasticloadbalancing.amazonaws.com/doc/2015-12-01/">
  <DescribeLoadBalancersResult>
    <LoadBalancers/>
  </DescribeLoadBalancersResult>
</DescribeLoadBalancersResponse>`
	}

	// ECS (AWS JSON 1.1)
	if strings.Contains(host, "ecs.") {
		target := req.Header.Get("X-Amz-Target")
		if strings.Contains(target, "ListClusters") {
			return 200, `{"clusterArns":[]}`
		}
		if strings.Contains(target, "ListServices") {
			return 200, `{"serviceArns":[]}`
		}
		return 200, `{}`
	}

	// CloudFormation (AWS Query XML)
	if strings.Contains(host, "cloudformation.") {
		return 200, `<?xml version="1.0" encoding="UTF-8"?>
<DescribeStacksResponse xmlns="http://cloudformation.amazonaws.com/doc/2010-05-15/">
  <DescribeStacksResult>
    <Stacks/>
  </DescribeStacksResult>
</DescribeStacksResponse>`
	}

	// IAM (AWS Query XML)
	if strings.Contains(host, "iam.") {
		if strings.Contains(bodyStr, "Action=ListPolicies") {
			return 200, `<?xml version="1.0" encoding="UTF-8"?>
<ListPoliciesResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/">
  <ListPoliciesResult>
    <Policies/>
  </ListPoliciesResult>
</ListPoliciesResponse>`
		}
		if strings.Contains(bodyStr, "Action=ListUsers") {
			return 200, `<?xml version="1.0" encoding="UTF-8"?>
<ListUsersResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/">
  <ListUsersResult>
    <Users/>
  </ListUsersResult>
</ListUsersResponse>`
		}
		if strings.Contains(bodyStr, "Action=ListGroups") {
			return 200, `<?xml version="1.0" encoding="UTF-8"?>
<ListGroupsResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/">
  <ListGroupsResult>
    <Groups/>
  </ListGroupsResult>
</ListGroupsResponse>`
		}
		return 200, `<?xml version="1.0" encoding="UTF-8"?>
<ListRolesResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/">
  <ListRolesResult>
    <Roles/>
  </ListRolesResult>
</ListRolesResponse>`
	}

	// CloudWatch Logs (AWS JSON 1.1)
	if strings.Contains(host, "logs.") {
		return 200, `{"logGroups":[]}`
	}

	// SSM (AWS JSON 1.1)
	if strings.Contains(host, "ssm.") {
		return 200, `{"Parameters":[]}`
	}

	// DynamoDB (AWS JSON 1.0)
	if strings.Contains(host, "dynamodb.") {
		return 200, `{"TableNames":[]}`
	}

	// ACM (AWS JSON 1.1)
	if strings.Contains(host, "acm.") {
		return 200, `{"CertificateSummaryList":[]}`
	}

	// Auto Scaling (AWS Query XML)
	if strings.Contains(host, "autoscaling.") {
		return 200, `<?xml version="1.0" encoding="UTF-8"?>
<DescribeAutoScalingGroupsResponse xmlns="http://autoscaling.amazonaws.com/doc/2011-01-01/">
  <DescribeAutoScalingGroupsResult>
    <AutoScalingGroups/>
  </DescribeAutoScalingGroupsResult>
</DescribeAutoScalingGroupsResponse>`
	}

	// Fallback: empty 200
	return 200, `{}`
}

// buildMockClients creates a *ServiceClients backed by a mock HTTP transport
// that returns valid empty responses for all AWS service calls.
func buildMockClients(t *testing.T) *awsclient.ServiceClients {
	t.Helper()

	transport := &mockRoundTripper{responseFunc: serviceRouter}
	httpClient := &http.Client{Transport: transport}

	cfg := aws.Config{
		Region:      "us-east-1",
		Credentials: credentials.NewStaticCredentialsProvider("AKID", "SECRET", "SESSION"),
		HTTPClient:  httpClient,
	}

	return &awsclient.ServiceClients{
		EC2:              ec2.NewFromConfig(cfg),
		S3:               s3.NewFromConfig(cfg, func(o *s3.Options) { o.UsePathStyle = true }),
		RDS:              rds.NewFromConfig(cfg),
		ElastiCache:      elasticache.NewFromConfig(cfg),
		DocDB:            docdb.NewFromConfig(cfg),
		EKS:              eks.NewFromConfig(cfg),
		SecretsManager:   secretsmanager.NewFromConfig(cfg),
		Lambda:           lambda.NewFromConfig(cfg),
		CloudWatch:       cloudwatch.NewFromConfig(cfg),
		SNS:              sns.NewFromConfig(cfg),
		SQS:              sqs.NewFromConfig(cfg),
		ELBv2:            elbv2.NewFromConfig(cfg),
		ECS:              ecs.NewFromConfig(cfg),
		CloudFormation:   cloudformation.NewFromConfig(cfg),
		IAM:              iam.NewFromConfig(cfg),
		CloudWatchLogs:   cloudwatchlogs.NewFromConfig(cfg),
		SSM:              ssm.NewFromConfig(cfg),
		DynamoDB:         dynamodb.NewFromConfig(cfg),
		ACM:              acm.NewFromConfig(cfg),
		AutoScaling:      autoscaling.NewFromConfig(cfg),
		CloudFront:       cloudfront.NewFromConfig(cfg),
		Route53:          route53.NewFromConfig(cfg),
		APIGatewayV2:     apigatewayv2.NewFromConfig(cfg),
		ECR:              ecr.NewFromConfig(cfg),
		EFS:              efs.NewFromConfig(cfg),
		EventBridge:      eventbridge.NewFromConfig(cfg),
		SFN:              sfn.NewFromConfig(cfg),
		CodePipeline:     codepipeline.NewFromConfig(cfg),
		Kinesis:          kinesis.NewFromConfig(cfg),
		WAFv2:            wafv2.NewFromConfig(cfg),
		Glue:             glue.NewFromConfig(cfg),
		ElasticBeanstalk: elasticbeanstalk.NewFromConfig(cfg),
		SESv2:            sesv2.NewFromConfig(cfg),
		Redshift:         redshift.NewFromConfig(cfg),
		CloudTrail:       cloudtrail.NewFromConfig(cfg),
		Athena:           athena.NewFromConfig(cfg),
		CodeArtifact:     codeartifact.NewFromConfig(cfg),
		CodeBuild:        codebuild.NewFromConfig(cfg),
		OpenSearch:       opensearch.NewFromConfig(cfg),
		KMS:              kms.NewFromConfig(cfg),
		MSK:              kafka.NewFromConfig(cfg),
		Backup:           backup.NewFromConfig(cfg),
	}
}

// buildModelWithMockClients creates a sized Model and injects mock AWS clients.
func buildModelWithMockClients(t *testing.T) tui.Model {
	t.Helper()
	tui.Version = "0.6.0"
	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 40})

	clients := buildMockClients(t)
	m, _ = rootApplyMsg(m, messages.ClientsReadyMsg{Clients: clients})
	return m
}

// executeBatchCmd recursively executes a tea.Cmd, collecting all resulting messages.
// For batch commands (returned by tea.Batch), it executes each sub-command.
func executeBatchCmd(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if msg == nil {
		return nil
	}

	// Check if it's a batch message (tea.BatchMsg is a []tea.Cmd)
	if batch, ok := msg.(tea.BatchMsg); ok {
		var msgs []tea.Msg
		for _, subCmd := range batch {
			msgs = append(msgs, executeBatchCmd(subCmd)...)
		}
		return msgs
	}

	return []tea.Msg{msg}
}

// findMsgOfType returns the first message of a specific type from a slice.
func findResourcesLoadedMsg(msgs []tea.Msg) *messages.ResourcesLoadedMsg {
	for _, msg := range msgs {
		if rl, ok := msg.(messages.ResourcesLoadedMsg); ok {
			return &rl
		}
	}
	return nil
}

func findAPIErrorMsg(msgs []tea.Msg) *messages.APIErrorMsg {
	for _, msg := range msgs {
		if ae, ok := msg.(messages.APIErrorMsg); ok {
			return &ae
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Tests: fetchResources for each resource type
// ---------------------------------------------------------------------------

func TestQA_FetchResources_S3Buckets(t *testing.T) {
	m := buildModelWithMockClients(t)

	_, cmd := rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "s3",
	})
	if cmd == nil {
		t.Fatal("navigating to s3 resource list should return a command")
	}

	msgs := executeBatchCmd(cmd)
	rl := findResourcesLoadedMsg(msgs)
	ae := findAPIErrorMsg(msgs)

	if ae != nil {
		t.Fatalf("s3 bucket fetch returned APIErrorMsg: %v", ae.Err)
	}
	if rl == nil {
		t.Fatal("s3 bucket fetch should return ResourcesLoadedMsg")
	}
	if rl.ResourceType != "s3" {
		t.Errorf("expected ResourceType 's3', got %q", rl.ResourceType)
	}
	if len(rl.Resources) == 0 {
		t.Error("expected at least one S3 bucket from mock")
	}
}

func TestQA_FetchResources_S3Objects(t *testing.T) {
	m := buildModelWithMockClients(t)

	// S3 objects are fetched via EnterChildViewMsg
	_, cmd := rootApplyMsg(m, messages.EnterChildViewMsg{
		ChildType:     "s3_objects",
		ParentContext: map[string]string{"bucket": "test-bucket"},
		DisplayName:   "test-bucket",
	})
	if cmd == nil {
		t.Fatal("entering S3 bucket should return a command")
	}

	msgs := executeBatchCmd(cmd)
	rl := findResourcesLoadedMsg(msgs)
	ae := findAPIErrorMsg(msgs)

	if ae != nil {
		t.Fatalf("s3 objects fetch returned APIErrorMsg: %v", ae.Err)
	}
	if rl == nil {
		t.Fatal("s3 objects fetch should return ResourcesLoadedMsg")
	}
	if rl.ResourceType != "s3_objects" {
		t.Errorf("expected ResourceType 's3_objects', got %q", rl.ResourceType)
	}
}

func TestQA_FetchResources_EC2(t *testing.T) {
	m := buildModelWithMockClients(t)

	_, cmd := rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	if cmd == nil {
		t.Fatal("navigating to ec2 resource list should return a command")
	}

	msgs := executeBatchCmd(cmd)
	rl := findResourcesLoadedMsg(msgs)
	ae := findAPIErrorMsg(msgs)

	if ae != nil {
		t.Fatalf("ec2 fetch returned APIErrorMsg: %v", ae.Err)
	}
	if rl == nil {
		t.Fatal("ec2 fetch should return ResourcesLoadedMsg")
	}
	if rl.ResourceType != "ec2" {
		t.Errorf("expected ResourceType 'ec2', got %q", rl.ResourceType)
	}
}

func TestQA_FetchResources_RDS(t *testing.T) {
	m := buildModelWithMockClients(t)

	_, cmd := rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "dbi",
	})
	if cmd == nil {
		t.Fatal("navigating to rds resource list should return a command")
	}

	msgs := executeBatchCmd(cmd)
	rl := findResourcesLoadedMsg(msgs)
	ae := findAPIErrorMsg(msgs)

	if ae != nil {
		t.Fatalf("rds fetch returned APIErrorMsg: %v", ae.Err)
	}
	if rl == nil {
		t.Fatal("rds fetch should return ResourcesLoadedMsg")
	}
	if rl.ResourceType != "dbi" {
		t.Errorf("expected ResourceType 'rds', got %q", rl.ResourceType)
	}
}

func TestQA_FetchResources_Redis(t *testing.T) {
	m := buildModelWithMockClients(t)

	_, cmd := rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "redis",
	})
	if cmd == nil {
		t.Fatal("navigating to redis resource list should return a command")
	}

	msgs := executeBatchCmd(cmd)
	rl := findResourcesLoadedMsg(msgs)
	ae := findAPIErrorMsg(msgs)

	if ae != nil {
		t.Fatalf("redis fetch returned APIErrorMsg: %v", ae.Err)
	}
	if rl == nil {
		t.Fatal("redis fetch should return ResourcesLoadedMsg")
	}
	if rl.ResourceType != "redis" {
		t.Errorf("expected ResourceType 'redis', got %q", rl.ResourceType)
	}
}

func TestQA_FetchResources_DocDB(t *testing.T) {
	m := buildModelWithMockClients(t)

	_, cmd := rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "dbc",
	})
	if cmd == nil {
		t.Fatal("navigating to docdb resource list should return a command")
	}

	msgs := executeBatchCmd(cmd)
	rl := findResourcesLoadedMsg(msgs)
	ae := findAPIErrorMsg(msgs)

	if ae != nil {
		t.Fatalf("docdb fetch returned APIErrorMsg: %v", ae.Err)
	}
	if rl == nil {
		t.Fatal("docdb fetch should return ResourcesLoadedMsg")
	}
	if rl.ResourceType != "dbc" {
		t.Errorf("expected ResourceType 'docdb', got %q", rl.ResourceType)
	}
}

func TestQA_FetchResources_EKS(t *testing.T) {
	m := buildModelWithMockClients(t)

	_, cmd := rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "eks",
	})
	if cmd == nil {
		t.Fatal("navigating to eks resource list should return a command")
	}

	msgs := executeBatchCmd(cmd)
	rl := findResourcesLoadedMsg(msgs)
	ae := findAPIErrorMsg(msgs)

	if ae != nil {
		t.Fatalf("eks fetch returned APIErrorMsg: %v", ae.Err)
	}
	if rl == nil {
		t.Fatal("eks fetch should return ResourcesLoadedMsg")
	}
	if rl.ResourceType != "eks" {
		t.Errorf("expected ResourceType 'eks', got %q", rl.ResourceType)
	}
}

func TestQA_FetchResources_Secrets(t *testing.T) {
	m := buildModelWithMockClients(t)

	_, cmd := rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "secrets",
	})
	if cmd == nil {
		t.Fatal("navigating to secrets resource list should return a command")
	}

	msgs := executeBatchCmd(cmd)
	rl := findResourcesLoadedMsg(msgs)
	ae := findAPIErrorMsg(msgs)

	if ae != nil {
		t.Fatalf("secrets fetch returned APIErrorMsg: %v", ae.Err)
	}
	if rl == nil {
		t.Fatal("secrets fetch should return ResourcesLoadedMsg")
	}
	if rl.ResourceType != "secrets" {
		t.Errorf("expected ResourceType 'secrets', got %q", rl.ResourceType)
	}
}

// ---------------------------------------------------------------------------
// Test: nil-clients error path
// ---------------------------------------------------------------------------

func TestQA_FetchResources_NilClients(t *testing.T) {
	tui.Version = "0.6.0"
	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	// Do NOT inject clients — they remain nil

	for _, rt := range resource.AllShortNames() {
		t.Run(rt, func(t *testing.T) {
			_, cmd := rootApplyMsg(m, messages.NavigateMsg{
				Target:       messages.TargetResourceList,
				ResourceType: rt,
			})
			if cmd == nil {
				t.Fatalf("navigating to %s with nil clients should return a command", rt)
			}

			msgs := executeBatchCmd(cmd)
			ae := findAPIErrorMsg(msgs)
			if ae == nil {
				t.Fatalf("fetch %s with nil clients should return APIErrorMsg", rt)
			}
			if !strings.Contains(ae.Err.Error(), "not initialized") {
				t.Errorf("expected 'not initialized' error, got: %v", ae.Err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Test: unsupported resource type
// ---------------------------------------------------------------------------

func TestQA_FetchResources_UnsupportedResourceType(t *testing.T) {
	m := buildModelWithMockClients(t)

	// Send a LoadResourcesMsg with an unknown type (bypasses NavigateMsg validation)
	_, cmd := rootApplyMsg(m, messages.LoadResourcesMsg{
		ResourceType: "bogus",
	})
	if cmd == nil {
		t.Fatal("LoadResourcesMsg with unsupported type should return a command")
	}

	msg := cmd()
	ae, ok := msg.(messages.APIErrorMsg)
	if !ok {
		t.Fatalf("expected APIErrorMsg, got %T", msg)
	}
	if !strings.Contains(ae.Err.Error(), "unsupported resource type") {
		t.Errorf("expected 'unsupported resource type' error, got: %v", ae.Err)
	}
	if ae.ResourceType != "bogus" {
		t.Errorf("expected ResourceType 'bogus', got %q", ae.ResourceType)
	}
}

// ---------------------------------------------------------------------------
// Test: S3 objects via EnterChildViewMsg with prefix
// ---------------------------------------------------------------------------

func TestQA_FetchResources_S3NavigatePrefix(t *testing.T) {
	m := buildModelWithMockClients(t)

	_, cmd := rootApplyMsg(m, messages.EnterChildViewMsg{
		ChildType:     "s3_objects",
		ParentContext: map[string]string{"bucket": "test-bucket", "prefix": "some/prefix/"},
		DisplayName:   "test-bucket",
	})
	if cmd == nil {
		t.Fatal("EnterChildViewMsg for S3 prefix should return a command")
	}

	msgs := executeBatchCmd(cmd)
	rl := findResourcesLoadedMsg(msgs)
	ae := findAPIErrorMsg(msgs)

	if ae != nil {
		t.Fatalf("s3 prefix navigation returned APIErrorMsg: %v", ae.Err)
	}
	if rl == nil {
		t.Fatal("s3 prefix navigation should return ResourcesLoadedMsg")
	}
	if rl.ResourceType != "s3_objects" {
		t.Errorf("expected ResourceType 's3_objects', got %q", rl.ResourceType)
	}
}

// ---------------------------------------------------------------------------
// Test: LoadResourcesMsg triggers fetchResources
// ---------------------------------------------------------------------------

func TestQA_FetchResources_ViaLoadResourcesMsg(t *testing.T) {
	m := buildModelWithMockClients(t)

	// Resources using CBOR/smithy-rpc-v2 protocol can't be mock-served with simple HTTP.
	// Their fetcher logic is fully tested via narrow-interface mocks in per-resource test files.
	cborResources := map[string]bool{
		"alarm": true, // CloudWatch uses rpc-v2-cbor
	}

	for _, rt := range resource.AllShortNames() {
		t.Run(rt, func(t *testing.T) {
			_, cmd := rootApplyMsg(m, messages.LoadResourcesMsg{
				ResourceType: rt,
			})
			if cmd == nil {
				t.Fatalf("LoadResourcesMsg for %s should return a command", rt)
			}

			msg := cmd()
			switch msg := msg.(type) {
			case messages.ResourcesLoadedMsg:
				if msg.ResourceType != rt {
					t.Errorf("expected ResourceType %q, got %q", rt, msg.ResourceType)
				}
			case messages.APIErrorMsg:
				if cborResources[rt] {
					t.Skipf("skipping %s: uses CBOR protocol not supported by mock HTTP server", rt)
				} else {
					t.Fatalf("LoadResourcesMsg for %s returned APIErrorMsg: %v", rt, msg.Err)
				}
			default:
				t.Fatalf("unexpected message type %T for %s", msg, rt)
			}
		})
	}
}
