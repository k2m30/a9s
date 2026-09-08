package unit

// partial_answer_test.go — the rule the repo already states in
// core/aws/issue_enrichment.go: a row that could not be inspected renders "?",
// never clean. Nine sites break it in their own way — a cap that drops targets
// without marking them, an error that discards a sibling check's success, a
// missing datum that defaults to the clean value, a partial page cached as a
// complete one. Each test drives the real enricher or fetcher and asserts the
// consequence the operator sees.

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	backupsvc "github.com/aws/aws-sdk-go-v2/service/backup"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"
	ec2svc "github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ecrsvc "github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	lambdasvc "github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	redshiftsvc "github.com/aws/aws-sdk-go-v2/service/redshift"
	redshifttypes "github.com/aws/aws-sdk-go-v2/service/redshift/types"
	smithy "github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/cache"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/session"
)

// partialAccessDenied is the error AWS returns when the caller's policy does
// not allow the read. It is neither a not-found nor a throttle, so every
// enricher under test reaches it through its generic error path — the path
// that must leave the row uninspected rather than clean.
func partialAccessDenied() error {
	return &smithy.GenericAPIError{
		Code:    "AccessDeniedException",
		Message: "User is not authorized to perform this operation",
		Fault:   smithy.FaultClient,
	}
}

// partialAssertUninspected pins the shared rule: the row could not be answered,
// so it must carry the per-row truncation marker the list renders as "?".
func partialAssertUninspected(t *testing.T, res awsclient.IssueEnricherResult, id string) {
	t.Helper()
	if !res.TruncatedIDs[id] {
		t.Errorf("%s is not in TruncatedIDs; a check that could not be run renders the row clean instead of \"?\"", id)
	}
}

func partialAssertInspected(t *testing.T, res awsclient.IssueEnricherResult, id string) {
	t.Helper()
	if res.TruncatedIDs[id] {
		t.Errorf("%s is in TruncatedIDs; every check on it answered, so the row is not a coverage gap", id)
	}
}

// ───────────────────────────────────────────────────────────────────────────
// row 1 — lambda: two independent checks, one failure
// ───────────────────────────────────────────────────────────────────────────

const (
	partialLambdaFn = "acme-webhook"

	// A resource policy that lets anyone on the internet invoke the function.
	partialLambdaPublicPolicy = `{"Version":"2012-10-17","Id":"default","Statement":[` +
		`{"Sid":"AllowPublicInvoke","Effect":"Allow","Principal":"*",` +
		`"Action":"lambda:InvokeFunction",` +
		`"Resource":"arn:aws:lambda:us-east-1:123456789012:function:acme-webhook"}]}`
)

// partialLambdaFake answers the two independent posture reads separately, so a
// test can fail one and leave the other correct.
type partialLambdaFake struct {
	awsclient.LambdaAPI

	policy    string
	policyErr error
	urlAuth   lambdatypes.FunctionUrlAuthType
	hasURL    bool
	urlErr    error
}

func (f *partialLambdaFake) GetPolicy(_ context.Context, _ *lambdasvc.GetPolicyInput, _ ...func(*lambdasvc.Options)) (*lambdasvc.GetPolicyOutput, error) {
	if f.policyErr != nil {
		return nil, f.policyErr
	}
	if f.policy == "" {
		return nil, &smithy.GenericAPIError{Code: "ResourceNotFoundException", Message: "The resource you requested does not exist."}
	}
	return &lambdasvc.GetPolicyOutput{Policy: aws.String(f.policy), RevisionId: aws.String("a1b2c3d4-1111-2222-3333-444455556666")}, nil
}

func (f *partialLambdaFake) ListFunctionUrlConfigs(_ context.Context, _ *lambdasvc.ListFunctionUrlConfigsInput, _ ...func(*lambdasvc.Options)) (*lambdasvc.ListFunctionUrlConfigsOutput, error) {
	if f.urlErr != nil {
		return nil, f.urlErr
	}
	if !f.hasURL {
		return &lambdasvc.ListFunctionUrlConfigsOutput{}, nil
	}
	return &lambdasvc.ListFunctionUrlConfigsOutput{FunctionUrlConfigs: []lambdatypes.FunctionUrlConfig{{
		FunctionArn:  aws.String("arn:aws:lambda:us-east-1:123456789012:function:" + partialLambdaFn),
		FunctionUrl:  aws.String("https://abcdefghij1234567890.lambda-url.us-east-1.on.aws/"),
		AuthType:     f.urlAuth,
		CreationTime: aws.String("2026-01-14T09:12:33.000Z"),
	}}}, nil
}

func partialEnrichLambda(t *testing.T, fake *partialLambdaFake) awsclient.IssueEnricherResult {
	t.Helper()
	store := session.NewIdentityStore()
	store.Set("123456789012", nil)
	clients := &awsclient.ServiceClients{Lambda: fake}
	clients.SetIdentityStore(store)
	res, _ := awsclient.EnrichLambdaPosture(context.Background(), clients,
		[]resource.Resource{w2Res(partialLambdaFn, nil)}, nil)
	w2AssertEnricherShape(t, res)
	return res
}

