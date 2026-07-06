// aws_classifier_fivepack_test.go — pins five verified production bugs found
// during the demo-state-coverage wave (qa_demo_state_coverage_test.go), each
// mirrored by a REMOVED allowlist entry there so the ratchet goes red until
// the paired coder fix lands in the same PR:
//
//  1. colorKMS reads Fields["key_state"], but FetchKMSKeysPage only ever
//     writes Fields["status"] — Broken/Dim/Warning are unreachable through
//     the real fetcher + real classifier pair. Per docs/resources/kms.md §3.2
//     the documented buckets are: PendingDeletion/PendingImport -> Broken,
//     Disabled -> Warning, Unavailable -> Broken.
//  2. colorIAMUser requires Fields["has_console_password"]=="true", but
//     FetchIAMUsersPage hardcodes "false" and EnrichIAMUserMFA (the type's
//     only Wave-2 enricher) never writes that field back — it only updates
//     "mfa" and "risk". Per docs/resources/iam-user.md §3.2 a console user
//     without MFA classifies Broken via the wave2 finding path (not via
//     colorIAMUser's dead "true" branch at all).
//  3. colorLambda checks Fields["dlq_target_arn"]=="" (forcing Warning)
//     BEFORE the Inactive->Dim and Healthy-fallthrough branches, but
//     FetchLambdaFunctionsPage never writes a "dlq_target_arn" field —
//     every non-Failed, non-deprecated-runtime function is permanently
//     forced into Warning regardless of its real state or DLQ config.
//  4. colorRedis matches phrase=="deleted", but computeRedisFindings has no
//     "deleted" case (docs/resources/redis.md §3.1/§3.2/§5 document no
//     deleted/dim state for redis at all — ElastiCache simply stops
//     returning a torn-down replication group instead of reporting
//     "deleted"). This is dead code: the pin is a source-scan asserting
//     colorRedis has no unreachable "deleted" branch, not fabricated
//     fixture data.
//  5. EnrichRDSDocDBMaintenance (rds_issue_enrichment.go) is a near-duplicate
//     of the already-wired EnrichDBIMaintenance (dbi's Wave2 in
//     catalog_databases.go) but is never itself assigned to any catalog
//     Wave2 field — it is dead code, referenced only by its own test suite.
//     Verdict for the "is a distinct rds.pending-maintenance signal
//     documented anywhere" question: docs/resources/dbi.md §4 documents
//     "Pending maintenance overdue" (dbi.pending-maintenance, Warning on
//     Healthy row) and docs/resources/dbc.md §3.2/§4 documents "Cluster has
//     a pending maintenance action ... -> Warning" / "maintenance overdue"
//     (dbc side, Broken/!). Both are served by EnrichDBIMaintenance and
//     EnrichDBCMaintenance respectively — there is no third, distinct
//     "rds" signal anywhere in the golden docs. EnrichRDSDocDBMaintenance
//     duplicates the dbi half of that coverage under a different resource
//     name that no catalog entry uses.
package unit_test

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// Bug 1 — colorKMS reads a field FetchKMSKeysPage never writes.
// ---------------------------------------------------------------------------

// fakeKMSFivepack drives FetchKMSKeysPage through the exact production code
// path (ListKeys -> ListAliases -> per-key DescribeKey), returning
// customer-managed keys in the three non-Enabled states documented in
// docs/resources/kms.md §3.2.
type fakeKMSFivepack struct {
	keys    []kmstypes.KeyListEntry
	byID    map[string]*kmstypes.KeyMetadata
	aliases []kmstypes.AliasListEntry
}

func (f *fakeKMSFivepack) ListKeys(_ context.Context, _ *kms.ListKeysInput, _ ...func(*kms.Options)) (*kms.ListKeysOutput, error) {
	return &kms.ListKeysOutput{Keys: f.keys, Truncated: false}, nil
}

func (f *fakeKMSFivepack) ListAliases(_ context.Context, _ *kms.ListAliasesInput, _ ...func(*kms.Options)) (*kms.ListAliasesOutput, error) {
	return &kms.ListAliasesOutput{Aliases: f.aliases, Truncated: false}, nil
}

func (f *fakeKMSFivepack) DescribeKey(_ context.Context, in *kms.DescribeKeyInput, _ ...func(*kms.Options)) (*kms.DescribeKeyOutput, error) {
	meta := f.byID[aws.ToString(in.KeyId)]
	return &kms.DescribeKeyOutput{KeyMetadata: meta}, nil
}

