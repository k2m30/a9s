package unit

// codex2_session_findings_test.go — behavioural pins for the Codex
// session-range findings 1..10 and w132: one cache directory name per
// profile/region pair, no lane that answers "clean" for a call that failed,
// one policy parser, one AWS error-code table.

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/backup"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/smithy-go"

	"github.com/k2m30/a9s/v3/core/app"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/cache"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// ─── Finding 2 — EncodePathElem leaves a boundary hyphen ────────────────────

// TestEncodePathElem_BoundaryHyphenCannotCollide pins injectivity at the one
// place the "--" joiner can be forged: a trailing hyphen on the profile or a
// leading hyphen on the region. Both pairs used to join to
// "team---us-east-1" and share one cache directory.
func TestEncodePathElem_BoundaryHyphenCannotCollide(t *testing.T) {
	root := t.TempDir()
	a := cache.DirIn(root, "team-", "us-east-1")
	b := cache.DirIn(root, "team", "-us-east-1")
	if a == b {
		t.Fatalf("two different pairs share one cache directory: %q\n"+
			"finding 2: EncodePathElem escapes a hyphen only when a neighbour is a hyphen, "+
			"so a boundary hyphen passes through and forges the \"--\" joiner", a)
	}
}

// TestEncodePathElem_OrdinaryNamesPassThroughUnchanged pins the other half of
// the rule: an ordinary profile and region keep the directory name (and so
// the cache content) they already have on disk.
func TestEncodePathElem_OrdinaryNamesPassThroughUnchanged(t *testing.T) {
	root := t.TempDir()
	for _, tc := range [][2]string{
		{"example-readonly", "us-east-1"},
		{"default", "eu-central-1"},
		{"acme-prod-readonly", "ap-southeast-2"},
	} {
		want := filepath.Join(root, tc[0]+"--"+tc[1])
		if got := cache.DirIn(root, tc[0], tc[1]); got != want {
			t.Errorf("DirIn(%q,%q) = %q, want byte-identical %q", tc[0], tc[1], got, want)
		}
	}
}

// ─── Finding 1 — the legacy sanitized-dir fallback is non-injective ─────────

// TestLoadDirIn_TransformedPairNeverInheritsALegacyDirectory pins finding 1:
// profile "team/a" sanitizes to "team_a", which is also the encoded name of
// the DIFFERENT profile "team_a". Reading the legacy directory hands one
// pair's cached rows to another.
func TestLoadDirIn_TransformedPairNeverInheritsALegacyDirectory(t *testing.T) {
	root := t.TempDir()

	// The legacy directory, written as profile "team_a" — a real, different pair.
	owner := cache.LoadDirIn(root, "team_a", "us-east-1")
	owner.Put("ec2", cache.TypeFile{HasResources: true, Count: 7, Exact: true, SavedAt: time.Now()})
	if err := owner.SaveType("ec2"); err != nil {
		t.Fatalf("seeding the legacy directory: %v", err)
	}

	got := cache.LoadDirIn(root, "team/a", "us-east-1")
	if tf, ok := got.Type("ec2"); ok {
		t.Fatalf("profile %q read profile %q's cache (count=%d)\n"+
			"finding 1: the legacy sanitized-dir fallback is not injective; a pair whose "+
			"encoded name differs from its legacy name must start cold, not inherit",
			"team/a", "team_a", tf.Count)
	}
}

// ─── Finding 3 — a denied inline-policy call renders the role clean ─────────

type codex2RoleFake struct {
	roles     []iamtypes.Role
	listErr   error
	policies  []string
	getErrFor map[string]error
	docs      map[string]string
}

func (f *codex2RoleFake) ListRoles(_ context.Context, _ *iam.ListRolesInput, _ ...func(*iam.Options)) (*iam.ListRolesOutput, error) {
	return &iam.ListRolesOutput{Roles: f.roles}, nil
}

