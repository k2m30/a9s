package unit

// One owner reads a policy attached to a resource and reports it: every type
// whose policy can open it to anyone raises its public finding with the same
// rows, every type whose policy can name another AWS account raises a
// cross-account finding, and a document that does not parse never reads as
// clean.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	apigwtypes "github.com/aws/aws-sdk-go-v2/service/apigateway/types"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"
	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/session"
)

const (
	t3bOwn      = "123456789012"
	t3bForeign  = "210987654321"
	t3bForeign2 = "987654321098"

	// t3bUnparseable is a policy document cut off mid-statement: not JSON, so
	// nothing about who it grants can be read from it.
	t3bUnparseable = `{"Version":"2012-10-17","Statement":[{"Sid":"AllowAnyone","Effect":"Allow","Principal":"*","Action":"*"`

	t3bCrossSuffix = " grants another account"
)

// t3bOut is what one run of a type's real entry point shows for the one
// resource it was given.
type t3bOut struct {
	findings []domain.Finding
	details  map[domain.FindingCode]domain.AttentionDetail
	fields   map[string]string
	mark     string
	marked   bool
	err      error
}

// t3bType is one resource type whose AWS policy a9s evaluates. run drives the
// type's registered Wave 2 enricher, or its fetcher for a Wave 1 type, over
// one resource carrying policy, with session as the account STS answered
// ("" when it did not) and arnAccount as the account the resource's own ARN
// names ("" when it names none).
type t3bType struct {
	short   string
	public  domain.FindingCode
	cross   domain.FindingCode
	actions []string
	// noResource: the type's policy has no Resource element (a repository
	// policy, a trust policy).
	noResource bool
	// fixedRows is the row set a finding keeps that reads only whether the
	// policy is public; nil means the finding carries the policy rows.
	fixedRows []domain.DetailRow
	run       func(t *testing.T, policy, session, arnAccount string) t3bOut
}

func (ty t3bType) wave1() bool { return catalog.FindAny(ty.short).Policy.Source == "wave1" }

var t3bTypes = []t3bType{ //nolint:gochecknoglobals // test-only table
	{short: "sqs", public: "sqs.public-policy", cross: "sqs.cross-account-policy",
		actions: []string{"sqs:SendMessage", "sqs:ReceiveMessage"}, run: t3bRunSQS},
	{short: "sns", public: "sns.public-policy", cross: "sns.cross-account-policy",
		actions: []string{"sns:Publish", "sns:Subscribe"}, run: t3bRunSNS},
	{short: "kms", public: "kms.public-policy", cross: "kms.cross-account-policy",
		actions: []string{"kms:Encrypt", "kms:Decrypt"}, run: t3bRunKMS},
	{short: "secrets", public: "secrets.public-policy", cross: "secrets.cross-account-policy",
		actions: []string{"secretsmanager:GetSecretValue", "secretsmanager:DescribeSecret"}, run: t3bRunSecrets},
	{short: "ddb", public: "ddb.public-policy", cross: "ddb.cross-account-policy",
		actions: []string{"dynamodb:GetItem", "dynamodb:Query"}, run: t3bRunDDB},
	{short: "efs", public: "efs.public-policy", cross: "efs.cross-account-policy",
		actions: []string{"elasticfilesystem:ClientMount", "elasticfilesystem:ClientWrite"}, run: t3bRunEFS},
	{short: "lambda", public: "lambda.public-policy", cross: "lambda.cross-account-policy",
		actions: []string{"lambda:InvokeFunction", "lambda:GetFunction"}, run: t3bRunLambda},
	{short: "ecr", public: "ecr.public-policy", cross: "ecr.cross-account-policy",
		actions: []string{"ecr:BatchGetImage", "ecr:GetDownloadUrlForLayer"}, noResource: true, run: t3bRunECR},
	{short: "codeartifact", public: "codeartifact.public-access-policy", cross: "codeartifact.cross-account-policy",
		actions: []string{"codeartifact:ReadFromRepository", "codeartifact:PublishPackageVersion"}, run: t3bRunCodeArtifact},
	{short: "apigw", public: "apigw.no-authorizer-public",
		actions: []string{"execute-api:Invoke"}, run: t3bRunAPIGW,
		fixedRows: []domain.DetailRow{{Label: "Authorizers", Value: "0", Tier: "!"}, {Label: "Endpoint", Value: "regional", Tier: "!"}}},
	{short: "opensearch", public: "opensearch.public",
		actions: []string{"es:ESHttpGet", "es:ESHttpPost"}, run: t3bRunOpenSearch,
		fixedRows: []domain.DetailRow{{Label: "Endpoint", Value: "public", Tier: "!"}, {Label: "Access policy", Value: "open", Tier: "!"}}},
	{short: "role", public: "role.trust.wildcard-principal",
		actions: []string{"sts:AssumeRole", "sts:TagSession"}, noResource: true, run: t3bRunRole},
	{short: "vpce", public: "vpce.policy-open",
		actions: []string{"*"}, run: t3bRunVPCE},
}

