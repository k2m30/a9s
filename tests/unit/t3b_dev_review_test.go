package unit

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/iampolicy"
	"github.com/k2m30/a9s/v3/core/resource"
)

func t3bDevDecl(t *testing.T, short string) catalog.PolicyDecl {
	t.Helper()
	td := catalog.FindAny(short)
	if td == nil || td.Policy == nil {
		t.Fatalf("type %s declares no Policy", short)
	}
	return *td.Policy
}

// A grant to everyone that a condition confines to one account is a grant to
// that account: aws:PrincipalAccount names the caller's account, and a
// service principal acting for a source account acts for that account. An
// organization is no account and raises nothing here.
func TestT3BDev_ConditionNamingAnotherAccountRaisesTheCrossFinding(t *testing.T) {
	for _, ty := range t3bCrossTypes() {
		t.Run(ty.short, func(t *testing.T) {
			cases := []struct {
				name      string
				principal any
				cond      map[string]any
				want      string
			}{
				{"principal account foreign", "*", map[string]any{"StringEquals": map[string]any{"aws:PrincipalAccount": t3bForeign}}, t3bForeign},
				{"principal account own", "*", map[string]any{"StringEquals": map[string]any{"aws:PrincipalAccount": t3bOwn}}, ""},
				{"principal ARN foreign", "*", map[string]any{"ArnEquals": map[string]any{"aws:PrincipalArn": "arn:aws:iam::" + t3bForeign + ":role/acme-partner-reader"}}, t3bForeign},
				{"service for a foreign source", map[string]any{"Service": "events.amazonaws.com"}, map[string]any{"StringEquals": map[string]any{"aws:SourceAccount": t3bForeign}}, t3bForeign},
				{"service for the own source", map[string]any{"Service": "events.amazonaws.com"}, map[string]any{"StringEquals": map[string]any{"aws:SourceAccount": t3bOwn}}, ""},
				{"organization", "*", map[string]any{"StringEquals": map[string]any{"aws:PrincipalOrgID": "o-acme12345"}}, ""},
			}
			for _, c := range cases {
				out := ty.run(t, t3bDoc(ty.stmt("Allow", c.principal, "Action", ty.actions, c.cond)), t3bOwn, t3bOwn)
				if _, ok := t3bFind(out.findings, ty.public); ok {
					t.Errorf("%s: raised %s for a grant a condition confines", c.name, ty.public)
				}
				f, ok := t3bFind(out.findings, ty.cross)
				switch {
				case c.want == "" && ok:
					t.Errorf("%s: raised %s", c.name, ty.cross)
				case c.want != "" && !ok:
					t.Errorf("%s: no %s; got %v", c.name, ty.cross, t3bCodes(out.findings))
				case c.want != "":
					_ = f
					t3bAssertRows(t, out.details[ty.cross].Rows, []domain.DetailRow{{Label: "Accounts", Value: c.want, Tier: "~"}})
				}
			}
		})
	}
}

// The queue side of the cross-account SNS subscription AWS documents (SQS
// Developer Guide, "Subscribing an Amazon SQS queue to an Amazon SNS topic",
// cross-account subscriptions): the queue lets sns.amazonaws.com send for a
// topic in another account.
func TestT3BDev_CrossAccountTopicSubscriptionRaisesTheQueueShare(t *testing.T) {
	ty := t3bTypes[slices.IndexFunc(t3bTypes, func(ty t3bType) bool { return ty.short == "sqs" })]
	doc := func(account string) string {
		return t3bDoc(map[string]any{
			"Effect": "Allow", "Principal": map[string]any{"Service": "sns.amazonaws.com"},
			"Action": "sqs:SendMessage", "Resource": "arn:aws:sqs:us-east-1:" + t3bOwn + ":acme-orders",
			"Condition": map[string]any{"ArnEquals": map[string]any{"aws:SourceArn": "arn:aws:sns:us-east-1:" + account + ":acme-order-events"}},
		})
	}
	out := ty.run(t, doc(t3bForeign), t3bOwn, t3bOwn)
	if _, ok := t3bFind(out.findings, ty.cross); !ok {
		t.Fatalf("a queue a foreign topic may send to raised no %s; got %v", ty.cross, t3bCodes(out.findings))
	}
	t3bAssertRows(t, out.details[ty.cross].Rows, []domain.DetailRow{{Label: "Accounts", Value: t3bForeign, Tier: "~"}})
	if own := ty.run(t, doc(t3bOwn), t3bOwn, t3bOwn); len(own.findings) != 0 || own.marked {
		t.Errorf("the own account's topic raised %v (marked %v)", t3bCodes(own.findings), own.marked)
	}
}

