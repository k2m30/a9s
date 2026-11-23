package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/resource"
)

// declaredRequestDifferences lists the requests the collector and the app
// legitimately do not share, keyed "<type> <Service>.<Operation> <question>"
// (that one request is left out of the type's comparison on both sides; the
// question is the input as requestQuestion writes it) or "<type>" (the type is
// compared by its first request only, in
// TestT11a_CollectorSendsTheAppsFirstRequestForEveryTopLevelType), each with
// the reason.
var declaredRequestDifferences = map[string]string{
	`ec2 EC2.DescribeInstanceStatus {"DryRun":null,"Filters":null,"IncludeAllInstances":true,"IncludeManagedResources":null,"InstanceIds":["synthetic-instanceid"]}`: "the fetcher asks for the listed instances by id; the collector asks the account-wide question Wave 2 asks, whose answer holds every listed instance",
	`cb CodeBuild.BatchGetBuilds {"Ids":["synthetic-ids","synthetic-ids"]}`:                                                                                          "the synthetic answers give every project the same newest build id, which the app asks for once per project and the collector once; a build id names one project's build, so an account never lists it for two",
	`cb CodeBuild.BatchGetBuilds {"Ids":["synthetic-ids"]}`:                                                                                                          "the collector's question for the build id the synthetic answers give every project",
	`cfn CloudFormation.DescribeStacks {"StackName":"synthetic-stackname"}`:                                                                                          "Wave 2 describes each listed stack again by name; the collector's unscoped DescribeStacks answers the same stack record",
}

// noChecklistFact is the reason an app operation is not captured: no value
// cmd/checklist compares is derived from its answer.
const noChecklistFact = "cmd/checklist compares no fact this operation answers"

// notCaptured lists the operations the app sends for a type and the collector
// does not, keyed "<type> <Service>.<Operation>", each with the reason.
var notCaptured = map[string]string{
	"apigw API Gateway.GetAuthorizers":                             noChecklistFact,
	"apigw API Gateway.GetStages":                                  noChecklistFact,
	"apigw ApiGatewayV2.GetAuthorizers":                            noChecklistFact,
	"asg Auto Scaling.DescribeLaunchConfigurations":                noChecklistFact,
	"cf CloudWatch Logs.DescribeDeliveries":                        noChecklistFact,
	"cf CloudWatch Logs.DescribeDeliverySources":                   noChecklistFact,
	"codeartifact codeartifact.GetRepositoryPermissionsPolicy":     noChecklistFact,
	"dbc DocDB.DescribePendingMaintenanceActions":                  noChecklistFact,
	"dbc DocDB.ListTagsForResource":                                noChecklistFact,
	"dbc-snap RDS.DescribeDBClusterSnapshotAttributes":             noChecklistFact,
	"dbi RDS.DescribeDBEngineVersions":                             noChecklistFact,
	"dbi RDS.ListTagsForResource":                                  noChecklistFact,
	"dbi-snap RDS.DescribeDBSnapshotAttributes":                    noChecklistFact,
	"ddb DynamoDB.GetResourcePolicy":                               noChecklistFact,
	"ddb DynamoDB.ListTagsOfResource":                              noChecklistFact,
	"eb Elastic Beanstalk.DescribeConfigurationSettings":           noChecklistFact,
	"ec2 EC2.DescribeInstanceAttribute":                            noChecklistFact,
	"ecr ECR.GetLifecyclePolicy":                                   noChecklistFact,
	"ecr ECR.GetRegistryScanningConfiguration":                     noChecklistFact,
	"ecr ECR.GetRepositoryPolicy":                                  noChecklistFact,
	"efs EFS.DescribeBackupPolicy":                                 noChecklistFact,
	"efs EFS.DescribeFileSystemPolicy":                             noChecklistFact,
	"elb Elastic Load Balancing v2.DescribeListeners":              noChecklistFact,
	"elb Elastic Load Balancing v2.DescribeLoadBalancerAttributes": noChecklistFact,
	"glue Glue.GetSecurityConfiguration":                           noChecklistFact,
	"iam-group IAM.ListAttachedGroupPolicies":                      noChecklistFact,
	"iam-group IAM.ListGroupPolicies":                              noChecklistFact,
	"iam-user IAM.ListAttachedUserPolicies":                        noChecklistFact,
	"kms KMS.GetKeyPolicy":                                         noChecklistFact,
	"lambda Lambda.GetPolicy":                                      noChecklistFact,
	"lambda Lambda.ListFunctionUrlConfigs":                         noChecklistFact,
	"msk Kafka.DescribeClusterV2":                                  noChecklistFact,
	"policy IAM.GetPolicy":                                         noChecklistFact,
	"r53 Route 53.ListQueryLoggingConfigs":                         noChecklistFact,
	"redshift Redshift.DescribeClusterParameters":                  noChecklistFact,
	"redshift Redshift.DescribeLoggingStatus":                      noChecklistFact,
	"role IAM.ListAttachedRolePolicies":                            noChecklistFact,
	"sfn SFN.DescribeStateMachine":                                 noChecklistFact,
	"secrets Secrets Manager.GetResourcePolicy":                    noChecklistFact,
	"sns SNS.ListSubscriptionsByTopic":                             noChecklistFact,
	"trail S3.GetBucketAcl":                                        noChecklistFact,
	"trail S3.GetBucketLocation":                                   noChecklistFact,
	"trail S3.GetBucketLogging":                                    noChecklistFact,
	"trail S3.GetBucketPolicyStatus":                               noChecklistFact,
	"trail S3.GetPublicAccessBlock":                                noChecklistFact,
	"transfer Transfer.DescribeSecurityPolicy":                     noChecklistFact,
	"waf CloudFront.ListDistributionsByWebACLId":                   noChecklistFact,
	"waf WAFV2.GetLoggingConfiguration":                            noChecklistFact,
	"waf WAFV2.ListResourcesForWebACL":                             noChecklistFact,
}