func t3bDoc(stmts ...map[string]any) string {
	b, err := json.Marshal(map[string]any{"Version": "2012-10-17", "Statement": stmts})
	if err != nil {
		panic(err)
	}
	return string(b)
}

func (ty t3bType) stmt(effect string, principal any, actionKey string, actions []string, cond map[string]any) map[string]any {
	st := map[string]any{"Effect": effect, "Principal": principal, actionKey: actions}
	if !ty.noResource {
		st["Resource"] = "*"
	}
	if cond != nil {
		st["Condition"] = cond
	}
	return st
}

// publicDoc grants the type's actions to everyone under a condition that
// scopes no caller, beside a conditional Deny that closes nothing: the
// finding has every row the owner renders.
func (ty t3bType) publicDoc() string {
	return t3bDoc(
		ty.stmt("Allow", "*", "Action", ty.actions, map[string]any{"Bool": map[string]any{"aws:SecureTransport": "true"}}),
		ty.stmt("Deny", "*", "Action", ty.actions, map[string]any{"Bool": map[string]any{"aws:SecureTransport": "false"}}),
	)
}

// notActionDoc grants everyone every action but one: there is no list of
// granted actions to show.
func (ty t3bType) notActionDoc() string {
	return t3bDoc(ty.stmt("Allow", "*", "NotAction", ty.actions[:1], nil))
}

func (ty t3bType) foreignDoc() string {
	return t3bDoc(ty.stmt("Allow", map[string]any{"AWS": []string{
		"arn:aws:iam::" + t3bForeign + ":root",
		"arn:aws:iam::" + t3bForeign + ":role/acme-partner-reader",
		"arn:aws:iam::" + t3bForeign2 + ":user/acme-partner-ci",
	}}, "Action", ty.actions, nil))
}

// ownDoc grants a role of the owning account and an AWS service principal:
// the normal shape of a resource policy, shared with nobody outside.
func (ty t3bType) ownDoc() string {
	return t3bDoc(
		ty.stmt("Allow", map[string]any{"AWS": "arn:aws:iam::" + t3bOwn + ":role/acme-app-role"}, "Action", ty.actions, nil),
		ty.stmt("Allow", map[string]any{"Service": "events.amazonaws.com"}, "Action", ty.actions[:1], nil),
	)
}

func t3bARN(service, region, account, res string) string {
	return "arn:aws:" + service + ":" + region + ":" + account + ":" + res
}

func t3bPtr(s string) *string {
	if s == "" {
		return nil
	}
	return aws.String(s)
}

func t3bIdentity(c *awsclient.ServiceClients, account string) {
	store := session.NewIdentityStore()
	if account == "" {
		store.Set("", errors.New("AccessDenied: sts:GetCallerIdentity"))
	} else {
		store.Set(account, nil)
	}
	c.SetIdentityStore(store)
}

func t3bWave2(t *testing.T, short string, c *awsclient.ServiceClients, session string, r resource.Resource, cache resource.ResourceCache) t3bOut {
	t.Helper()
	t3bIdentity(c, session)
	res, err := w2Enricher(t, short)(context.Background(), c, []resource.Resource{r}, cache)
	mark, marked := res.TruncatedIDs[r.ID]
	return t3bOut{findings: res.Findings[r.ID], details: res.AttentionDetails[r.ID], mark: mark, marked: marked, err: err}
}

func t3bWave1(r resource.Resource) t3bOut {
	return t3bOut{findings: r.Findings, details: r.AttentionDetails, fields: r.Fields}
}

func t3bRunSQS(t *testing.T, policy, session, acct string) t3bOut {
	const name = "acme-orders"
	url := "https://sqs.us-east-1.amazonaws.com/" + acct + "/" + name
	arn := ""
	if acct != "" {
		arn = t3bARN("sqs", "us-east-1", acct, name)
	}
	f := newW5SQSFake()
	f.attrs[url] = map[string]string{
		"QueueArn":       arn,
		"RedrivePolicy":  `{"deadLetterTargetArn":"arn:aws:sqs:us-east-1:123456789012:acme-orders-dlq","maxReceiveCount":"5"}`,
		"KmsMasterKeyId": "alias/aws/sqs",
		"Policy":         policy,
	}
	r := resource.Resource{ID: name, Name: name, Type: "sqs", Fields: map[string]string{
		"queue_name": name, "queue_url": url, "arn": arn,
	}}
	return t3bWave2(t, "sqs", &awsclient.ServiceClients{SQS: f, Region: "us-east-1"}, session, r, nil)
}