func (f *codex2RoleFake) ListRolePolicies(_ context.Context, _ *iam.ListRolePoliciesInput, _ ...func(*iam.Options)) (*iam.ListRolePoliciesOutput, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return &iam.ListRolePoliciesOutput{PolicyNames: f.policies}, nil
}

func (f *codex2RoleFake) GetRolePolicy(_ context.Context, in *iam.GetRolePolicyInput, _ ...func(*iam.Options)) (*iam.GetRolePolicyOutput, error) {
	name := aws.ToString(in.PolicyName)
	if err, ok := f.getErrFor[name]; ok {
		return nil, err
	}
	doc, ok := f.docs[name]
	if !ok {
		return nil, errors.New("no such policy")
	}
	return &iam.GetRolePolicyOutput{
		RoleName:       in.RoleName,
		PolicyName:     in.PolicyName,
		PolicyDocument: aws.String(doc),
	}, nil
}

func codex2Denied(op string) error {
	return &smithy.OperationError{
		ServiceID:     "IAM",
		OperationName: op,
		Err: &smithy.GenericAPIError{
			Code:    "AccessDenied",
			Message: "User: arn:aws:iam::123456789012:user/example is not authorized to perform: iam:" + op,
		},
	}
}

const codex2PrivEscDoc = `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["iam:CreatePolicyVersion"],"Resource":"*"}]}`

const codex2ReadDoc = `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["s3:GetObject"],"Resource":"arn:aws:s3:::acme-reports/*"}]}`

func codex2Role(name string) iamtypes.Role {
	return iamtypes.Role{
		RoleName: aws.String(name),
		RoleId:   aws.String("AROA00000000000EXAMPLE"),
		Path:     aws.String("/"),
		Arn:      aws.String("arn:aws:iam::123456789012:role/" + name),
		AssumeRolePolicyDocument: aws.String(
			`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":"ec2.amazonaws.com"},"Action":"sts:AssumeRole"}]}`),
	}
}

// TestIAMRoles_DeniedListRolePoliciesIsNotAnEmptyScan pins finding 3: the
// role's inline policies were never read, so the role's policy-derived facts
// are unknown and the page carries a partial error. The old code returned an
// empty scan, which renders the role inspected and clean.
func TestIAMRoles_DeniedListRolePoliciesIsNotAnEmptyScan(t *testing.T) {
	fake := &codex2RoleFake{
		roles:   []iamtypes.Role{codex2Role("acme-deploy-role")},
		listErr: codex2Denied("ListRolePolicies"),
	}
	res, err := awsclient.FetchIAMRolesPage(context.Background(), fake, "")
	if err == nil {
		t.Errorf("FetchIAMRolesPage returned no error for a denied ListRolePolicies; " +
			"the list lane must carry the partial failure")
	} else if !awsclient.IsAccessDenied(err) {
		t.Errorf("partial error class = %q, want %q", awsclient.ErrClass(err), awsclient.ClassAccessDenied)
	}
	if len(res.Resources) != 1 {
		t.Fatalf("got %d rows, want the role itself to survive the partial failure", len(res.Resources))
	}
	if got := res.Resources[0].Fields["policy_resources"]; got != "?" {
		t.Errorf("policy_resources = %q, want %q — the inline policies were never read", got, "?")
	}
}

// TestIAMRoles_DeniedGetRolePolicyIsNotAPartialScan pins the sibling case:
// one of two GetRolePolicy calls is refused, so the resource list a9s built
// is incomplete and must not read as the role's whole policy surface.
func TestIAMRoles_DeniedGetRolePolicyIsNotAPartialScan(t *testing.T) {
	fake := &codex2RoleFake{
		roles:    []iamtypes.Role{codex2Role("acme-deploy-role")},
		policies: []string{"readable", "denied"},
		docs:     map[string]string{"readable": codex2ReadDoc},
		getErrFor: map[string]error{
			"denied": codex2Denied("GetRolePolicy"),
		},
	}
	res, err := awsclient.FetchIAMRolesPage(context.Background(), fake, "")
	if err == nil {
		t.Errorf("FetchIAMRolesPage returned no error for a denied GetRolePolicy")
	}
	if len(res.Resources) != 1 {
		t.Fatalf("got %d rows, want 1", len(res.Resources))
	}
	if got := res.Resources[0].Fields["policy_resources"]; got != "?" {
		t.Errorf("policy_resources = %q, want %q — one of the two documents was refused", got, "?")
	}
}

