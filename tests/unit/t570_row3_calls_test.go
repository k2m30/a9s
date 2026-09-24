package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	apigwv1types "github.com/aws/aws-sdk-go-v2/service/apigateway/types"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	apigwv2types "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	athenatypes "github.com/aws/aws-sdk-go-v2/service/athena/types"
	cwltypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	opensearchtypes "github.com/aws/aws-sdk-go-v2/service/opensearch/types"
	"github.com/aws/aws-sdk-go-v2/service/wafv2"
	wafv2types "github.com/aws/aws-sdk-go-v2/service/wafv2/types"
	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
)

// t570APIGWv2 answers the HTTP/WebSocket API service the way AWS does for a
// REST API id: NotFound, since a REST API lives in API Gateway v1.
type t570APIGWv2 struct {
	awsclient.APIGatewayV2API
	rest map[string]bool
}

func (f *t570APIGWv2) notFound(id string) error {
	return &apigwv2types.NotFoundException{Message: aws.String("Invalid API identifier specified " + id)}
}

func (f *t570APIGWv2) GetIntegrations(ctx context.Context, in *apigatewayv2.GetIntegrationsInput, opt ...func(*apigatewayv2.Options)) (*apigatewayv2.GetIntegrationsOutput, error) {
	if f.rest[aws.ToString(in.ApiId)] {
		return nil, f.notFound(aws.ToString(in.ApiId))
	}
	return f.APIGatewayV2API.GetIntegrations(ctx, in, opt...)
}

func (f *t570APIGWv2) GetAuthorizers(ctx context.Context, in *apigatewayv2.GetAuthorizersInput, opt ...func(*apigatewayv2.Options)) (*apigatewayv2.GetAuthorizersOutput, error) {
	if f.rest[aws.ToString(in.ApiId)] {
		return nil, f.notFound(aws.ToString(in.ApiId))
	}
	return f.APIGatewayV2API.GetAuthorizers(ctx, in, opt...)
}

// t570APIGWv1 serves a REST API's resources, their method integrations and
// authorizers as API Gateway v1 does.
type t570APIGWv1 struct {
	awsclient.APIGatewayV1API
	api         string
	resources   []apigwv1types.Resource
	authorizers []apigwv1types.Authorizer
	vpcLinks    []apigwv1types.VpcLink
}

func (f *t570APIGWv1) GetResources(_ context.Context, in *apigateway.GetResourcesInput, _ ...func(*apigateway.Options)) (*apigateway.GetResourcesOutput, error) {
	if aws.ToString(in.RestApiId) != f.api {
		return &apigateway.GetResourcesOutput{}, nil
	}
	return &apigateway.GetResourcesOutput{Items: f.resources}, nil
}

func (f *t570APIGWv1) method(api, resourceID, httpMethod string) (apigwv1types.Method, bool) {
	if api != f.api {
		return apigwv1types.Method{}, false
	}
	for _, r := range f.resources {
		if aws.ToString(r.Id) == resourceID {
			m, ok := r.ResourceMethods[httpMethod]
			return m, ok
		}
	}
	return apigwv1types.Method{}, false
}

func (f *t570APIGWv1) GetMethod(_ context.Context, in *apigateway.GetMethodInput, _ ...func(*apigateway.Options)) (*apigateway.GetMethodOutput, error) {
	m, ok := f.method(aws.ToString(in.RestApiId), aws.ToString(in.ResourceId), aws.ToString(in.HttpMethod))
	if !ok {
		return nil, &apigwv1types.NotFoundException{Message: aws.String("Invalid Method identifier specified")}
	}
	return &apigateway.GetMethodOutput{HttpMethod: m.HttpMethod, AuthorizationType: m.AuthorizationType, MethodIntegration: m.MethodIntegration}, nil
}

func (f *t570APIGWv1) GetIntegration(_ context.Context, in *apigateway.GetIntegrationInput, _ ...func(*apigateway.Options)) (*apigateway.GetIntegrationOutput, error) {
	m, ok := f.method(aws.ToString(in.RestApiId), aws.ToString(in.ResourceId), aws.ToString(in.HttpMethod))
	if !ok || m.MethodIntegration == nil {
		return nil, &apigwv1types.NotFoundException{Message: aws.String("Invalid Integration identifier specified")}
	}
	i := m.MethodIntegration
	return &apigateway.GetIntegrationOutput{Type: i.Type, Uri: i.Uri, Credentials: i.Credentials, ConnectionType: i.ConnectionType, ConnectionId: i.ConnectionId, HttpMethod: i.HttpMethod}, nil
}