// TestPartialLambda_OneFailedCheckKeepsTheOtherOne pins that the resource
// policy and the function URL are two separate questions. Losing the answer to
// one says nothing about the other, so the check that did answer keeps its
// finding and only the check that failed leaves the row uninspected.
func TestPartialLambda_OneFailedCheckKeepsTheOtherOne(t *testing.T) {
	t.Run("url read denied, public policy still reported", func(t *testing.T) {
		res := partialEnrichLambda(t, &partialLambdaFake{
			policy: partialLambdaPublicPolicy,
			urlErr: partialAccessDenied(),
		})
		w4AssertFinding(t, res.Findings[partialLambdaFn], "lambda.public-policy",
			"invokable by anyone", domain.SevBroken, "wave2")
		partialAssertUninspected(t, res, partialLambdaFn)
	})

	t.Run("policy read denied, open function url still reported", func(t *testing.T) {
		res := partialEnrichLambda(t, &partialLambdaFake{
			policyErr: partialAccessDenied(),
			hasURL:    true,
			urlAuth:   lambdatypes.FunctionUrlAuthTypeNone,
		})
		w4AssertFinding(t, res.Findings[partialLambdaFn], "lambda.function-url-public",
			"function endpoint open without authentication", domain.SevBroken, "wave2")
		partialAssertUninspected(t, res, partialLambdaFn)
	})

	t.Run("both checks answer and the function is private", func(t *testing.T) {
		res := partialEnrichLambda(t, &partialLambdaFake{
			hasURL:  true,
			urlAuth: lambdatypes.FunctionUrlAuthTypeAwsIam,
		})
		w4AssertNoCode(t, res.Findings[partialLambdaFn], "lambda.public-policy")
		w4AssertNoCode(t, res.Findings[partialLambdaFn], "lambda.function-url-public")
		partialAssertInspected(t, res, partialLambdaFn)
	})
}

// ───────────────────────────────────────────────────────────────────────────
// row 2 — eks: an unread version catalogue is unknown, not standard
// ───────────────────────────────────────────────────────────────────────────

// The third word of the version-support vocabulary. "standard" is a claim
// about what AWS says, and a claim can only be made from an answer AWS gave.
const partialEKSSupportUnknown = "unknown"

func partialEKSVersionSupport(t *testing.T, fake *w6bEKSFake, name string) string {
	t.Helper()
	rs := w6bFetchEKS(t, fake)
	return pw1ResourceByID(t, rs, name).Fields["version_support"]
}

// TestPartialEKS_SupportIsUnknownUntilTheCatalogueSaysOtherwise pins that the
// support verdict is only ever read out of a catalogue entry. A cluster whose
// minor AWS no longer publishes, or a catalogue call that failed outright, is
// a version a9s knows nothing about — and "standard" is the one thing it
// cannot be, because that is the answer an operator acts on by doing nothing.
func TestPartialEKS_SupportIsUnknownUntilTheCatalogueSaysOtherwise(t *testing.T) {
	const name = "acme-platform"

	t.Run("catalogue read fails", func(t *testing.T) {
		got := partialEKSVersionSupport(t, &w6bEKSFake{
			order:       []string{name},
			clusters:    map[string]*ekstypes.Cluster{name: w6bEKSCluster(name, "1.28")},
			versionsErr: partialAccessDenied(),
		}, name)
		if got != partialEKSSupportUnknown {
			t.Errorf("version_support = %q, want %q: no catalogue was read, so nothing is known about 1.28", got, partialEKSSupportUnknown)
		}
	})

	t.Run("version absent from the catalogue", func(t *testing.T) {
		got := partialEKSVersionSupport(t, &w6bEKSFake{
			order:    []string{name},
			clusters: map[string]*ekstypes.Cluster{name: w6bEKSCluster(name, "1.21")},
			versions: w6bEKSSupportedVersions(),
		}, name)
		if got != partialEKSSupportUnknown {
			t.Errorf("version_support = %q, want %q: the catalogue has no entry for 1.21", got, partialEKSSupportUnknown)
		}
	})

	t.Run("catalogue says standard support", func(t *testing.T) {
		got := partialEKSVersionSupport(t, &w6bEKSFake{
			order:    []string{name},
			clusters: map[string]*ekstypes.Cluster{name: w6bEKSCluster(name, "1.33")},
			versions: w6bEKSSupportedVersions(),
		}, name)
		if got != "standard" {
			t.Errorf("version_support = %q, want %q: the catalogue reports 1.33 on standard support", got, "standard")
		}
	})

	t.Run("catalogue says extended support", func(t *testing.T) {
		got := partialEKSVersionSupport(t, &w6bEKSFake{
			order:    []string{name},
			clusters: map[string]*ekstypes.Cluster{name: w6bEKSCluster(name, "1.28")},
			versions: w6bEKSSupportedVersions(),
		}, name)
		if got != "out of standard support" {
			t.Errorf("version_support = %q, want %q", got, "out of standard support")
		}
	})
}

// ───────────────────────────────────────────────────────────────────────────
// row 3 — ecs-task: definitions past the cap
// ───────────────────────────────────────────────────────────────────────────

// partialECSTaskID builds a 32-hex task id whose last two digits index it, so
// tasks sort in the same order as the definitions they run.
func partialECSTaskID(i int) string {
	return fmt.Sprintf("0aaaa1111bbbb2222cccc3333dddd%04d", i)
}

func partialECSDefARN(i int) string {
	return fmt.Sprintf("arn:aws:ecs:us-east-1:123456789012:task-definition/acme-app-%02d:7", i)
}

// partialECSFleet wires n tasks, each on its own definition. The definition at
// index `bad` carries a privileged container; every other one is healthy.
func partialECSFleet(n, bad int) (*pw1ECSTaskFake, []resource.Resource) {
	fake := &pw1ECSTaskFake{
		tasks: make(map[string]ecstypes.Task, n),
		defs:  make(map[string]ecstypes.TaskDefinition, n),
	}
	rows := make([]resource.Resource, 0, n)
	for i := range n {
		id, defARN := partialECSTaskID(i), partialECSDefARN(i)
		fake.tasks[id] = pw1Task(id, defARN)
		container := pw1NoRefContainer("app")
		if i == bad {
			container.Privileged = aws.Bool(true)
		}
		fake.defs[defARN] = pw1TaskDef(defARN, container)
		rows = append(rows, pw1ECSTaskResource(id, defARN))
	}
	return fake, rows
}