// Where a successful read always carries a document, an answer with none —
// empty, or the JSON null — says nothing about who may call: the row is not
// inspected. Where AWS answers "no policy" with an empty attribute, the same
// answer is no policy and raises nothing.
func TestT3BDev_NoDocumentIsUnknownWhereThePolicyIsRequired(t *testing.T) {
	emptySuccess := []string{"kms", "ecr", "efs", "lambda", "sqs", "sns"}
	// KMS: every key has exactly one key policy. ECR, EFS, Lambda, DynamoDB and
	// CodeArtifact answer a resource with no policy with an error.
	required := []string{"kms", "ecr", "efs", "lambda", "ddb", "codeartifact"}
	for _, ty := range t3bTypes {
		t.Run(ty.short, func(t *testing.T) {
			decl := t3bDevDecl(t, ty.short)
			if want := slices.Contains(required, ty.short); decl.Required != want {
				t.Errorf("Required = %v, want %v", decl.Required, want)
			}
			docs := []string{"null"}
			if slices.Contains(emptySuccess, ty.short) {
				docs = append(docs, "")
			}
			for _, doc := range docs {
				out := ty.run(t, doc, t3bOwn, t3bOwn)
				for _, code := range []domain.FindingCode{ty.public, ty.cross, decl.Unread} {
					// A REST API with no resource policy and no authorizer is open:
					// its finding reads the absence as a verdict.
					if _, ok := t3bFind(out.findings, code); code != "" && ok && ty.short != "apigw" {
						t.Errorf("%q raised %s", doc, code)
					}
				}
				switch {
				case decl.Required && !out.marked:
					t.Errorf("%q: the policy is required, yet the row is not marked not inspected", doc)
				case !decl.Required && out.marked:
					t.Errorf("%q: no policy marked the row %q", doc, out.mark)
				}
			}
		})
	}
}

func TestT3BDev_PolicyFindingWordingComesFromTheDeclaration(t *testing.T) {
	n := 0
	for _, td := range append(catalog.All(), catalog.AllChildren()...) {
		p := td.Policy
		if p == nil {
			continue
		}
		for code, suffix := range map[domain.FindingCode]string{p.Public: " policy open to anyone", p.Cross: " policy grants another account"} {
			if code == "" {
				continue
			}
			n++
			if got, want := catalog.Phrase(code), p.Noun+suffix; got != want {
				t.Errorf("%s phrase = %q, want %q", code, got, want)
			}
			if d := catalog.Detail(code); !strings.Contains(d, "depending on the actions allowed") || !strings.Contains(d, "this "+p.Subject) {
				t.Errorf("%s detail %q does not state the grant on this %s", code, d, p.Subject)
			}
		}
	}
	if n < 20 {
		t.Fatalf("%d declared policy findings, want at least 20", n)
	}
}

// A policy that does not parse is shown on its row: a Wave 2 row is marked
// not inspected, and a Wave 1 row, which has no such mark, carries the
// type's "<noun> policy not inspected" warning.
func TestT3BDev_UnreadablePolicyIsShownOnTheRow(t *testing.T) {
	for _, ty := range t3bTypes {
		t.Run(ty.short, func(t *testing.T) {
			decl := t3bDevDecl(t, ty.short)
			out := ty.run(t, t3bUnparseable, t3bOwn, t3bOwn)
			if !ty.wave1() {
				if decl.Unread != "" || out.mark != awsclient.CheckPolicyUnreadable {
					t.Errorf("Unread = %q, mark = %q; want no Unread code and mark %q", decl.Unread, out.mark, awsclient.CheckPolicyUnreadable)
				}
				return
			}
			f, ok := t3bFind(out.findings, decl.Unread)
			switch {
			case decl.Unread == "" || !ok:
				t.Fatalf("no not-inspected finding (declared %q); got %v", decl.Unread, t3bCodes(out.findings))
			case f.Severity != domain.SevWarn || catalog.Phrase(f.Code) != decl.Noun+" policy not inspected":
				t.Errorf("finding %s severity %v phrase %q", f.Code, f.Severity, catalog.Phrase(f.Code))
			}
		})
	}
}

