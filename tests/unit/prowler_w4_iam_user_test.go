package unit

// prowler_w4_iam_user_test.go — behavioural tests for the batch-w4 iam-user
// rows: an admin-equivalent managed policy attached to a human user, a
// console password that was never used, an access key nobody has signed a
// request with, and both key slots active at once.

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	w4CodeUserAdminAttached   domain.FindingCode = "iam-user.admin-attached"
	w4CodeUserConsoleNeverUse domain.FindingCode = "iam-user.console-never-used"
	w4CodeUserKeyUnused       domain.FindingCode = "iam-user.access-key-unused"
	w4CodeUserTwoActiveKeys   domain.FindingCode = "iam-user.two-active-keys"

	w4PhraseUserTwoActiveKeys = "two active access keys"
	w4PhraseUserConsoleNever  = "console password never used"
	w4SourceUserWave2         = "wave2"
)

// w4UserFake serves the five calls the user enricher makes. Every map is
// keyed by user name so one batch can carry a flagged user and a healthy one.
type w4UserFake struct {
	awsclient.IAMAPI
	consoleUsers map[string]bool
	mfaUsers     map[string]bool
	keys         map[string][]iamtypes.AccessKeyMetadata
	// keyLastUsed maps AccessKeyId → last-use time; a missing entry means the
	// key has never been used.
	keyLastUsed map[string]*time.Time
	// attached maps user name → attached policy name → ARN.
	attached map[string]map[string]string
}

func (f *w4UserFake) GetLoginProfile(
	_ context.Context, in *iam.GetLoginProfileInput, _ ...func(*iam.Options),
) (*iam.GetLoginProfileOutput, error) {
	name := aws.ToString(in.UserName)
	if !f.consoleUsers[name] {
		return nil, &iamtypes.NoSuchEntityException{Message: aws.String("Login Profile for user cannot be found")}
	}
	created := time.Now().Add(-200 * 24 * time.Hour)
	return &iam.GetLoginProfileOutput{LoginProfile: &iamtypes.LoginProfile{
		UserName: in.UserName, CreateDate: &created,
	}}, nil
}

func (f *w4UserFake) ListMFADevices(
	_ context.Context, in *iam.ListMFADevicesInput, _ ...func(*iam.Options),
) (*iam.ListMFADevicesOutput, error) {
	name := aws.ToString(in.UserName)
	if !f.mfaUsers[name] {
		return &iam.ListMFADevicesOutput{}, nil
	}
	return &iam.ListMFADevicesOutput{MFADevices: []iamtypes.MFADevice{{
		UserName:     in.UserName,
		SerialNumber: aws.String("arn:aws:iam::123456789012:mfa/" + name),
	}}}, nil
}

func (f *w4UserFake) ListAccessKeys(
	_ context.Context, in *iam.ListAccessKeysInput, _ ...func(*iam.Options),
) (*iam.ListAccessKeysOutput, error) {
	return &iam.ListAccessKeysOutput{AccessKeyMetadata: f.keys[aws.ToString(in.UserName)]}, nil
}

func (f *w4UserFake) GetAccessKeyLastUsed(
	_ context.Context, in *iam.GetAccessKeyLastUsedInput, _ ...func(*iam.Options),
) (*iam.GetAccessKeyLastUsedOutput, error) {
	id := aws.ToString(in.AccessKeyId)
	return &iam.GetAccessKeyLastUsedOutput{
		UserName: aws.String("acme-user"),
		AccessKeyLastUsed: &iamtypes.AccessKeyLastUsed{
			ServiceName:  aws.String("s3"),
			Region:       aws.String("us-east-1"),
			LastUsedDate: f.keyLastUsed[id],
		},
	}, nil
}

func (f *w4UserFake) ListAttachedUserPolicies(
	_ context.Context, in *iam.ListAttachedUserPoliciesInput, _ ...func(*iam.Options),
) (*iam.ListAttachedUserPoliciesOutput, error) {
	var out []iamtypes.AttachedPolicy
	for name, arn := range f.attached[aws.ToString(in.UserName)] {
		out = append(out, iamtypes.AttachedPolicy{
			PolicyName: aws.String(name), PolicyArn: aws.String(arn),
		})
	}
	return &iam.ListAttachedUserPoliciesOutput{AttachedPolicies: out}, nil
}