// The remaining methods satisfy awsclient.KMSAPI but are unused by
// FetchKMSKeysPage — this pin only exercises ListKeys/ListAliases/DescribeKey.
func (f *fakeKMSFivepack) GetKeyRotationStatus(_ context.Context, _ *kms.GetKeyRotationStatusInput, _ ...func(*kms.Options)) (*kms.GetKeyRotationStatusOutput, error) {
	return &kms.GetKeyRotationStatusOutput{}, nil
}

func (f *fakeKMSFivepack) ListGrants(_ context.Context, _ *kms.ListGrantsInput, _ ...func(*kms.Options)) (*kms.ListGrantsOutput, error) {
	return &kms.ListGrantsOutput{}, nil
}

func (f *fakeKMSFivepack) GetKeyPolicy(_ context.Context, _ *kms.GetKeyPolicyInput, _ ...func(*kms.Options)) (*kms.GetKeyPolicyOutput, error) {
	return &kms.GetKeyPolicyOutput{}, nil
}

func newFakeKMSFivepack() *fakeKMSFivepack {
	mk := func(id string, state kmstypes.KeyState) *kmstypes.KeyMetadata {
		return &kmstypes.KeyMetadata{
			KeyId:      aws.String(id),
			Arn:        aws.String("arn:aws:kms:us-east-1:123456789012:key/" + id),
			KeyState:   state,
			KeyManager: kmstypes.KeyManagerTypeCustomer,
		}
	}
	byID := map[string]*kmstypes.KeyMetadata{
		"key-disabled":         mk("key-disabled", kmstypes.KeyStateDisabled),
		"key-pending-deletion": mk("key-pending-deletion", kmstypes.KeyStatePendingDeletion),
		"key-unavailable":      mk("key-unavailable", kmstypes.KeyStateUnavailable),
	}
	return &fakeKMSFivepack{
		keys: []kmstypes.KeyListEntry{
			{KeyId: aws.String("key-disabled")},
			{KeyId: aws.String("key-pending-deletion")},
			{KeyId: aws.String("key-unavailable")},
		},
		byID: byID,
		aliases: []kmstypes.AliasListEntry{
			{AliasName: aws.String("alias/disabled-key"), TargetKeyId: aws.String("key-disabled")},
			{AliasName: aws.String("alias/pending-deletion-key"), TargetKeyId: aws.String("key-pending-deletion")},
			{AliasName: aws.String("alias/unavailable-key"), TargetKeyId: aws.String("key-unavailable")},
		},
	}
}

// TestColorKMS_RealFetcherReachesDocumentedBuckets drives the real
// FetchKMSKeysPage against Disabled/PendingDeletion/Unavailable customer
// keys and asserts the real td.ResolveColor("kms") lands each in the bucket
// docs/resources/kms.md §3.2 documents: Disabled->Warning,
// PendingDeletion->Broken, Unavailable->Broken. Fails today because
// FetchKMSKeysPage writes Fields["status"] but colorKMS reads
// Fields["key_state"], which is never set — every key falls through to the
// default ColorHealthy branch regardless of its real AWS KeyState.
func TestColorKMS_RealFetcherReachesDocumentedBuckets(t *testing.T) {
	td := resource.FindResourceType("kms")
	if td == nil {
		t.Fatal("resource.FindResourceType(\"kms\") returned nil — kms type not registered")
	}
	if td.Color == nil {
		t.Fatal("kms ResourceTypeDef.Color is nil")
	}

	clients := &awsclient.ServiceClients{KMS: newFakeKMSFivepack()}
	result, err := awsclient.FetchKMSKeysPage(context.Background(), clients, "")
	if err != nil {
		t.Fatalf("FetchKMSKeysPage returned error: %v", err)
	}
	if len(result.Resources) != 3 {
		t.Fatalf("expected 3 fetched KMS keys, got %d", len(result.Resources))
	}

	byID := make(map[string]resource.Resource, len(result.Resources))
	for _, r := range result.Resources {
		byID[r.ID] = r
	}

	cases := []struct {
		id       string
		wantName string
		want     domain.Color
	}{
		{"key-disabled", "Warning", domain.ColorWarning},
		{"key-pending-deletion", "Broken", domain.ColorBroken},
		{"key-unavailable", "Broken", domain.ColorBroken},
	}

	for _, tc := range cases {
		r, ok := byID[tc.id]
		if !ok {
			t.Fatalf("fetched resources missing expected id %q", tc.id)
		}
		got := td.ResolveColor(r)
		if got != tc.want {
			t.Errorf("kms %s: ResolveColor = %v, want %v (%s) per docs/resources/kms.md §3.2 "+
				"(Fields[\"status\"]=%q, Fields[\"key_state\"]=%q)",
				tc.id, got, tc.want, tc.wantName, r.Fields["status"], r.Fields["key_state"])
		}
	}
}