// callBudget bounds one run: a walk that never ends is a failure, not a hang.
const callBudget = 2000

var errCallBudget = errors.New("call budget spent")

// sdkOutputTypes maps the input type of every operation of every SDK client
// the app builds to its output type, read from the method signatures of the
// concrete clients in awsclient.ServiceClients (a coalescing wrapper holds its
// client in its first, embedded member). The collector's clients are the same
// SDK packages.
var sdkOutputTypes = sync.OnceValue(func() map[reflect.Type]reflect.Type {
	m := map[reflect.Type]reflect.Type{}
	sc := reflect.ValueOf(awsclient.CreateServiceClients(aws.Config{Region: "eu-west-1"})).Elem()
	for i := range sc.NumField() {
		if !sc.Type().Field(i).IsExported() {
			continue
		}
		c := sc.Field(i)
		for c.Kind() == reflect.Interface || (c.Kind() == reflect.Pointer && !c.IsNil() && c.Type().Elem().Name() != "Client") {
			if c.Kind() == reflect.Interface {
				c = c.Elem()
				continue
			}
			c = c.Elem().Field(0)
		}
		if !c.IsValid() || c.Kind() != reflect.Pointer {
			continue
		}
		ct := c.Type()
		for j := range ct.NumMethod() {
			mt := ct.Method(j).Type
			if mt.NumOut() != 2 {
				continue
			}
			out := mt.Out(0)
			for k := range mt.NumIn() {
				in := mt.In(k)
				if in.Kind() == reflect.Pointer && strings.HasSuffix(in.Elem().Name(), "Input") &&
					out.Kind() == reflect.Pointer && strings.HasSuffix(out.Elem().Name(), "Output") {
					m[in] = out
				}
			}
		}
	}
	return m
})

var t11aSyntheticTime = time.Date(2026, 11, 1, 8, 0, 0, 0, time.UTC)