var _ awsclient.IAMAPI = (*w4UserFake)(nil)

// w4AccessKey builds a ListAccessKeys entry.
func w4AccessKey(user, id string, status iamtypes.StatusType, ageDays int) iamtypes.AccessKeyMetadata {
	created := time.Now().Add(-time.Duration(ageDays) * 24 * time.Hour)
	return iamtypes.AccessKeyMetadata{
		UserName:    aws.String(user),
		AccessKeyId: aws.String(id),
		Status:      status,
		CreateDate:  &created,
	}
}

// w4UserResource builds a user resource the way the user fetcher does.
// passwordLastUsed is "Never" for a password that has not been used.
func w4UserResource(name string, createdDaysAgo int, passwordLastUsed string) resource.Resource {
	created := time.Now().Add(-time.Duration(createdDaysAgo) * 24 * time.Hour)
	return resource.Resource{
		ID: name, Name: name, Type: "iam-user",
		Fields: map[string]string{
			"user_name":            name,
			"create_date":          created.Format("2006-01-02 15:04"),
			"password_last_used":   passwordLastUsed,
			"has_console_password": "false",
		},
		RawStruct: iamtypes.User{UserName: aws.String(name), CreateDate: &created},
	}
}

func w4EnrichUsers(t *testing.T, fake *w4UserFake, rs []resource.Resource) awsclient.IssueEnricherResult {
	t.Helper()
	res, err := w4EnrichUsersErr(t, fake, rs)
	if err != nil {
		t.Fatalf("EnrichIAMUserMFA: %v", err)
	}
	return res
}

// w4EnrichUsersErr is w4EnrichUsers for the case that expects a refusal: a
// call the role may not make is a recorded failure ("skipped" spec row 5).
func w4EnrichUsersErr(t *testing.T, fake *w4UserFake, rs []resource.Resource) (awsclient.IssueEnricherResult, error) {
	t.Helper()
	clients := &awsclient.ServiceClients{IAM: fake, Region: "us-east-1"}
	return awsclient.EnrichIAMUserMFA(context.Background(), clients, rs, nil)
}

// --- row 6: admin policy attached -------------------------------------------

// TestW4UserAdminAttached pins that a user holding an admin-equivalent
// managed policy is flagged and a user with a scoped policy is not.
func TestW4UserAdminAttached(t *testing.T) {
	fake := &w4UserFake{
		mfaUsers: map[string]bool{"acme-ops-user": true, "acme-reports-user": true},
		attached: map[string]map[string]string{
			"acme-ops-user":     {"AdministratorAccess": w4AdminAccessARN},
			"acme-reports-user": {"AmazonS3ReadOnlyAccess": "arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess"},
		},
	}
	res := w4EnrichUsers(t, fake, []resource.Resource{
		w4UserResource("acme-ops-user", 400, "2026-01-02 09:00"),
		w4UserResource("acme-reports-user", 400, "2026-01-02 09:00"),
	})

	w4AssertFinding(t, res.Findings["acme-ops-user"], w4CodeUserAdminAttached,
		"has an administrator policy", domain.SevWarn, w4SourceUserWave2)
	w4AssertRows(t, res.AttentionDetails["acme-ops-user"], w4CodeUserAdminAttached,
		[]domain.DetailRow{{Label: "Policy", Value: "AdministratorAccess"}})
	w4AssertNoCode(t, res.Findings["acme-reports-user"], w4CodeUserAdminAttached)
}

// --- row 7: console password never used -------------------------------------