func t3bRunSNS(t *testing.T, policy, session, acct string) t3bOut {
	const name = "acme-order-events"
	arn := t3bARN("sns", "us-east-1", acct, name)
	f := newW5SNSFake()
	f.topicAttrs[arn] = map[string]string{"TopicArn": arn, "Policy": policy, "KmsMasterKeyId": "alias/aws/sns"}
	f.subs[arn] = []snstypes.Subscription{{
		SubscriptionArn: aws.String(arn + ":11111111-2222-3333-4444-555555555555"),
		TopicArn:        aws.String(arn),
		Protocol:        aws.String("sqs"),
		Endpoint:        aws.String("arn:aws:sqs:us-east-1:123456789012:acme-orders"),
	}}
	r := resource.Resource{ID: arn, Name: name, Type: "sns", Fields: map[string]string{"topic_arn": arn}}
	return t3bWave2(t, "sns", &awsclient.ServiceClients{SNS: f, Region: "us-east-1"}, session, r, nil)
}

func t3bRunKMS(t *testing.T, policy, session, acct string) t3bOut {
	const id = "1a2b3c4d-1111-2222-3333-444455556666"
	arn := t3bARN("kms", "us-east-1", acct, "key/"+id)
	r := resource.Resource{
		ID: id, Name: "alias/acme-shared-key", Type: "kms",
		Fields: map[string]string{"key_id": id, "alias": "alias/acme-shared-key", "status": "Enabled", "arn": arn},
		RawStruct: kmstypes.KeyMetadata{
			KeyId: aws.String(id), Arn: aws.String(arn), AWSAccountId: t3bPtr(acct),
			KeyManager: kmstypes.KeyManagerTypeCustomer, KeyState: kmstypes.KeyStateEnabled,
		},
	}
	f := &w4KMSFake{policies: map[string]string{id: policy}}
	return t3bWave2(t, "kms", &awsclient.ServiceClients{KMS: f, Region: "us-east-1"}, session, r, nil)
}

func t3bRunSecrets(t *testing.T, policy, session, acct string) t3bOut {
	const name = "acme-partner-token"
	arn := t3bARN("secretsmanager", "us-east-1", acct, "secret:"+name+"-AbCdEf")
	f := &w4SecretsFake{policies: map[string]string{name: policy, arn: policy}}
	r := resource.Resource{ID: name, Name: name, Type: "secrets", Fields: map[string]string{
		"secret_name": name, "arn": arn, "status": "OK", "rotation_enabled": "true",
	}}
	return t3bWave2(t, "secrets", &awsclient.ServiceClients{SecretsManager: f, Region: "us-east-1"}, session, r, nil)
}

// t3bDDBFake answers the two DynamoDB reads the table enricher makes: the
// table's resource policy, and point-in-time recovery on.
type t3bDDBFake struct {
	awsclient.DynamoDBAPI
	policy string
}

func (f *t3bDDBFake) GetResourcePolicy(_ context.Context, _ *dynamodb.GetResourcePolicyInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetResourcePolicyOutput, error) {
	if f.policy == "" {
		return nil, &smithy.GenericAPIError{Code: "PolicyNotFoundException", Message: "no policy"}
	}
	return &dynamodb.GetResourcePolicyOutput{Policy: aws.String(f.policy), RevisionId: aws.String("1730000000000")}, nil
}

func (f *t3bDDBFake) DescribeContinuousBackups(_ context.Context, _ *dynamodb.DescribeContinuousBackupsInput, _ ...func(*dynamodb.Options)) (*dynamodb.DescribeContinuousBackupsOutput, error) {
	return &dynamodb.DescribeContinuousBackupsOutput{ContinuousBackupsDescription: &ddbtypes.ContinuousBackupsDescription{
		ContinuousBackupsStatus: ddbtypes.ContinuousBackupsStatusEnabled,
		PointInTimeRecoveryDescription: &ddbtypes.PointInTimeRecoveryDescription{
			PointInTimeRecoveryStatus: ddbtypes.PointInTimeRecoveryStatusEnabled,
		},
	}}, nil
}

func t3bRunDDB(t *testing.T, policy, session, acct string) t3bOut {
	const name = "acme-orders"
	arn := t3bARN("dynamodb", "eu-central-1", acct, "table/"+name)
	tb := w2DDBTable(name)
	tb.TableArn = aws.String(arn)
	r := w2Res(name, tb)
	r.Type = "ddb"
	r.Fields["arn"] = arn
	backupLoaded := resource.ResourceCache{"backup": {Resources: []resource.Resource{}}}
	return t3bWave2(t, "ddb", &awsclient.ServiceClients{DynamoDB: &t3bDDBFake{policy: policy}, Region: "eu-central-1"}, session, r, backupLoaded)
}