// synthesize builds the answer to one SDK call: every list holds one element,
// every struct pointer is set, every string member carries a value derived
// from its member name (an ARN for an *Arn member, the account for an account
// member, the Region for a Region member), every enum its first value,
// booleans and numbers are zero, and every member that names a further page
// is nil, so a walk ends after one page.
func synthesize(service string, outType reflect.Type) any {
	v := reflect.New(outType.Elem())
	fillSynthetic(v.Elem(), service, outType.Elem().Name(), map[reflect.Type]bool{})
	return v.Interface()
}

// syntheticListedValues are the members, keyed "<Service>.<Member>", that a
// fetcher filters its rows on; each carries a value the type lists, so the
// row reaches the calls the app sends for it.
var syntheticListedValues = map[string]string{
	"ElastiCache.Engine": "redis",
	"KMS.KeyManager":     "CUSTOMER",
}

func isPageMember(name string) bool {
	switch name {
	case "Marker", "Position", "PaginationToken", "ContinuationToken", "StartingToken":
		return true
	}
	return strings.HasPrefix(name, "Next") || strings.HasPrefix(name, "LastEvaluated")
}

func syntheticString(service, name string) string {
	n := strings.ToLower(name)
	svc := strings.ToLower(strings.ReplaceAll(service, " ", ""))
	switch {
	case strings.Contains(name, "Arn") || strings.Contains(name, "ARN"):
		return "arn:aws:" + svc + ":eu-west-1:123456789012:" + n + "/synthetic-" + n
	case strings.HasSuffix(name, "AccountId") || name == "OwnerId" || name == "Owner" || name == "Account":
		return "123456789012"
	case strings.HasSuffix(name, "Region"):
		return "eu-west-1"
	}
	return "synthetic-" + n
}

func fillSynthetic(v reflect.Value, service, name string, path map[reflect.Type]bool) {
	t := v.Type()
	switch t.Kind() {
	case reflect.Pointer:
		if isPageMember(name) {
			return
		}
		e := reflect.New(t.Elem())
		fillSynthetic(e.Elem(), service, name, path)
		v.Set(e)
	case reflect.Struct:
		if t == reflect.TypeFor[time.Time]() {
			v.Set(reflect.ValueOf(t11aSyntheticTime))
			return
		}
		if !strings.HasPrefix(t.PkgPath(), "github.com/aws/aws-sdk-go-v2/service/") || path[t] {
			return
		}
		path[t] = true
		defer delete(path, t)
		for i := range t.NumField() {
			f := t.Field(i)
			if f.IsExported() {
				fillSynthetic(v.Field(i), service, f.Name, path)
			}
		}
	case reflect.Slice:
		if t.Elem().Kind() == reflect.Uint8 {
			return
		}
		if t.Elem().Kind() == reflect.Struct && path[t.Elem()] {
			return
		}
		s := reflect.MakeSlice(t, 1, 1)
		fillSynthetic(s.Index(0), service, name, path)
		v.Set(s)
	case reflect.String:
		if s, ok := syntheticListedValues[service+"."+name]; ok {
			v.SetString(s)
			return
		}
		if m := reflect.New(t).Elem().MethodByName("Values"); m.IsValid() {
			if vals := m.Call(nil)[0]; vals.Len() > 0 {
				v.Set(vals.Index(0))
				return
			}
		}
		v.SetString(syntheticString(service, name))
	}
}

// requestLog answers every SDK call with synthesize and records it as
// (service, operation, input without page size and page token), counting
// repeats. STS GetCallerIdentity is answered with a synthetic identity and
// not recorded: it names the account, it asks nothing about a type.
type requestLog struct {
	mu        sync.Mutex
	calls     map[string]int
	ops       map[string]bool
	n         int
	unmatched []string
}

func newRequestLog() *requestLog {
	return &requestLog{calls: map[string]int{}, ops: map[string]bool{}}
}