// TestW4UserConsoleNeverUsed pins the unguarded sign-in path: a console
// password created long enough ago to be past the grace period and never used
// once.
func TestW4UserConsoleNeverUsed(t *testing.T) {
	user := w4UserResource("acme-dormant-console-user", 200, "Never")
	fake := &w4UserFake{
		consoleUsers: map[string]bool{"acme-dormant-console-user": true},
		mfaUsers:     map[string]bool{"acme-dormant-console-user": true},
	}
	res := w4EnrichUsers(t, fake, []resource.Resource{user})

	w4AssertFinding(t, res.Findings["acme-dormant-console-user"], w4CodeUserConsoleNeverUse,
		w4PhraseUserConsoleNever, domain.SevWarn, w4SourceUserWave2)
	w4AssertRows(t, res.AttentionDetails["acme-dormant-console-user"], w4CodeUserConsoleNeverUse,
		[]domain.DetailRow{{Label: "Created", Value: user.Fields["create_date"]}})
}

// TestW4UserConsoleNeverUsedNegatives pins the three healthy counterparts: the
// password has been used, the user is too new to judge, and the user has no
// console password at all.
func TestW4UserConsoleNeverUsedNegatives(t *testing.T) {
	cases := []struct {
		name             string
		console          bool
		createdDaysAgo   int
		passwordLastUsed string
	}{
		{"password_has_been_used", true, 400, "2026-06-01 10:30"},
		{"user_within_grace_period", true, 10, "Never"},
		{"no_console_password", false, 400, "Never"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := &w4UserFake{
				consoleUsers: map[string]bool{"acme-user": tc.console},
				mfaUsers:     map[string]bool{"acme-user": true},
			}
			res := w4EnrichUsers(t, fake, []resource.Resource{
				w4UserResource("acme-user", tc.createdDaysAgo, tc.passwordLastUsed),
			})
			w4AssertNoCode(t, res.Findings["acme-user"], w4CodeUserConsoleNeverUse)
		})
	}
}

// --- row 8: access key unused -----------------------------------------------

// TestW4UserAccessKeyNeverUsed pins the live-but-untouched credential: an
// active key that has never signed a request, counted from its creation. The
// row shows only the last four characters of the key id.
func TestW4UserAccessKeyNeverUsed(t *testing.T) {
	fake := &w4UserFake{
		mfaUsers: map[string]bool{"acme-batch-user": true},
		keys: map[string][]iamtypes.AccessKeyMetadata{
			"acme-batch-user": {w4AccessKey("acme-batch-user", "AKIAIOSFODNN7EXAMPLE", iamtypes.StatusTypeActive, 200)},
		},
	}
	res := w4EnrichUsers(t, fake, []resource.Resource{w4UserResource("acme-batch-user", 400, "Never")})

	// Inverted for spec row "phrase": the idle days were in the phrase, so the
	// first idle key's number stood for every idle key on the user. They are an
	// Idle row now. Do not restore the old phrase.
	w4AssertFinding(t, res.Findings["acme-batch-user"], w4CodeUserKeyUnused,
		catalog.Phrase(w4CodeUserKeyUnused), domain.SevWarn, w4SourceUserWave2)
	w4AssertRows(t, res.AttentionDetails["acme-batch-user"], w4CodeUserKeyUnused, []domain.DetailRow{
		{Label: "Key", Value: "…MPLE"},
		{Label: "Last used", Value: "never"},
		{Label: "Idle", Value: "200 days"},
	})
}

// TestW4UserAccessKeyIdleSinceLastUse pins the second half of the row: a key
// that was used once and then abandoned is measured from that use, not from
// its creation.
func TestW4UserAccessKeyIdleSinceLastUse(t *testing.T) {
	lastUsed := time.Now().Add(-120 * 24 * time.Hour)
	fake := &w4UserFake{
		mfaUsers:    map[string]bool{"acme-batch-user": true},
		keyLastUsed: map[string]*time.Time{"AKIAIOSFODNN7EXAMPLE": &lastUsed},
		keys: map[string][]iamtypes.AccessKeyMetadata{
			"acme-batch-user": {w4AccessKey("acme-batch-user", "AKIAIOSFODNN7EXAMPLE", iamtypes.StatusTypeActive, 400)},
		},
	}
	res := w4EnrichUsers(t, fake, []resource.Resource{w4UserResource("acme-batch-user", 400, "Never")})

	w4AssertFinding(t, res.Findings["acme-batch-user"], w4CodeUserKeyUnused,
		catalog.Phrase(w4CodeUserKeyUnused), domain.SevWarn, w4SourceUserWave2)
	w4AssertRows(t, res.AttentionDetails["acme-batch-user"], w4CodeUserKeyUnused, []domain.DetailRow{
		{Label: "Key", Value: "…MPLE"},
		{Label: "Last used", Value: lastUsed.Format("2006-01-02")},
		{Label: "Idle", Value: "120 days"},
	})
}