// IAM returns a policy document percent-encoded (RFC 3986), other APIs as
// JSON; a literal "+" in a value is a "+" either way.
func TestT3BDev_EncodedAndRawDocumentsParseAlike(t *testing.T) {
	raw := `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":"s3:GetObject","Resource":"arn:aws:s3:::acme+logs/*"}]}`
	esc := strings.NewReplacer(`{`, "%7B", `}`, "%7D", `"`, "%22", `:`, "%3A", `,`, "%2C", `[`, "%5B", `]`, "%5D", `/`, "%2F", `*`, "%2A")
	for name, doc := range map[string]string{
		"raw":             raw,
		"encoded, + kept": esc.Replace(raw),
		"encoded, %2B":    strings.ReplaceAll(esc.Replace(raw), "+", "%2B"),
	} {
		d, err := iampolicy.Parse(doc)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if len(d.Statement) != 1 || !slices.Equal(d.Statement[0].Resource, []string{"arn:aws:s3:::acme+logs/*"}) {
			t.Errorf("%s: statements = %+v", name, d.Statement)
		}
	}
}

var t3bDevGuardedSelectors = []string{ //nolint:gochecknoglobals // test-only list
	"Statement", "Principal", "NotPrincipal", "PublicActions", "PublicConditions", "PublicDenyConditions",
	"CrossAccount", "Conditioned", "ex", "doc", "own", "present", "parsed", "decl",
}

var t3bDevGuardedCalls = []string{"Parse", "Evaluate", "EvaluateEndpoint", "EvaluateTrust", "AllowedPrincipals"} //nolint:gochecknoglobals // test-only list

var t3bDevGuardedLabels = []string{"Principal", "Actions", "Accounts"} //nolint:gochecknoglobals // test-only list

func t3bDevViolations(fset *token.FileSet, name string, f *ast.File) []string {
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
			x, isIdent := n.X.(*ast.Ident)
			switch {
			case isIdent && pkg != "" && x.Name == pkg && slices.Contains(t3bDevGuardedCalls, n.Sel.Name):
				out = append(out, fmt.Sprintf("%s: iampolicy.%s", fset.Position(n.Pos()), n.Sel.Name))
			case slices.Contains(t3bDevGuardedSelectors, n.Sel.Name) && !(isIdent && x.Name == pkg):
				out = append(out, fmt.Sprintf("%s: .%s", fset.Position(n.Pos()), n.Sel.Name))
			}
		case *ast.KeyValueExpr:
			if k, ok := n.Key.(*ast.Ident); ok && k.Name == "Label" {
				if v, ok := n.Value.(*ast.BasicLit); ok {
					if s, err := strconv.Unquote(v.Value); err == nil && slices.Contains(t3bDevGuardedLabels, s) {
						out = append(out, fmt.Sprintf("%s: row %q", fset.Position(n.Pos()), s))
					}
				}
			}
		}
		return true
	})
	return out
}

// Outside the owner no code in core/aws decides a policy verdict: it neither
// parses or evaluates a document attached to a resource, nor reads a parsed
// statement's principals, an Exposure or a reading's internals, nor builds the
// rows of a policy finding. The identity-policy analyses reach a parsed
// document only through named functions of the owner or of core/iampolicy.
func TestT3BDev_NoPolicyVerdictIsBuiltOutsideTheOwner(t *testing.T) {
	dir := filepath.Join("..", "..", "core", "aws")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
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
		violations = append(violations, t3bDevViolations(fset, name, f)...)
	}
	if len(violations) > 0 {
		t.Errorf("%d policy verdicts built outside %s:\n%s", len(violations), t3bOwnerFile, strings.Join(violations, "\n"))
	}
}