// TestPartialECSTask_TasksOnADroppedDefinitionAreNotInspected pins the cap
// rule for the one enricher whose cap falls on a key the rows are grouped by
// rather than on the rows themselves. Slicing the definition list silently
// drops every task running the definitions past the cap, and those tasks then
// render as inspected and clean — a privileged container on the 51st
// definition is reported nowhere.
func TestPartialECSTask_TasksOnADroppedDefinitionAreNotInspected(t *testing.T) {
	const n = 51
	dropped := partialECSTaskID(n - 1)

	fake, rows := partialECSFleet(n, n-1)
	res := pw1EnrichECSTasks(t, fake, rows...)

	w4AssertNoCode(t, res.Findings[dropped], "ecs-task.privileged")
	partialAssertUninspected(t, res, dropped)
	partialAssertInspected(t, res, partialECSTaskID(0))
}

// TestPartialECSTask_EveryDefinitionAtTheCapIsInspected is the boundary's
// other side: at exactly the cap nothing is dropped, so the last definition's
// finding is reported and no task is a coverage gap.
func TestPartialECSTask_EveryDefinitionAtTheCapIsInspected(t *testing.T) {
	const n = 50
	last := partialECSTaskID(n - 1)

	fake, rows := partialECSFleet(n, n-1)
	res := pw1EnrichECSTasks(t, fake, rows...)

	if _, ok := w2Find(res.Findings[last], "ecs-task.privileged"); !ok {
		t.Errorf("the %dth definition was inspected, so its privileged container must be reported; got %s", n, w4CodesOf(res.Findings[last]))
	}
	for i := range n {
		partialAssertInspected(t, res, partialECSTaskID(i))
	}
}

// ───────────────────────────────────────────────────────────────────────────
// row 4 — ec2: user-data targets past the cap
// ───────────────────────────────────────────────────────────────────────────

// partialEC2UserDataWithKey is a boot script that leaves a long-lived access
// key on the instance — the condition the user-data scan exists to find.
const partialEC2UserDataWithKey = "#!/bin/bash\n" +
	"export AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE\n" +
	"export AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY\n" +
	"aws s3 sync s3://acme-config /etc/acme\n"

func partialEC2InstanceID(i int) string {
	return fmt.Sprintf("i-0a1b2c3d4e5f6%04d", i)
}

// partialEC2Fake serves the account-wide status call with nothing to report,
// so the only thing under test is the per-instance user-data read.
type partialEC2Fake struct {
	awsclient.EC2API

	userData map[string]string
}

func (f *partialEC2Fake) DescribeInstanceStatus(_ context.Context, _ *ec2svc.DescribeInstanceStatusInput, _ ...func(*ec2svc.Options)) (*ec2svc.DescribeInstanceStatusOutput, error) {
	return &ec2svc.DescribeInstanceStatusOutput{}, nil
}

func (f *partialEC2Fake) DescribeInstanceAttribute(_ context.Context, in *ec2svc.DescribeInstanceAttributeInput, _ ...func(*ec2svc.Options)) (*ec2svc.DescribeInstanceAttributeOutput, error) {
	script, ok := f.userData[aws.ToString(in.InstanceId)]
	if !ok {
		return &ec2svc.DescribeInstanceAttributeOutput{}, nil
	}
	return &ec2svc.DescribeInstanceAttributeOutput{
		UserData: &ec2types.AttributeValue{Value: aws.String(base64.StdEncoding.EncodeToString([]byte(script)))},
	}, nil
}

func partialEC2Fleet(n, withKey int) (*partialEC2Fake, []resource.Resource) {
	fake := &partialEC2Fake{userData: map[string]string{}}
	rows := make([]resource.Resource, 0, n)
	for i := range n {
		id := partialEC2InstanceID(i)
		fake.userData[id] = "#!/bin/bash\nyum update -y\n"
		if i == withKey {
			fake.userData[id] = partialEC2UserDataWithKey
		}
		rows = append(rows, resource.Resource{
			ID:     id,
			Name:   fmt.Sprintf("acme-worker-%02d", i),
			Fields: map[string]string{"state": "running", "instance_id": id},
		})
	}
	return fake, rows
}

func partialEnrichEC2(t *testing.T, fake *partialEC2Fake, rows []resource.Resource) awsclient.IssueEnricherResult {
	t.Helper()
	res, _ := awsclient.EnrichEC2InstanceStatus(context.Background(),
		&awsclient.ServiceClients{EC2: fake}, rows, nil)
	w2AssertEnricherShape(t, res)
	return res
}

// TestPartialEC2_InstancesPastTheUserDataCapAreNotInspected pins that the cap
// on the user-data scan is a coverage gap, not a clean bill of health. The
// 51st running instance is never read, so its row cannot claim its boot script
// holds no credentials.
func TestPartialEC2_InstancesPastTheUserDataCapAreNotInspected(t *testing.T) {
	const n = 51
	dropped := partialEC2InstanceID(n - 1)

	fake, rows := partialEC2Fleet(n, n-1)
	res := partialEnrichEC2(t, fake, rows)

	w4AssertNoCode(t, res.Findings[dropped], "ec2.user-data-secret")
	partialAssertUninspected(t, res, dropped)
	partialAssertInspected(t, res, partialEC2InstanceID(0))
}

// TestPartialEC2_EveryInstanceAtTheCapIsInspected is the boundary's other
// side: at exactly the cap every script is read, so the credential is reported
// and no row is a coverage gap.
func TestPartialEC2_EveryInstanceAtTheCapIsInspected(t *testing.T) {
	const n = 50
	last := partialEC2InstanceID(n - 1)

	fake, rows := partialEC2Fleet(n, n-1)
	res := partialEnrichEC2(t, fake, rows)

	w4AssertFinding(t, res.Findings[last], "ec2.user-data-secret",
		"credential in user data", domain.SevBroken, "wave2")
	for i := range n {
		partialAssertInspected(t, res, partialEC2InstanceID(i))
	}
}

// ───────────────────────────────────────────────────────────────────────────
// row 5 — backup: selection enumeration
// ───────────────────────────────────────────────────────────────────────────

const partialBackupPlanID = "0a1b2c3d-4e5f-6071-8293-a4b5c6d7e8f9"