// TestW4UserAccessKeyUnusedNegatives pins the healthy counterparts: a key in
// regular use, and an inactive key (already deactivated, nothing to do).
func TestW4UserAccessKeyUnusedNegatives(t *testing.T) {
	t.Run("key_in_regular_use", func(t *testing.T) {
		recent := time.Now().Add(-3 * 24 * time.Hour)
		fake := &w4UserFake{
			mfaUsers:    map[string]bool{"acme-batch-user": true},
			keyLastUsed: map[string]*time.Time{"AKIAIOSFODNN7EXAMPLE": &recent},
			keys: map[string][]iamtypes.AccessKeyMetadata{
				"acme-batch-user": {w4AccessKey("acme-batch-user", "AKIAIOSFODNN7EXAMPLE", iamtypes.StatusTypeActive, 400)},
			},
		}
		res := w4EnrichUsers(t, fake, []resource.Resource{w4UserResource("acme-batch-user", 400, "Never")})
		w4AssertNoCode(t, res.Findings["acme-batch-user"], w4CodeUserKeyUnused)
	})

	t.Run("inactive_key", func(t *testing.T) {
		fake := &w4UserFake{
			mfaUsers: map[string]bool{"acme-batch-user": true},
			keys: map[string][]iamtypes.AccessKeyMetadata{
				"acme-batch-user": {w4AccessKey("acme-batch-user", "AKIAIOSFODNN7EXAMPLE", iamtypes.StatusTypeInactive, 400)},
			},
		}
		res := w4EnrichUsers(t, fake, []resource.Resource{w4UserResource("acme-batch-user", 400, "Never")})
		w4AssertNoCode(t, res.Findings["acme-batch-user"], w4CodeUserKeyUnused)
	})
}

// TestW4UserKeyIDNeverLeavesThePackage pins the redaction: no emitted string
// may carry the full access key id.
func TestW4UserKeyIDNeverLeavesThePackage(t *testing.T) {
	const keyID = "AKIAIOSFODNN7EXAMPLE"
	fake := &w4UserFake{
		mfaUsers: map[string]bool{"acme-batch-user": true},
		keys: map[string][]iamtypes.AccessKeyMetadata{
			"acme-batch-user": {w4AccessKey("acme-batch-user", keyID, iamtypes.StatusTypeActive, 200)},
		},
	}
	res := w4EnrichUsers(t, fake, []resource.Resource{w4UserResource("acme-batch-user", 400, "Never")})

	f, ok := w4FindingByCode(t, res.Findings["acme-batch-user"], w4CodeUserKeyUnused)
	if !ok {
		t.Fatalf("no %q finding", w4CodeUserKeyUnused)
	}
	for _, s := range []string{f.Phrase, f.Detail} {
		if strings.Contains(s, keyID) {
			t.Errorf("emitted text carries the full key id: %q", s)
		}
	}
	for _, row := range res.AttentionDetails["acme-batch-user"][w4CodeUserKeyUnused].Rows {
		if strings.Contains(row.Value, keyID) {
			t.Errorf("row %q carries the full key id: %q", row.Label, row.Value)
		}
	}
}

// --- row 9: two active keys -------------------------------------------------