func t3bRunEFS(t *testing.T, policy, session, acct string) t3bOut {
	const id = "fs-0acme00000000001"
	fs := w2EFSFileSystem(id, "acme-shared")
	fs.FileSystemArn = aws.String(t3bARN("elasticfilesystem", "eu-central-1", acct, "file-system/"+id))
	fs.OwnerId = t3bPtr(acct)
	r := w2Res(id, fs)
	r.Type = "efs"
	r.Fields["file_system_id"] = id
	f := &w2EFSPolicyFake{policies: map[string]string{id: policy}}
	return t3bWave2(t, "efs", &awsclient.ServiceClients{EFS: f, Region: "eu-central-1"}, session, r, nil)
}

func t3bRunLambda(t *testing.T, policy, session, acct string) t3bOut {
	const name = "acme-checkout"
	r := resource.Resource{ID: name, Name: name, Type: "lambda", Fields: map[string]string{
		"function_name": name, "arn": t3bARN("lambda", "us-east-1", acct, "function:"+name), "state": "Active",
	}}
	f := &pw1LambdaPostureFake{policies: map[string]string{name: policy}}
	return t3bWave2(t, "lambda", &awsclient.ServiceClients{Lambda: f, Region: "us-east-1"}, session, r, nil)
}

func t3bRunECR(t *testing.T, policy, session, acct string) t3bOut {
	const name = "acme/base-images"
	arn := t3bARN("ecr", "us-east-1", acct, "repository/"+name)
	repo := w6bECRRepo(name)
	repo.RepositoryArn = aws.String(arn)
	repo.RegistryId = t3bPtr(acct)
	r := resource.Resource{ID: name, Name: name, Type: "ecr", Fields: map[string]string{"repository_name": name, "arn": arn}, RawStruct: repo}
	f := &w6bECRFake{policies: map[string]string{name: policy}, lifecycles: map[string]string{name: w6bECRLifecycleDoc}}
	return t3bWave2(t, "ecr", &awsclient.ServiceClients{ECR: f, Region: "us-east-1"}, session, r, nil)
}

func t3bRunCodeArtifact(t *testing.T, policy, session, acct string) t3bOut {
	r := resource.Resource{ID: "acme-artifacts/acme-npm", Name: "acme-npm", Type: "codeartifact", Fields: map[string]string{
		"repo_name": "acme-npm", "domain_name": "acme-artifacts", "domain_owner": acct,
		"arn": t3bARN("codeartifact", "us-east-1", acct, "repository/acme-artifacts/acme-npm"),
	}}
	f := &d3CodeArtifactFake{policy: policy}
	return t3bWave2(t, "codeartifact", &awsclient.ServiceClients{CodeArtifact: f, Region: "us-east-1"}, session, r, nil)
}

func t3bRunAPIGW(t *testing.T, policy, session, _ string) t3bOut {
	r := w6aRESTRes("a1b2c3d4e5", "acme-orders-rest", "REGIONAL", policy)
	r.Type = "apigw"
	c := &awsclient.ServiceClients{
		APIGatewayV1: &w6aAPIGWV1Fake{stages: []apigwtypes.Stage{w6aV1Stage("prod")}},
		APIGatewayV2: &w6aAPIGWV2Fake{},
		Region:       "eu-central-1",
	}
	return t3bWave2(t, "apigw", c, session, r, nil)
}

func t3bRunOpenSearch(t *testing.T, policy, _, _ string) t3bOut {
	d := w2OSDomain("acme-search")
	d.VPCOptions = nil
	d.Endpoint = aws.String("search-acme-search-abc123.eu-central-1.es.amazonaws.com")
	d.AccessPolicies = aws.String(policy)
	rows, _ := w2OSRun(t, d)
	return t3bWave1(rows["acme-search"])
}

func t3bRunRole(t *testing.T, policy, _, _ string) t3bOut {
	const name = "acme-deploy-role"
	fake := &w4RoleListFake{roles: []iamtypes.Role{w4Role(name, "/", policy)}}
	return t3bWave1(w4FetchRole(t, fake, name))
}

func t3bRunVPCE(t *testing.T, policy, _, _ string) t3bOut {
	return t3bWave1(d3FetchVPCE(t, policy))
}

func t3bFind(fs []domain.Finding, code domain.FindingCode) (domain.Finding, bool) {
	for _, f := range fs {
		if f.Code == code {
			return f, true
		}
	}
	return domain.Finding{}, false
}

