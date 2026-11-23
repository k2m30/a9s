package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	v2types "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	cttypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	"github.com/aws/aws-sdk-go-v2/service/codeartifact"
	catypes "github.com/aws/aws-sdk-go-v2/service/codeartifact/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
	"github.com/aws/aws-sdk-go-v2/service/mwaa"
	mwaatypes "github.com/aws/aws-sdk-go-v2/service/mwaa/types"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	sfntypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"
	"github.com/aws/aws-sdk-go-v2/service/wafv2"
	wafv2types "github.com/aws/aws-sdk-go-v2/service/wafv2/types"
)

var errT11aThrottled = t11aAPIError("ThrottlingException", "Rate exceeded")

func t11aJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// composedFailures answers the five types whose rows come from more than one
// list call, with one source of each refused: the REST APIs, the rules of one
// of two event buses, the CLOUDFRONT-scope web ACLs, and the DocumentDB
// clusters and cluster snapshots.
func composedFailures(service, op string, in any) (any, error) {
	refused := func() (any, error) {
		return nil, t11aAPIError("AccessDeniedException", "User: arn:aws:iam::123456789012:user/example-readonly is not authorized to perform: "+op)
	}
	switch {
	case service == "API Gateway" && op == "GetRestApis",
		service == "DocDB" && (op == "DescribeDBClusters" || op == "DescribeDBClusterSnapshots"):
		return refused()
	case op == "ListEventBuses":
		return &eventbridge.ListEventBusesOutput{EventBuses: []ebtypes.EventBus{
			{Name: aws.String("default"), Arn: aws.String("arn:aws:events:eu-west-1:123456789012:event-bus/default")},
			{Name: aws.String("orders"), Arn: aws.String("arn:aws:events:eu-west-1:123456789012:event-bus/orders")},
		}}, nil
	case op == "ListRules" && aws.ToString(in.(*eventbridge.ListRulesInput).EventBusName) == "orders":
		return refused()
	case op == "ListWebACLs" && in.(*wafv2.ListWebACLsInput).Scope == wafv2types.ScopeCloudfront:
		return refused()
	}
	return t11aDefault(service, in)
}

var t11aComposedTypes = []string{"apigw", "eb-rule", "waf", "dbc", "dbc-snap"}

// A run that lost a source of a type has lost that source's rows: the oracle
// the web e2e checks compare against is then shorter than the app's list for
// a reason the run must state. main reports it as it reports a failed type: a
// stderr line naming the type and the refusal, and a count in the summary;
// the rows the other sources returned are still written.
func TestT11a_MainReportsAFailedSourceOfAComposedType(t *testing.T) {
	if out := os.Getenv("T11A_MAIN_OUT"); out != "" {
		for _, name := range t11aComposedTypes {
			c := captures[name]
			captures[name] = func(ctx context.Context, _ aws.Config) (any, error) {
				return c(ctx, answeringConfig(composedFailures))
			}
		}
		os.Args = []string{"snapshot", "--profile", "example-readonly", "--region", "eu-west-1", "--out", out, "--types", strings.Join(t11aComposedTypes, ",")}
		main()
		return
	}

	dir := t.TempDir()
	conf := filepath.Join(dir, "config")
	if err := os.WriteFile(conf, []byte("[profile example-readonly]\nregion = eu-west-1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out")
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestT11a_MainReportsAFailedSourceOfAComposedType$", "-test.count=1") //nolint:gosec // re-runs this test binary
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "AWS_") {
			cmd.Env = append(cmd.Env, kv)
		}
	}
	cmd.Env = append(cmd.Env, "T11A_MAIN_OUT="+out, "AWS_CONFIG_FILE="+conf, "AWS_SHARED_CREDENTIALS_FILE="+filepath.Join(dir, "credentials"), "AWS_EC2_METADATA_DISABLED=true")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("snapshot run: %v\nstdout:\n%s\nstderr:\n%s", err, &stdout, &stderr)
	}

	lines := map[string]string{}
	sc := bufio.NewScanner(&stderr)
	for sc.Scan() {
		if f := strings.Fields(sc.Text()); len(f) > 0 {
			lines[f[0]] = sc.Text()
		}
	}
	for _, name := range t11aComposedTypes {
		line := lines[name]
		if !strings.Contains(line, "AccessDenied") || strings.Contains(line, " ok (") {
			t.Errorf("%s: stderr line %q, want it to report the refused source (AccessDenied), not ok", name, line)
		}
	}
	if want := fmt.Sprintf("(%d types, %d errors)", len(t11aComposedTypes), len(t11aComposedTypes)); !strings.Contains(stdout.String(), want) {
		t.Errorf("summary %q, want %q", strings.TrimSpace(stdout.String()), want)
	}

	files, _ := filepath.Glob(filepath.Join(out, "*", "snapshot.json")) //nolint:errcheck // a malformed pattern is a test bug
	if len(files) != 1 {
		t.Fatalf("snapshot files = %v, want one", files)
	}
	b, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	var snap struct {
		Types map[string]json.RawMessage `json:"types"`
	}
	if err := json.Unmarshal(b, &snap); err != nil {
		t.Fatal(err)
	}
	surviving := map[string][3]string{
		"apigw":    {"v2_apis", "", ""},
		"eb-rule":  {"rules", "", ""},
		"waf":      {"web_acls", "scope", "REGIONAL"},
		"dbc":      {"clusters", "source", "rds"},
		"dbc-snap": {"snapshots", "source", "rds"},
	}
	for _, name := range t11aComposedTypes {
		var section map[string]any
		if err := json.Unmarshal(snap.Types[name], &section); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		want := surviving[name]
		rows, _ := section[want[0]].([]any)
		if len(rows) == 0 {
			t.Errorf("%s: no %s row of the source that answered is written", name, want[0])
		}
		for _, r := range rows {
			if row, _ := r.(map[string]any); want[1] != "" && row[want[1]] != want[2] {
				t.Errorf("%s: row %v, want every %s %q (the source that answered)", name, row, want[1], want[2])
			}
		}
	}
}