// TestW4UserTwoActiveKeys pins the doubled-exposure row: both key slots
// active at once, which also means a rotation was never finished.
func TestW4UserTwoActiveKeys(t *testing.T) {
	fake := &w4UserFake{
		mfaUsers: map[string]bool{"acme-ci-user": true},
		keys: map[string][]iamtypes.AccessKeyMetadata{
			"acme-ci-user": {
				w4AccessKey("acme-ci-user", "AKIAIOSFODNN7EXAMPL1", iamtypes.StatusTypeActive, 10),
				w4AccessKey("acme-ci-user", "AKIAIOSFODNN7EXAMPL2", iamtypes.StatusTypeActive, 5),
			},
		},
	}
	res := w4EnrichUsers(t, fake, []resource.Resource{w4UserResource("acme-ci-user", 400, "Never")})

	w4AssertFinding(t, res.Findings["acme-ci-user"], w4CodeUserTwoActiveKeys,
		w4PhraseUserTwoActiveKeys, domain.SevWarn, w4SourceUserWave2)
	w4AssertRows(t, res.AttentionDetails["acme-ci-user"], w4CodeUserTwoActiveKeys,
		[]domain.DetailRow{{Label: "Keys", Value: "2 active"}})
}

// TestW4UserTwoActiveKeysNegatives pins that only genuinely active pairs
// count: one active key beside a deactivated one is a completed rotation.
func TestW4UserTwoActiveKeysNegatives(t *testing.T) {
	fake := &w4UserFake{
		mfaUsers: map[string]bool{"acme-ci-user": true},
		keys: map[string][]iamtypes.AccessKeyMetadata{
			"acme-ci-user": {
				w4AccessKey("acme-ci-user", "AKIAIOSFODNN7EXAMPL1", iamtypes.StatusTypeActive, 10),
				w4AccessKey("acme-ci-user", "AKIAIOSFODNN7EXAMPL2", iamtypes.StatusTypeInactive, 400),
			},
		},
	}
	res := w4EnrichUsers(t, fake, []resource.Resource{w4UserResource("acme-ci-user", 400, "Never")})
	w4AssertNoCode(t, res.Findings["acme-ci-user"], w4CodeUserTwoActiveKeys)
}

// --- independence and batch resilience --------------------------------------

// TestW4UserIndependentConditionsEachGetAFinding pins that conditions do not
// mask one another: one user can be an admin, have a never-used console
// password, and hold two idle keys, and every one of those is reported with
// its own code and phrase.
func TestW4UserIndependentConditionsEachGetAFinding(t *testing.T) {
	user := w4UserResource("acme-legacy-admin", 400, "Never")
	fake := &w4UserFake{
		consoleUsers: map[string]bool{"acme-legacy-admin": true},
		mfaUsers:     map[string]bool{"acme-legacy-admin": true},
		attached: map[string]map[string]string{
			"acme-legacy-admin": {"AdministratorAccess": w4AdminAccessARN},
		},
		keys: map[string][]iamtypes.AccessKeyMetadata{
			"acme-legacy-admin": {
				w4AccessKey("acme-legacy-admin", "AKIAIOSFODNN7EXAMPL1", iamtypes.StatusTypeActive, 200),
				w4AccessKey("acme-legacy-admin", "AKIAIOSFODNN7EXAMPL2", iamtypes.StatusTypeActive, 300),
			},
		},
	}
	res := w4EnrichUsers(t, fake, []resource.Resource{user})

	for _, code := range []domain.FindingCode{
		w4CodeUserAdminAttached, w4CodeUserConsoleNeverUse, w4CodeUserTwoActiveKeys, w4CodeUserKeyUnused,
	} {
		if _, ok := w4FindingByCode(t, res.Findings["acme-legacy-admin"], code); !ok {
			t.Errorf("missing %q; got %s", code, w4CodesOf(res.Findings["acme-legacy-admin"]))
		}
	}
}

// w4UserListKeysErrFake fails ListAccessKeys for one named user.
type w4UserListKeysErrFake struct {
	*w4UserFake
	failFor string
}

func (f *w4UserListKeysErrFake) ListAccessKeys(
	ctx context.Context, in *iam.ListAccessKeysInput, opts ...func(*iam.Options),
) (*iam.ListAccessKeysOutput, error) {
	if aws.ToString(in.UserName) == f.failFor {
		return nil, errors.New("Throttling: rate exceeded")
	}
	return f.w4UserFake.ListAccessKeys(ctx, in, opts...)
}

