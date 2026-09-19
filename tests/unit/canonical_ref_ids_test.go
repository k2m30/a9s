package unit_test

// A reference AWS hands back is one fact with one reading. A related row
// counts the target's rows by that reading and a navigable field opens the
// target by it, so both go through the target type's own resolver
// (resource.ResolveRef → catalog RefToID); a second parse would let the count
// show a row the drill cannot open.

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"

	"github.com/k2m30/a9s/v3/core/app"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/session"
)

const (
	refAccount        = "123456789012"
	refForeignAccount = "210987654321"
	refRegion         = "us-east-1"
)

// refBench is the demo fixture bench: every type's rows as its real fetcher
// emits them, plus the account and region the demo session runs in.
type refBench struct {
	byType map[string][]resource.Resource
	cache  resource.ResourceCache
}

func newRefBench(t *testing.T) refBench {
	t.Helper()
	byType, cache := buildDemoTypeCache(t)
	return refBench{byType: byType, cache: cache}
}

func (b refBench) rc(target string) domain.RefContext {
	return domain.RefContext{AccountID: refAccount, Region: refRegion, Targets: b.byType[target]}
}

func (b refBench) row(t *testing.T, typ, id string) resource.Resource {
	t.Helper()
	for _, r := range b.byType[typ] {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("demo bench has no %s row %q", typ, id)
	return resource.Resource{}
}

func (b refBench) has(typ, id string) bool {
	for _, r := range b.byType[typ] {
		if r.ID == id {
			return true
		}
	}
	return false
}

// refClients are the demo clients with the caller's account resolved, which
// is the state a session is in once STS has answered.
func refClients() *awsclient.ServiceClients {
	c := demo.NewServiceClients()
	c.Region = refRegion
	store := session.NewIdentityStore()
	store.Set(refAccount, nil)
	c.SetIdentityStore(store)
	return c
}

func refChecker(t *testing.T, source, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated(source) {
		if def.TargetType == target && def.Checker != nil {
			return def.Checker
		}
	}
	t.Fatalf("no %s related checker for target %q", source, target)
	return nil
}

func sortedIDs(r resource.RelatedCheckResult) []string {
	ids := append([]string(nil), r.ResourceIDs()...)
	sort.Strings(ids)
	return ids
}

type refCase struct {
	name   string
	target string
	ref    string
	wantID string
	wantOK bool
}

func runRefCases(t *testing.T, b refBench, cases []refCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.target+"/"+tc.name, func(t *testing.T) {
			if tc.wantOK && !b.has(tc.target, tc.wantID) {
				t.Fatalf("demo bench has no %s row %q to resolve to", tc.target, tc.wantID)
			}
			id, ok := resource.ResolveRef(tc.target, tc.ref, b.rc(tc.target))
			if ok != tc.wantOK || (tc.wantOK && id != tc.wantID) {
				t.Errorf("ResolveRef(%q, %q) = (%q, %v), want (%q, %v)", tc.target, tc.ref, id, ok, tc.wantID, tc.wantOK)
			}
		})
	}
}