// ---------------------------------------------------------------------------
// Bug 2 — colorIAMUser requires a field the fetcher/enricher pair never sets.
// ---------------------------------------------------------------------------

// TestColorIAMUser_ConsoleUserWithoutMFAClassifiesBroken drives the real
// FetchIAMUsersPage against the demo IAM fixtures (alice.johnson: a console
// user with PasswordLastUsed set and zero registered MFA devices — see
// internal/demo/fixtures/iam.go) and the real Wave-2 EnrichIAMUserMFA
// enricher, then asserts the real td.ResolveColor("iam-user") lands
// alice.johnson in the Broken bucket per docs/resources/iam-user.md §3.2
// ("GetLoginProfile(UserName) returns a profile AND ListMFADevices(UserName)
// returns MFADevices==[] -> State bucket: Broken"). Fails today because
// FetchIAMUsersPage hardcodes Fields["has_console_password"]="false" and
// EnrichIAMUserMFA never writes that field back (it only writes "mfa" and
// "risk" via FieldUpdates) — colorIAMUser's Warning branch requires
// Fields["has_console_password"]=="true", which alice.johnson never carries,
// so she is permanently misclassified Healthy despite the Broken-severity
// wave2 finding actually being emitted.
func TestColorIAMUser_ConsoleUserWithoutMFAClassifiesBroken(t *testing.T) {
	td := resource.FindResourceType("iam-user")
	if td == nil {
		t.Fatal("resource.FindResourceType(\"iam-user\") returned nil — iam-user type not registered")
	}
	if td.Color == nil {
		t.Fatal("iam-user ResourceTypeDef.Color is nil")
	}

	clients := demo.NewServiceClients()
	fetchResult, err := awsclient.FetchIAMUsersPage(context.Background(), clients.IAM, "")
	if err != nil {
		t.Fatalf("FetchIAMUsersPage returned error: %v", err)
	}

	var alice *resource.Resource
	for i := range fetchResult.Resources {
		if fetchResult.Resources[i].Fields["user_name"] == "alice.johnson" {
			alice = &fetchResult.Resources[i]
			break
		}
	}
	if alice == nil {
		t.Fatal("demo IAM fixtures missing expected console-user-without-MFA fixture \"alice.johnson\" " +
			"(internal/demo/fixtures/iam.go ConsoleUsers)")
	}

	enricher, ok := awsclient.Wave2EnricherFor("iam-user")
	if !ok || enricher.Fn == nil {
		t.Fatal("iam-user has no registered Wave-2 enricher — expected EnrichIAMUserMFA")
	}
	cache := resource.ResourceCache{"iam-user": resource.ResourceCacheEntry{Resources: fetchResult.Resources}}
	enrichResult, err := enricher.Fn(context.Background(), clients, fetchResult.Resources, cache)
	if err != nil {
		t.Fatalf("iam-user Wave-2 enricher returned error: %v", err)
	}
	finding, hasFinding := enrichResult.Findings[alice.ID]
	if !hasFinding || finding.Severity != domain.SevBroken {
		t.Fatalf("expected alice.johnson to carry a Broken wave2 finding (console user without MFA, "+
			"CIS IAM.5) from EnrichIAMUserMFA, got finding=%+v hasFinding=%v", finding, hasFinding)
	}
	alice.Findings = append(alice.Findings, finding)

	// Mirror production's field-update application (Controller.ApplyListFieldUpdates
	// -> applyFieldUpdatesToSlice in internal/app/list_body.go): FieldUpdates is a
	// distinct merge step from the Findings append above, keyed by resource ID.
	if kv, ok := enrichResult.FieldUpdates[alice.ID]; ok {
		if alice.Fields == nil {
			alice.Fields = make(map[string]string, len(kv))
		}
		maps.Copy(alice.Fields, kv)
	}

	got := td.ResolveColor(*alice)
	if got != domain.ColorBroken {
		t.Errorf("iam-user alice.johnson: ResolveColor = %v, want %v (Broken) per "+
			"docs/resources/iam-user.md §3.2 console-login-without-MFA signal "+
			"(Fields[\"has_console_password\"]=%q)",
			got, domain.ColorBroken, alice.Fields["has_console_password"])
	}
}

