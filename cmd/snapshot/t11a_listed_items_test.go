package main

import (
	"context"
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	athenatypes "github.com/aws/aws-sdk-go-v2/service/athena/types"
	"github.com/aws/aws-sdk-go-v2/service/codebuild"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
	"github.com/aws/aws-sdk-go-v2/service/mwaa"
	mwaatypes "github.com/aws/aws-sdk-go-v2/service/mwaa/types"
	"github.com/aws/aws-sdk-go-v2/service/opensearch"
	ostypes "github.com/aws/aws-sdk-go-v2/service/opensearch/types"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/transfer"
	transfertypes "github.com/aws/aws-sdk-go-v2/service/transfer/types"
	smithy "github.com/aws/smithy-go"
)

func t11aAPIError(code, message string) error {
	return &smithy.GenericAPIError{Code: code, Message: message}
}

var errT11aDenied = t11aAPIError("AccessDeniedException", "User: arn:aws:iam::123456789012:user/example-readonly is not authorized to perform this action")

// A listed item's describe answer is one of four: it describes the item
// ("-ok"), it names nothing for it ("-left": absent from a batch answer that
// does not report it gone, or a per-item answer with no body), the service
// reports the item gone ("-gone": deleted between the list and the describe),
// or the call is refused ("-denied").
func t11aStateOf(id string) string {
	for _, s := range []string{"ok", "left", "gone", "denied"} {
		if strings.HasSuffix(id, "-"+s) {
			return s
		}
	}
	return ""
}

func t11aIDs(prefix string, states ...string) []string {
	ids := make([]string, len(states))
	for i, s := range states {
		ids[i] = prefix + "-" + s
	}
	return ids
}

// t11aOutcomes returns, per element of the list under listKey, the values of
// every member named "outcome" or "<x>_outcome" at any depth, keyed by the
// element's idKey member.
func t11aOutcomes(t *testing.T, data any, listKey, idKey string) map[string][]string {
	t.Helper()
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	var elems []map[string]any
	if err := json.Unmarshal(doc[listKey], &elems); err != nil {
		t.Fatalf("%s: %v in %s", listKey, err, b)
	}
	out := map[string][]string{}
	for _, e := range elems {
		id, _ := e[idKey].(string)
		var words []string
		var walk func(v any)
		walk = func(v any) {
			switch x := v.(type) {
			case map[string]any:
				for k, m := range x {
					if s, ok := m.(string); ok && (k == "outcome" || strings.HasSuffix(k, "_outcome")) {
						words = append(words, s)
					}
					walk(m)
				}
			case []any:
				for _, m := range x {
					walk(m)
				}
			}
		}
		walk(e)
		out[id] = words
	}
	return out
}

// t11aDefault answers any call a case does not name with the synthetic
// answer, so a read the capture adds later does not break the case.
func t11aDefault(service string, in any) (any, error) {
	outType, ok := sdkOutputTypes()[reflect.TypeOf(in)]
	if !ok {
		return nil, t11aAPIError("UnknownOperationException", "no answer")
	}
	return synthesize(service, outType), nil
}

type listedCase struct {
	typ     string
	capture captureFunc
	listKey string
	idKey   string
	// batch: one describe call answers many listed items, so a refused call
	// refuses each of them; the items are then listed without a "-denied" one.
	batch  bool
	listed []string
	answer func(op string, in any, refuse bool) (any, error)
}

const (
	t11aECSCluster = "arn:aws:ecs:eu-west-1:123456789012:cluster/prod"
)