// A list walk that fails on a later page has read the earlier ones: the app
// keeps those rows and reports the failure (core/aws/waf.go, eb_rule.go,
// apigw.go read through PageAll), so the oracle holds the same rows and
// records the failure.
func TestT11a_APageReadBeforeAFailedPageKeepsItsRows(t *testing.T) {
	cases := []struct {
		name    string
		capture captureFunc
		answer  answerFunc
		row     string
	}{
		{"waf ListWebACLs", captureWAF, func(service, op string, in any) (any, error) {
			if op == "ListWebACLs" {
				l := in.(*wafv2.ListWebACLsInput)
				switch {
				case l.Scope == wafv2types.ScopeCloudfront:
					return &wafv2.ListWebACLsOutput{}, nil
				case l.NextMarker == nil:
					return &wafv2.ListWebACLsOutput{WebACLs: []wafv2types.WebACLSummary{{Name: aws.String("edge-acl-a"), Id: aws.String("a1b2c3d4-5678-90ab-cdef-EXAMPLE11111"),
						ARN: aws.String("arn:aws:wafv2:eu-west-1:123456789012:regional/webacl/edge-acl-a/a1b2c3d4-5678-90ab-cdef-EXAMPLE11111")}}, NextMarker: aws.String("acl-page-2")}, nil
				}
				return nil, errT11aThrottled
			}
			return t11aDefault(service, in)
		}, "edge-acl-a"},
		{"apigw GetApis", captureAPIGW, func(service, op string, in any) (any, error) {
			switch {
			case service == "API Gateway":
				return t11aDefault(service, in)
			case op == "GetApis" && in.(*apigatewayv2.GetApisInput).NextToken == nil:
				return &apigatewayv2.GetApisOutput{Items: []v2types.Api{{ApiId: aws.String("a1b2c3d4e5"), Name: aws.String("orders-http"), ProtocolType: v2types.ProtocolTypeHttp}},
					NextToken: aws.String("apis-page-2")}, nil
			case op == "GetApis":
				return nil, errT11aThrottled
			}
			return t11aDefault(service, in)
		}, "orders-http"},
		{"eb-rule ListRules", captureEBRule, func(service, op string, in any) (any, error) {
			switch op {
			case "ListEventBuses":
				return &eventbridge.ListEventBusesOutput{EventBuses: []ebtypes.EventBus{{Name: aws.String("default")}}}, nil
			case "ListRules":
				if in.(*eventbridge.ListRulesInput).NextToken == nil {
					return &eventbridge.ListRulesOutput{Rules: []ebtypes.Rule{{Name: aws.String("nightly-report"), Arn: aws.String("arn:aws:events:eu-west-1:123456789012:rule/nightly-report"),
						State: ebtypes.RuleStateEnabled, EventBusName: aws.String("default")}}, NextToken: aws.String("rules-page-2")}, nil
				}
				return nil, errT11aThrottled
			}
			return t11aDefault(service, in)
		}, "nightly-report"},
		{"eb-rule ListEventBuses", captureEBRule, func(service, op string, in any) (any, error) {
			switch op {
			case "ListEventBuses":
				if in.(*eventbridge.ListEventBusesInput).NextToken == nil {
					return &eventbridge.ListEventBusesOutput{EventBuses: []ebtypes.EventBus{{Name: aws.String("orders")}}, NextToken: aws.String("buses-page-2")}, nil
				}
				return nil, errT11aThrottled
			case "ListRules":
				return &eventbridge.ListRulesOutput{Rules: []ebtypes.Rule{{Name: aws.String("order-created"), Arn: aws.String("arn:aws:events:eu-west-1:123456789012:rule/orders/order-created"),
					State: ebtypes.RuleStateEnabled, EventBusName: aws.String("orders")}}}, nil
			}
			return t11aDefault(service, in)
		}, "order-created"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			data, err := c.capture(context.Background(), answeringConfig(c.answer))
			if err != nil {
				t.Fatalf("capture failed as a whole: %v", err)
			}
			got := t11aJSON(t, data)
			if !strings.Contains(got, `"`+c.row+`"`) {
				t.Errorf("the page-1 row %s is missing: %s", c.row, got)
			}
			if !strings.Contains(got, "ThrottlingException") {
				t.Errorf("the failed page is not recorded: %s", got)
			}
		})
	}
}