// partialBackupFake serves the three calls FetchBackupPlansPage makes. Every
// field is a knob one case of row 5 turns: an enumeration that fails, one that
// spans two pages, one whose selection is expressed as a structured condition.
type partialBackupFake struct {
	selections    [][]backuptypes.BackupSelectionsListMember
	selectionsErr error
	selectionByID map[string]backuptypes.BackupSelection
}

func (f *partialBackupFake) ListBackupPlans(_ context.Context, _ *backupsvc.ListBackupPlansInput, _ ...func(*backupsvc.Options)) (*backupsvc.ListBackupPlansOutput, error) {
	return &backupsvc.ListBackupPlansOutput{BackupPlansList: []backuptypes.BackupPlansListMember{{
		BackupPlanId:   aws.String(partialBackupPlanID),
		BackupPlanName: aws.String("acme-nightly"),
		BackupPlanArn:  aws.String("arn:aws:backup:us-east-1:123456789012:backup-plan:" + partialBackupPlanID),
	}}}, nil
}

func (f *partialBackupFake) ListBackupSelections(_ context.Context, in *backupsvc.ListBackupSelectionsInput, _ ...func(*backupsvc.Options)) (*backupsvc.ListBackupSelectionsOutput, error) {
	if f.selectionsErr != nil {
		return nil, f.selectionsErr
	}
	page := 0
	if in.NextToken != nil {
		n, err := strconv.Atoi(*in.NextToken)
		if err != nil {
			return nil, fmt.Errorf("unexpected continuation token %q", *in.NextToken)
		}
		page = n
	}
	if page >= len(f.selections) {
		return &backupsvc.ListBackupSelectionsOutput{}, nil
	}
	out := &backupsvc.ListBackupSelectionsOutput{BackupSelectionsList: f.selections[page]}
	if page+1 < len(f.selections) {
		out.NextToken = aws.String(strconv.Itoa(page + 1))
	}
	return out, nil
}

func (f *partialBackupFake) GetBackupSelection(_ context.Context, in *backupsvc.GetBackupSelectionInput, _ ...func(*backupsvc.Options)) (*backupsvc.GetBackupSelectionOutput, error) {
	id := aws.ToString(in.SelectionId)
	sel, ok := f.selectionByID[id]
	if !ok {
		return nil, fmt.Errorf("no selection %q", id)
	}
	return &backupsvc.GetBackupSelectionOutput{
		BackupPlanId:    in.BackupPlanId,
		SelectionId:     in.SelectionId,
		BackupSelection: &sel,
	}, nil
}

func partialBackupSelectionMember(id string) backuptypes.BackupSelectionsListMember {
	return backuptypes.BackupSelectionsListMember{
		SelectionId:   aws.String(id),
		SelectionName: aws.String("sel-" + id),
		BackupPlanId:  aws.String(partialBackupPlanID),
		IamRoleArn:    aws.String("arn:aws:iam::123456789012:role/service-role/AWSBackupDefaultServiceRole"),
	}
}

func partialBackupSelection(resources []string, conditions *backuptypes.Conditions) backuptypes.BackupSelection {
	return backuptypes.BackupSelection{
		SelectionName: aws.String("acme-selection"),
		IamRoleArn:    aws.String("arn:aws:iam::123456789012:role/service-role/AWSBackupDefaultServiceRole"),
		Resources:     resources,
		Conditions:    conditions,
	}
}

// partialBackupCache runs the real plans fetcher against the fake and returns
// the cache the coverage join reads, exactly as the app assembles it.
func partialBackupCache(t *testing.T, fake *partialBackupFake) resource.ResourceCache {
	t.Helper()
	out, err := awsclient.FetchBackupPlansPage(context.Background(), fake, "")
	if err != nil {
		t.Fatalf("FetchBackupPlansPage: %v", err)
	}
	return w7CacheWith(out.Resources...)
}

// TestPartialBackup_SelectionEnumerationNeverInventsCoverage pins that the
// plan row only ever says what the enumeration actually established. Every
// case here is a selection that DOES take the volume in; reporting "not
// covered" for any of them tells an operator to act on a plan that already
// protects the resource.
func TestPartialBackup_SelectionEnumerationNeverInventsCoverage(t *testing.T) {
	volume := w7Volume("in-use")

	t.Run("enumeration denied", func(t *testing.T) {
		// The plan may well select the volume — nobody could read it. An
		// unreadable selection list is unknown coverage, and the warning is a
		// claim the enumeration never earned.
		cacheEntry := partialBackupCache(t, &partialBackupFake{selectionsErr: partialAccessDenied()})
		res := w7EnrichEBS(t, []resource.Resource{volume}, cacheEntry)
		w4AssertNoCode(t, res.Findings[w7VolumeID], awsclient.CodeEBSNotInBackupPlan)
	})

	t.Run("matching selection on the second page", func(t *testing.T) {
		cacheEntry := partialBackupCache(t, &partialBackupFake{
			selections: [][]backuptypes.BackupSelectionsListMember{
				{partialBackupSelectionMember("sel-page-one")},
				{partialBackupSelectionMember("sel-page-two")},
			},
			selectionByID: map[string]backuptypes.BackupSelection{
				"sel-page-one": partialBackupSelection([]string{"arn:aws:rds:us-east-1:123456789012:db:acme-orders-db"}, nil),
				"sel-page-two": partialBackupSelection([]string{w7VolumeARN}, nil),
			},
		})
		res := w7EnrichEBS(t, []resource.Resource{volume}, cacheEntry)
		w4AssertNoCode(t, res.Findings[w7VolumeID], awsclient.CodeEBSNotInBackupPlan)
	})

	t.Run("selection expressed as a structured tag condition", func(t *testing.T) {
		// Conditions is the second of the two tag-selection shapes AWS
		// accepts; a plan built in the console today uses it, and the volume
		// carries the backup=nightly tag it names.
		cacheEntry := partialBackupCache(t, &partialBackupFake{
			selections: [][]backuptypes.BackupSelectionsListMember{{partialBackupSelectionMember("sel-tagged")}},
			selectionByID: map[string]backuptypes.BackupSelection{
				"sel-tagged": partialBackupSelection(nil, &backuptypes.Conditions{
					StringEquals: []backuptypes.ConditionParameter{{
						ConditionKey:   aws.String("aws:ResourceTag/backup"),
						ConditionValue: aws.String("nightly"),
					}},
				}),
			},
		})
		res := w7EnrichEBS(t, []resource.Resource{volume}, cacheEntry)
		w4AssertNoCode(t, res.Findings[w7VolumeID], awsclient.CodeEBSNotInBackupPlan)
	})

	t.Run("a complete enumeration that selects nothing still reports", func(t *testing.T) {
		// The negative half: when the whole selection list was read and none
		// of it takes the volume in, "not covered" is knowledge, and the
		// abstention rules above must not swallow it.
		cacheEntry := partialBackupCache(t, &partialBackupFake{
			selections: [][]backuptypes.BackupSelectionsListMember{{partialBackupSelectionMember("sel-elsewhere")}},
			selectionByID: map[string]backuptypes.BackupSelection{
				"sel-elsewhere": partialBackupSelection([]string{"arn:aws:rds:us-east-1:123456789012:db:acme-orders-db"}, nil),
			},
		})
		res := w7EnrichEBS(t, []resource.Resource{volume}, cacheEntry)
		w4AssertFinding(t, res.Findings[w7VolumeID], awsclient.CodeEBSNotInBackupPlan,
			"not covered by a backup plan", domain.SevWarn, "wave2")
	})
}