func (l *requestLog) config() aws.Config {
	return answeringConfig(func(service, op string, in any) (any, error) {
		if service == "STS" && op == "GetCallerIdentity" {
			return &sts.GetCallerIdentityOutput{
				Account: aws.String("123456789012"),
				Arn:     aws.String("arn:aws:iam::123456789012:user/example-readonly"),
				UserId:  aws.String("AIDAEXAMPLEUSERID"),
			}, nil
		}
		key := service + "." + op + " " + requestQuestion(in)
		l.mu.Lock()
		l.n++
		spent := l.n > callBudget
		l.calls[key]++
		l.ops[service+"."+op] = true
		outType, ok := sdkOutputTypes()[reflect.TypeOf(in)]
		if !ok {
			l.unmatched = append(l.unmatched, fmt.Sprintf("%s.%s (%T)", service, op, in))
		}
		l.mu.Unlock()
		if spent {
			return nil, errCallBudget
		}
		if !ok {
			return nil, fmt.Errorf("no output type for %T", in)
		}
		return synthesize(service, outType), nil
	})
}

// requestQuestion is the input as JSON with its top-level page-size and
// page-token members removed and every timestamp replaced by its distance
// from the clock to the hour: a window computed from the clock is the same
// question asked a moment later, and a window of another length is not.
func requestQuestion(in any) string {
	b, err := json.Marshal(in)
	if err != nil {
		return fmt.Sprintf("%T unmarshalable: %v", in, err)
	}
	m := map[string]any{}
	if err := json.Unmarshal(b, &m); err != nil {
		return string(b)
	}
	for k := range m {
		if paginationFields[k] {
			delete(m, k)
		}
	}
	out, _ := json.Marshal(stampTimes(m)) //nolint:errchkjson // decoded JSON always marshals
	return string(out)
}

func stampTimes(v any) any {
	switch x := v.(type) {
	case map[string]any:
		for k, e := range x {
			x[k] = stampTimes(e)
		}
	case []any:
		for i, e := range x {
			x[i] = stampTimes(e)
		}
	case string:
		if at, err := time.Parse(time.RFC3339Nano, x); err == nil {
			return "<now" + fmt.Sprintf("%+v", at.Sub(time.Now()).Round(time.Hour)) + ">"
		}
	}
	return v
}

// appRequests runs the app's path for a type: its fetcher, then its Wave-2
// enricher on the rows the fetcher returned, then its detail enricher on each
// row. It returns the fetcher's log and the log of the later calls. The
// enricher's declared Reads are listed through a separate, unrecorded log, as
// the session holds them before Wave 2 runs.
func appRequests(t *testing.T, name string) (fetchLog, laterLog *requestLog) {
	t.Helper()
	ctx := context.Background()
	fetch := resource.GetPaginatedFetcher(name)
	if fetch == nil {
		t.Fatalf("no fetcher is registered for top-level type %q", name)
	}
	fetchLog, laterLog = newRequestLog(), newRequestLog()
	page, err := fetch(ctx, awsclient.CreateServiceClients(fetchLog.config()), "")
	if len(page.Resources) == 0 {
		t.Fatalf("the app's fetcher listed no row from the synthetic answers (error: %v), so its later requests are never sent", err)
	}
	later := awsclient.CreateServiceClients(laterLog.config())
	if detail := resource.GetDetailEnricher(name); detail != nil {
		for _, r := range page.Resources {
			_, _ = detail(ctx, later, r) //nolint:errcheck // a failed call is in the log
		}
	}
	e, ok := awsclient.Wave2EnricherFor(name)
	if !ok {
		return fetchLog, laterLog
	}
	cache := resource.ResourceCache{name: {Resources: page.Resources}}
	seed := awsclient.CreateServiceClients(newRequestLog().config())
	for _, r := range e.Reads {
		if f := resource.GetPaginatedFetcher(r); f != nil {
			p, _ := f(ctx, seed, "") //nolint:errcheck // a seed that fails leaves the read empty, as a denied list does
			cache[r] = resource.ResourceCacheEntry{Resources: p.Resources}
		}
	}
	_, _ = e.Fn(ctx, later, page.Resources, cache) //nolint:errcheck // a failed call is in the log
	return fetchLog, laterLog
}