// ---------------------------------------------------------------------------
// Bug 3 — colorLambda's dlq_target_arn check runs before Inactive/Healthy.
// ---------------------------------------------------------------------------

type fakeLambdaFivepack struct {
	functions []lambdatypes.FunctionConfiguration
}

func (f *fakeLambdaFivepack) ListFunctions(_ context.Context, _ *lambda.ListFunctionsInput, _ ...func(*lambda.Options)) (*lambda.ListFunctionsOutput, error) {
	return &lambda.ListFunctionsOutput{Functions: f.functions}, nil
}

// TestColorLambda_RealFetcherReachesDimAndHealthy drives the real
// FetchLambdaFunctionsPage against an Active function that has a
// DeadLetterConfig set (should classify Healthy) and an Inactive function
// (should classify Dim per docs/resources/lambda.md §3.1
// "State in Inactive -> Dim"), and asserts the real td.ResolveColor("lambda")
// reaches both buckets. Fails today because FetchLambdaFunctionsPage never
// writes a "dlq_target_arn" field at all, so colorLambda's
// `Fields["dlq_target_arn"] == ""` check is always true and forces every
// non-Failed, non-deprecated-runtime function into Warning before the
// Inactive->Dim check or the Healthy fallthrough are ever reached.
func TestColorLambda_RealFetcherReachesDimAndHealthy(t *testing.T) {
	td := resource.FindResourceType("lambda")
	if td == nil {
		t.Fatal("resource.FindResourceType(\"lambda\") returned nil — lambda type not registered")
	}
	if td.Color == nil {
		t.Fatal("lambda ResourceTypeDef.Color is nil")
	}

	functions := []lambdatypes.FunctionConfiguration{
		{
			FunctionName:     aws.String("healthy-with-dlq"),
			FunctionArn:      aws.String("arn:aws:lambda:us-east-1:123456789012:function:healthy-with-dlq"),
			Runtime:          lambdatypes.RuntimeNodejs20x,
			State:            lambdatypes.StateActive,
			LastUpdateStatus: lambdatypes.LastUpdateStatusSuccessful,
			DeadLetterConfig: &lambdatypes.DeadLetterConfig{
				TargetArn: aws.String("arn:aws:sqs:us-east-1:123456789012:dead-letter-queue"),
			},
		},
		{
			FunctionName:     aws.String("idle-inactive"),
			FunctionArn:      aws.String("arn:aws:lambda:us-east-1:123456789012:function:idle-inactive"),
			Runtime:          lambdatypes.RuntimeNodejs20x,
			State:            lambdatypes.StateInactive,
			LastUpdateStatus: lambdatypes.LastUpdateStatusSuccessful,
			DeadLetterConfig: &lambdatypes.DeadLetterConfig{
				TargetArn: aws.String("arn:aws:sqs:us-east-1:123456789012:dead-letter-queue"),
			},
		},
	}

	fetchResult, err := awsclient.FetchLambdaFunctionsPage(context.Background(), &fakeLambdaFivepack{functions: functions}, "")
	if err != nil {
		t.Fatalf("FetchLambdaFunctionsPage returned error: %v", err)
	}
	if len(fetchResult.Resources) != 2 {
		t.Fatalf("expected 2 fetched Lambda functions, got %d", len(fetchResult.Resources))
	}

	byID := make(map[string]resource.Resource, len(fetchResult.Resources))
	for _, r := range fetchResult.Resources {
		byID[r.ID] = r
	}

	cases := []struct {
		id       string
		wantName string
		want     domain.Color
	}{
		{"healthy-with-dlq", "Healthy", domain.ColorHealthy},
		{"idle-inactive", "Dim", domain.ColorDim},
	}

	for _, tc := range cases {
		r, ok := byID[tc.id]
		if !ok {
			t.Fatalf("fetched resources missing expected id %q", tc.id)
		}
		got := td.ResolveColor(r)
		if got != tc.want {
			t.Errorf("lambda %s: ResolveColor = %v, want %v (%s) per docs/resources/lambda.md §3.1 "+
				"(Fields[\"state\"]=%q, Fields[\"dlq_target_arn\"]=%q)",
				tc.id, got, tc.want, tc.wantName, r.Fields["state"], r.Fields["dlq_target_arn"])
		}
	}
}