func t11aListedCases() []listedCase {
	ecsClusters := t11aIDs("arn:aws:ecs:eu-west-1:123456789012:cluster/web", "ok", "left", "gone")
	ecsServices := t11aIDs("arn:aws:ecs:eu-west-1:123456789012:service/prod/api", "ok", "left", "gone")
	ecsTasks := t11aIDs("arn:aws:ecs:eu-west-1:123456789012:task/prod/0a1b2c3d4e5f", "ok", "left", "gone")
	cbProjects := t11aIDs("build", "ok", "left", "gone")
	osDomains := t11aIDs("search", "ok", "left")
	perItem := []string{"ok", "left", "gone", "denied"}
	eksClusters := t11aIDs("eks", perItem...)
	ngGroups := t11aIDs("workers", perItem...)
	ltIDs := t11aIDs("lt-0abc123def4567890", perItem...)
	transferIDs := t11aIDs("s-01234567890abcdef", perItem...)
	mwaaNames := t11aIDs("airflow", perItem...)
	ddbTables := t11aIDs("orders", perItem...)
	athenaGroups := t11aIDs("analytics", perItem...)
	kmsKeys := t11aIDs("1234abcd-12ab-34cd-56ef-1234567890ab", perItem...)
	sqsURLs := t11aIDs("https://sqs.eu-west-1.amazonaws.com/123456789012/jobs", perItem...)

	ecsMissing := func(id string) ecstypes.Failure {
		return ecstypes.Failure{Arn: aws.String(id), Reason: aws.String("MISSING")}
	}
	perItemErr := func(id, goneCode string) error {
		switch t11aStateOf(id) {
		case "gone":
			return t11aAPIError(goneCode, "not found: "+id)
		case "denied":
			return errT11aDenied
		}
		return nil
	}

	return []listedCase{
		{
			typ: "ecs", capture: captureECS, listKey: "clusters", idKey: "cluster_arn", batch: true, listed: ecsClusters,
			answer: func(op string, in any, refuse bool) (any, error) {
				switch op {
				case "ListClusters":
					return &ecs.ListClustersOutput{ClusterArns: ecsClusters}, nil
				case "DescribeClusters":
					if refuse {
						return nil, errT11aDenied
					}
					out := &ecs.DescribeClustersOutput{}
					for _, id := range in.(*ecs.DescribeClustersInput).Clusters {
						switch t11aStateOf(id) {
						case "ok":
							out.Clusters = append(out.Clusters, ecstypes.Cluster{ClusterArn: aws.String(id), ClusterName: aws.String("web-ok"), Status: aws.String("ACTIVE"), RunningTasksCount: 2})
						case "gone":
							out.Failures = append(out.Failures, ecsMissing(id))
						}
					}
					return out, nil
				}
				return nil, nil
			},
		},
		{
			typ: "ecs-svc", capture: captureECSSvc, listKey: "services", idKey: "service_arn", batch: true, listed: ecsServices,
			answer: func(op string, in any, refuse bool) (any, error) {
				switch op {
				case "ListClusters":
					return &ecs.ListClustersOutput{ClusterArns: []string{t11aECSCluster}}, nil
				case "ListServices":
					return &ecs.ListServicesOutput{ServiceArns: ecsServices}, nil
				case "DescribeServices":
					if refuse {
						return nil, errT11aDenied
					}
					out := &ecs.DescribeServicesOutput{}
					for _, id := range in.(*ecs.DescribeServicesInput).Services {
						switch t11aStateOf(id) {
						case "ok":
							out.Services = append(out.Services, ecstypes.Service{ServiceArn: aws.String(id), ServiceName: aws.String("api-ok"), ClusterArn: aws.String(t11aECSCluster), Status: aws.String("ACTIVE"), DesiredCount: 2, RunningCount: 2})
						case "gone":
							out.Failures = append(out.Failures, ecsMissing(id))
						}
					}
					return out, nil
				}
				return nil, nil
			},
		},
		{
			typ: "ecs-task", capture: captureECSTask, listKey: "tasks", idKey: "task_arn", batch: true, listed: ecsTasks,
			answer: func(op string, in any, refuse bool) (any, error) {
				switch op {
				case "ListClusters":
					return &ecs.ListClustersOutput{ClusterArns: []string{t11aECSCluster}}, nil
				case "ListTasks":
					return &ecs.ListTasksOutput{TaskArns: ecsTasks}, nil
				case "DescribeTasks":
					if refuse {
						return nil, errT11aDenied
					}
					out := &ecs.DescribeTasksOutput{}
					for _, id := range in.(*ecs.DescribeTasksInput).Tasks {
						switch t11aStateOf(id) {
						case "ok":
							out.Tasks = append(out.Tasks, ecstypes.Task{TaskArn: aws.String(id), ClusterArn: aws.String(t11aECSCluster), LastStatus: aws.String("RUNNING"),
								TaskDefinitionArn: aws.String("arn:aws:ecs:eu-west-1:123456789012:task-definition/api:7")})
						case "gone":
							out.Failures = append(out.Failures, ecsMissing(id))
						}
					}
					return out, nil
				case "DescribeTaskDefinition":
					return &ecs.DescribeTaskDefinitionOutput{TaskDefinition: &ecstypes.TaskDefinition{
						ContainerDefinitions: []ecstypes.ContainerDefinition{{Name: aws.String("api"), Essential: aws.Bool(true)}},
					}}, nil
				}
				return nil, nil
			},
		},
		{
			typ: "cb", capture: captureCB, listKey: "projects", idKey: "name", batch: true, listed: cbProjects,
			answer: func(op string, in any, refuse bool) (any, error) {
				switch op {
				case "ListProjects":
					return &codebuild.ListProjectsOutput{Projects: cbProjects}, nil
				case "BatchGetProjects":
					if refuse {
						return nil, errT11aDenied
					}
					out := &codebuild.BatchGetProjectsOutput{}
					for _, n := range in.(*codebuild.BatchGetProjectsInput).Names {
						switch t11aStateOf(n) {
						case "ok":
							out.Projects = append(out.Projects, cbtypes.Project{Name: aws.String(n), Arn: aws.String("arn:aws:codebuild:eu-west-1:123456789012:project/" + n),
								ServiceRole: aws.String("arn:aws:iam::123456789012:role/codebuild-" + n)})
						case "gone":
							out.ProjectsNotFound = append(out.ProjectsNotFound, n)
						}
					}
					return out, nil
				case "ListBuildsForProject":
					return &codebuild.ListBuildsForProjectOutput{}, nil
				}
				return nil, nil
			},
		},
		{
			typ: "opensearch", capture: captureOpenSearch, listKey: "domains", idKey: "domain_name", batch: true, listed: osDomains,
			answer: func(op string, in any, refuse bool) (any, error) {
				switch op {
				case "ListDomainNames":
					out := &opensearch.ListDomainNamesOutput{}
					for _, n := range osDomains {
						out.DomainNames = append(out.DomainNames, ostypes.DomainInfo{DomainName: aws.String(n), EngineType: ostypes.EngineTypeOpenSearch})
					}
					return out, nil
				case "DescribeDomains":
					if refuse {
						return nil, errT11aDenied
					}
					out := &opensearch.DescribeDomainsOutput{}
					for _, n := range in.(*opensearch.DescribeDomainsInput).DomainNames {
						if t11aStateOf(n) == "ok" {
							out.DomainStatusList = append(out.DomainStatusList, ostypes.DomainStatus{DomainName: aws.String(n), DomainId: aws.String("123456789012/" + n),
								ARN: aws.String("arn:aws:es:eu-west-1:123456789012:domain/" + n), EngineVersion: aws.String("OpenSearch_2.13")})
						}
					}
					return out, nil
				}
				return nil, nil
			},
		},
		{
			typ: "eks", capture: captureEKS, listKey: "clusters", idKey: "name", listed: eksClusters,
			answer: func(op string, in any, _ bool) (any, error) {
				switch op {
				case "ListClusters":
					return &eks.ListClustersOutput{Clusters: eksClusters}, nil
				case "DescribeCluster":
					n := aws.ToString(in.(*eks.DescribeClusterInput).Name)
					if err := perItemErr(n, "ResourceNotFoundException"); err != nil {
						return nil, err
					}
					if t11aStateOf(n) == "left" {
						return &eks.DescribeClusterOutput{}, nil
					}
					return &eks.DescribeClusterOutput{Cluster: &ekstypes.Cluster{Name: aws.String(n), Arn: aws.String("arn:aws:eks:eu-west-1:123456789012:cluster/" + n),
						Status: ekstypes.ClusterStatusActive, Version: aws.String("1.31")}}, nil
				}
				return nil, nil
			},
		},
		{
			typ: "ng", capture: captureNG, listKey: "nodegroups", idKey: "nodegroup_name", listed: ngGroups,
			answer: func(op string, in any, _ bool) (any, error) {
				switch op {
				case "ListClusters":
					return &eks.ListClustersOutput{Clusters: []string{"prod"}}, nil
				case "ListNodegroups":
					return &eks.ListNodegroupsOutput{Nodegroups: ngGroups}, nil
				case "DescribeNodegroup":
					n := aws.ToString(in.(*eks.DescribeNodegroupInput).NodegroupName)
					if err := perItemErr(n, "ResourceNotFoundException"); err != nil {
						return nil, err
					}
					if t11aStateOf(n) == "left" {
						return &eks.DescribeNodegroupOutput{}, nil
					}
					return &eks.DescribeNodegroupOutput{Nodegroup: &ekstypes.Nodegroup{NodegroupName: aws.String(n), ClusterName: aws.String("prod"),
						NodegroupArn: aws.String("arn:aws:eks:eu-west-1:123456789012:nodegroup/prod/" + n + "/1a2b3c4d"), Status: ekstypes.NodegroupStatusActive}}, nil
				}
				return nil, nil
			},
		},
		{
			typ: "lt", capture: captureLT, listKey: "launch_templates", idKey: "launch_template_id", listed: ltIDs,
			answer: func(op string, in any, _ bool) (any, error) {
				switch op {
				case "DescribeLaunchTemplates":
					out := &ec2.DescribeLaunchTemplatesOutput{}
					for _, id := range ltIDs {
						out.LaunchTemplates = append(out.LaunchTemplates, ec2types.LaunchTemplate{LaunchTemplateId: aws.String(id), LaunchTemplateName: aws.String("web-" + t11aStateOf(id)),
							DefaultVersionNumber: aws.Int64(1), LatestVersionNumber: aws.Int64(1)})
					}
					return out, nil
				case "DescribeLaunchTemplateVersions":
					id := aws.ToString(in.(*ec2.DescribeLaunchTemplateVersionsInput).LaunchTemplateId)
					switch t11aStateOf(id) {
					case "gone":
						return nil, t11aAPIError("InvalidLaunchTemplateId.NotFound", "The specified launch template, with template ID "+id+", does not exist.")
					case "denied":
						return nil, t11aAPIError("UnauthorizedOperation", "You are not authorized to perform this operation.")
					case "left":
						return &ec2.DescribeLaunchTemplateVersionsOutput{}, nil
					}
					return &ec2.DescribeLaunchTemplateVersionsOutput{LaunchTemplateVersions: []ec2types.LaunchTemplateVersion{{
						LaunchTemplateId: aws.String(id), VersionNumber: aws.Int64(1), DefaultVersion: aws.Bool(true),
						LaunchTemplateData: &ec2types.ResponseLaunchTemplateData{ImageId: aws.String("ami-0abcdef1234567890"),
							MetadataOptions: &ec2types.LaunchTemplateInstanceMetadataOptions{HttpTokens: ec2types.LaunchTemplateHttpTokensStateRequired}},
					}}}, nil
				}
				return nil, nil
			},
		},
		{
			typ: "transfer", capture: captureTransfer, listKey: "servers", idKey: "server_id", listed: transferIDs,
			answer: func(op string, in any, _ bool) (any, error) {
				switch op {
				case "ListServers":
					out := &transfer.ListServersOutput{}
					for _, id := range transferIDs {
						out.Servers = append(out.Servers, transfertypes.ListedServer{ServerId: aws.String(id), Arn: aws.String("arn:aws:transfer:eu-west-1:123456789012:server/" + id),
							State: transfertypes.StateOnline})
					}
					return out, nil
				case "DescribeServer":
					id := aws.ToString(in.(*transfer.DescribeServerInput).ServerId)
					if err := perItemErr(id, "ResourceNotFoundException"); err != nil {
						return nil, err
					}
					if t11aStateOf(id) == "left" {
						return &transfer.DescribeServerOutput{}, nil
					}
					return &transfer.DescribeServerOutput{Server: &transfertypes.DescribedServer{ServerId: aws.String(id), Arn: aws.String("arn:aws:transfer:eu-west-1:123456789012:server/" + id),
						State: transfertypes.StateOnline, SecurityPolicyName: aws.String("TransferSecurityPolicy-2024-01")}}, nil
				}
				return nil, nil
			},
		},
		{
			typ: "mwaa", capture: captureMWAA, listKey: "environments", idKey: "name", listed: mwaaNames,
			answer: func(op string, in any, _ bool) (any, error) {
				switch op {
				case "ListEnvironments":
					return &mwaa.ListEnvironmentsOutput{Environments: mwaaNames}, nil
				case "GetEnvironment":
					n := aws.ToString(in.(*mwaa.GetEnvironmentInput).Name)
					if err := perItemErr(n, "ResourceNotFoundException"); err != nil {
						return nil, err
					}
					if t11aStateOf(n) == "left" {
						return &mwaa.GetEnvironmentOutput{}, nil
					}
					return &mwaa.GetEnvironmentOutput{Environment: &mwaatypes.Environment{Name: aws.String(n), Arn: aws.String("arn:aws:airflow:eu-west-1:123456789012:environment/" + n),
						Status: mwaatypes.EnvironmentStatusAvailable, AirflowVersion: aws.String("2.10.3")}}, nil
				}
				return nil, nil
			},
		},
		{
			typ: "ddb", capture: captureDDB, listKey: "tables", idKey: "table_name", listed: ddbTables,
			answer: func(op string, in any, _ bool) (any, error) {
				switch op {
				case "ListTables":
					return &dynamodb.ListTablesOutput{TableNames: ddbTables}, nil
				case "DescribeTable":
					n := aws.ToString(in.(*dynamodb.DescribeTableInput).TableName)
					if err := perItemErr(n, "ResourceNotFoundException"); err != nil {
						return nil, err
					}
					if t11aStateOf(n) == "left" {
						return &dynamodb.DescribeTableOutput{}, nil
					}
					return &dynamodb.DescribeTableOutput{Table: &ddbtypes.TableDescription{TableName: aws.String(n), TableArn: aws.String("arn:aws:dynamodb:eu-west-1:123456789012:table/" + n),
						TableStatus: ddbtypes.TableStatusActive}}, nil
				case "DescribeContinuousBackups":
					n := aws.ToString(in.(*dynamodb.DescribeContinuousBackupsInput).TableName)
					if t11aStateOf(n) == "gone" {
						return nil, t11aAPIError("TableNotFoundException", "Table not found: "+n)
					}
					return &dynamodb.DescribeContinuousBackupsOutput{ContinuousBackupsDescription: &ddbtypes.ContinuousBackupsDescription{
						ContinuousBackupsStatus:        ddbtypes.ContinuousBackupsStatusEnabled,
						PointInTimeRecoveryDescription: &ddbtypes.PointInTimeRecoveryDescription{PointInTimeRecoveryStatus: ddbtypes.PointInTimeRecoveryStatusEnabled},
					}}, nil
				}
				return nil, nil
			},
		},
		{
			typ: "athena", capture: captureAthena, listKey: "workgroups", idKey: "name", listed: athenaGroups,
			answer: func(op string, in any, _ bool) (any, error) {
				switch op {
				case "ListWorkGroups":
					out := &athena.ListWorkGroupsOutput{}
					for _, n := range athenaGroups {
						out.WorkGroups = append(out.WorkGroups, athenatypes.WorkGroupSummary{Name: aws.String(n), State: athenatypes.WorkGroupStateEnabled})
					}
					return out, nil
				case "GetWorkGroup":
					n := aws.ToString(in.(*athena.GetWorkGroupInput).WorkGroup)
					switch t11aStateOf(n) {
					case "gone":
						return nil, t11aAPIError("InvalidRequestException", "WorkGroup "+n+" is not found.")
					case "denied":
						return nil, errT11aDenied
					case "left":
						return &athena.GetWorkGroupOutput{}, nil
					}
					return &athena.GetWorkGroupOutput{WorkGroup: &athenatypes.WorkGroup{Name: aws.String(n), State: athenatypes.WorkGroupStateEnabled,
						Configuration: &athenatypes.WorkGroupConfiguration{EnforceWorkGroupConfiguration: aws.Bool(true)}}}, nil
				}
				return nil, nil
			},
		},
		{
			typ: "kms", capture: captureKMS, listKey: "keys", idKey: "key_id", listed: kmsKeys,
			answer: func(op string, in any, _ bool) (any, error) {
				switch op {
				case "ListKeys":
					out := &kms.ListKeysOutput{}
					for _, id := range kmsKeys {
						out.Keys = append(out.Keys, kmstypes.KeyListEntry{KeyId: aws.String(id), KeyArn: aws.String("arn:aws:kms:eu-west-1:123456789012:key/" + id)})
					}
					return out, nil
				case "DescribeKey":
					id := aws.ToString(in.(*kms.DescribeKeyInput).KeyId)
					if err := perItemErr(id, "NotFoundException"); err != nil {
						return nil, err
					}
					if t11aStateOf(id) == "left" {
						return &kms.DescribeKeyOutput{}, nil
					}
					return &kms.DescribeKeyOutput{KeyMetadata: &kmstypes.KeyMetadata{KeyId: aws.String(id), Arn: aws.String("arn:aws:kms:eu-west-1:123456789012:key/" + id),
						KeyManager: kmstypes.KeyManagerTypeCustomer, KeyState: kmstypes.KeyStateEnabled, Enabled: true}}, nil
				case "GetKeyRotationStatus":
					return &kms.GetKeyRotationStatusOutput{KeyRotationEnabled: true}, nil
				}
				return nil, nil
			},
		},
		{
			typ: "sqs", capture: captureSQS, listKey: "queues", idKey: "queue_url", listed: sqsURLs,
			answer: func(op string, in any, _ bool) (any, error) {
				switch op {
				case "ListQueues":
					return &sqs.ListQueuesOutput{QueueUrls: sqsURLs}, nil
				case "GetQueueAttributes":
					u := aws.ToString(in.(*sqs.GetQueueAttributesInput).QueueUrl)
					if err := perItemErr(u, "AWS.SimpleQueueService.NonExistentQueue"); err != nil {
						return nil, err
					}
					if t11aStateOf(u) == "left" {
						return &sqs.GetQueueAttributesOutput{}, nil
					}
					return &sqs.GetQueueAttributesOutput{Attributes: map[string]string{
						"QueueArn": "arn:aws:sqs:eu-west-1:123456789012:jobs", "VisibilityTimeout": "30", "ApproximateNumberOfMessages": "0",
					}}, nil
				}
				return nil, nil
			},
		},
	}
}

