package unit

// prowler_w2_ddb_test.go — ddb rows 20–21 of the w2 Prowler batch:
// deletion protection off (Wave 1) and a resource policy that grants another
// account or the world (Wave 2).
//
// Row 21 is the policy-engine row: the verdict comes from iampolicy.Evaluate,
// not from a substring search, so the cases below include the shapes a regex
// gets wrong — a single Statement object rather than an array, and a wildcard
// principal narrowed by a Condition.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	smithy "github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/session"
)

const (
	w2DDBCodeDeletionProtectionOff = "ddb.deletion-protection-off"
	w2DDBCodeCrossAccountPolicy    = "ddb.cross-account-policy"
	w2DDBCodePublicPolicy          = "ddb.public-policy"
)

// ---------------------------------------------------------------------------
// row 20 — deletion protection (Wave 1)
// ---------------------------------------------------------------------------

type w2DDBTablesFake struct {
	tables map[string]*ddbtypes.TableDescription
	order  []string
}

func (f *w2DDBTablesFake) ListTables(_ context.Context, _ *dynamodb.ListTablesInput, _ ...func(*dynamodb.Options)) (*dynamodb.ListTablesOutput, error) {
	return &dynamodb.ListTablesOutput{TableNames: f.order}, nil
}

func (f *w2DDBTablesFake) DescribeTable(_ context.Context, in *dynamodb.DescribeTableInput, _ ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error) {
	return &dynamodb.DescribeTableOutput{Table: f.tables[aws.ToString(in.TableName)]}, nil
}

func w2DDBTable(name string) *ddbtypes.TableDescription {
	return &ddbtypes.TableDescription{
		TableName:                 aws.String(name),
		TableArn:                  aws.String("arn:aws:dynamodb:eu-central-1:123456789012:table/" + name),
		TableStatus:               ddbtypes.TableStatusActive,
		ItemCount:                 aws.Int64(4200),
		TableSizeBytes:            aws.Int64(1 << 20),
		DeletionProtectionEnabled: aws.Bool(true),
		BillingModeSummary:        &ddbtypes.BillingModeSummary{BillingMode: ddbtypes.BillingModePayPerRequest},
	}
}

func w2DDBFetch(t *testing.T, tables ...*ddbtypes.TableDescription) map[string]resource.Resource {
	t.Helper()
	fake := &w2DDBTablesFake{tables: map[string]*ddbtypes.TableDescription{}}
	for _, tb := range tables {
		n := aws.ToString(tb.TableName)
		fake.tables[n] = tb
		fake.order = append(fake.order, n)
	}
	out, err := awsclient.FetchDynamoDBTablesPage(context.Background(), fake, fake, "")
	if err != nil {
		t.Fatalf("FetchDynamoDBTablesPage: %v", err)
	}
	return w2ByID(out.Resources)
}

func TestW2DDBDeletionProtectionOff(t *testing.T) {
	off := w2DDBTable("acme-orders")
	off.DeletionProtectionEnabled = aws.Bool(false)

	// Tables created before the setting existed report nil; DynamoDB treats
	// those as unprotected, so nil is a definite off here.
	absent := w2DDBTable("acme-sessions")
	absent.DeletionProtectionEnabled = nil

	got := w2DDBFetch(t, off, absent, w2DDBTable("acme-billing"))

	w2AssertFinding(t, got["acme-orders"].Findings, w2DDBCodeDeletionProtectionOff, "deletion protection off", domain.SevWarn, "wave1")
	w2AssertFinding(t, got["acme-sessions"].Findings, w2DDBCodeDeletionProtectionOff, "deletion protection off", domain.SevWarn, "wave1")
	w2AssertNoCode(t, got["acme-billing"].Findings, w2DDBCodeDeletionProtectionOff)
	w2AssertFindingDef(t, "ddb", w2DDBCodeDeletionProtectionOff, "deletion protection off", domain.SevWarn, "wave1")
}