func TestT3BDev_VerdictScanCatchesAHandBuiltCheck(t *testing.T) {
	const enricher = `package aws
func enrichX(p, own string, result *IssueEnricherResult) {
	if d, err := parsePolicy(p); err == nil && d.Statement[0].Principal.Wildcard {
		setWave2Finding(result, "x", "x.public-policy", []domain.DetailRow{{Label: "Principal", Value: "*"}})
	}
	if rp := readPolicy("x", p, own, ""); len(rp.ex.CrossAccount) > 0 {
		setWave2Finding(result, "x", "x.cross-account-policy", nil)
	}
	_ = iampolicy.AllowedPrincipals
}`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x_issue_enrichment.go", strings.Replace(enricher, "package aws\n", "package aws\nimport \"github.com/k2m30/a9s/v3/core/iampolicy\"\n", 1), 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := t3bDevViolations(fset, "x_issue_enrichment.go", f); len(got) != 6 {
		t.Errorf("scan found %d sites, want 6 (.Statement, .Principal, row Principal, .ex, .CrossAccount, iampolicy.AllowedPrincipals): %v", len(got), got)
	}
	if got := t3bDevViolations(fset, t3bOwnerFile, f); len(got) != 0 {
		t.Errorf("scan reported the owner file itself: %v", got)
	}
}

type t3bDevDeletingLambda struct{ *pw1LambdaPostureFake }

func (f t3bDevDeletingLambda) GetFunction(_ context.Context, in *lambda.GetFunctionInput, _ ...func(*lambda.Options)) (*lambda.GetFunctionOutput, error) {
	return &lambda.GetFunctionOutput{Configuration: &lambdatypes.FunctionConfiguration{FunctionName: in.FunctionName, State: lambdatypes.StateDeleting}}, nil
}

// A function being deleted carries only its state: neither policy code
// applies while it tears down.
func TestT3BDev_DeletingFunctionRaisesNoPolicyFinding(t *testing.T) {
	var ty t3bType
	for _, c := range t3bTypes {
		if c.short == "lambda" {
			ty = c
		}
	}
	docs := map[domain.FindingCode]string{
		ty.public: t3bDoc(ty.stmt("Allow", "*", "Action", ty.actions, nil)),
		ty.cross:  t3bDoc(ty.stmt("Allow", map[string]any{"AWS": "arn:aws:iam::" + t3bForeign + ":root"}, "Action", ty.actions, nil)),
	}
	for code, doc := range docs {
		const name = "acme-checkout"
		r := resource.Resource{ID: name, Name: name, Type: "lambda", Fields: map[string]string{
			"function_name": name, "arn": t3bARN("lambda", "us-east-1", t3bOwn, "function:"+name),
		}}
		f := t3bDevDeletingLambda{&pw1LambdaPostureFake{policies: map[string]string{name: doc}}}
		out := t3bWave2(t, "lambda", &awsclient.ServiceClients{Lambda: f, Region: "us-east-1"}, t3bOwn, r, nil)
		if _, ok := t3bFind(out.findings, code); ok {
			t.Errorf("a deleting function raised %s; got %v", code, t3bCodes(out.findings))
		}
		if _, ok := t3bFind(out.findings, awsclient.CodeLambdaDeleting); !ok {
			t.Errorf("no deleting state for %s's policy; got %v", code, t3bCodes(out.findings))
		}
	}
}

type t3bDevDDBEmptyAnswer struct{ *t3bDDBFake }

func (t3bDevDDBEmptyAnswer) GetResourcePolicy(_ context.Context, _ *dynamodb.GetResourcePolicyInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetResourcePolicyOutput, error) {
	return &dynamodb.GetResourcePolicyOutput{}, nil
}

// DynamoDB answers a table with no resource policy with
// PolicyNotFoundException, so a successful answer without a document says
// nothing about who may call the table.
func TestT3BDev_TableAnsweredWithoutADocumentIsNotInspected(t *testing.T) {
	const name = "acme-orders"
	r := resource.Resource{ID: name, Name: name, Type: "ddb", Fields: map[string]string{
		"table_name": name, "arn": t3bARN("dynamodb", "eu-central-1", t3bOwn, "table/"+name),
	}}
	backupLoaded := resource.ResourceCache{"backup": {Resources: []resource.Resource{}}}
	f := t3bDevDDBEmptyAnswer{&t3bDDBFake{}}
	out := t3bWave2(t, "ddb", &awsclient.ServiceClients{DynamoDB: f, Region: "eu-central-1"}, t3bOwn, r, backupLoaded)
	if out.mark != awsclient.CheckPolicyMissing {
		t.Errorf("mark = %q (marked %v), want %q", out.mark, out.marked, awsclient.CheckPolicyMissing)
	}
	if out.err != nil {
		t.Errorf("an empty answer reported a failed call: %v", out.err)
	}
}