// TestIAMRoles_ReadableInlinePoliciesStillReportPrivEsc is the negative
// control: nothing failed, so the scan is complete and the finding lands.
func TestIAMRoles_ReadableInlinePoliciesStillReportPrivEsc(t *testing.T) {
	fake := &codex2RoleFake{
		roles:    []iamtypes.Role{codex2Role("acme-deploy-role")},
		policies: []string{"escalate"},
		docs:     map[string]string{"escalate": codex2PrivEscDoc},
	}
	res, err := awsclient.FetchIAMRolesPage(context.Background(), fake, "")
	if err != nil {
		t.Fatalf("FetchIAMRolesPage: unexpected error on a fully readable role: %v", err)
	}
	if got := res.Resources[0].Fields["policy_resources"]; got != "*" {
		t.Errorf("policy_resources = %q, want %q", got, "*")
	}
	if !codex2HasCode(res.Resources[0].Findings, "role.inline-privilege-escalation") {
		t.Errorf("no privilege-escalation finding on a readable escalating policy; findings=%+v",
			res.Resources[0].Findings)
	}
}

func codex2HasCode(fs []domain.Finding, code domain.FindingCode) bool {
	for _, f := range fs {
		if f.Code == code {
			return true
		}
	}
	return false
}

// ─── Finding 4 — a cut account-wide walk leaves a sighted row unanswered ────

type codex2BackupFake struct {
	awsclient.BackupAPI
	page  []backuptypes.BackupJob
	calls int
}

func (f *codex2BackupFake) ListBackupJobs(_ context.Context, _ *backup.ListBackupJobsInput, _ ...func(*backup.Options)) (*backup.ListBackupJobsOutput, error) {
	f.calls++
	// Never terminates: every page names the next one, so the walk always
	// ends at the cap.
	return &backup.ListBackupJobsOutput{BackupJobs: f.page, NextToken: aws.String("more")}, nil
}

// TestBackupJobs_CutWalkLeavesASightedPlanUninspected pins finding 4: a plan
// seen on page 1 with a healthy job is NOT answered when the walk is cut —
// its failed job may sit on a page nobody read.
func TestBackupJobs_CutWalkLeavesASightedPlanUninspected(t *testing.T) {
	const planID = "acme-render-plan-0000-1111-2222-333333333333"
	now := time.Now()
	fake := &codex2BackupFake{page: []backuptypes.BackupJob{{
		BackupJobId:  aws.String("job-0001"),
		State:        backuptypes.BackupJobStateCompleted,
		CreationDate: &now,
		CreatedBy:    &backuptypes.RecoveryPointCreator{BackupPlanId: aws.String(planID)},
	}}}

	rows := []resource.Resource{{ID: planID, Name: "acme-render-plan", Type: "backup"}}
	result, err := awsclient.EnrichBackupJobs(context.Background(),
		&awsclient.ServiceClients{Backup: fake}, rows, nil)
	if err != nil {
		t.Fatalf("EnrichBackupJobs: %v", err)
	}
	if !result.TruncatedIDs[planID] {
		t.Fatalf("plan %s is not in TruncatedIDs after a cut walk (%d pages read)\n"+
			"finding 4: one sighting on an early page is not the plan's whole answer; "+
			"a failed job of the same plan may be on a page the cap stopped short of",
			planID, fake.calls)
	}
}