// A table being deleted is not a posture problem.
func TestW2DDBDeletingTableEmitsNoPostureFinding(t *testing.T) {
	deleting := w2DDBTable("acme-old")
	deleting.TableStatus = ddbtypes.TableStatusDeleting
	deleting.DeletionProtectionEnabled = aws.Bool(false)

	got := w2DDBFetch(t, deleting)
	w2AssertNoCode(t, got["acme-old"].Findings, w2DDBCodeDeletionProtectionOff)
}

// ---------------------------------------------------------------------------
// row 21 — resource policy (Wave 2)
// ---------------------------------------------------------------------------

type w2DDBPolicyFake struct {
	awsclient.DynamoDBAPI

	policies map[string]string
	errs     map[string]error
}

func (f *w2DDBPolicyFake) GetResourcePolicy(_ context.Context, in *dynamodb.GetResourcePolicyInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetResourcePolicyOutput, error) {
	arn := aws.ToString(in.ResourceArn)
	name := arn[len("arn:aws:dynamodb:eu-central-1:123456789012:table/"):]
	if err := f.errs[name]; err != nil {
		return nil, err
	}
	doc, ok := f.policies[name]
	if !ok {
		return nil, &smithy.GenericAPIError{Code: "PolicyNotFoundException", Message: "no policy"}
	}
	return &dynamodb.GetResourcePolicyOutput{Policy: aws.String(doc)}, nil
}

func (f *w2DDBPolicyFake) DescribeContinuousBackups(_ context.Context, in *dynamodb.DescribeContinuousBackupsInput, _ ...func(*dynamodb.Options)) (*dynamodb.DescribeContinuousBackupsOutput, error) {
	return &dynamodb.DescribeContinuousBackupsOutput{
		ContinuousBackupsDescription: &ddbtypes.ContinuousBackupsDescription{
			ContinuousBackupsStatus: ddbtypes.ContinuousBackupsStatusEnabled,
			PointInTimeRecoveryDescription: &ddbtypes.PointInTimeRecoveryDescription{
				PointInTimeRecoveryStatus: ddbtypes.PointInTimeRecoveryStatusEnabled,
			},
		},
	}, nil
}

func w2DDBPolicyResource(name string) resource.Resource {
	r := w2Res(name, w2DDBTable(name))
	r.Fields["arn"] = "arn:aws:dynamodb:eu-central-1:123456789012:table/" + name
	return r
}

func w2DDBEnrich(t *testing.T, fake *w2DDBPolicyFake, names ...string) awsclient.IssueEnricherResult {
	t.Helper()
	rs := make([]resource.Resource, 0, len(names))
	for _, n := range names {
		rs = append(rs, w2DDBPolicyResource(n))
	}
	res, err := w2DDBRun(t, fake, rs)
	if err != nil {
		t.Fatalf("enricher returned error: %v", err)
	}
	return res
}

// w2DDBClients seeds the resolved account ID the policy engine compares
// principals against. Without it every AWS principal reads as another
// account's, which would make the cross-account row fire on tables that only
// grant their own roles.
func w2DDBClients(fake *w2DDBPolicyFake) *awsclient.ServiceClients {
	c := &awsclient.ServiceClients{DynamoDB: fake}
	store := session.NewIdentityStore()
	store.Set("123456789012", nil)
	c.SetIdentityStore(store)
	return c
}

func w2DDBRun(t *testing.T, fake *w2DDBPolicyFake, rs []resource.Resource) (awsclient.IssueEnricherResult, error) {
	t.Helper()
	res, err := w2Enricher(t, "ddb")(context.Background(), w2DDBClients(fake), rs, nil)
	w2AssertEnricherShape(t, res)
	return res, err
}

const w2DDBPublicPolicyDoc = `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "AnyoneRead",
      "Effect": "Allow",
      "Principal": "*",
      "Action": ["dynamodb:GetItem", "dynamodb:Query"],
      "Resource": "arn:aws:dynamodb:eu-central-1:123456789012:table/acme-public"
    }
  ]
}`

const w2DDBCrossAccountPolicyDoc = `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "PartnerRead",
      "Effect": "Allow",
      "Principal": {"AWS": ["arn:aws:iam::210987654321:root", "arn:aws:iam::333333333333:role/reader"]},
      "Action": "dynamodb:GetItem",
      "Resource": "arn:aws:dynamodb:eu-central-1:123456789012:table/acme-shared"
    }
  ]
}`