// TestW4UserBatchErrorLeavesSiblingsEvaluated pins that a failed call on one
// user marks that user unknown and does not stop the others being judged.
func TestW4UserBatchErrorLeavesSiblingsEvaluated(t *testing.T) {
	base := &w4UserFake{
		mfaUsers: map[string]bool{"acme-ci-user": true, "acme-broken-user": true},
		keys: map[string][]iamtypes.AccessKeyMetadata{
			"acme-ci-user": {
				w4AccessKey("acme-ci-user", "AKIAIOSFODNN7EXAMPL1", iamtypes.StatusTypeActive, 10),
				w4AccessKey("acme-ci-user", "AKIAIOSFODNN7EXAMPL2", iamtypes.StatusTypeActive, 5),
			},
		},
	}
	fake := &w4UserListKeysErrFake{w4UserFake: base, failFor: "acme-broken-user"}
	clients := &awsclient.ServiceClients{IAM: fake, Region: "us-east-1"}
	res, err := awsclient.EnrichIAMUserMFA(context.Background(), clients, []resource.Resource{
		w4UserResource("acme-broken-user", 400, "Never"),
		w4UserResource("acme-ci-user", 400, "Never"),
	}, nil)
	// INVERTED for the "skipped" spec row 5: this required err == nil, so the
	// user whose keys could not be listed was marked "?" with nothing in the
	// log to say what refused. The siblings are still evaluated below.
	if err == nil {
		t.Fatal("a failed ListAccessKeys returned no error")
	}

	if _, marked := res.TruncatedIDs["acme-broken-user"]; !marked {
		t.Errorf("TruncatedIDs[acme-broken-user] = false, want true")
	}
	if len(res.Findings["acme-broken-user"]) != 0 {
		t.Errorf("a failed call produced findings: %s", w4CodesOf(res.Findings["acme-broken-user"]))
	}
	w4AssertFinding(t, res.Findings["acme-ci-user"], w4CodeUserTwoActiveKeys,
		w4PhraseUserTwoActiveKeys, domain.SevWarn, w4SourceUserWave2)
}

// TestW4UserFindingDefs pins the registry rows for the four new user codes.
func TestW4UserFindingDefs(t *testing.T) {
	cases := []struct {
		code   domain.FindingCode
		phrase string
	}{
		{w4CodeUserAdminAttached, "has an administrator policy"},
		{w4CodeUserConsoleNeverUse, w4PhraseUserConsoleNever},
		// Inverted for spec row "phrase": the idle days moved to a supporting
		// row, so the declaration no longer carries a placeholder for them.
		{w4CodeUserKeyUnused, "access key unused"},
		{w4CodeUserTwoActiveKeys, w4PhraseUserTwoActiveKeys},
	}
	for _, tc := range cases {
		t.Run(string(tc.code), func(t *testing.T) {
			def := w4FindingDef(t, "iam-user", tc.code)
			if def.Phrase != tc.phrase {
				t.Errorf("Phrase = %q, want %q", def.Phrase, tc.phrase)
			}
			if def.Severity != domain.SevWarn {
				t.Errorf("Severity = %v, want SevWarn", def.Severity)
			}
			if def.Source != "wave2" {
				t.Errorf("Source = %q, want wave2", def.Source)
			}
		})
	}
}

// TestW4UserNilClient pins the nil-client contract.
func TestW4UserNilClient(t *testing.T) {
	res, err := awsclient.EnrichIAMUserMFA(context.Background(), &awsclient.ServiceClients{},
		[]resource.Resource{w4UserResource("acme-ci-user", 400, "Never")}, nil)
	if err != nil {
		t.Fatalf("EnrichIAMUserMFA: %v", err)
	}
	if res.Findings == nil || res.TruncatedIDs == nil || res.FieldUpdates == nil {
		t.Fatalf("result maps must be non-nil")
	}
	if len(res.Findings) != 0 {
		t.Errorf("Findings = %v, want empty", res.Findings)
	}
}