// ListRules answers ResourceNotFoundException for a bus deleted after
// ListEventBuses named it; the app skips that bus without a failure
// (core/aws/eb_rule.go ebRulesOnBuses), so the oracle records no failure for
// it and keeps the other buses' rules, and a gone bus that was the only one
// is an empty list, not an error.
func TestT11a_CaptureEBRule_ABusDeletedAfterTheListIsSkipped(t *testing.T) {
	answer := func(buses ...string) answerFunc {
		return func(service, op string, in any) (any, error) {
			switch op {
			case "ListEventBuses":
				out := &eventbridge.ListEventBusesOutput{}
				for _, b := range buses {
					out.EventBuses = append(out.EventBuses, ebtypes.EventBus{Name: aws.String(b)})
				}
				return out, nil
			case "ListRules":
				if aws.ToString(in.(*eventbridge.ListRulesInput).EventBusName) == "retired" {
					return nil, t11aAPIError("ResourceNotFoundException", "Event bus retired does not exist.")
				}
				return &eventbridge.ListRulesOutput{Rules: []ebtypes.Rule{{Name: aws.String("nightly-report"), State: ebtypes.RuleStateEnabled, EventBusName: aws.String("default")}}}, nil
			}
			return t11aDefault(service, in)
		}
	}

	data, err := captureEBRule(context.Background(), answeringConfig(answer("default", "retired")))
	if err != nil {
		t.Fatalf("capture: %v", err)
	}
	got := data.(ebRuleData)
	if len(got.Rules) != 1 || got.Rules[0].Name != "nightly-report" || len(got.PartialErrors) != 0 {
		t.Errorf("rules %+v, partial errors %v; want the default bus's rule and no failure", got.Rules, got.PartialErrors)
	}

	data, err = captureEBRule(context.Background(), answeringConfig(answer("retired")))
	if err != nil {
		t.Fatalf("the only bus deleted after the list: capture error %v, want an empty list", err)
	}
	if got := data.(ebRuleData); len(got.Rules) != 0 || len(got.PartialErrors) != 0 {
		t.Errorf("rules %+v, partial errors %v; want none", got.Rules, got.PartialErrors)
	}
}