// The row ID is what the type's list keys on; anything else drills into
// nothing.
func TestResolveRef_EachTargetParsesItsOwnReferences(t *testing.T) {
	b := newRefBench(t)
	const (
		acmARN   = "arn:aws:acm:us-east-1:123456789012:certificate/b2c3d4e5-6789-01ab-cdef-222222222222"
		kmsKey   = "a1b2c3d4-5678-90ab-cdef-111111111111"
		secretID = "prod/database/primary"
		secretAR = "arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/database/primary-AbCdEf"
		elbALB   = "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/acme-prod-web/1234567890abcdef"
		elbNLB   = "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/net/acme-prod-nlb/50dc6c495c0c9188"
		tgARN    = "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/acme-web-tg/1234567890abcdef"
		zone     = "/hostedzone/Z0123456789ABCDEFGHIJ"
	)
	runRefCases(t, b, []refCase{
		// acm keys its rows on the full certificate ARN: the ARN is the ID.
		{"certificate ARN is kept whole", "acm", acmARN, acmARN, true},

		{"key ARN", "kms", "arn:aws:kms:us-east-1:123456789012:key/" + kmsKey, kmsKey, true},
		{"bare key ID", "kms", kmsKey, kmsKey, true},
		{"alias name", "kms", "alias/acme-prod-key", kmsKey, true},
		{"alias ARN", "kms", "arn:aws:kms:us-east-1:123456789012:alias/acme-prod-key", kmsKey, true},
		{"alias no key carries", "kms", "alias/acme-no-such-key", "", false},

		// Secrets Manager appends "-" and six random characters to the name
		// in the ARN; ECS ValueFrom may add ":json-key:version-stage:version-id".
		{"secret ARN", "secrets", secretAR, secretID, true},
		{"secret ARN with json-key tail", "secrets", secretAR + ":password::", secretID, true},
		{"secret ARN with json-key and stage", "secrets", secretAR + ":username:AWSCURRENT:", secretID, true},
		{"bare secret name", "secrets", secretID, secretID, true},

		{"unqualified function ARN", "lambda", "arn:aws:lambda:us-east-1:123456789012:function:api-gateway-authorizer", "api-gateway-authorizer", true},
		{"alias-qualified function ARN", "lambda", "arn:aws:lambda:us-east-1:123456789012:function:api-gateway-authorizer:live", "api-gateway-authorizer", true},
		{"version-qualified function ARN", "lambda", "arn:aws:lambda:us-east-1:123456789012:function:api-gateway-authorizer:$LATEST", "api-gateway-authorizer", true},

		{"application load balancer ARN", "elb", elbALB, "acme-prod-web", true},
		{"network load balancer ARN", "elb", elbNLB, "acme-prod-nlb", true},
		{"target group ARN", "tg", tgARN, "acme-web-tg", true},

		{"file system ARN", "efs", "arn:aws:elasticfilesystem:us-east-1:123456789012:file-system/" + fixtures.ProdEFSID, fixtures.ProdEFSID, true},
		{"access point ARN resolves to its file system", "efs", fixtures.ProdEFSAccessPointAARN, fixtures.ProdEFSID, true},

		{"stream ARN", "kinesis", "arn:aws:kinesis:us-east-1:123456789012:stream/clickstream-ingest", "clickstream-ingest", true},
		{"consumer ARN resolves to its stream", "kinesis", "arn:aws:kinesis:us-east-1:123456789012:stream/clickstream-ingest/consumer/analytics-app:1718000000", "clickstream-ingest", true},

		{"cluster ARN", "msk", "arn:aws:kafka:us-east-1:123456789012:cluster/acme-events-prod/a1b2c3d4", "acme-events-prod", true},

		{"log group ARN with :* suffix", "logs", "arn:aws:logs:us-east-1:123456789012:log-group:/aws/cloudtrail:*", "/aws/cloudtrail", true},
		{"log group ARN", "logs", "arn:aws:logs:us-east-1:123456789012:log-group:/aws/cloudtrail", "/aws/cloudtrail", true},

		{"bare zone ID", "r53", "Z0123456789ABCDEFGHIJ", zone, true},
		{"/hostedzone/ form", "r53", zone, zone, true},
		{"zone ARN", "r53", "arn:aws:route53:::hostedzone/Z0123456789ABCDEFGHIJ", zone, true},

		// IAM ARNs carry no region; a path is not part of the name.
		{"role ARN", "role", "arn:aws:iam::123456789012:role/acme-ecs-service-role", "acme-ecs-service-role", true},
		{"role ARN under a path", "role", "arn:aws:iam::123456789012:role/service-role/acme-ecs-service-role", "acme-ecs-service-role", true},
		{"user ARN", "iam-user", "arn:aws:iam::123456789012:user/alice.johnson", "alice.johnson", true},

		// S3 ARNs carry neither account nor region.
		{"bucket ARN", "s3", "arn:aws:s3:::a9s-demo-logs", "a9s-demo-logs", true},
		{"cluster ARN", "ecs", "arn:aws:ecs:us-east-1:123456789012:cluster/acme-services", "acme-services", true},
		{"queue ARN", "sqs", "arn:aws:sqs:us-east-1:123456789012:order-processing-queue", "order-processing-queue", true},
		{"topic ARN is kept whole", "sns", "arn:aws:sns:us-east-1:123456789012:order-events", "arn:aws:sns:us-east-1:123456789012:order-events", true},
	})
}

// A custom domain's certificate is counted by the ACM row ID, the full ARN,
// so the drill lands on the certificate the badge counted.
func TestApigwCertificates_CountAndDrillAgree(t *testing.T) {
	b := newRefBench(t)
	src := b.row(t, "apigw", "abc123def4")
	got := refChecker(t, "apigw", "acm")(context.Background(), refClients(), src, b.cache)
	want := []string{"arn:aws:acm:us-east-1:123456789012:certificate/b2c3d4e5-6789-01ab-cdef-222222222222"}
	if ids := sortedIDs(got); !slices.Equal(ids, want) {
		t.Errorf("apigw abc123def4 → Certificates IDs = %v, want %v", ids, want)
	}

	// An API no custom domain maps to has no certificate.
	other := b.row(t, "apigw", "rst001noauth")
	if n := refChecker(t, "apigw", "acm")(context.Background(), refClients(), other, b.cache).Count(); n != 0 {
		t.Errorf("apigw rst001noauth → Certificates count = %d, want 0", n)
	}
}

func TestLambdaMSK_ClusterCountedByItsRowID(t *testing.T) {
	b := newRefBench(t)
	check := refChecker(t, "lambda", "msk")
	got := check(context.Background(), refClients(), b.row(t, "lambda", "data-pipeline-transform"), b.cache)
	if ids := sortedIDs(got); !slices.Equal(ids, []string{"acme-events-prod"}) {
		t.Errorf("data-pipeline-transform → MSK IDs = %v, want [acme-events-prod]", ids)
	}
	none := check(context.Background(), refClients(), b.row(t, "lambda", "api-gateway-authorizer"), b.cache)
	if none.Count() != 0 {
		t.Errorf("api-gateway-authorizer has no Kafka trigger; MSK IDs = %v, want none", none.ResourceIDs())
	}
}