func t3bCodes(fs []domain.Finding) []domain.FindingCode {
	out := make([]domain.FindingCode, 0, len(fs))
	for _, f := range fs {
		out = append(out, f.Code)
	}
	return out
}

func t3bTier(code domain.FindingCode) string {
	if catalog.Severity(code) == domain.SevBroken {
		return "!"
	}
	return "~"
}

func t3bAssertRows(t *testing.T, got, want []domain.DetailRow) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Errorf("rows =\n  %+v\nwant\n  %+v", got, want)
	}
}

func t3bSortedActions(actions []string) string {
	s := slices.Clone(actions)
	slices.Sort(s)
	return strings.Join(s, ", ")
}

func t3bCrossTypes() []t3bType {
	var out []t3bType
	for _, ty := range t3bTypes {
		if ty.cross != "" {
			out = append(out, ty)
		}
	}
	return out
}

func t3bDef(short string, code domain.FindingCode) (catalog.FindingDef, bool) {
	td := catalog.FindAny(short)
	if td == nil {
		return catalog.FindingDef{}, false
	}
	for _, d := range td.Findings {
		if d.Code == code {
			return d, true
		}
	}
	return catalog.FindingDef{}, false
}

// One owner renders the rows of a public grant, so an operator comparing an
// open key with an open queue reads the same facts in the same place and
// emphasis. A finding that reads only whether the policy is public keeps the
// rows it has.
func TestT3B_PublicGrantRaisesTheTypesFindingWithTheSharedRows(t *testing.T) {
	for _, ty := range t3bTypes {
		t.Run(ty.short, func(t *testing.T) {
			out := ty.run(t, ty.publicDoc(), t3bOwn, t3bOwn)
			if out.err != nil {
				t.Errorf("entry point returned %v for a run where every call answered", out.err)
			}
			f, ok := t3bFind(out.findings, ty.public)
			if !ok {
				t.Fatalf("no %s finding; got %v", ty.public, t3bCodes(out.findings))
			}
			if f.Phrase != catalog.Phrase(ty.public) || f.Severity != catalog.Severity(ty.public) {
				t.Errorf("finding = %q/%v, want the declared %q/%v", f.Phrase, f.Severity, catalog.Phrase(ty.public), catalog.Severity(ty.public))
			}
			want := ty.fixedRows
			if want == nil {
				tier := t3bTier(ty.public)
				want = []domain.DetailRow{
					{Label: "Principal", Value: "*", Tier: tier},
					{Label: "Actions", Value: t3bSortedActions(ty.actions), Tier: tier},
					{Label: "Condition", Value: "Bool aws:SecureTransport true", Tier: tier},
					{Label: "Deny when", Value: "Bool aws:SecureTransport false", Tier: tier},
				}
			}
			t3bAssertRows(t, out.details[ty.public].Rows, want)

			healthy := ty.run(t, ty.ownDoc(), t3bOwn, t3bOwn)
			if _, ok := t3bFind(healthy.findings, ty.public); ok {
				t.Errorf("a policy granting only the owning account raised %s", ty.public)
			}
		})
	}
}

// Whether a type reports a grant to another account is a declaration on the
// type, not an accident of who wrote its enricher. Secrets and DynamoDB set
// the wording and grade, so a share reads the same on every type.
func TestT3B_EveryPolicyTypeDeclaresACrossAccountCode(t *testing.T) {
	for _, ty := range t3bCrossTypes() {
		t.Run(ty.short, func(t *testing.T) {
			def, ok := t3bDef(ty.short, ty.cross)
			if !ok {
				t.Fatalf("type %s declares no %s finding", ty.short, ty.cross)
			}
			if def.Severity != domain.SevWarn || def.Source != "wave2" || !strings.HasSuffix(def.Phrase, t3bCrossSuffix) {
				t.Errorf("%s = %q/%v/%s, want a wave2 warning whose phrase ends %q", ty.cross, def.Phrase, def.Severity, def.Source, t3bCrossSuffix)
			}
		})
	}
}