// TestBackupJobs_CompletedWalkAnswersEveryPlan is the negative control: a
// walk that reached the last page answers for every plan on screen.
func TestBackupJobs_CompletedWalkAnswersEveryPlan(t *testing.T) {
	const planID = "acme-render-plan-0000-1111-2222-333333333333"
	now := time.Now()
	rows := []resource.Resource{{ID: planID, Name: "acme-render-plan", Type: "backup"}}
	result, err := awsclient.EnrichBackupJobs(context.Background(),
		&awsclient.ServiceClients{Backup: &codex2BackupOnePageFake{jobs: []backuptypes.BackupJob{{
			BackupJobId:  aws.String("job-0001"),
			State:        backuptypes.BackupJobStateCompleted,
			CreationDate: &now,
			CreatedBy:    &backuptypes.RecoveryPointCreator{BackupPlanId: aws.String(planID)},
		}}}}, rows, nil)
	if err != nil {
		t.Fatalf("EnrichBackupJobs: %v", err)
	}
	if result.TruncatedIDs[planID] {
		t.Errorf("plan %s marked uninspected though the walk read every page", planID)
	}
}

type codex2BackupOnePageFake struct {
	awsclient.BackupAPI
	jobs []backuptypes.BackupJob
}

func (f *codex2BackupOnePageFake) ListBackupJobs(_ context.Context, _ *backup.ListBackupJobsInput, _ ...func(*backup.Options)) (*backup.ListBackupJobsOutput, error) {
	return &backup.ListBackupJobsOutput{BackupJobs: f.jobs}, nil
}

// ─── Finding 5 — the public-snapshot cap sets only the aggregate flag ───────

type codex2SnapFake struct {
	awsclient.EC2API
	public []ec2types.Snapshot
	calls  int
}

func (f *codex2SnapFake) DescribeSnapshots(_ context.Context, in *ec2.DescribeSnapshotsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSnapshotsOutput, error) {
	f.calls++
	for _, u := range in.RestorableByUserIds {
		if u == "all" {
			return &ec2.DescribeSnapshotsOutput{Snapshots: f.public, NextToken: aws.String("more")}, nil
		}
	}
	return &ec2.DescribeSnapshotsOutput{}, nil
}

// TestEBSSnapPublic_CutWalkMarksTheUnansweredSnapshot pins finding 5: the
// public-snapshot walk stopped at its cap, so a snapshot the walked pages
// never named is unknown, not private.
func TestEBSSnapPublic_CutWalkMarksTheUnansweredSnapshot(t *testing.T) {
	snap := pw1Snapshot("snap-0unanswered0aa1", "vol-0aaaa1111bbbb2222")
	fake := &codex2SnapFake{}

	e, ok := awsclient.Wave2EnricherFor("ebs-snap")
	if !ok || e.Fn == nil {
		t.Fatal("no Wave 2 enricher registered for ebs-snap")
	}
	rows := []resource.Resource{{
		ID:        aws.ToString(snap.SnapshotId),
		Name:      aws.ToString(snap.SnapshotId),
		RawStruct: snap,
		Fields:    map[string]string{"snapshot_id": aws.ToString(snap.SnapshotId), "volume_id": aws.ToString(snap.VolumeId)},
	}}
	res, _ := e.Fn(context.Background(), &awsclient.ServiceClients{EC2: fake}, rows,
		pw1EBSCache("vol-0aaaa1111bbbb2222"))

	if !res.TruncatedIDs[aws.ToString(snap.SnapshotId)] {
		t.Fatalf("snapshot %s is not in TruncatedIDs after a cut walk (%d pages read)\n"+
			"finding 5: the cap sets only the aggregate Truncated flag, so a snapshot past "+
			"it renders private rather than unknown", aws.ToString(snap.SnapshotId), fake.calls)
	}
}

// ─── Finding 6 — a second, array-only admin detector ────────────────────────