// A function mounts an access point, and the EFS list is keyed by file system.
func TestLambdaEFS_AccessPointCountedAsItsFileSystem(t *testing.T) {
	b := newRefBench(t)
	got := refChecker(t, "lambda", "efs")(context.Background(), refClients(), b.row(t, "lambda", "efs-data-processor"), b.cache)
	if ids := sortedIDs(got); !slices.Equal(ids, []string{fixtures.ProdEFSID}) {
		t.Errorf("efs-data-processor → EFS IDs = %v, want [%s]", ids, fixtures.ProdEFSID)
	}
}

// A container can inject one JSON key of a secret; the reference then carries
// ":json-key:stage:version" after the secret ARN, and still names that one
// secret. Two keys of the same secret are one secret.
func TestEcsTaskSecrets_JSONKeyTailCountsTheSecret(t *testing.T) {
	b := newRefBench(t)
	const primary = "arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/database/primary-AbCdEf"
	const gateway = "arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/api/gateway-key-XyZ123"
	task := b.row(t, "ecs-task", "a1b2c3d4e5f6a1b2c3d4e5f6")
	task.Fields = maps.Clone(task.Fields)
	task.Fields["secret_arns"] = strings.Join([]string{primary + ":password::", primary + ":username:AWSCURRENT:", gateway}, ",")

	got := refChecker(t, "ecs-task", "secrets")(context.Background(), refClients(), task, b.cache)
	want := []string{"prod/api/gateway-key", "prod/database/primary"}
	if ids := sortedIDs(got); !slices.Equal(ids, want) || got.Count() != 2 {
		t.Errorf("ecs-task → Secrets = %v (count %d), want %v (count 2)", ids, got.Count(), want)
	}
	if got.Truncated() {
		t.Error("Truncated = true, want false: both secrets are local rows")
	}
}

// The demo bench carries the json-key reference shape, so ./a9s --demo shows it.
func TestEcsTaskSecrets_DemoCarriesAJSONKeyReference(t *testing.T) {
	b := newRefBench(t)
	check := refChecker(t, "ecs-task", "secrets")
	for _, task := range b.byType["ecs-task"] {
		for ref := range strings.SplitSeq(task.Fields["secret_arns"], ",") {
			if strings.Count(ref, ":") < 7 {
				continue
			}
			got := check(context.Background(), refClients(), task, b.cache)
			if got.Count() == 0 {
				t.Errorf("ecs-task %s references %q but Secrets counts nothing", task.ID, ref)
			}
			for _, id := range got.ResourceIDs() {
				if !b.has("secrets", id) {
					t.Errorf("ecs-task %s → Secrets ID %q is not a secrets row", task.ID, id)
				}
			}
			return
		}
	}
	t.Fatal("no demo ecs-task injects a JSON key of a secret (ValueFrom \"<secret ARN>:<json-key>::\"); " +
		"the ecs-task witness is not reproducible in --demo")
}

// Each pivot reaching Secrets Manager from an ARN counts by secret name: the
// ecs service reads its task definition, glue its connection, msk its
// SASL/SCRAM secret list.
func TestSecretsPivots_EverySourceCountsBySecretName(t *testing.T) {
	b := newRefBench(t)
	cases := []struct {
		source, id string
		want       []string
	}{
		{"ecs-svc", "api-gateway", []string{"prod/api/gateway-key", "prod/database/primary"}},
		{"glue", "acme-etl-orders", []string{"prod/database/primary"}},
		{"msk", "acme-events-prod", []string{"prod/database/primary"}},
	}
	for _, tc := range cases {
		t.Run(tc.source, func(t *testing.T) {
			got := refChecker(t, tc.source, "secrets")(context.Background(), refClients(), b.row(t, tc.source, tc.id), b.cache)
			if ids := sortedIDs(got); !slices.Equal(ids, tc.want) {
				t.Errorf("%s %s → Secrets IDs = %v, want %v", tc.source, tc.id, ids, tc.want)
			}
		})
	}
}

// relatedParseCall reports whether call is one of the reference parses the
// resolvers own: arnLastSegment; strings.LastIndex, TrimPrefix, TrimSuffix,
// Cut, CutPrefix or CutSuffix on anything; strings.Index on a string literal,
// since a host name such as "<id>.execute-api.<region>.amazonaws.com" is cut
// at a marker as surely as an ARN; strings.Split, SplitN, SplitSeq or
// SplitAfter on a literal holding "/" or ":". A comma-joined field list split
// on "," is not a reference and is not matched.
func relatedParseCall(call *ast.CallExpr) string {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		if fn.Name == "arnLastSegment" {
			return "arnLastSegment"
		}
	case *ast.SelectorExpr:
		pkg, ok := fn.X.(*ast.Ident)
		if !ok || pkg.Name != "strings" || len(call.Args) < 2 {
			return ""
		}
		switch fn.Sel.Name {
		case "LastIndex", "TrimPrefix", "TrimSuffix", "Cut", "CutPrefix", "CutSuffix":
			return "strings." + fn.Sel.Name
		case "Split", "SplitN", "SplitSeq", "SplitAfter":
			lit, ok := call.Args[1].(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return ""
			}
			sep, err := strconv.Unquote(lit.Value)
			if err == nil && strings.ContainsAny(sep, "/:") {
				return "strings." + fn.Sel.Name + "(…, " + strconv.Quote(sep) + ")"
			}
		case "Index":
			if lit, ok := call.Args[1].(*ast.BasicLit); ok && lit.Kind == token.STRING {
				return "strings.Index(…, " + lit.Value + ")"
			}
		}
	}
	return ""
}