func (f *t570APIGWv1) GetAuthorizers(ctx context.Context, in *apigateway.GetAuthorizersInput, opt ...func(*apigateway.Options)) (*apigateway.GetAuthorizersOutput, error) {
	if aws.ToString(in.RestApiId) == f.api {
		return &apigateway.GetAuthorizersOutput{Items: f.authorizers}, nil
	}
	return f.APIGatewayV1API.(interface {
		GetAuthorizers(context.Context, *apigateway.GetAuthorizersInput, ...func(*apigateway.Options)) (*apigateway.GetAuthorizersOutput, error)
	}).GetAuthorizers(ctx, in, opt...)
}

func (f *t570APIGWv1) GetVpcLinks(context.Context, *apigateway.GetVpcLinksInput, ...func(*apigateway.Options)) (*apigateway.GetVpcLinksOutput, error) {
	return &apigateway.GetVpcLinksOutput{Items: f.vpcLinks}, nil
}

// A REST API's integrations and authorizers live in API Gateway v1
// (GetResources / GetIntegration, GetAuthorizers). The apigatewayv2 service
// answers NotFound for its id, so reading them there errors every pivot.
func TestT570_ApigwREST_PivotsReadTheRESTAPI(t *testing.T) {
	c := t570Demo()
	const restID = "rst001noauth"
	const invokeRole = "arn:aws:iam::123456789012:role/acme-ci-deploy-role"
	const authorizerRole = "arn:aws:iam::123456789012:role/service-role/acme-lambda-execution"
	c.APIGatewayV2 = &t570APIGWv2{APIGatewayV2API: c.APIGatewayV2, rest: map[string]bool{restID: true}}
	c.APIGatewayV1 = &t570APIGWv1{APIGatewayV1API: c.APIGatewayV1, api: restID,
		resources: []apigwv1types.Resource{
			{Id: aws.String("root01"), Path: aws.String("/")},
			{Id: aws.String("ord001"), ParentId: aws.String("root01"), Path: aws.String("/orders"), PathPart: aws.String("orders"),
				ResourceMethods: map[string]apigwv1types.Method{"POST": {
					HttpMethod:        aws.String("POST"),
					AuthorizationType: aws.String("CUSTOM"),
					AuthorizerId:      aws.String("auth01"),
					MethodIntegration: &apigwv1types.Integration{
						Type:        apigwv1types.IntegrationTypeAwsProxy,
						HttpMethod:  aws.String("POST"),
						Uri:         aws.String("arn:aws:apigateway:us-east-1:lambda:path/2015-03-31/functions/arn:aws:lambda:us-east-1:123456789012:function:audit-logger/invocations"),
						Credentials: aws.String(invokeRole),
					},
				}}},
		},
		authorizers: []apigwv1types.Authorizer{{
			Id:                    aws.String("auth01"),
			Name:                  aws.String("token-authorizer"),
			Type:                  apigwv1types.AuthorizerTypeToken,
			AuthorizerUri:         aws.String("arn:aws:apigateway:us-east-1:lambda:path/2015-03-31/functions/arn:aws:lambda:us-east-1:123456789012:function:api-gateway-authorizer/invocations"),
			AuthorizerCredentials: aws.String(authorizerRole),
			IdentitySource:        aws.String("method.request.header.Authorization"),
		}},
	}
	api := t570Row(t, c, "apigw", restID)

	t570Contains(t, t570Pivot(t, c, api, "apigw", "lambda"), []string{"audit-logger"}, nil)
	t570Contains(t, t570Pivot(t, c, api, "apigw", "role"), []string{"acme-ci-deploy-role", "acme-lambda-execution"}, nil)
	for _, target := range []string{"kms", "elb"} {
		got := t570Pivot(t, c, api, "apigw", target)
		if got.Err() != nil || got.State() != domain.RelatedResolved {
			t.Errorf("apigw -> %s on a REST API: state %v, err %v; want the REST API read", target, got.State(), got.Err())
		}
	}
}

// DescribeImages does not return a block device's KmsKeyId (EbsBlockDevice:
// supported only on RunInstances and Spot requests); an AMI's key is the one
// its snapshots are encrypted with (DescribeSnapshots Snapshot.KmsKeyId).
func TestT570_AMIKMS_ReadsTheSnapshotsKey(t *testing.T) {
	c := t570Demo()
	c.EC2 = &t570EC2{EC2API: c.EC2, images: func(imgs []ec2types.Image) []ec2types.Image {
		for i := range imgs {
			for j := range imgs[i].BlockDeviceMappings {
				if ebs := imgs[i].BlockDeviceMappings[j].Ebs; ebs != nil {
					cp := *ebs
					cp.KmsKeyId = nil
					imgs[i].BlockDeviceMappings[j].Ebs = &cp
				}
			}
		}
		return imgs
	}}
	ami := t570Row(t, c, "ami", "ami-0a1b2c3d4e5f60001")
	t570Exact(t, t570Pivot(t, c, ami, "ami", "kms"), "a1b2c3d4-5678-90ab-cdef-111111111111")
}