// ───────────────────────────────────────────────────────────────────────────
// row 6 — ebs: the same volume must get the same verdict after a cache replay
// ───────────────────────────────────────────────────────────────────────────

// partialCacheReplay puts a row through the real disk cache and reads it back
// the way the app seeds a list from it: ID, Name, Fields and Findings survive,
// RawStruct does not. Anything an enricher needs that lives only on RawStruct
// is therefore gone on the replayed row, which is the whole point of the pin.
func partialCacheReplay(t *testing.T, shortName string, r resource.Resource) resource.Resource {
	t.Helper()
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	store := cache.LoadDirForTest("example-readonly", "us-east-1")
	store.Put(shortName, cache.TypeFile{
		HasResources: true,
		Count:        1,
		Rows:         []cache.Row{{ID: r.ID, Name: r.Name, Fields: r.Fields, Findings: r.Findings}},
	})
	if err := store.SaveType(shortName); err != nil {
		t.Fatalf("SaveType(%q): %v", shortName, err)
	}
	tf, ok := cache.LoadDirForTest("example-readonly", "us-east-1").Type(shortName)
	if !ok {
		t.Fatalf("reloaded store has no %q entry", shortName)
	}
	if len(tf.Rows) != 1 {
		t.Fatalf("reloaded %q holds %d rows, want 1", shortName, len(tf.Rows))
	}
	row := tf.Rows[0]
	return resource.Resource{ID: row.ID, Name: row.Name, Type: shortName, Fields: row.Fields, Findings: row.Findings}
}

// partialEBSCoverageVerdict renders the coverage verdict as the operator reads
// it: the phrase of the backup finding, or "covered" when there is none.
func partialEBSCoverageVerdict(t *testing.T, res awsclient.IssueEnricherResult, id string) string {
	t.Helper()
	if f, ok := w2Find(res.Findings[id], string(awsclient.CodeEBSNotInBackupPlan)); ok {
		return f.Phrase
	}
	return "covered"
}

// TestPartialEBS_CoverageVerdictSurvivesACacheReplay pins that a volume's
// backup verdict does not change when the row comes back from disk instead of
// from the API. The tags a selection matches on live on RawStruct, which the
// cache deliberately does not persist — so the replayed row has to either read
// them from a field the cache does keep or abstain. What it must not do is
// read "no tags" as "no matching tag" and turn a covered volume into a warning
// the next time the app starts.
func TestPartialEBS_CoverageVerdictSurvivesACacheReplay(t *testing.T) {
	tagged := w7CacheWith(w7Plan("", "", "backup=nightly"))
	byARNElsewhere := w7CacheWith(w7Plan("arn:aws:ec2:us-east-1:123456789012:volume/vol-0999999999999999", "", ""))

	for _, tc := range []struct {
		name       string
		plans      resource.ResourceCache
		wantLive   string
		wantReplay string
	}{
		{
			name:       "tag-selected volume stays covered",
			plans:      tagged,
			wantLive:   "covered",
			wantReplay: "covered",
		},
		{
			// The negative half: no plan selects by tag here, so the tags
			// cannot change the answer and both rows must still warn. An
			// abstention that swallowed this case would trade one wrong
			// verdict for another.
			name:       "volume no plan names stays uncovered",
			plans:      byARNElsewhere,
			wantLive:   "not covered by a backup plan",
			wantReplay: "not covered by a backup plan",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			live := w7Volume("in-use")
			liveRes := w7EnrichEBS(t, []resource.Resource{live}, tc.plans)
			if got := partialEBSCoverageVerdict(t, liveRes, w7VolumeID); got != tc.wantLive {
				t.Fatalf("live verdict = %q, want %q", got, tc.wantLive)
			}

			replayed := partialCacheReplay(t, "ebs", live)
			if replayed.RawStruct != nil {
				t.Fatal("the replayed row still carries RawStruct; the cache round trip is not exercising the disk shape")
			}
			replayRes := w7EnrichEBS(t, []resource.Resource{replayed}, tc.plans)
			if got := partialEBSCoverageVerdict(t, replayRes, w7VolumeID); got != tc.wantReplay {
				t.Errorf("verdict after a cache replay = %q, want %q (live said %q)", got, tc.wantReplay, tc.wantLive)
			}
		})
	}
}

// ───────────────────────────────────────────────────────────────────────────
// row 7 — ebs: a failed status read keeps the findings already computed
// ───────────────────────────────────────────────────────────────────────────

// partialEBSStatusFake fails the account-wide volume-status call and nothing
// else, so the cache-only backup and snapshot joins have already produced
// their findings by the time it is reached.
type partialEBSStatusFake struct {
	awsclient.EC2API
}