// A checker that splits an ARN itself is a second reading of a fact the
// target type's resolver owns. Only a RefToID function may parse.
func TestRelatedCheckers_ParseNoReferenceThemselves(t *testing.T) {
	files, err := filepath.Glob("../../core/aws/*_related*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("no related checker files found: %v", err)
	}
	var hits []string
	fset := token.NewFileSet()
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, decl := range f.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Body == nil || strings.Contains(fd.Name.Name, "RefToID") {
				continue
			}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				if what := relatedParseCall(call); what != "" {
					pos := fset.Position(call.Pos())
					hits = append(hits, filepath.Base(pos.Filename)+":"+strconv.Itoa(pos.Line)+" "+fd.Name.Name+" "+what)
				}
				return true
			})
		}
	}
	if len(hits) > 0 {
		sort.Strings(hits)
		t.Errorf("%d reference parse(s) in related checker files outside a RefToID:\n  %s", len(hits), strings.Join(hits, "\n  "))
	}
}

// refDetailController is a controller on the demo bench with every type's
// rows loaded and navigability bootstrapped as the app does at startup.
func refDetailController(t *testing.T, b refBench, skip ...string) *app.Controller {
	t.Helper()
	c, _ := refDetailControllerCore(t, b, skip...)
	return c
}

// refDetailControllerCore also returns the runtime core, for tests that land
// a related result the way the result lane does.
func refDetailControllerCore(t *testing.T, b refBench, skip ...string) (*app.Controller, *runtime.Core) {
	t.Helper()
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := session.New()
	s.Profile = demo.DemoProfile
	s.Region = refRegion
	core := runtime.New(s, nil)
	c := newBlessedController(t, core)
	c.SetViewConfig(config.DefaultConfig())
	for typ, rows := range b.byType {
		resource.SetNavigableFieldsForTest(typ, resource.GetNavigableFields(typ))
		t.Cleanup(func() { resource.CleanupNavigableFieldsForTest(typ) })
		if !slices.Contains(skip, typ) {
			c.ApplyResourcesLoaded(typ, rows, nil, false)
		}
	}
	return c, core
}

// openDetail pushes a detail screen for res and returns its rendered rows.
func openDetail(c *app.Controller, typ string, res resource.Resource) []app.FieldRow {
	c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenDetail}})
	c.EnsureDetailState(res, typ)
	return c.Snapshot().Body.Detail.Fields
}

// navTarget is the ID Enter hands the navigation: NavID when set, else Value.
func navTarget(f app.FieldRow) string {
	if f.NavID != "" {
		return f.NavID
	}
	return f.Value
}

func fieldAt(t *testing.T, fields []app.FieldRow, path string) app.FieldRow {
	t.Helper()
	for _, f := range fields {
		if f.Path == path {
			return f
		}
	}
	t.Fatalf("detail has no %q row", path)
	return app.FieldRow{}
}

// kmsKeyByAlias returns the demo kms row carrying alias.
func kmsKeyByAlias(t *testing.T, b refBench, alias string) string {
	t.Helper()
	for _, r := range b.byType["kms"] {
		if r.Fields["alias"] == alias {
			return r.ID
		}
	}
	t.Fatalf("demo kms has no key with alias %q", alias)
	return ""
}

// TestSSMKeyId_AliasFieldAndRelatedRowOpenTheSameKey: a SecureString
// parameter names a customer key by alias. The KMS row counts the key the
// alias points at, and Enter on KeyId must open that same key.
func TestSSMKeyId_AliasFieldAndRelatedRowOpenTheSameKey(t *testing.T) {
	b := newRefBench(t)
	const alias = "alias/acme-prod-key"
	wantKey := kmsKeyByAlias(t, b, alias)
	param := b.row(t, "ssm", "/acme/prod/db/connection-string")

	related := refChecker(t, "ssm", "kms")(context.Background(), refClients(), param, b.cache)
	if ids := sortedIDs(related); !slices.Equal(ids, []string{wantKey}) {
		t.Errorf("ssm → KMS Key IDs = %v, want [%s]", ids, wantKey)
	}
	if got := resource.NavIDFromValue("kms", alias, b.rc("kms")); got != wantKey {
		t.Errorf("NavIDFromValue(kms, %q) = %q, want %q", alias, got, wantKey)
	}

	c := refDetailController(t, b)
	f := fieldAt(t, openDetail(c, "ssm", param), "KeyId")
	if !f.IsNavigable || f.TargetType != "kms" || navTarget(f) != wantKey {
		t.Errorf("KeyId row: navigable=%v target=%q opens %q, want navigable kms row opening %q",
			f.IsNavigable, f.TargetType, navTarget(f), wantKey)
	}
}