// ---------------------------------------------------------------------------
// Bug 4 — colorRedis has a dead "deleted" branch documented nowhere.
// ---------------------------------------------------------------------------

// TestColorRedis_NoUnreachableDeletedBranch is a source-scan pin: it parses
// internal/aws/catalog_databases.go's colorRedis function body and asserts
// it never compares a phrase/status string against the literal "deleted".
// docs/resources/redis.md §3.1, §3.2, and §5 document no deleted/dim state
// for redis anywhere — real ElastiCache simply stops returning a torn-down
// replication group from DescribeReplicationGroups rather than reporting a
// "deleted" status, and computeRedisFindings' switch has no case that can
// ever produce the phrase "deleted" (only the in-progress "deleting" form).
// The fix removes the dead branch; it does not fabricate fixture data, so
// this pin cannot be satisfied by adding a fixture — it can only be
// satisfied by deleting the phrase=="deleted" comparison from colorRedis.
func TestColorRedis_NoUnreachableDeletedBranch(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed — cannot locate test file")
	}
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	targetFile := filepath.Join(repoRoot, "internal", "aws", "catalog_databases.go")

	fset := token.NewFileSet()
	src, err := parser.ParseFile(fset, targetFile, nil, 0)
	if err != nil {
		t.Fatalf("parse error in %s: %v", targetFile, err)
	}

	var found *ast.FuncDecl
	for _, decl := range src.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name == nil {
			continue
		}
		if fn.Name.Name == "colorRedis" {
			found = fn
			break
		}
	}
	if found == nil {
		t.Fatalf("colorRedis function declaration not found in %s", targetFile)
	}
	if found.Body == nil {
		t.Fatal("colorRedis has no body")
	}

	var hasDeletedLiteral bool
	ast.Inspect(found.Body, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		if strings.Trim(lit.Value, "\"`") == "deleted" {
			hasDeletedLiteral = true
		}
		return true
	})

	if hasDeletedLiteral {
		t.Errorf("colorRedis (internal/aws/catalog_databases.go) still compares a status/phrase string "+
			"against the literal \"deleted\", but docs/resources/redis.md §3.1/§3.2/§5 document no "+
			"deleted/dim state for redis — real ElastiCache never reports a \"deleted\" status "+
			"(torn-down replication groups simply stop appearing in DescribeReplicationGroups), and "+
			"computeRedisFindings has no case that produces that phrase. This branch is dead and "+
			"structurally unreachable; remove it from colorRedis in %s", targetFile)
	}
}

// ---------------------------------------------------------------------------
// Bug 5 — EnrichRDSDocDBMaintenance is dead: wired to no catalog Wave2 field.
// ---------------------------------------------------------------------------