func (f *partialEBSStatusFake) DescribeVolumeStatus(_ context.Context, _ *ec2svc.DescribeVolumeStatusInput, _ ...func(*ec2svc.Options)) (*ec2svc.DescribeVolumeStatusOutput, error) {
	return nil, partialAccessDenied()
}

// TestPartialEBS_StatusFailureKeepsTheCoverageFindings pins that one failed
// call marks one check unknown, not the whole row. The backup and snapshot
// joins answered from the cache before the API was ever touched; discarding
// their findings because a later, unrelated call failed loses facts a9s
// already had, and leaves the volume looking healthier than it is.
func TestPartialEBS_StatusFailureKeepsTheCoverageFindings(t *testing.T) {
	clients := &awsclient.ServiceClients{EC2: &partialEBSStatusFake{}}
	store := session.NewIdentityStore()
	store.Set(w7Account, nil)
	clients.SetIdentityStore(store)

	plans := w7CacheWith(w7Plan("arn:aws:rds:us-east-1:123456789012:db:acme-orders-db", "", ""))
	plans["ebs-snap"] = resource.ResourceCacheEntry{IsTruncated: false}

	res, err := awsclient.EnrichEBSVolumeStatus(context.Background(), clients,
		[]resource.Resource{w7Volume("in-use")}, plans)
	if err == nil {
		t.Fatal("a denied DescribeVolumeStatus returned no error")
	}

	w4AssertFinding(t, res.Findings[w7VolumeID], awsclient.CodeEBSNotInBackupPlan,
		"not covered by a backup plan", domain.SevWarn, "wave2")
	w4AssertFinding(t, res.Findings[w7VolumeID], awsclient.CodeEBSNoSnapshot,
		"no snapshot exists", domain.SevWarn, "wave2")
	partialAssertUninspected(t, res, w7VolumeID)
}

// ───────────────────────────────────────────────────────────────────────────
// row 8 — ecr: only NotFound means the lifecycle policy is missing
// ───────────────────────────────────────────────────────────────────────────

const partialECRRepo = "acme/api"

// partialECRFake answers the three reads EnrichECRRepository makes. The
// lifecycle read is the one under test; the other two are healthy so nothing
// else can account for the row's verdict.
type partialECRFake struct {
	awsclient.ECRAPI

	lifecycleErr error
}

func (f *partialECRFake) DescribeImages(_ context.Context, _ *ecrsvc.DescribeImagesInput, _ ...func(*ecrsvc.Options)) (*ecrsvc.DescribeImagesOutput, error) {
	return &ecrsvc.DescribeImagesOutput{ImageDetails: []ecrtypes.ImageDetail{{
		RepositoryName: aws.String(partialECRRepo),
		ImageDigest:    aws.String("sha256:1111111111111111111111111111111111111111111111111111111111111111"),
		ImageTags:      []string{"1.4.2"},
		ImageScanFindingsSummary: &ecrtypes.ImageScanFindingsSummary{
			FindingSeverityCounts: map[string]int32{"MEDIUM": 2},
		},
	}}}, nil
}

func (f *partialECRFake) GetRepositoryPolicy(_ context.Context, _ *ecrsvc.GetRepositoryPolicyInput, _ ...func(*ecrsvc.Options)) (*ecrsvc.GetRepositoryPolicyOutput, error) {
	return nil, &ecrtypes.RepositoryPolicyNotFoundException{Message: aws.String("Repository policy does not exist")}
}

func (f *partialECRFake) GetLifecyclePolicy(_ context.Context, _ *ecrsvc.GetLifecyclePolicyInput, _ ...func(*ecrsvc.Options)) (*ecrsvc.GetLifecyclePolicyOutput, error) {
	if f.lifecycleErr != nil {
		return nil, f.lifecycleErr
	}
	return &ecrsvc.GetLifecyclePolicyOutput{
		RepositoryName:      aws.String(partialECRRepo),
		LifecyclePolicyText: aws.String(`{"rules":[{"rulePriority":1,"selection":{"tagStatus":"untagged","countType":"sinceImagePushed","countUnit":"days","countNumber":14},"action":{"type":"expire"}}]}`),
	}, nil
}

func partialEnrichECR(t *testing.T, fake *partialECRFake) awsclient.IssueEnricherResult {
	t.Helper()
	return partialEnrichECRAPI(t, fake)
}

// partialEnrichECRAPI drives the enricher against any ECR client shape, so an
// attack can substitute a client that serves a different subset of the reads.
func partialEnrichECRAPI(t *testing.T, api awsclient.ECRAPI) awsclient.IssueEnricherResult {
	t.Helper()
	store := session.NewIdentityStore()
	store.Set("123456789012", nil)
	clients := &awsclient.ServiceClients{ECR: api}
	clients.SetIdentityStore(store)
	res, _ := awsclient.EnrichECRRepository(context.Background(), clients,
		[]resource.Resource{{ID: partialECRRepo, Name: partialECRRepo, Fields: map[string]string{}}}, nil)
	w2AssertEnricherShape(t, res)
	return res
}

// TestPartialECR_OnlyNotFoundMeansTheLifecyclePolicyIsMissing pins the three
// answers the lifecycle read can give and keeps them apart. NotFound is the
// finding; success is the healthy case; anything else is a read that did not
// happen, and a repository nobody could read is not a repository with a
// policy.
func TestPartialECR_OnlyNotFoundMeansTheLifecyclePolicyIsMissing(t *testing.T) {
	t.Run("read denied", func(t *testing.T) {
		res := partialEnrichECR(t, &partialECRFake{lifecycleErr: partialAccessDenied()})
		w4AssertNoCode(t, res.Findings[partialECRRepo], "ecr.no-lifecycle-policy")
		partialAssertUninspected(t, res, partialECRRepo)
	})

	t.Run("policy genuinely absent", func(t *testing.T) {
		res := partialEnrichECR(t, &partialECRFake{
			lifecycleErr: &ecrtypes.LifecyclePolicyNotFoundException{Message: aws.String("Lifecycle policy does not exist for the repository")},
		})
		w4AssertFinding(t, res.Findings[partialECRRepo], "ecr.no-lifecycle-policy",
			"no lifecycle policy", domain.SevWarn, "wave2")
		partialAssertInspected(t, res, partialECRRepo)
	})

	t.Run("policy present", func(t *testing.T) {
		res := partialEnrichECR(t, &partialECRFake{})
		w4AssertNoCode(t, res.Findings[partialECRRepo], "ecr.no-lifecycle-policy")
		partialAssertInspected(t, res, partialECRRepo)
	})
}