func TestW2DDBPublicPolicy(t *testing.T) {
	fake := &w2DDBPolicyFake{policies: map[string]string{"acme-public": w2DDBPublicPolicyDoc}}
	res := w2DDBEnrich(t, fake, "acme-public", "acme-billing")

	w2AssertFinding(t, res.Findings["acme-public"], w2DDBCodePublicPolicy, "resource policy open to anyone", domain.SevBroken, "wave2:ddb")
	w2AssertRow(t, w2Rows(t, res, "acme-public", w2DDBCodePublicPolicy), "Principal", "*")
	w2AssertFindingDef(t, "ddb", w2DDBCodePublicPolicy, "resource policy open to anyone", domain.SevBroken, "wave2")

	// A public policy is not also a cross-account policy — one table, one row.
	w2AssertNoCode(t, res.Findings["acme-public"], w2DDBCodeCrossAccountPolicy)

	// A table with no policy at all is clean and not unknown.
	w2AssertNoCode(t, res.Findings["acme-billing"], w2DDBCodePublicPolicy)
	if res.TruncatedIDs["acme-billing"] {
		t.Error("PolicyNotFoundException marked the table unknown; it is a definite no-policy answer")
	}
}

func TestW2DDBCrossAccountPolicy(t *testing.T) {
	fake := &w2DDBPolicyFake{policies: map[string]string{"acme-shared": w2DDBCrossAccountPolicyDoc}}
	res := w2DDBEnrich(t, fake, "acme-shared")

	w2AssertFinding(t, res.Findings["acme-shared"], w2DDBCodeCrossAccountPolicy, "resource policy grants another account", domain.SevWarn, "wave2:ddb")
	w2AssertRow(t, w2Rows(t, res, "acme-shared", w2DDBCodeCrossAccountPolicy), "Accounts", "210987654321, 333333333333")
	w2AssertNoCode(t, res.Findings["acme-shared"], w2DDBCodePublicPolicy)
	w2AssertFindingDef(t, "ddb", w2DDBCodeCrossAccountPolicy, "resource policy grants another account", domain.SevWarn, "wave2")
}

// A policy naming only this account grants nobody new anything.
func TestW2DDBOwnAccountPolicyIsClean(t *testing.T) {
	doc := `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":"arn:aws:iam::123456789012:role/app"},"Action":"dynamodb:GetItem","Resource":"*"}]}`
	fake := &w2DDBPolicyFake{policies: map[string]string{"acme-internal": doc}}
	res := w2DDBEnrich(t, fake, "acme-internal")

	w2AssertNoCode(t, res.Findings["acme-internal"], w2DDBCodeCrossAccountPolicy)
	w2AssertNoCode(t, res.Findings["acme-internal"], w2DDBCodePublicPolicy)
}

// IAM accepts a single Statement object as well as an array. A reader that
// only handles the array form silently sees no statements and reports clean.
func TestW2DDBSingleStatementObjectIsStillEvaluated(t *testing.T) {
	doc := `{"Version":"2012-10-17","Statement":{"Effect":"Allow","Principal":"*","Action":"dynamodb:Scan","Resource":"*"}}`
	fake := &w2DDBPolicyFake{policies: map[string]string{"acme-public": doc}}
	res := w2DDBEnrich(t, fake, "acme-public")

	w2AssertFinding(t, res.Findings["acme-public"], w2DDBCodePublicPolicy, "resource policy open to anyone", domain.SevBroken, "wave2:ddb")
}

// A wildcard principal narrowed by a source-VPCE condition is a scoped grant,
// not an open door. Flagging it trains operators to ignore the signal.
func TestW2DDBConditionedWildcardIsNotPublic(t *testing.T) {
	doc := `{
      "Version":"2012-10-17",
      "Statement":[{
        "Effect":"Allow",
        "Principal":"*",
        "Action":"dynamodb:GetItem",
        "Resource":"*",
        "Condition":{"StringEquals":{"aws:SourceVpce":["vpce-0abc123"]}}
      }]
    }`
	fake := &w2DDBPolicyFake{policies: map[string]string{"acme-scoped": doc}}
	res := w2DDBEnrich(t, fake, "acme-scoped")

	w2AssertNoCode(t, res.Findings["acme-scoped"], w2DDBCodePublicPolicy)
	w2AssertNoCode(t, res.Findings["acme-scoped"], w2DDBCodeCrossAccountPolicy)
}