// TestNoOrphanedIssueEnrichmentFunctions walks every internal/aws/
// *_issue_enrichment.go Enrich* top-level function declaration and asserts
// each one is reachable from at least one registered
// catalog.ResourceTypeDef.Wave2 field (cast to awsclient.IssueEnricher) via
// resource.AllResourceTypes() — either directly (its function pointer IS the
// registered Wave2.Fn) or one level of indirection (it is called by name
// from inside a registered Wave2.Fn's body, the "combiner" pattern used by
// e.g. EnrichCFNCombined, which fans out to EnrichCFNStackEvents and
// EnrichCFNDrift). This is the source-scan analog of
// TestNoSingleCallListAPIEnrichers (enrichment_pagination_audit_test.go):
// a new orphaned enricher — implemented but never wired to any type,
// directly or via a combiner — fails this test.
//
// docs/resources/dbi.md §4 documents "Pending maintenance overdue"
// (dbi.pending-maintenance, Warning-on-Healthy, "~") and
// docs/resources/dbc.md §3.2/§4 documents "Cluster has a pending
// maintenance action ... -> Warning" / "maintenance overdue" (dbc side).
// Both signals are already served by the registered EnrichDBIMaintenance
// (catalog_databases.go dbi Wave2) and EnrichDBCMaintenance (catalog_
// databases.go dbc Wave2) respectively — no golden doc documents a third,
// distinct "rds" pending-maintenance signal. EnrichRDSDocDBMaintenance
// (rds_issue_enrichment.go) duplicates the dbi half of that coverage under
// a resource name ("rds") no catalog entry uses, is not called by any
// registered combiner, and is referenced by nothing but its own test
// suite — it fails this pin today as an orphan.
func TestNoOrphanedIssueEnrichmentFunctions(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed — cannot locate test file")
	}
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")

	matches, err := filepath.Glob(filepath.Join(repoRoot, "internal", "aws", "*_issue_enrichment.go"))
	if err != nil {
		t.Fatalf("filepath.Glob failed: %v", err)
	}
	if len(matches) == 0 {
		t.Fatal("filepath.Glob returned zero matches for internal/aws/*_issue_enrichment.go — check repo layout")
	}

	// Build the set of directly registered Wave2.Fn names, keyed by bare
	// function name (e.g. "EnrichDBIMaintenance").
	registered := make(map[string]bool)
	for _, td := range resource.AllResourceTypes() {
		if td.Wave2 == nil {
			continue
		}
		enricher, ok := td.Wave2.(awsclient.IssueEnricher)
		if !ok || enricher.Fn == nil {
			continue
		}
		ptr := reflect.ValueOf(enricher.Fn).Pointer()
		fn := runtime.FuncForPC(ptr)
		if fn == nil {
			continue
		}
		name := fn.Name()
		if idx := strings.LastIndex(name, "."); idx >= 0 {
			name = name[idx+1:]
		}
		registered[name] = true
	}

	fset := token.NewFileSet()

	// declByName holds every Enrich* function's AST body, and callers.go
	// tracks which Enrich* identifiers each Enrich* function calls by name —
	// this lets an orphan check walk one level of indirection through
	// combiner functions like EnrichCFNCombined.
	declByName := make(map[string]*ast.FuncDecl)
	callsByFunc := make(map[string]map[string]bool)
	var declOrder []string
	fileOf := make(map[string]string)

	for _, filePath := range matches {
		if strings.HasSuffix(filePath, "_test.go") {
			continue
		}
		src, parseErr := parser.ParseFile(fset, filePath, nil, 0)
		if parseErr != nil {
			t.Fatalf("parse error in %s: %v", filePath, parseErr)
		}
		baseName := filepath.Base(filePath)

		for _, decl := range src.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Name == nil || fn.Body == nil {
				continue
			}
			if !strings.HasPrefix(fn.Name.Name, "Enrich") {
				continue
			}
			declByName[fn.Name.Name] = fn
			declOrder = append(declOrder, fn.Name.Name)
			fileOf[fn.Name.Name] = baseName

			calls := make(map[string]bool)
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				if ident, ok := call.Fun.(*ast.Ident); ok && strings.HasPrefix(ident.Name, "Enrich") {
					calls[ident.Name] = true
				}
				return true
			})
			callsByFunc[fn.Name.Name] = calls
		}
	}

	reachable := make(map[string]bool, len(registered))
	for name := range registered {
		reachable[name] = true
	}
	for callerName := range registered {
		for callee := range callsByFunc[callerName] {
			if _, isEnrichDecl := declByName[callee]; isEnrichDecl {
				reachable[callee] = true
			}
		}
	}

	var orphans []string
	for _, name := range declOrder {
		if !reachable[name] {
			orphans = append(orphans, fileOf[name]+": "+name)
		}
	}

	if len(orphans) > 0 {
		t.Errorf("orphaned Enrich* function(s) in internal/aws/*_issue_enrichment.go — implemented but not "+
			"reachable (directly or via a one-level combiner call) from any registered "+
			"catalog.ResourceTypeDef.Wave2 field: %v. Either wire the function to a catalog entry's Wave2 "+
			"field (directly or via a combiner) or delete it (and its dedicated tests) — dead enrichment "+
			"code with no catalog wiring can never run in production. See docs/resources/dbi.md §4 and "+
			"docs/resources/dbc.md §3.2/§4 for the pending-maintenance signals already served by "+
			"EnrichDBIMaintenance / EnrichDBCMaintenance.", orphans)
	}
}