// ───────────────────────────────────────────────────────────────────────────
// row 9 — redshift: a parameter walk cut short is not an empty answer
// ───────────────────────────────────────────────────────────────────────────

const partialRedshiftGroup = "acme-analytics-params"

// partialRedshiftPagingFake pages DescribeClusterParameters. require_ssl sits
// on the page named by requireSSLPage; every earlier page is filler with a
// continuation marker, which is how AWS returns a group holding dozens of
// parameters.
type partialRedshiftPagingFake struct {
	awsclient.RedshiftAPI

	pages          int
	requireSSLPage int
	requireSSL     string
}

func (f *partialRedshiftPagingFake) DescribeLoggingStatus(_ context.Context, _ *redshiftsvc.DescribeLoggingStatusInput, _ ...func(*redshiftsvc.Options)) (*redshiftsvc.DescribeLoggingStatusOutput, error) {
	return &redshiftsvc.DescribeLoggingStatusOutput{
		LoggingEnabled: aws.Bool(true),
		BucketName:     aws.String("acme-redshift-audit"),
	}, nil
}

func (f *partialRedshiftPagingFake) DescribeClusterParameters(_ context.Context, in *redshiftsvc.DescribeClusterParametersInput, _ ...func(*redshiftsvc.Options)) (*redshiftsvc.DescribeClusterParametersOutput, error) {
	page := 0
	if in.Marker != nil {
		n, err := strconv.Atoi(*in.Marker)
		if err != nil {
			return nil, fmt.Errorf("unexpected marker %q", *in.Marker)
		}
		page = n
	}
	out := &redshiftsvc.DescribeClusterParametersOutput{
		Parameters: []redshifttypes.Parameter{{
			ParameterName:  aws.String(fmt.Sprintf("wlm_json_configuration_%d", page)),
			ParameterValue: aws.String("[{\"query_concurrency\":5}]"),
			Source:         aws.String("engine-default"),
		}},
	}
	if page == f.requireSSLPage {
		out.Parameters = append(out.Parameters, redshifttypes.Parameter{
			ParameterName:  aws.String("require_ssl"),
			ParameterValue: aws.String(f.requireSSL),
			Source:         aws.String("user"),
		})
	}
	if page+1 < f.pages {
		out.Marker = aws.String(strconv.Itoa(page + 1))
	}
	return out, nil
}

func partialEnrichRedshift(t *testing.T, fake *partialRedshiftPagingFake, id string) awsclient.IssueEnricherResult {
	t.Helper()
	res, _ := awsclient.EnrichRedshiftPosture(context.Background(),
		&awsclient.ServiceClients{Redshift: fake},
		[]resource.Resource{w2Res(id, w2RedshiftCluster(id, partialRedshiftGroup))}, nil)
	w2AssertEnricherShape(t, res)
	return res
}

// TestPartialRedshift_ParameterWalkCutShortIsNotAnEmptyAnswer pins that
// running out of pages before require_ssl appears leaves the setting unread.
// The walk is bounded, and the bound is a limit on what a9s looked at — not
// evidence that the parameter is unset. Reading it as unset warns an operator
// about a cluster that may well require SSL, and caching that empty value
// spreads the wrong verdict to every other cluster sharing the group.
func TestPartialRedshift_ParameterWalkCutShortIsNotAnEmptyAnswer(t *testing.T) {
	t.Run("require_ssl sits past the page cap", func(t *testing.T) {
		const id = "acme-analytics"
		res := partialEnrichRedshift(t, &partialRedshiftPagingFake{
			pages:          awsclient.PerParentPageCap + 5,
			requireSSLPage: awsclient.PerParentPageCap + 2,
			requireSSL:     "true",
		}, id)
		w4AssertNoCode(t, res.Findings[id], "redshift.require-ssl-off")
		partialAssertUninspected(t, res, id)
	})

	t.Run("require_ssl found within the cap", func(t *testing.T) {
		const id = "acme-reporting"
		res := partialEnrichRedshift(t, &partialRedshiftPagingFake{
			pages:          awsclient.PerParentPageCap,
			requireSSLPage: 2,
			requireSSL:     "false",
		}, id)
		w4AssertFinding(t, res.Findings[id], "redshift.require-ssl-off",
			"SSL not required", domain.SevWarn, "wave2")
		partialAssertInspected(t, res, id)
	})
}

// ───────────────────────────────────────────────────────────────────────────
// row 10 — backup: a Conditions block the flat tag list cannot represent
// ───────────────────────────────────────────────────────────────────────────

// partialRow10Volume is w7Volume carrying exactly the tags a case needs, so
// what the selection does and does not take in is decided by the condition
// under test rather than by the fixture's own tags.
func partialRow10Volume(tags map[string]string) resource.Resource {
	r := w7Volume("in-use")
	vol := r.RawStruct.(ec2types.Volume)
	vol.Tags = nil
	for k, v := range tags {
		vol.Tags = append(vol.Tags, ec2types.Tag{Key: aws.String(k), Value: aws.String(v)})
	}
	r.RawStruct = vol
	return r
}

func partialRow10Param(key, value string) backuptypes.ConditionParameter {
	return backuptypes.ConditionParameter{
		ConditionKey:   aws.String("aws:ResourceTag/" + key),
		ConditionValue: aws.String(value),
	}
}