// The newest ctEventsCap events are kept; the list is truncated only when
// CloudTrail has more after them: another event on the page, or another page.
func TestT11a_CaptureCTEvents_TruncatedOnlyWhenMoreEventsExist(t *testing.T) {
	events := func(n int) []cttypes.Event {
		out := make([]cttypes.Event, n)
		for i := range out {
			out[i] = cttypes.Event{EventId: aws.String(fmt.Sprintf("0f1e2d3c-4b5a-6978-8a9b-%012d", i)), EventName: aws.String("DescribeInstances"),
				EventSource: aws.String("ec2.amazonaws.com"), EventTime: aws.Time(time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC).Add(-time.Duration(i) * time.Minute))}
		}
		return out
	}
	cases := []struct {
		name      string
		page      []cttypes.Event
		next      *string
		truncated bool
	}{
		{"exactly the cap on the last page", events(ctEventsCap), nil, false},
		{"one more than the cap", events(ctEventsCap + 1), nil, true},
		{"the cap and another page", events(ctEventsCap), aws.String("events-page-2"), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := answeringConfig(func(service, op string, in any) (any, error) {
				if op == "LookupEvents" {
					if in.(*cloudtrail.LookupEventsInput).NextToken == nil {
						return &cloudtrail.LookupEventsOutput{Events: c.page, NextToken: c.next}, nil
					}
					return &cloudtrail.LookupEventsOutput{Events: events(1)}, nil
				}
				return t11aDefault(service, in)
			})
			data, err := captureCTEvents(context.Background(), cfg)
			if err != nil {
				t.Fatal(err)
			}
			got := data.(ctEventsData)
			if len(got.Events) != ctEventsCap || got.Truncated != c.truncated {
				t.Errorf("events %d truncated %v, want %d truncated %v", len(got.Events), got.Truncated, ctEventsCap, c.truncated)
			}
		})
	}
}