// An alias/aws/* alias names an AWS-managed key, which the kms list (customer
// keys only) never holds. The related row still counts that key by its key
// ID, found through the kms type's own by-ID lookup, and once the related
// result lazy-adds the key, KeyId opens it.
func TestAWSManagedKeys_ResolveThroughTheKeyLookup(t *testing.T) {
	b := newRefBench(t)
	ctx := context.Background()
	cases := []struct{ source, id, alias string }{
		{"ssm", "/acme/legacy/db/password", "alias/aws/ssm"},
		{"s3", "a9s-demo-managed-kms", "alias/aws/s3"},
	}
	for _, tc := range cases {
		t.Run(tc.alias, func(t *testing.T) {
			src := b.row(t, tc.source, tc.id)
			related := refChecker(t, tc.source, "kms")(ctx, refClients(), src, b.cache)
			ids := sortedIDs(related)
			if len(ids) != 1 {
				t.Fatalf("%s %s → KMS Key IDs = %v, want the one key behind %s", tc.source, tc.id, ids, tc.alias)
			}
			key := ids[0]
			if tc.alias == "alias/aws/ssm" && key != fixtures.SSMDefaultKeyID {
				t.Errorf("KMS Key ID = %q, want %q", key, fixtures.SSMDefaultKeyID)
			}
			if strings.HasPrefix(key, "alias/") {
				t.Errorf("KMS Key ID = %q, want a key ID, not an alias", key)
			}
			if b.has("kms", key) {
				t.Errorf("key %q behind %s is a kms list row; AWS-managed keys are not in the customer-key list", key, tc.alias)
			}

			rows, err := resource.GetFetchByIDs("kms")(ctx, refClients(), []string{key})
			var got *resource.Resource
			for i := range rows {
				if rows[i].ID == key {
					got = &rows[i]
				}
			}
			if got == nil {
				t.Fatalf("kms FetchByIDs(%q) returned no row with that ID (err %v)", key, err)
			}
			if meta, ok := got.RawStruct.(*kmstypes.KeyMetadata); !ok || meta.KeyManager != kmstypes.KeyManagerTypeAws {
				t.Errorf("key %q: RawStruct %T is not an AWS-managed *KeyMetadata", key, got.RawStruct)
			}

			if tc.source != "ssm" {
				return
			}
			c, core := refDetailControllerCore(t, b)
			openDetail(c, "ssm", src)
			intents, _ := core.HandleRelatedCheckResult(runtime.RelatedCheckResultEvent{
				ResourceType:       "ssm",
				SourceResourceID:   src.ID,
				DefDisplayName:     "KMS Key",
				Result:             related,
				LazyAddedResources: map[string][]resource.Resource{"kms": rows},
			})
			c.ApplyIntents(intents)
			f := fieldAt(t, c.Snapshot().Body.Detail.Fields, "KeyId")
			if !f.IsNavigable || f.TargetType != "kms" || navTarget(f) != key {
				t.Errorf("KeyId after the key was lazy-added: navigable=%v target=%q opens %q, want %q",
					f.IsNavigable, f.TargetType, navTarget(f), key)
			}
		})
	}
}

// refKMSFake serves one key under several aliases. aliases lists what
// ListAliases reports; describe answers DescribeKey by key ID or any alias,
// including an alias ListAliases has not reported yet.
type refKMSFake struct {
	awsclient.KMSAPI
	meta    kmstypes.KeyMetadata
	aliases []string
	lookup  []string
}

func (f *refKMSFake) ListKeys(context.Context, *kms.ListKeysInput, ...func(*kms.Options)) (*kms.ListKeysOutput, error) {
	return &kms.ListKeysOutput{Keys: []kmstypes.KeyListEntry{{KeyId: f.meta.KeyId, KeyArn: f.meta.Arn}}}, nil
}

func (f *refKMSFake) ListAliases(context.Context, *kms.ListAliasesInput, ...func(*kms.Options)) (*kms.ListAliasesOutput, error) {
	out := &kms.ListAliasesOutput{}
	for _, a := range f.aliases {
		out.Aliases = append(out.Aliases, kmstypes.AliasListEntry{AliasName: aws.String(a), TargetKeyId: f.meta.KeyId})
	}
	return out, nil
}

func (f *refKMSFake) DescribeKey(_ context.Context, in *kms.DescribeKeyInput, _ ...func(*kms.Options)) (*kms.DescribeKeyOutput, error) {
	id := aws.ToString(in.KeyId)
	if id == aws.ToString(f.meta.KeyId) || slices.Contains(f.aliases, id) || slices.Contains(f.lookup, id) {
		meta := f.meta
		return &kms.DescribeKeyOutput{KeyMetadata: &meta}, nil
	}
	return nil, &kmstypes.NotFoundException{Message: aws.String("key " + id + " not found")}
}

func refKMSClients(f *refKMSFake) *awsclient.ServiceClients {
	c := refClients()
	c.KMS = f
	return c
}