// partialRow10Plan runs the real plans fetcher over one selection carrying
// conditions, and returns the plan row alongside the cache the join reads.
func partialRow10Plan(t *testing.T, conditions *backuptypes.Conditions) (resource.Resource, resource.ResourceCache) {
	t.Helper()
	return partialSelectionPlan(t, partialBackupSelection(nil, conditions))
}

// partialSelectionPlan runs the real plans fetcher over one selection and
// returns the plan row alongside the cache the join and the related panel read.
func partialSelectionPlan(t *testing.T, sel backuptypes.BackupSelection) (resource.Resource, resource.ResourceCache) {
	t.Helper()
	cacheEntry := partialBackupCache(t, &partialBackupFake{
		selections: [][]backuptypes.BackupSelectionsListMember{{partialBackupSelectionMember("sel-conditions")}},
		selectionByID: map[string]backuptypes.BackupSelection{
			"sel-conditions": sel,
		},
	})
	plans := cacheEntry["backup"].Resources
	if len(plans) != 1 {
		t.Fatalf("the fetcher returned %d plan rows, want 1", len(plans))
	}
	return plans[0], cacheEntry
}

// TestPartialRow10_ConditionsTheFlatListCannotRepresentAbstain pins the limit
// of the flat selection_tags list. That list is an OR of "k=v" pairs, while a
// Conditions block is an AND across its four operators — so the only blocks
// the list can carry without changing their meaning are the ones holding a
// single positive parameter. Anything else read through the flat list widens
// what the plan appears to cover, and a plan that appears to cover more is a
// plan that silences warnings it has no business silencing.
//
// The rule is the same one row 5 established for a walk that was cut short:
// abstain rather than model. A block a9s cannot represent is a selection it
// did not finish reading.
func TestPartialRow10_ConditionsTheFlatListCannotRepresentAbstain(t *testing.T) {
	for _, tc := range []struct {
		name        string
		conditions  *backuptypes.Conditions
		tags        map[string]string
		wantPartial bool
		wantWarning bool
	}{
		{
			// A negative operator has no "k=v" spelling at all, so today it
			// folds to nothing and the volume it selects reads uncovered.
			name: "one negative parameter",
			conditions: &backuptypes.Conditions{
				StringNotEquals: []backuptypes.ConditionParameter{partialRow10Param("environment", "sandbox")},
			},
			tags:        map[string]string{"environment": "production"},
			wantPartial: true,
			wantWarning: false,
		},
		{
			name: "one negative wildcard parameter",
			conditions: &backuptypes.Conditions{
				StringNotLike: []backuptypes.ConditionParameter{partialRow10Param("environment", "sandbox*")},
			},
			tags:        map[string]string{"environment": "production"},
			wantPartial: true,
			wantWarning: false,
		},
		{
			// Two parameters are an AND; the flat list ORs them, so a volume
			// carrying either one alone reads covered when the plan selects
			// neither. This volume carries neither, and today it warns while
			// the plan's real reach is unknown.
			name: "two positive parameters in one block",
			conditions: &backuptypes.Conditions{
				StringEquals: []backuptypes.ConditionParameter{
					partialRow10Param("backup", "nightly"),
					partialRow10Param("tier", "critical"),
				},
			},
			tags:        map[string]string{"environment": "production"},
			wantPartial: true,
			wantWarning: false,
		},
		{
			// Two parameters still count as two when they are spread across
			// operators, which is the shape the console produces for
			// "tagged nightly AND named prod-anything".
			name: "one parameter in each positive operator",
			conditions: &backuptypes.Conditions{
				StringEquals: []backuptypes.ConditionParameter{partialRow10Param("backup", "nightly")},
				StringLike:   []backuptypes.ConditionParameter{partialRow10Param("environment", "prod*")},
			},
			tags:        map[string]string{"environment": "production"},
			wantPartial: true,
			wantWarning: false,
		},
		{
			// The shapes the flat list represents exactly keep their fold. A
			// single positive parameter is its own AND, so "k=v" says the
			// whole thing.
			name: "a single StringEquals still covers what it names",
			conditions: &backuptypes.Conditions{
				StringEquals: []backuptypes.ConditionParameter{partialRow10Param("backup", "nightly")},
			},
			tags:        map[string]string{"backup": "nightly"},
			wantPartial: false,
			wantWarning: false,
		},
		{
			name: "a single StringLike still covers what it globs",
			conditions: &backuptypes.Conditions{
				StringLike: []backuptypes.ConditionParameter{partialRow10Param("environment", "prod*")},
			},
			tags:        map[string]string{"environment": "production"},
			wantPartial: false,
			wantWarning: false,
		},
		{
			// The negative half. A block a9s does represent still decides, so
			// the abstention above cannot be a blanket one: a volume this
			// selection does not name is uncovered, and says so.
			name: "a single StringEquals still reports what it misses",
			conditions: &backuptypes.Conditions{
				StringEquals: []backuptypes.ConditionParameter{partialRow10Param("backup", "nightly")},
			},
			tags:        map[string]string{"backup": "weekly"},
			wantPartial: false,
			wantWarning: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan, plans := partialRow10Plan(t, tc.conditions)

			// selections_partial is the field the coverage join reads to tell
			// a plan it could not finish reading from one that protects
			// nothing.
			gotPartial := plan.Fields["selections_partial"] != ""
			if gotPartial != tc.wantPartial {
				t.Errorf("plan row selections_partial = %q, want partial = %v; selection_tags folded to %q",
					plan.Fields["selections_partial"], tc.wantPartial, plan.Fields["selection_tags"])
			}

			res := w7EnrichEBS(t, []resource.Resource{partialRow10Volume(tc.tags)}, plans)
			if tc.wantWarning {
				w4AssertFinding(t, res.Findings[w7VolumeID], awsclient.CodeEBSNotInBackupPlan,
					"not covered by a backup plan", domain.SevWarn, "wave2")
				return
			}
			w4AssertNoCode(t, res.Findings[w7VolumeID], awsclient.CodeEBSNotInBackupPlan)
		})
	}
}