func TestW2DDBPolicyErrorMarksTruncatedAndSparesTheRest(t *testing.T) {
	fake := &w2DDBPolicyFake{
		policies: map[string]string{"acme-public": w2DDBPublicPolicyDoc},
		errs: map[string]error{
			"acme-denied": &smithy.GenericAPIError{Code: "AccessDeniedException", Message: "denied"},
		},
	}
	rs := []resource.Resource{w2DDBPolicyResource("acme-denied"), w2DDBPolicyResource("acme-public")}
	res, err := w2DDBRun(t, fake, rs)
	if err == nil {
		t.Error("a denied policy read was not folded into the composite error")
	}

	if !res.TruncatedIDs["acme-denied"] {
		t.Error("table whose policy could not be read was not marked in TruncatedIDs")
	}
	w2AssertNoCode(t, res.Findings["acme-denied"], w2DDBCodePublicPolicy)
	w2AssertFinding(t, res.Findings["acme-public"], w2DDBCodePublicPolicy, "resource policy open to anyone", domain.SevBroken, "wave2:ddb")
}

func TestW2DDBPolicyNilClientIsSafe(t *testing.T) {
	res, err := w2Enricher(t, "ddb")(context.Background(), &awsclient.ServiceClients{},
		[]resource.Resource{w2DDBPolicyResource("acme-public")}, nil)
	w2AssertEnricherInvariants(t, res, err)
	w2AssertNoCode(t, res.Findings["acme-public"], w2DDBCodePublicPolicy)
}

// Same contract as the redshift case: a denied policy read makes the issue
// count a lower bound, and the flag saying so must reach the caller — here the
// mutating call is likewise an operand of the returning statement.
func TestW2DDBFailureAloneRaisesTruncated(t *testing.T) {
	fake := &w2DDBPolicyFake{
		policies: map[string]string{"acme-public": w2DDBPublicPolicyDoc},
		errs: map[string]error{
			"acme-denied": &smithy.GenericAPIError{Code: "AccessDeniedException", Message: "denied"},
		},
	}
	rs := []resource.Resource{w2DDBPolicyResource("acme-denied"), w2DDBPolicyResource("acme-public")}
	res, err := w2DDBRun(t, fake, rs)
	if err == nil {
		t.Fatal("expected a composite error for the denied table")
	}
	if !res.Truncated {
		t.Error("a failed item left Truncated false; the issue count is presented as complete when it is a lower bound")
	}
}

// Same contract on the Wave-2 side: a table being deleted takes its resource
// policy with it, so the policy rows must stay silent even though the read
// still succeeds.
func TestW2DDBDeletingTableEmitsNoWave2Finding(t *testing.T) {
	deleting := w2DDBTable("acme-old")
	deleting.TableStatus = ddbtypes.TableStatusDeleting

	listStub := &w2DDBTablesFake{
		tables: map[string]*ddbtypes.TableDescription{"acme-old": deleting},
		order:  []string{"acme-old"},
	}
	out, err := awsclient.FetchDynamoDBTablesPage(context.Background(), listStub, listStub, "")
	if err != nil {
		t.Fatalf("FetchDynamoDBTablesPage: %v", err)
	}

	fake := &w2DDBPolicyFake{policies: map[string]string{"acme-old": w2DDBPublicPolicyDoc}}
	res, err := w2Enricher(t, "ddb")(context.Background(), w2DDBClients(fake), out.Resources, nil)
	w2AssertEnricherShape(t, res)
	if err != nil {
		t.Fatalf("enricher returned error: %v", err)
	}

	w2AssertNoCode(t, res.Findings["acme-old"], w2DDBCodePublicPolicy)
	w2AssertNoCode(t, res.Findings["acme-old"], w2DDBCodeCrossAccountPolicy)
}