// The collector is the oracle the web e2e checks compare the app against, so
// for every type it asks AWS every question the app asks for that type, and
// no other: the set of (service, operation, input without page size and page
// token) its capture sends equals the one the app's fetcher sends plus the
// requests the app's Wave-2 and detail enrichers send for the fetched rows.
// A question is one request however often it is sent: the fetcher and Wave 2
// may each ask it, and AWS answers both the same. An app operation the
// collector does not send at all is declared in notCaptured with its reason;
// one that is declared there and sent, or declared for no operation the app
// sends, fails. An operation the collector does send must be sent with the
// inputs the app sends it, but for a request declared in
// declaredRequestDifferences.
func TestT11a_CollectorSendsEveryRequestTheAppSendsForEveryTopLevelType(t *testing.T) {
	awsclient.Install()
	used := map[string]bool{}
	for _, td := range catalog.All() {
		name := td.ShortName
		t.Run(name, func(t *testing.T) {
			capture, ok := captures[name]
			if !ok {
				t.Fatalf("no capture is registered for top-level type %q", name)
			}
			if reason, ok := declaredRequestDifferences[name]; ok {
				used[name] = true
				if reason == "" {
					t.Fatalf("%s is declared with no reason", name)
				}
				t.Skipf("compared by first request only: %s", reason)
			}

			fetchLog, laterLog := appRequests(t, name)
			colLog := newRequestLog()
			_, colErr := capture(context.Background(), colLog.config())

			for _, l := range []*requestLog{fetchLog, laterLog, colLog} {
				if len(l.unmatched) > 0 {
					t.Fatalf("the answerer has no output type for %v", l.unmatched)
				}
				if l.n > callBudget {
					t.Fatalf("a run sent more than %d requests; a walk does not end", callBudget)
				}
			}

			app := map[string]bool{}
			for _, l := range []*requestLog{fetchLog, laterLog} {
				for k := range l.calls {
					app[k] = true
				}
			}
			col := map[string]bool{}
			for k := range colLog.calls {
				col[k] = true
			}
			for key, reason := range declaredRequestDifferences {
				if req, ok := strings.CutPrefix(key, name+" "); ok {
					used[key] = true
					if reason == "" {
						t.Errorf("%s is declared with no reason", key)
					}
					if !app[req] && !col[req] {
						t.Errorf("declared difference %q is a request neither side sends", key)
					}
					delete(app, req)
					delete(col, req)
				}
			}

			var undeclared []string
			for op := range fetchLog.ops {
				laterLog.ops[op] = true
			}
			for op := range laterLog.ops {
				key := name + " " + op
				reason, declared := notCaptured[key]
				switch {
				case colLog.ops[op] && declared:
					used[key] = true
					t.Errorf("%s is declared not captured, and the collector sends it", key)
				case colLog.ops[op]:
				case !declared:
					undeclared = append(undeclared, op)
				case reason == "":
					used[key] = true
					t.Errorf("%s is declared with no reason", key)
				default:
					used[key] = true
					for k := range app {
						if strings.HasPrefix(k, op+" ") {
							delete(app, k)
						}
					}
				}
			}
			slices.Sort(undeclared)
			for _, op := range undeclared {
				t.Errorf("the app sends %s and the collector does not; capture it or declare %q in notCaptured", op, name+" "+op)
			}

			var onlyApp, onlyCol []string
			for k := range app {
				if !col[k] {
					onlyApp = append(onlyApp, k)
				}
			}
			for k := range col {
				if !app[k] {
					onlyCol = append(onlyCol, k)
				}
			}
			slices.Sort(onlyApp)
			slices.Sort(onlyCol)
			if len(onlyApp)+len(onlyCol) > 0 {
				t.Errorf("requests differ (collector error: %v)\n  app only:\n    %s\n  collector only:\n    %s",
					colErr, strings.Join(onlyApp, "\n    "), strings.Join(onlyCol, "\n    "))
			}
		})
	}
	for key := range declaredRequestDifferences {
		if !used[key] {
			t.Errorf("declared difference %q matches no top-level type", key)
		}
	}
	for key := range notCaptured {
		if !used[key] {
			t.Errorf("notCaptured %q matches no operation the app sends for a top-level type", key)
		}
	}
}