type codex2PolicyFake struct {
	awsclient.IAMAPI
	doc string
}

func (f *codex2PolicyFake) GetPolicy(_ context.Context, in *iam.GetPolicyInput, _ ...func(*iam.Options)) (*iam.GetPolicyOutput, error) {
	return &iam.GetPolicyOutput{Policy: &iamtypes.Policy{
		Arn:              in.PolicyArn,
		DefaultVersionId: aws.String("v1"),
	}}, nil
}

func (f *codex2PolicyFake) GetPolicyVersion(_ context.Context, _ *iam.GetPolicyVersionInput, _ ...func(*iam.Options)) (*iam.GetPolicyVersionOutput, error) {
	return &iam.GetPolicyVersionOutput{PolicyVersion: &iamtypes.PolicyVersion{
		VersionId: aws.String("v1"),
		Document:  aws.String(f.doc),
	}}, nil
}

// TestIAMPolicyAdmin_SingleObjectStatementIsAdmin pins finding 6: AWS accepts
// a bare Statement object as well as an array. The local detector read only
// the array form, so an admin policy written the other legal way was missed
// by the very check iampolicy.Document.IsAdmin already answers.
func TestIAMPolicyAdmin_SingleObjectStatementIsAdmin(t *testing.T) {
	const arn = "arn:aws:iam::123456789012:policy/acme-admin"
	const doc = `{"Version":"2012-10-17","Statement":{"Effect":"Allow","Action":"*","Resource":"*"}}`

	rows := []resource.Resource{{ID: arn, Name: "acme-admin", Type: "policy",
		Fields: map[string]string{"attachment_count": "1"}}}
	res, err := awsclient.EnrichIAMPolicy(context.Background(),
		&awsclient.ServiceClients{IAM: &codex2PolicyFake{doc: doc}}, rows, nil)
	if err != nil {
		t.Fatalf("EnrichIAMPolicy: %v", err)
	}
	if !codex2HasCode(res.Findings[arn], "iam-policy.admin-star") {
		t.Fatalf("single-object Statement produced no iam-policy.admin-star; findings=%+v\n"+
			"finding 6: the local detector accepts only an array Statement", res.Findings[arn])
	}
}

// ─── Finding 7 — the vpce full-access finding ignores Resource ──────────────

// TestVPCEPolicyOpen_BucketScopedWildcardIsNotFullAccess pins finding 7: a
// wildcard principal and a wildcard action confined to ONE bucket is not the
// unrestricted default policy AWS attaches to a new endpoint.
func TestVPCEPolicyOpen_BucketScopedWildcardIsNotFullAccess(t *testing.T) {
	const doc = `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":"*","Action":"*","Resource":["arn:aws:s3:::acme-reports","arn:aws:s3:::acme-reports/*"]}]}`
	rows := w3FetchVPCEs(t, w3VPCE("vpce-0aa11bb22cc33dd44", "Available", doc))
	if f, ok := w3FindingByCode(rows[0].Findings, domain.FindingCode("vpce.policy-open")); ok {
		t.Errorf("got %s (%q) for a bucket-scoped wildcard grant, want no finding\n"+
			"finding 7: the check reads Principal and Action but never Resource", f.Code, f.Phrase)
	}
	if got := rows[0].Fields["policy_exposure"]; got != "scoped" {
		t.Errorf("policy_exposure = %q, want %q", got, "scoped")
	}
}

// ─── Finding 8 — PowerUserAccess is not administrator-equivalent ────────────