func refOrdersKey() kmstypes.KeyMetadata {
	return kmstypes.KeyMetadata{
		KeyId:      aws.String("0d1e2f3a-4b5c-4d6e-8f70-819203a4b5c6"),
		Arn:        aws.String("arn:aws:kms:us-east-1:123456789012:key/0d1e2f3a-4b5c-4d6e-8f70-819203a4b5c6"),
		KeyManager: kmstypes.KeyManagerTypeCustomer,
		KeyState:   kmstypes.KeyStateEnabled,
		Enabled:    true,
	}
}

// TestKMSAliases_EveryAliasOfAKeyResolves: a key can carry several aliases,
// and a resource may name it by any of them. The fetched row must answer to
// each, or the resource that uses the second alias counts no key.
func TestKMSAliases_EveryAliasOfAKeyResolves(t *testing.T) {
	ctx := context.Background()
	f := &refKMSFake{meta: refOrdersKey(), aliases: []string{"alias/acme-orders", "alias/acme-orders-legacy"}}
	keyID := aws.ToString(f.meta.KeyId)

	page, err := awsclient.FetchKMSKeysPage(ctx, refKMSClients(f), "")
	if err != nil {
		t.Fatalf("FetchKMSKeysPage: %v", err)
	}
	byIDs, err := awsclient.FetchKMSKeysByIDs(ctx, refKMSClients(f), []string{keyID})
	if err != nil {
		t.Fatalf("FetchKMSKeysByIDs: %v", err)
	}
	for name, rows := range map[string][]resource.Resource{"FetchKMSKeysPage": page.Resources, "FetchKMSKeysByIDs": byIDs} {
		rc := domain.RefContext{AccountID: refAccount, Region: refRegion, Targets: rows}
		for _, alias := range f.aliases {
			for _, ref := range []string{alias, "arn:aws:kms:us-east-1:123456789012:" + alias} {
				if id, ok := resource.ResolveRef("kms", ref, rc); !ok || id != keyID {
					t.Errorf("%s rows: ResolveRef(kms, %q) = (%q, %v), want (%q, true)", name, ref, id, ok, keyID)
				}
			}
		}
	}
}

// TestKMSAliases_ByIDLookupRowCarriesTheRequestedAlias: an alias created
// after the last ListAliases page was read is still an alias of the key
// DescribeKey returns for it, so the row fetched for that alias answers to it.
func TestKMSAliases_ByIDLookupRowCarriesTheRequestedAlias(t *testing.T) {
	ctx := context.Background()
	const fresh = "alias/acme-orders-2026"
	f := &refKMSFake{meta: refOrdersKey(), aliases: []string{"alias/acme-orders"}, lookup: []string{fresh}}
	keyID := aws.ToString(f.meta.KeyId)

	rows, err := awsclient.FetchKMSKeysByIDs(ctx, refKMSClients(f), []string{fresh})
	if err != nil {
		t.Fatalf("FetchKMSKeysByIDs: %v", err)
	}
	rc := domain.RefContext{AccountID: refAccount, Region: refRegion, Targets: rows}
	if id, ok := resource.ResolveRef("kms", fresh, rc); !ok || id != keyID {
		t.Errorf("ResolveRef(kms, %q) over the row fetched for it = (%q, %v), want (%q, true)", fresh, id, ok, keyID)
	}
}

// TestCfS3_StandardLogBucketCountsUnderS3: a distribution's standard access
// logs land in an S3 bucket (DistributionConfig.Logging.Bucket, domain form),
// which the S3 Buckets row counts beside the origin buckets. A distribution
// with logging off adds no bucket.
func TestCfS3_StandardLogBucketCountsUnderS3(t *testing.T) {
	b := newRefBench(t)
	check := refChecker(t, "cf", "s3")

	got := check(context.Background(), refClients(), b.row(t, "cf", "E1A2B3C4D5E6F7"), b.cache)
	// Its only S3-hosted origin is a website endpoint, which the origin rule
	// does not count, so the log bucket is the whole answer.
	want := []string{fixtures.LogsBucketName}
	if ids := sortedIDs(got); !slices.Equal(ids, want) {
		t.Errorf("E1A2B3C4D5E6F7 → S3 Buckets = %v, want %v (the standard-log bucket)", ids, want)
	}

	off := check(context.Background(), refClients(), b.row(t, "cf", fixtures.CFLoggingOff), b.cache)
	if ids := sortedIDs(off); len(ids) != 0 {
		t.Errorf("%s (logging off, custom origin) → S3 Buckets = %v, want none", fixtures.CFLoggingOff, ids)
	}
}

// TestSSMKeyId_KeyListArrivingAfterTheDetailStillResolves covers the order an
// operator meets cold: the parameter's detail opens before the key list has
// loaded, and the list lands while the detail is on screen.
func TestSSMKeyId_KeyListArrivingAfterTheDetailStillResolves(t *testing.T) {
	b := newRefBench(t)
	wantKey := kmsKeyByAlias(t, b, "alias/acme-prod-key")
	c := refDetailController(t, b, "kms")
	openDetail(c, "ssm", b.row(t, "ssm", "/acme/prod/db/connection-string"))

	c.ApplyResourcesLoaded("kms", b.byType["kms"], nil, false)

	f := fieldAt(t, c.Snapshot().Body.Detail.Fields, "KeyId")
	if !f.IsNavigable || navTarget(f) != wantKey {
		t.Errorf("KeyId after the key list landed: navigable=%v opens %q, want %q", f.IsNavigable, navTarget(f), wantKey)
	}
}