// A log group's S3 archive is where its export tasks wrote
// (DescribeExportTasks ExportTask.Destination for ExportTask.LogGroupName).
// Subscription filters never deliver to S3.
func TestT570_LogsS3_ReadsExportTasks(t *testing.T) {
	c := t570Demo()
	b := t570Buckets(t, c, 2)
	const group = "/aws/cloudtrail"
	c.CloudWatchLogs = &t570CWLogs{CWLogsAPI: c.CloudWatchLogs, exports: []cwltypes.ExportTask{
		{TaskId: aws.String("0f1e2d3c-0000-4000-8000-000000000001"), TaskName: aws.String("trail-archive-2026-08"),
			LogGroupName: aws.String(group), Destination: aws.String(b[0]), DestinationPrefix: aws.String("cloudtrail/2026-08"),
			From: aws.Int64(1785542400000), To: aws.Int64(1788220800000),
			Status: &cwltypes.ExportTaskStatus{Code: cwltypes.ExportTaskStatusCodeCompleted}},
		{TaskId: aws.String("0f1e2d3c-0000-4000-8000-000000000002"), TaskName: aws.String("other-group-archive"),
			LogGroupName: aws.String("/app/legacy/orphan-old"), Destination: aws.String(b[1]),
			Status: &cwltypes.ExportTaskStatus{Code: cwltypes.ExportTaskStatusCodeCompleted}},
	}}
	g := t570Row(t, c, "logs", group)
	t570Exact(t, t570Pivot(t, c, g, "logs", "s3"), b[0])
}

type t570APIError struct{ code string }

func (e t570APIError) Error() string                 { return e.code + ": not authorized" }
func (e t570APIError) ErrorCode() string             { return e.code }
func (e t570APIError) ErrorMessage() string          { return "not authorized" }
func (e t570APIError) ErrorFault() smithy.ErrorFault { return smithy.FaultClient }

// A domain's custom-endpoint certificate is on the row itself
// (DomainStatus.DomainEndpointOptions.CustomEndpointCertificateArn, set when
// CustomEndpointEnabled). Reading it needs no DescribeDomainConfig, so a role
// without es:DescribeDomainConfig still sees it.
func TestT570_OpenSearchACM_ReadsTheRowsEndpointOptions(t *testing.T) {
	c := t570Demo()
	domain := t570List(t, c, "opensearch")[0].ID
	cert := t570List(t, c, "acm")[0]
	c.OpenSearch = &t570OpenSearch{OpenSearchAPI: c.OpenSearch, domain: domain,
		cfgErr: t570APIError{code: "AccessDeniedException"},
		edit: func(d *opensearchtypes.DomainStatus) {
			d.DomainEndpointOptions = &opensearchtypes.DomainEndpointOptions{
				EnforceHTTPS:                 aws.Bool(true),
				CustomEndpointEnabled:        aws.Bool(true),
				CustomEndpoint:               aws.String("search.acme.example.com"),
				CustomEndpointCertificateArn: aws.String(cert.ID),
			}
		}}
	d := t570Row(t, c, "opensearch", domain)
	t570Exact(t, t570Pivot(t, c, d, "opensearch", "acm"), cert.ID)
}

// With CustomEndpointEnabled false the domain serves only its AWS endpoint,
// so a certificate ARN left on the options terminates nothing.
func TestT570_OpenSearchACM_DisabledCustomEndpointIsZero(t *testing.T) {
	c := t570Demo()
	domain := t570List(t, c, "opensearch")[0].ID
	cert := t570List(t, c, "acm")[0]
	c.OpenSearch = &t570OpenSearch{OpenSearchAPI: c.OpenSearch, domain: domain,
		cfgErr: t570APIError{code: "AccessDeniedException"},
		edit: func(d *opensearchtypes.DomainStatus) {
			d.DomainEndpointOptions = &opensearchtypes.DomainEndpointOptions{
				EnforceHTTPS:                 aws.Bool(true),
				CustomEndpointEnabled:        aws.Bool(false),
				CustomEndpointCertificateArn: aws.String(cert.ID),
			}
		}}
	d := t570Row(t, c, "opensearch", domain)
	t570Exact(t, t570Pivot(t, c, d, "opensearch", "acm"))
}