// TestAdminManagedPolicySet_ExcludesPowerUserAccess pins finding 8: AWS's own
// definition of PowerUserAccess excludes IAM, Organizations and Account
// management, so a principal holding only it is not an administrator.
func TestAdminManagedPolicySet_ExcludesPowerUserAccess(t *testing.T) {
	powerUser := []iamtypes.AttachedPolicy{{
		PolicyName: aws.String("PowerUserAccess"),
		PolicyArn:  aws.String("arn:aws:iam::aws:policy/PowerUserAccess"),
	}}
	if got := awsclient.AdminAttachedPolicyNameForTest(powerUser); got != "" {
		t.Errorf("PowerUserAccess reported as the admin policy %q, want no admin attachment\n"+
			"finding 8: PowerUserAccess grants no IAM, Organizations or Account access", got)
	}

	admin := []iamtypes.AttachedPolicy{{
		PolicyName: aws.String("AdministratorAccess"),
		PolicyArn:  aws.String("arn:aws:iam::aws:policy/AdministratorAccess"),
	}}
	if got := awsclient.AdminAttachedPolicyNameForTest(admin); got != "AdministratorAccess" {
		t.Errorf("AdminAttachedPolicyNameForTest(AdministratorAccess) = %q, want %q", got, "AdministratorAccess")
	}
}

// ─── Finding 9 — two AWS error-code lists ───────────────────────────────────

func codex2APIErr(code string) error {
	return &smithy.GenericAPIError{Code: code, Message: "synthetic " + code}
}

// TestErrClass_ReadsTheSameCodeTableAsTheRetryClassifier pins finding 9:
// SlowDown is retryable in ClassifyAWSError but classified as a generic error
// by ErrClass, and UnauthorizedOperation is EC2's spelling of access denied.
func TestErrClass_ReadsTheSameCodeTableAsTheRetryClassifier(t *testing.T) {
	for _, tc := range []struct{ code, want string }{
		{"SlowDown", awsclient.ClassThrottled},
		{"Throttling", awsclient.ClassThrottled},
		{"ThrottlingException", awsclient.ClassThrottled},
		{"TooManyRequestsException", awsclient.ClassThrottled},
		{"RequestLimitExceeded", awsclient.ClassThrottled},
		{"UnauthorizedOperation", awsclient.ClassAccessDenied},
		{"AccessDenied", awsclient.ClassAccessDenied},
		{"AccessDeniedException", awsclient.ClassAccessDenied},
		{"ExpiredToken", "expired"},
		{"ExpiredTokenException", "expired"},
		{"RequestExpired", "expired"},
	} {
		if got := awsclient.ErrClass(codex2APIErr(tc.code)); got != tc.want {
			t.Errorf("ErrClass(%s) = %q, want %q", tc.code, got, tc.want)
		}
	}
	// A code neither table names still passes through as itself.
	if got := awsclient.ErrClass(codex2APIErr("ValidationException")); got != "ValidationException" {
		t.Errorf("ErrClass(ValidationException) = %q, want the code itself", got)
	}
}

// TestErrClass_RetryClassifierAgreesOnSlowDown pins the other reader of the
// one table: a class of "throttled" and a retryable verdict are the same fact.
func TestErrClass_RetryClassifierAgreesOnSlowDown(t *testing.T) {
	for _, code := range []string{"SlowDown", "Throttling", "ThrottlingException", "TooManyRequestsException", "RequestLimitExceeded"} {
		_, _, retryable := awsclient.ClassifyAWSError(codex2APIErr(code))
		if !retryable {
			t.Errorf("ClassifyAWSError(%s) retryable = false, want true", code)
		}
		if got := awsclient.ErrClass(codex2APIErr(code)); got != awsclient.ClassThrottled {
			t.Errorf("ErrClass(%s) = %q, want %q", code, got, awsclient.ClassThrottled)
		}
	}
	if _, _, retryable := awsclient.ClassifyAWSError(codex2APIErr("UnauthorizedOperation")); retryable {
		t.Error("ClassifyAWSError(UnauthorizedOperation) retryable = true, want false")
	}
}