// rowsNamedBy returns every rendered row whose value is ref, with or without
// the list-item dash a string list renders.
func rowsNamedBy(fields []app.FieldRow, ref string) []app.FieldRow {
	var out []app.FieldRow
	for _, f := range fields {
		v := strings.TrimPrefix(strings.TrimSpace(f.Value), "- ")
		k := strings.TrimPrefix(strings.TrimSpace(f.Key), "- ")
		if v == ref || k == ref {
			out = append(out, f)
		}
	}
	return out
}

// Every reference the detail shows as navigable opens the row it names,
// including the entries of a list of ARNs.
func TestNavigableFields_ARNListsAndGatewaysOpenTheirRow(t *testing.T) {
	b := newRefBench(t)
	const (
		albARN = "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/acme-prod-web/1234567890abcdef"
		tgARN  = "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/acme-web-tg/1234567890abcdef"
	)
	cases := []struct {
		name, source, id, ref, target, want string
	}{
		{"tg LoadBalancerArns", "tg", "acme-web-tg", albARN, "elb", "acme-prod-web"},
		{"asg TargetGroupARNs", "asg", "acme-web-prod-asg", tgARN, "tg", "acme-web-tg"},
		{"ecs-svc LoadBalancers.TargetGroupArn", "ecs-svc", "api-gateway", tgARN, "tg", "acme-web-tg"},
		{"rtb Routes.GatewayId", "rtb", "rtb-0bbb222222222222b", "igw-0aaa111111111111a", "igw", "igw-0aaa111111111111a"},
		{"rtb Routes.VpcPeeringConnectionId", "rtb", "rtb-0aaa111111111111a", "pcx-0prodpeershared1a", "vpc-peer", "pcx-0prodpeershared1a"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := refDetailController(t, b)
			rows := rowsNamedBy(openDetail(c, tc.source, b.row(t, tc.source, tc.id)), tc.ref)
			if len(rows) == 0 {
				t.Fatalf("%s %s detail shows no row for %q", tc.source, tc.id, tc.ref)
			}
			for _, f := range rows {
				if !f.IsNavigable || f.TargetType != tc.target || navTarget(f) != tc.want {
					t.Errorf("row %q: navigable=%v target=%q opens %q, want navigable %s row opening %q",
						f.Path, f.IsNavigable, f.TargetType, navTarget(f), tc.target, tc.want)
				}
			}
		})
	}
}

// The VPC-local route's GatewayId is "local", which is not an internet
// gateway, so Enter on it must not open an empty IGW list.
func TestNavigableFields_RouteToNoGatewayIsNotAnIGW(t *testing.T) {
	b := newRefBench(t)
	c := refDetailController(t, b)
	for _, f := range rowsNamedBy(openDetail(c, "rtb", b.row(t, "rtb", "rtb-0aaa111111111111a")), "local") {
		if f.IsNavigable && f.TargetType == "igw" {
			t.Errorf("row %q: GatewayId \"local\" is navigable to igw, opening %q", f.Path, navTarget(f))
		}
	}
	if _, ok := resource.ResolveRef("igw", "local", b.rc("igw")); ok {
		t.Error(`ResolveRef(igw, "local") ok = true, want false`)
	}
	if _, ok := resource.ResolveRef("igw", "vgw-0aaa111111111111a", b.rc("igw")); ok {
		t.Error("ResolveRef(igw, a virtual private gateway) ok = true, want false")
	}
}

func TestNavIDFromValue_TargetsWithoutAnExtractorResolve(t *testing.T) {
	b := newRefBench(t)
	cases := []struct{ target, value, want string }{
		{"tg", "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/acme-api-tg/0987654321fedcba", "acme-api-tg"},
		{"elb", "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/acme-internal-api/0987654321fedcba", "acme-internal-api"},
		{"secrets", "arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/api/stripe-key-GhIjKl", "prod/api/stripe-key"},
		{"igw", "igw-0bbb222222222222b", "igw-0bbb222222222222b"},
		{"vpc-peer", "pcx-0prodpeershared1a", "pcx-0prodpeershared1a"},
	}
	for _, tc := range cases {
		t.Run(tc.target, func(t *testing.T) {
			if got := resource.NavIDFromValue(tc.target, tc.value, b.rc(tc.target)); got != tc.want {
				t.Errorf("NavIDFromValue(%q, %q) = %q, want %q", tc.target, tc.value, got, tc.want)
			}
		})
	}
	if got := resource.NavIDFromValue("igw", "local", b.rc("igw")); got != "" {
		t.Errorf(`NavIDFromValue(igw, "local") = %q, want "" (names no row)`, got)
	}
}