// A policy naming principals of other accounts — a root, a role, a user —
// raises the type's cross-account finding listing each account once. The
// same policy shape naming only the owning account, beside an AWS service
// principal, shares nothing.
func TestT3B_ForeignAccountGrantRaisesTheCrossAccountFinding(t *testing.T) {
	for _, ty := range t3bCrossTypes() {
		t.Run(ty.short, func(t *testing.T) {
			out := ty.run(t, ty.foreignDoc(), t3bOwn, t3bOwn)
			if out.err != nil {
				t.Errorf("entry point returned %v for a run where every call answered", out.err)
			}
			if _, ok := t3bFind(out.findings, ty.public); ok {
				t.Errorf("a grant to named accounts raised the public finding %s", ty.public)
			}
			f, ok := t3bFind(out.findings, ty.cross)
			if !ok {
				t.Fatalf("no %s finding; got %v", ty.cross, t3bCodes(out.findings))
			}
			if f.Severity != domain.SevWarn || f.Source != "wave2" || f.Phrase != catalog.Phrase(ty.cross) || !strings.HasSuffix(f.Phrase, t3bCrossSuffix) {
				t.Errorf("finding = %q/%v/%s, want the declared wave2 warning ending %q", f.Phrase, f.Severity, f.Source, t3bCrossSuffix)
			}
			t3bAssertRows(t, out.details[ty.cross].Rows, []domain.DetailRow{
				{Label: "Accounts", Value: t3bForeign + ", " + t3bForeign2, Tier: "~"},
			})

			own := ty.run(t, ty.ownDoc(), t3bOwn, t3bOwn)
			if codes := t3bCodes(own.findings); slices.Contains(codes, ty.cross) || slices.Contains(codes, ty.public) {
				t.Errorf("a policy granting only the owning account raised %v", t3bCodes(own.findings))
			}
			if own.marked {
				t.Errorf("a policy granting only the owning account left the row not inspected (%q)", own.mark)
			}
		})
	}
}

// When STS has not answered, the owning account is the one the resource's ARN
// names: a foreign grant is still foreign, and the account's own role is not
// an outside share. When neither names it, whether a grant is foreign cannot
// be told, and the row says the check did not run.
func TestT3B_OwnAccountUnknownFallsBackToTheResourceARN(t *testing.T) {
	for _, ty := range t3bCrossTypes() {
		t.Run(ty.short, func(t *testing.T) {
			foreign := ty.run(t, ty.foreignDoc(), "", t3bOwn)
			if f, ok := t3bFind(foreign.findings, ty.cross); !ok {
				t.Errorf("STS unknown, ARN names %s: no %s finding; got %v", t3bOwn, ty.cross, t3bCodes(foreign.findings))
			} else if f.Phrase != catalog.Phrase(ty.cross) {
				t.Errorf("phrase %q, want %q", f.Phrase, catalog.Phrase(ty.cross))
			} else {
				t3bAssertRows(t, foreign.details[ty.cross].Rows, []domain.DetailRow{
					{Label: "Accounts", Value: t3bForeign + ", " + t3bForeign2, Tier: "~"},
				})
			}

			own := ty.run(t, ty.ownDoc(), "", t3bOwn)
			if _, ok := t3bFind(own.findings, ty.cross); ok {
				t.Errorf("STS unknown, ARN names %s: the account's own role was reported as another account", t3bOwn)
			}
			if own.marked {
				t.Errorf("STS unknown, ARN names %s: row left not inspected (%q)", t3bOwn, own.mark)
			}

			unknown := ty.run(t, ty.foreignDoc(), "", "")
			if _, ok := t3bFind(unknown.findings, ty.cross); ok {
				t.Errorf("owning account unknown: %s raised without knowing which account is foreign", ty.cross)
			}
			if !unknown.marked || unknown.mark != awsclient.CheckOwnAccountUnknown {
				t.Errorf("owning account unknown: mark = %q (marked %v), want %q", unknown.mark, unknown.marked, awsclient.CheckOwnAccountUnknown)
			}
		})
	}
}

// A NotAction grant opens every action but the listed ones, so there is no
// list of granted actions: the finding has no Actions row, not an empty one.
func TestT3B_NotActionGrantHasNoActionsRow(t *testing.T) {
	for _, ty := range t3bTypes {
		if ty.fixedRows != nil || ty.short == "vpce" {
			continue
		}
		t.Run(ty.short, func(t *testing.T) {
			out := ty.run(t, ty.notActionDoc(), t3bOwn, t3bOwn)
			if _, ok := t3bFind(out.findings, ty.public); !ok {
				t.Fatalf("a NotAction grant to everyone raised no %s; got %v", ty.public, t3bCodes(out.findings))
			}
			t3bAssertRows(t, out.details[ty.public].Rows, []domain.DetailRow{
				{Label: "Principal", Value: "*", Tier: t3bTier(ty.public)},
			})
		})
	}
}