// A VPC endpoint owns no flow log. Its traffic is logged by the flow logs of
// its VPC and of its subnets (docs/resources/vpce.md), delivered to
// CloudWatch Logs.
func TestT570_VPCELogs_ReadsTheEndpointsVPCAndSubnetFlowLogs(t *testing.T) {
	c := t570Demo()
	var endpoint ec2types.VpcEndpoint
	var id string
	for _, r := range t570List(t, c, "vpce") {
		if e := r.RawStruct.(ec2types.VpcEndpoint); len(e.SubnetIds) > 0 {
			endpoint, id = e, r.ID
			break
		}
	}
	if id == "" {
		t.Fatal("demo account holds no interface endpoint with subnets")
	}
	const vpcGroup = "/aws/vpc/flowlogs/vpce-s3-endpoint"
	const subnetGroup = "/app/acme-unencrypted-audit"
	fl := func(flID, resourceID, group string) ec2types.FlowLog {
		return ec2types.FlowLog{
			FlowLogId:          aws.String(flID),
			ResourceId:         aws.String(resourceID),
			LogGroupName:       aws.String(group),
			LogDestinationType: ec2types.LogDestinationTypeCloudWatchLogs,
			LogDestination:     aws.String("arn:aws:logs:us-east-1:123456789012:log-group:" + group),
			TrafficType:        ec2types.TrafficTypeAll,
			FlowLogStatus:      aws.String("ACTIVE"),
		}
	}
	c.EC2 = &t570EC2{EC2API: c.EC2, flowLogs: []ec2types.FlowLog{
		fl("fl-0vpc000000000001", aws.ToString(endpoint.VpcId), vpcGroup),
		fl("fl-0sub000000000002", endpoint.SubnetIds[0], subnetGroup),
		{
			FlowLogId:          aws.String("fl-0s3d000000000003"),
			ResourceId:         aws.String(aws.ToString(endpoint.VpcId)),
			LogDestinationType: ec2types.LogDestinationTypeS3,
			LogDestination:     aws.String("arn:aws:s3:::a9s-demo-logs/flow/"),
			TrafficType:        ec2types.TrafficTypeReject,
		},
	}}
	src := t570Row(t, c, "vpce", id)
	t570Exact(t, t570Pivot(t, c, src, "vpce", "logs"), vpcGroup, subnetGroup)
}

type t570WAF struct {
	awsclient.WAFv2API
	scopes map[wafv2types.LogScope][]string
}

// GetLoggingConfiguration answers per LogScope: CUSTOMER is the default and
// holds only configurations the customer manages.
func (f *t570WAF) GetLoggingConfiguration(_ context.Context, in *wafv2.GetLoggingConfigurationInput, _ ...func(*wafv2.Options)) (*wafv2.GetLoggingConfigurationOutput, error) {
	scope := in.LogScope
	if scope == "" {
		scope = wafv2types.LogScopeCustomer
	}
	dests, ok := f.scopes[scope]
	if !ok {
		return nil, &wafv2types.WAFNonexistentItemException{Message: aws.String("AWS WAF couldn't perform the operation because your resource doesn't exist.")}
	}
	return &wafv2.GetLoggingConfigurationOutput{LoggingConfiguration: &wafv2types.LoggingConfiguration{
		ResourceArn:           in.ResourceArn,
		LogDestinationConfigs: dests,
		LogScope:              scope,
		LogType:               wafv2types.LogTypeWafLogs,
	}}, nil
}

// A web ACL whose logging is managed by a CloudWatch telemetry rule
// (LogScope CLOUDWATCH_TELEMETRY_RULE_MANAGED) delivers to a log group; the
// default CUSTOMER scope answering "no configuration" does not make that 0.
func TestT570_WAFLogs_ReadsEveryLogScope(t *testing.T) {
	c := t570Demo()
	const group = "aws-waf-logs-acme-prod-api"
	c.WAFv2 = &t570WAF{WAFv2API: c.WAFv2, scopes: map[wafv2types.LogScope][]string{
		wafv2types.LogScopeCloudwatchTelemetryRuleManaged: {"arn:aws:logs:us-east-1:123456789012:log-group:" + group},
	}}
	acl := t570List(t, c, "waf")[0]
	got := t570Pivot(t, c, acl, "waf", "logs")
	t570NotProvenZero(t, got)
	t570Contains(t, got, []string{group}, nil)
}

// One detail open reads a workgroup once: the kms, logs, s3 and role pivots
// all answer from the same GetWorkGroup.
func TestT570_Athena_OneGetWorkGroupPerDetailOpen(t *testing.T) {
	c := t570Demo()
	fake := &t570Athena{AthenaAPI: c.Athena, workgroup: "primary", config: athenatypes.WorkGroupConfiguration{
		ResultConfiguration: &athenatypes.ResultConfiguration{OutputLocation: aws.String("s3://a9s-demo-healthy/athena/")},
		ExecutionRole:       aws.String("arn:aws:iam::123456789012:role/acme-ci-deploy-role"),
	}}
	c.Athena = fake
	wg := t570Row(t, c, "athena", "primary")
	fake.calls = 0
	for _, target := range []string{"kms", "logs", "s3", "role"} {
		t570Pivot(t, c, wg, "athena", target)
	}
	if fake.calls > 1 {
		t.Errorf("GetWorkGroup called %d times for one detail open, want at most 1", fake.calls)
	}
}