// A same-named or same-ID resource in another account or region never
// resolves to the local row. IAM and S3 ARNs carry no region and stay local.
func TestResolveRef_ForeignAccountOrRegionIsNotLocal(t *testing.T) {
	b := newRefBench(t)
	runRefCases(t, b, []refCase{
		{"key in another account", "kms", "arn:aws:kms:us-east-1:" + refForeignAccount + ":key/a1b2c3d4-5678-90ab-cdef-111111111111", "", false},
		{"key in another region", "kms", "arn:aws:kms:eu-west-1:123456789012:key/a1b2c3d4-5678-90ab-cdef-111111111111", "", false},
		{"role in another account", "role", "arn:aws:iam::" + refForeignAccount + ":role/a9s-demo-s3-access-role", "", false},
		{"secret in another account", "secrets", "arn:aws:secretsmanager:us-east-1:" + refForeignAccount + ":secret:prod/database/primary-AbCdEf", "", false},
		{"function in another region", "lambda", "arn:aws:lambda:eu-west-1:123456789012:function:api-gateway-authorizer", "", false},
		{"topic in another account", "sns", "arn:aws:sns:us-east-1:" + refForeignAccount + ":order-events", "", false},
		{"certificate in another region", "acm", "arn:aws:acm:eu-west-1:123456789012:certificate/b2c3d4e5-6789-01ab-cdef-222222222222", "", false},
		{"local role (IAM has no region)", "role", "arn:aws:iam::123456789012:role/a9s-demo-s3-access-role", "a9s-demo-s3-access-role", true},
		{"bucket ARN (no account, no region)", "s3", "arn:aws:s3:::a9s-demo-logs", "a9s-demo-logs", true},
	})
	if got := resource.NavIDFromValue("kms", "arn:aws:kms:us-east-1:"+refForeignAccount+":key/a1b2c3d4-5678-90ab-cdef-111111111111", b.rc("kms")); got != "" {
		t.Errorf("NavIDFromValue on another account's key = %q, want \"\"", got)
	}
}

type refBucketPolicyFake struct {
	awsclient.S3API
	policy string
}

func (f *refBucketPolicyFake) GetBucketPolicy(_ context.Context, _ *s3.GetBucketPolicyInput, _ ...func(*s3.Options)) (*s3.GetBucketPolicyOutput, error) {
	return &s3.GetBucketPolicyOutput{Policy: aws.String(f.policy)}, nil
}

// A bucket policy grants a role in another account whose name a local role
// shares. The Roles row must not count the local role for it, and says the
// count is a lower bound because a principal was left out.
func TestS3Roles_ForeignRoleWithALocalNameIsNotCounted(t *testing.T) {
	b := newRefBench(t)
	const name = "a9s-demo-s3-access-role"
	local := "arn:aws:iam::123456789012:role/" + name
	foreign := "arn:aws:iam::" + refForeignAccount + ":role/" + name
	policy := func(principals ...string) string {
		quoted := make([]string, len(principals))
		for i, p := range principals {
			quoted[i] = strconv.Quote(p)
		}
		return `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":[` +
			strings.Join(quoted, ",") + `]},"Action":"s3:GetObject","Resource":"arn:aws:s3:::a9s-demo-healthy/*"}]}`
	}
	cases := []struct {
		name          string
		principals    []string
		wantIDs       []string
		wantTruncated bool
	}{
		{"foreign role only", []string{foreign}, nil, true},
		{"local role only", []string{local}, []string{name}, false},
		{"both", []string{foreign, local}, []string{name}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clients := refClients()
			clients.S3 = &refBucketPolicyFake{policy: policy(tc.principals...)}
			got := refChecker(t, "s3", "role")(context.Background(), clients, b.row(t, "s3", "a9s-demo-healthy"), b.cache)
			if ids := sortedIDs(got); !slices.Equal(ids, tc.wantIDs) || got.Truncated() != tc.wantTruncated {
				t.Errorf("Roles = %v truncated=%v, want %v truncated=%v", ids, got.Truncated(), tc.wantIDs, tc.wantTruncated)
			}
		})
	}
}

// TestSSMKMS_ForeignKeyWithALocalKeyIDIsNotCounted is the same rule on a
// second pivot: key IDs are UUIDs, so a key in another account can carry the
// very ID of a local key.
func TestSSMKMS_ForeignKeyWithALocalKeyIDIsNotCounted(t *testing.T) {
	b := newRefBench(t)
	param := resource.Resource{
		ID:   "/acme/partner/shared-token",
		Name: "/acme/partner/shared-token",
		Type: "ssm",
		RawStruct: ssmtypes.ParameterMetadata{
			Name:  aws.String("/acme/partner/shared-token"),
			Type:  ssmtypes.ParameterTypeSecureString,
			KeyId: aws.String("arn:aws:kms:us-east-1:" + refForeignAccount + ":key/a1b2c3d4-5678-90ab-cdef-111111111111"),
		},
	}
	got := refChecker(t, "ssm", "kms")(context.Background(), refClients(), param, b.cache)
	if got.Count() != 0 || !got.Truncated() {
		t.Errorf("KMS Key = %v truncated=%v, want none, truncated=true", got.ResourceIDs(), got.Truncated())
	}
}