// A Wave 1 verdict read from a policy that does not parse is unknown: the
// row carries the type's "not inspected" finding instead of claiming the
// domain is private, the trust is not a wildcard, or the endpoint policy is
// scoped, and the finding that needs the verdict is not raised as if it had
// been read.
func TestT3B_Wave1UnparseablePolicyIsUnknown(t *testing.T) {
	for _, ty := range t3bTypes {
		if !ty.wave1() {
			continue
		}
		t.Run(ty.short, func(t *testing.T) {
			out := ty.run(t, t3bUnparseable, t3bOwn, t3bOwn)
			if unread := catalog.FindAny(ty.short).Policy.Unread; unread == "" || !slices.ContainsFunc(out.findings, func(f domain.Finding) bool { return f.Code == unread }) {
				t.Errorf("no %q finding for a policy that does not parse; got %v", unread, t3bCodes(out.findings))
			}
			if _, ok := t3bFind(out.findings, ty.public); ok {
				t.Errorf("%s raised from a policy that does not parse", ty.public)
			}
		})
	}
}

// A Wave 2 row whose policy does not parse is not inspected, and no finding
// is read from it. Nothing refused: the type's run returns no error.
func TestT3B_Wave2UnparseablePolicyMarksTheRowNotInspected(t *testing.T) {
	for _, ty := range t3bTypes {
		if ty.wave1() {
			continue
		}
		t.Run(ty.short, func(t *testing.T) {
			out := ty.run(t, t3bUnparseable, t3bOwn, t3bOwn)
			if out.err != nil {
				t.Errorf("returned %v; an unparseable document is no failed call", out.err)
			}
			if !out.marked {
				t.Errorf("row not marked not inspected")
			}
			for _, code := range []domain.FindingCode{ty.public, ty.cross} {
				if _, ok := t3bFind(out.findings, code); code != "" && ok {
					t.Errorf("%s raised from a policy that does not parse", code)
				}
			}
		})
	}
}

// t3bPolicyCodes is every catalog finding read from a resource policy: the
// public and cross-account codes each type's policy declaration names, and
// the findings whose verdict is "is the policy open".
func t3bPolicyCodes(t *testing.T) map[domain.FindingCode]catalog.FindingDef {
	t.Helper()
	out := map[domain.FindingCode]catalog.FindingDef{}
	fixed := []domain.FindingCode{"opensearch.public", "apigw.no-authorizer-public", "apigw.no-authorizer"}
	for _, td := range append(catalog.All(), catalog.AllChildren()...) {
		for _, d := range td.Findings {
			declared := td.Policy != nil && (d.Code == td.Policy.Public || d.Code == td.Policy.Cross)
			if declared || slices.Contains(fixed, d.Code) {
				out[d.Code] = d
			}
		}
	}
	if len(out) < 16 {
		t.Fatalf("found %d policy finding definitions, want at least 16: the catalog is not installed", len(out))
	}
	return out
}

// A policy is public when any action is open to everyone, and cross-account
// when any action is granted to another account: neither verdict says which
// action. The detail sentence states the grant; the Actions row states the
// reach. A sentence naming one consequence — decrypting, reading the
// credential, pulling images, reading and writing, invoking, sending or
// receiving, publishing or subscribing, calling every route — claims what the
// code did not check.
func TestT3B_PolicyFindingDetailStatesTheGrantNotAConsequence(t *testing.T) {
	banned := []string{"decrypt", "read the credential", "pull the images", "pull images", "read and write",
		"invoke it", "can invoke", "send or receive", "publish or subscribe", "every route"}
	for code, d := range t3bPolicyCodes(t) {
		detail := strings.ToLower(d.Detail)
		for _, b := range banned {
			if strings.Contains(detail, b) {
				t.Errorf("%s detail names %q, a consequence the verdict did not check: %q", code, b, d.Detail)
			}
		}
	}
}

// A type that starts declaring a policy code is held to the same rows as the
// rest: with no driver it would pass every row test by never being run.
func TestT3B_EveryTypeDeclaringAPolicyCodeIsDriven(t *testing.T) {
	driven := map[string]t3bType{}
	for _, ty := range t3bTypes {
		driven[ty.short] = ty
	}
	for code := range t3bPolicyCodes(t) {
		if short, _, _ := strings.Cut(string(code), "."); driven[short].short == "" {
			t.Errorf("%s declares %s and no row of t3bTypes drives it", short, code)
		}
	}
	for _, td := range append(catalog.All(), catalog.AllChildren()...) {
		p := td.Policy
		if p == nil {
			continue
		}
		ty, ok := driven[td.ShortName]
		switch {
		case !ok:
			t.Errorf("%s declares a policy and no row of t3bTypes drives it", td.ShortName)
		case p.Public != "" && ty.public != p.Public:
			t.Errorf("%s declares %s; its t3bTypes row expects %q", td.ShortName, p.Public, ty.public)
		case ty.cross != p.Cross:
			t.Errorf("%s declares cross %q; its t3bTypes row expects %q", td.ShortName, p.Cross, ty.cross)
		}
	}
}