// Step Functions does not list the executions of an EXPRESS state machine
// (ListExecutions answers StateMachineTypeNotSupported); the app does not ask
// (core/aws/sfn_issue_enrichment.go), so the collector does not ask either and
// records no failure for it.
func TestT11a_CaptureSFN_AnExpressMachineIsNotAskedForExecutions(t *testing.T) {
	const express = "arn:aws:states:eu-west-1:123456789012:stateMachine:ingest-express"
	const standard = "arn:aws:states:eu-west-1:123456789012:stateMachine:billing-standard"
	var asked []string
	cfg := answeringConfig(func(service, op string, in any) (any, error) {
		switch op {
		case "ListStateMachines":
			return &sfn.ListStateMachinesOutput{StateMachines: []sfntypes.StateMachineListItem{
				{Name: aws.String("ingest-express"), StateMachineArn: aws.String(express), Type: sfntypes.StateMachineTypeExpress, CreationDate: aws.Time(t11aIAMTime)},
				{Name: aws.String("billing-standard"), StateMachineArn: aws.String(standard), Type: sfntypes.StateMachineTypeStandard, CreationDate: aws.Time(t11aIAMTime)},
			}}, nil
		case "ListExecutions":
			arn := aws.ToString(in.(*sfn.ListExecutionsInput).StateMachineArn)
			asked = append(asked, arn)
			if arn == express {
				return nil, t11aAPIError("StateMachineTypeNotSupported", "This operation is not supported by this type of state machine")
			}
			return &sfn.ListExecutionsOutput{Executions: []sfntypes.ExecutionListItem{{ExecutionArn: aws.String("arn:aws:states:eu-west-1:123456789012:execution:billing-standard:run-42"),
				StateMachineArn: aws.String(standard), Name: aws.String("run-42"), Status: sfntypes.ExecutionStatusTimedOut, StartDate: aws.Time(t11aIAMTime)}}}, nil
		}
		return t11aDefault(service, in)
	})
	data, err := captureSFN(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(asked, express) {
		t.Errorf("ListExecutions was sent for the EXPRESS machine; asked %v", asked)
	}
	got := t11aJSON(t, data)
	if strings.Contains(got, "StateMachineTypeNotSupported") {
		t.Errorf("the EXPRESS machine records a failure: %s", got)
	}
	if !slices.Contains(asked, standard) || !strings.Contains(got, "run-42") || !strings.Contains(got, string(sfntypes.ExecutionStatusTimedOut)) {
		t.Errorf("the STANDARD machine's newest execution (run-42, TIMED_OUT) is not recorded; asked %v, snapshot %s", asked, got)
	}
}

// ltAnswer answers one launch template whose $Default version holds data,
// with the Region's EBS encryption-by-default setting and the AMI's IMDS
// support as given.
func ltAnswer(data ec2types.ResponseLaunchTemplateData, encryptByDefault bool, imdsSupport ec2types.ImdsSupportValues) answerFunc {
	return func(service, op string, in any) (any, error) {
		switch op {
		case "DescribeLaunchTemplates":
			return &ec2.DescribeLaunchTemplatesOutput{LaunchTemplates: []ec2types.LaunchTemplate{{LaunchTemplateId: aws.String("lt-0abc123def4567890"),
				LaunchTemplateName: aws.String("web"), DefaultVersionNumber: aws.Int64(1), LatestVersionNumber: aws.Int64(1)}}}, nil
		case "DescribeLaunchTemplateVersions":
			return &ec2.DescribeLaunchTemplateVersionsOutput{LaunchTemplateVersions: []ec2types.LaunchTemplateVersion{{LaunchTemplateId: aws.String("lt-0abc123def4567890"),
				VersionNumber: aws.Int64(1), DefaultVersion: aws.Bool(true), LaunchTemplateData: &data}}}, nil
		case "GetEbsEncryptionByDefault":
			return &ec2.GetEbsEncryptionByDefaultOutput{EbsEncryptionByDefault: aws.Bool(encryptByDefault)}, nil
		case "DescribeImages":
			return &ec2.DescribeImagesOutput{Images: []ec2types.Image{{ImageId: aws.String("ami-0abcdef1234567890"), ImdsSupport: imdsSupport}}}, nil
		}
		return t11aDefault(service, in)
	}
}

// The app judges a launch template on facts a block device mapping and the
// metadata options do not state alone (core/aws/lt_issue_enrichment.go): an
// unset Encrypted is the Region's default, not "off", and the Region's
// GetEbsEncryptionByDefault cancels an explicit false; unset HttpTokens takes
// the AMI's ImdsSupport (DescribeImages), and HttpEndpoint disabled turns the
// metadata service off. Each pair below is two states of AWS the app tells
// apart, so the snapshot records them differently.
func TestT11a_CaptureLT_RecordsWhatTheAppJudgesATemplateOn(t *testing.T) {
	ebs := func(enc *bool) ec2types.ResponseLaunchTemplateData {
		return ec2types.ResponseLaunchTemplateData{ImageId: aws.String("ami-0abcdef1234567890"),
			MetadataOptions: &ec2types.LaunchTemplateInstanceMetadataOptions{HttpTokens: ec2types.LaunchTemplateHttpTokensStateRequired},
			BlockDeviceMappings: []ec2types.LaunchTemplateBlockDeviceMapping{{DeviceName: aws.String("/dev/xvda"),
				Ebs: &ec2types.LaunchTemplateEbsBlockDevice{VolumeSize: aws.Int32(30), VolumeType: ec2types.VolumeTypeGp3, Encrypted: enc}}}}
	}
	imds := func(tokens ec2types.LaunchTemplateHttpTokensState, endpoint ec2types.LaunchTemplateInstanceMetadataEndpointState) ec2types.ResponseLaunchTemplateData {
		return ec2types.ResponseLaunchTemplateData{ImageId: aws.String("ami-0abcdef1234567890"),
			MetadataOptions: &ec2types.LaunchTemplateInstanceMetadataOptions{HttpTokens: tokens, HttpEndpoint: endpoint}}
	}
	snap := func(a answerFunc) string {
		data, err := captureLT(context.Background(), answeringConfig(a))
		if err != nil {
			t.Fatal(err)
		}
		return t11aJSON(t, data)
	}

	unset := snap(ltAnswer(ebs(nil), false, ""))
	if strings.Contains(unset, `"has_unencrypted_ebs":true`) {
		t.Errorf("a volume with no Encrypted value is recorded as unencrypted: %s", unset)
	}
	pairs := []struct {
		name string
		a, b answerFunc
	}{
		{"Encrypted unset vs false", ltAnswer(ebs(nil), false, ""), ltAnswer(ebs(aws.Bool(false)), false, "")},
		{"Encrypted false, Region default on vs off", ltAnswer(ebs(aws.Bool(false)), true, ""), ltAnswer(ebs(aws.Bool(false)), false, "")},
		{"HttpTokens unset, AMI ImdsSupport v2.0 vs unset", ltAnswer(imds("", ""), false, ec2types.ImdsSupportValuesV20), ltAnswer(imds("", ""), false, "")},
		{"HttpTokens optional, HttpEndpoint disabled vs enabled",
			ltAnswer(imds(ec2types.LaunchTemplateHttpTokensStateOptional, ec2types.LaunchTemplateInstanceMetadataEndpointStateDisabled), false, ""),
			ltAnswer(imds(ec2types.LaunchTemplateHttpTokensStateOptional, ec2types.LaunchTemplateInstanceMetadataEndpointStateEnabled), false, "")},
	}
	for _, p := range pairs {
		if a, b := snap(p.a), snap(p.b); a == b {
			t.Errorf("%s: the snapshot is the same for both: %s", p.name, a)
		}
	}
}

// GetEnvironment answers every field the app's mwaa list shows
// (.a9s/views/mwaa.yaml: Airflow version, environment class, max workers,
// schedulers); a change in any of them changes the snapshot.
func TestT11a_CaptureMWAA_RecordsTheListColumns(t *testing.T) {
	base := mwaatypes.Environment{Name: aws.String("airflow-prod"), Arn: aws.String("arn:aws:airflow:eu-west-1:123456789012:environment/airflow-prod"),
		Status: mwaatypes.EnvironmentStatusAvailable, AirflowVersion: aws.String("2.10.3"), EnvironmentClass: aws.String("mw1.small"),
		MaxWorkers: aws.Int32(10), Schedulers: aws.Int32(2)}
	snap := func(env mwaatypes.Environment) string {
		data, err := captureMWAA(context.Background(), answeringConfig(func(service, op string, in any) (any, error) {
			switch op {
			case "ListEnvironments":
				return &mwaa.ListEnvironmentsOutput{Environments: []string{"airflow-prod"}}, nil
			case "GetEnvironment":
				return &mwaa.GetEnvironmentOutput{Environment: &env}, nil
			}
			return t11aDefault(service, in)
		}))
		if err != nil {
			t.Fatal(err)
		}
		return t11aJSON(t, data)
	}
	want := snap(base)
	for name, change := range map[string]func(*mwaatypes.Environment){
		"AirflowVersion":   func(e *mwaatypes.Environment) { e.AirflowVersion = aws.String("2.9.2") },
		"EnvironmentClass": func(e *mwaatypes.Environment) { e.EnvironmentClass = aws.String("mw1.large") },
		"MaxWorkers":       func(e *mwaatypes.Environment) { e.MaxWorkers = aws.Int32(25) },
		"Schedulers":       func(e *mwaatypes.Environment) { e.Schedulers = aws.Int32(3) },
	} {
		env := base
		change(&env)
		if snap(env) == want {
			t.Errorf("%s changed and the snapshot did not: %s", name, want)
		}
	}
}

// ListPackages pages through a repository's packages; the app walks every
// page and shows the exact count (core/aws/codeartifact_issue_enrichment.go),
// so a repository with one package and one with two (on two pages) are
// recorded differently.
func TestT11a_CaptureCodeArtifact_CountsThePackagesOfEveryPage(t *testing.T) {
	snap := func(pages int) string {
		data, err := captureCodeArtifact(context.Background(), answeringConfig(func(service, op string, in any) (any, error) {
			switch op {
			case "ListRepositories":
				return &codeartifact.ListRepositoriesOutput{Repositories: []catypes.RepositorySummary{{Name: aws.String("npm-internal"), DomainName: aws.String("acme"),
					DomainOwner: aws.String("123456789012"), Arn: aws.String("arn:aws:codeartifact:eu-west-1:123456789012:repository/acme/npm-internal")}}}, nil
			case "ListPackages":
				pkg := []catypes.PackageSummary{{Package: aws.String("left-pad"), Format: catypes.PackageFormatNpm}}
				if in.(*codeartifact.ListPackagesInput).NextToken == nil && pages == 2 {
					return &codeartifact.ListPackagesOutput{Packages: pkg, NextToken: aws.String("packages-page-2")}, nil
				}
				return &codeartifact.ListPackagesOutput{Packages: pkg}, nil
			}
			return t11aDefault(service, in)
		}))
		if err != nil {
			t.Fatal(err)
		}
		return t11aJSON(t, data)
	}
	if one, two := snap(1), snap(2); one == two {
		t.Errorf("one package and two packages are recorded the same: %s", one)
	}
}