// TestSweepTitle_AllDeniedReadsAsAccessDenied pins the operator-facing end of
// the same table: an account-wide sweep in which every probe was refused with
// EC2's spelling says "access denied", not "error".
func TestSweepTitle_AllDeniedReadsAsAccessDenied(t *testing.T) {
	word := awsclient.RowWord(awsclient.ErrClass(codex2APIErr("UnauthorizedOperation")))
	if got := awsclient.SweepTitleForWord(word); got != "sweep: access denied" {
		t.Errorf("SweepTitleForWord(%q) = %q, want %q", word, got, "sweep: access denied")
	}
}

// ─── Finding 10 — the overlap script's branch→file name is non-injective ────

// TestTaskFileOverlapScript_DistinctBranchesGetDistinctFiles pins finding 10:
// "task/a_b" and "task_a/b" both map to "task_a_b", so one branch's file list
// overwrites the other's and the clash rule reports PASS on a real overlap.
func TestTaskFileOverlapScript_DistinctBranchesGetDistinctFiles(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	script, err := os.ReadFile(filepath.Join("..", "..", "scripts", "task-file-overlap.sh"))
	if err != nil {
		t.Fatalf("reading the script: %v", err)
	}

	repo := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.CommandContext(t.Context(), args[0], args[1:]...)
		cmd.Dir = repo
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=a9s", "GIT_AUTHOR_EMAIL=a9s@example.invalid",
			"GIT_COMMITTER_NAME=a9s", "GIT_COMMITTER_EMAIL=a9s@example.invalid")
		out, runErr := cmd.CombinedOutput()
		if runErr != nil {
			t.Fatalf("%v: %v\n%s", args, runErr, out)
		}
		return string(out)
	}
	write := func(name, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(filepath.Join(repo, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(repo, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	run("git", "init", "-q", "-b", "main", ".")
	write("shared.go", "package shared\n")
	run("git", "add", "-A")
	run("git", "commit", "-qm", "base")

	// The script resolves its own repo root from its location.
	write("scripts/task-file-overlap.sh", string(script))
	if err := os.Chmod(filepath.Join(repo, "scripts", "task-file-overlap.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	run("git", "add", "-A")
	run("git", "commit", "-qm", "script")

	for _, br := range []string{"task/a_b", "task_a/b"} {
		run("git", "checkout", "-q", "-b", br, "main")
		write("shared.go", "package shared // touched by "+br+"\n")
		run("git", "add", "-A")
		run("git", "commit", "-qm", "touch "+br)
	}
	run("git", "checkout", "-q", "main")

	cmd := exec.CommandContext(t.Context(), "bash", "scripts/task-file-overlap.sh", "task/a_b", "task_a/b")
	cmd.Dir = repo
	out, runErr := cmd.CombinedOutput()
	if runErr == nil {
		t.Fatalf("the script reported no overlap for two branches that both touch shared.go:\n%s\n"+
			"finding 10: `tr / _` maps \"task/a_b\" and \"task_a/b\" to the same temp file", out)
	}
	if !strings.Contains(string(out), "shared.go") {
		t.Errorf("overlap output does not name shared.go:\n%s", out)
	}
}

// ─── w132 — the title fallback ranges over a map with case-duplicate keys ───

// TestExtractCellValue_CaseDuplicateFieldKeysAreDeterministic pins w132: two
// Fields keys differing only by case both satisfy the EqualFold match, so the
// cell used to show whichever key Go's map iteration reached first.
func TestExtractCellValue_CaseDuplicateFieldKeysAreDeterministic(t *testing.T) {
	col := app.ColumnDef{Title: "Time"}
	r := resource.Resource{
		ID:     "row-1",
		Fields: map[string]string{"Time": "2026-09-08 06:29", "time": "2026-01-01 00:00"},
	}
	first := app.ExtractCellValue(col, nil, r)
	for i := range 50 {
		if got := app.ExtractCellValue(col, nil, r); got != first {
			t.Fatalf("iteration %d rendered %q after %q\n"+
				"w132: the title fallback ranges over Fields, so two keys differing only by "+
				"case make the cell depend on map order", i, got, first)
		}
	}
}