func (c listedCase) run(t *testing.T, refuse bool) (map[string][]string, error) {
	t.Helper()
	cfg := answeringConfig(func(service, op string, in any) (any, error) {
		out, err := c.answer(op, in, refuse)
		if out == nil && err == nil {
			return t11aDefault(service, in)
		}
		return out, err
	})
	data, err := c.capture(context.Background(), cfg)
	if err != nil {
		return nil, err
	}
	return t11aOutcomes(t, data, c.listKey, c.idKey), nil
}

// The app keeps a row for every item a list names unless the service states
// the item is gone (core/aws/degraded_resource.go keepListed): a describe that
// answers for the item gives the described row, a describe that answers
// nothing for it or is refused gives a degraded row, and a describe the
// service answers with "gone" drops it. The oracle holds the same rows, each
// with the outcome word of its describe: "ok", outcomeNotFound for an answer
// that named nothing, "error" for a refused call, and a refused batch refuses
// each item it named without failing the type.
func TestT11a_EveryListedItemKeepsARowWithItsDescribeOutcome(t *testing.T) {
	want := map[string]string{"ok": "ok", "left": outcomeNotFound, "denied": "error"}
	for _, c := range t11aListedCases() {
		t.Run(c.typ, func(t *testing.T) {
			got, err := c.run(t, false)
			if err != nil {
				t.Fatalf("the capture failed as a whole: %v", err)
			}
			var gotIDs, wantIDs []string
			for id := range got {
				gotIDs = append(gotIDs, id)
			}
			for _, id := range c.listed {
				if t11aStateOf(id) != "gone" {
					wantIDs = append(wantIDs, id)
				}
			}
			slices.Sort(gotIDs)
			slices.Sort(wantIDs)
			if !slices.Equal(gotIDs, wantIDs) {
				t.Errorf("rows = %v, want %v (an item reported gone is dropped, every other listed item keeps its row)", gotIDs, wantIDs)
			}
			for _, id := range wantIDs {
				w := want[t11aStateOf(id)]
				words := got[id]
				if !slices.Contains(words, w) {
					t.Errorf("%s: outcome words %v, want %q", id, words, w)
				}
				if w == "ok" && (slices.Contains(words, "error") || slices.Contains(words, outcomeNotFound)) {
					t.Errorf("%s: described item carries a failure word: %v", id, words)
				}
			}

			if !c.batch {
				return
			}
			refused, err := c.run(t, true)
			if err != nil {
				t.Fatalf("a refused describe batch failed the whole capture: %v", err)
			}
			for _, id := range c.listed {
				if words, ok := refused[id]; !ok || !slices.Contains(words, "error") {
					t.Errorf("refused batch: %s has row %v with outcome words %v, want a row with %q", id, ok, words, "error")
				}
			}
		})
	}
}