var t3bEvaluators = []string{"Parse", "Evaluate", "EvaluateEndpoint", "EvaluateTrust"} //nolint:gochecknoglobals // test-only list

const t3bOwnerFile = "policy_findings.go"

// t3bRowBuilders is every function the owner file declares that builds rows
// for a policy finding. The admin-attached row describes an identity policy
// attachment, not a policy document, and is not one of them.
func t3bRowBuilders(t *testing.T, owner *ast.File) []string {
	t.Helper()
	names := []string{"publicPolicyRows", "publicConditionRows", "crossAccountPolicyRows"}
	for _, decl := range owner.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv != nil || fn.Type.Results == nil || len(fn.Type.Results.List) != 1 || fn.Name.Name == "adminAttachedRows" {
			continue
		}
		if arr, ok := fn.Type.Results.List[0].Type.(*ast.ArrayType); ok {
			if sel, ok := arr.Elt.(*ast.SelectorExpr); ok && sel.Sel.Name == "DetailRow" && !slices.Contains(names, fn.Name.Name) {
				names = append(names, fn.Name.Name)
			}
		}
	}
	return names
}

// t3bOwnerViolations reads the iampolicy import name from the file itself, so
// an aliased import is caught too.
func t3bOwnerViolations(fset *token.FileSet, name string, f *ast.File, builders []string) []string {
	if name == t3bOwnerFile {
		return nil
	}
	pkg := ""
	for _, imp := range f.Imports {
		if strings.Trim(imp.Path.Value, `"`) == "github.com/k2m30/a9s/v3/core/iampolicy" {
			pkg = "iampolicy"
			if imp.Name != nil {
				pkg = imp.Name.Name
			}
		}
	}
	var out []string
	ast.Inspect(f, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.SelectorExpr:
			if x, ok := n.X.(*ast.Ident); ok && pkg != "" && x.Name == pkg && slices.Contains(t3bEvaluators, n.Sel.Name) {
				out = append(out, fmt.Sprintf("%s: iampolicy.%s", fset.Position(n.Pos()), n.Sel.Name))
			}
		case *ast.Ident:
			if slices.Contains(builders, n.Name) {
				out = append(out, fmt.Sprintf("%s: %s", fset.Position(n.Pos()), n.Name))
			}
		}
		return true
	})
	return out
}

func t3bParseOwner(t *testing.T, fset *token.FileSet, dir string) *ast.File {
	t.Helper()
	owner, err := parser.ParseFile(fset, filepath.Join(dir, t3bOwnerFile), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	return owner
}

// policy_findings.go is the one place a policy attached to a resource is
// parsed, evaluated and turned into rows, so "unparseable is never clean",
// the own-account fallback and the row shape hold for every type at once. A
// parse, an evaluation or a row builder called anywhere else in core/aws is a
// second answer to the same question.
func TestT3B_PolicyIsParsedEvaluatedAndRenderedOnlyByTheOwner(t *testing.T) {
	dir := filepath.Join("..", "..", "core", "aws")
	fset := token.NewFileSet()
	builders := t3bRowBuilders(t, t3bParseOwner(t, fset, dir))
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var violations []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		violations = append(violations, t3bOwnerViolations(fset, name, f, builders)...)
	}
	if len(violations) > 0 {
		t.Errorf("%d policy parses, evaluations or row builds outside %s:\n%s", len(violations), t3bOwnerFile, strings.Join(violations, "\n"))
	}
}

// A scan that finds nothing proves nothing by itself: this is the call a new
// enricher would add.
func TestT3B_OwnerScanCatchesACallAddedInAnEnricher(t *testing.T) {
	const enricher = `package aws
import ip "github.com/k2m30/a9s/v3/core/iampolicy"
func enrichX(policy string) {
	doc, _ := ip.Parse(policy)
	ex := ip.Evaluate(doc, "")
	_ = publicPolicyRows(ex)
}`
	fset := token.NewFileSet()
	builders := t3bRowBuilders(t, t3bParseOwner(t, fset, filepath.Join("..", "..", "core", "aws")))
	f, err := parser.ParseFile(fset, "x_issue_enrichment.go", enricher, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := t3bOwnerViolations(fset, "x_issue_enrichment.go", f, builders); len(got) != 3 {
		t.Errorf("scan of an enricher calling Parse, Evaluate and a row builder found %d sites, want 3: %v", len(got), got)
	}
	if got := t3bOwnerViolations(fset, t3bOwnerFile, f, builders); len(got) != 0 {
		t.Errorf("scan reported the owner file itself: %v", got)
	}
}
