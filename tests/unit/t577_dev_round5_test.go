package unit_test

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/opensearch"
	opensearchtypes "github.com/aws/aws-sdk-go-v2/service/opensearch/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
)

type t577OSList struct{ names []string }

func (l t577OSList) ListDomainNames(context.Context, *opensearch.ListDomainNamesInput, ...func(*opensearch.Options)) (*opensearch.ListDomainNamesOutput, error) {
	out := &opensearch.ListDomainNamesOutput{}
	for _, n := range l.names {
		out.DomainNames = append(out.DomainNames, opensearchtypes.DomainInfo{DomainName: aws.String(n)})
	}
	return out, nil
}

type t577OSDescribe struct {
	answer []string
	fail   map[string]error
}

func (d t577OSDescribe) DescribeDomains(_ context.Context, in *opensearch.DescribeDomainsInput, _ ...func(*opensearch.Options)) (*opensearch.DescribeDomainsOutput, error) {
	for _, n := range in.DomainNames {
		if err := d.fail[n]; err != nil {
			return nil, err
		}
	}
	out := &opensearch.DescribeDomainsOutput{}
	for _, n := range d.answer {
		if slices.Contains(in.DomainNames, n) {
			out.DomainStatusList = append(out.DomainStatusList, opensearchtypes.DomainStatus{DomainName: aws.String(n)})
		}
	}
	return out, nil
}

// A listed domain keeps its place in the list whether DescribeDomains answered
// it or not, and a domain the service reports gone was deleted after the list
// call: it is no row and no failure.
func TestT577Dev_OpenSearchKeepsListOrderAndDropsGone(t *testing.T) {
	list := t577OSList{names: []string{"a", "b", "c", "d", "e", "f"}}
	describe := t577OSDescribe{
		answer: []string{"e", "a", "c"},
		fail:   map[string]error{"f": t576APIError("OpenSearch", "DescribeDomains", "ResourceNotFoundException", "Domain not found: f")},
	}
	rows, err := awsclient.FetchOpenSearchDomainsAt(context.Background(), list, describe, time.Now())
	var ids []string
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	if want := []string{"a", "b", "c", "d", "e"}; !slices.Equal(ids, want) {
		t.Errorf("rows = %v, want %v", ids, want)
	}
	for _, r := range rows {
		degraded := r.Fields[awsclient.DegradedFindingField] != ""
		if degraded != (r.ID == "b" || r.ID == "d") {
			t.Errorf("%s: degraded = %v", r.ID, degraded)
		}
	}
	if err == nil || !strings.Contains(err.Error(), "failed for 2 of 6 IDs") {
		t.Errorf("error = %v, want the two unanswered domains of six", err)
	}
}

// t577InOwner reports whether the path to n hands a function literal, or a
// function by name, to keepListed or describeTally.one: the owner that
// decides which listed names keep a row.
func t577InOwner(n ast.Node, stack []ast.Node) bool {
	path := append(slices.Clone(stack), n)
	for i, node := range path[:len(path)-1] {
		call, ok := node.(*ast.CallExpr)
		if !ok || !slices.ContainsFunc(call.Args, func(a ast.Expr) bool { return a == path[i+1] }) {
			continue
		}
		switch path[i+1].(type) {
		case *ast.FuncLit, *ast.Ident:
		default:
			continue
		}
		switch f := call.Fun.(type) {
		case *ast.Ident:
			if f.Name == "keepListed" {
				return true
			}
		case *ast.SelectorExpr:
			if f.Sel.Name == "one" {
				return true
			}
		}
	}
	return false
}

func t577LocalFunc(n ast.Node) (string, bool) {
	as, ok := n.(*ast.AssignStmt)
	if !ok || as.Tok != token.DEFINE || len(as.Lhs) != 1 || len(as.Rhs) != 1 {
		return "", false
	}
	id, isID := as.Lhs[0].(*ast.Ident)
	_, isLit := as.Rhs[0].(*ast.FuncLit)
	if !isID || !isLit {
		return "", false
	}
	return id.Name, true
}

func t577EnclosingFunc(stack []ast.Node) string {
	for i := len(stack) - 1; i >= 0; i-- {
		if name, ok := t577LocalFunc(stack[i]); ok && i+1 < len(stack) && stack[i+1] == stack[i].(*ast.AssignStmt).Rhs[0] {
			return name
		}
	}
	return ""
}

type t577Ref struct {
	pos     token.Pos
	decl    string
	inOwner bool
}

func t577ReachedFromOwner(decl string, refs map[string][]t577Ref, visiting map[string]bool) bool {
	if len(refs[decl]) == 0 || visiting[decl] {
		return false
	}
	visiting[decl] = true
	defer delete(visiting, decl)
	for _, r := range refs[decl] {
		if !r.inOwner && (r.decl == "" || !t577ReachedFromOwner(r.decl, refs, visiting)) {
			return false
		}
	}
	return true
}

// A degraded row is built only where the shared owner decides which listed
// names keep a row: every DegradedDetails or degradedDetailsFinding call in
// core/aws sits in a literal handed to keepListed or describeTally.one, or in
// a function reached only from there.
func TestT577Dev_DegradedRowsAreBuiltOnlyThroughTheOwner(t *testing.T) {
	fset := token.NewFileSet()
	refs := map[string][]t577Ref{}
	err := filepath.WalkDir("../../core/aws", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || filepath.Base(path) == "degraded_resource.go" {
			return err
		}
		file, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return perr
		}
		for _, d := range file.Decls {
			decl := ""
			var skip *ast.Ident
			if fd, ok := d.(*ast.FuncDecl); ok {
				decl, skip = fd.Name.Name, fd.Name
			}
			locals := map[string]bool{}
			ast.Inspect(d, func(n ast.Node) bool {
				if name, ok := t577LocalFunc(n); ok {
					locals[name] = true
				}
				return true
			})
			key := func(name string) string {
				if locals[name] {
					return decl + "/" + name
				}
				return name
			}
			ast.PreorderStack(d, nil, func(n ast.Node, stack []ast.Node) bool {
				id, ok := n.(*ast.Ident)
				if !ok || id == skip {
					return true
				}
				if as, ok := stack[len(stack)-1].(*ast.AssignStmt); ok && as.Tok == token.DEFINE && slices.Contains(as.Lhs, ast.Expr(id)) {
					return true
				}
				encl := decl
				if name := t577EnclosingFunc(stack); name != "" {
					encl = decl + "/" + name
				}
				refs[key(id.Name)] = append(refs[key(id.Name)], t577Ref{id.Pos(), encl, t577InOwner(id, stack)})
				return true
			})
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, builder := range []string{"DegradedDetails", "degradedDetailsFinding"} {
		if len(refs[builder]) == 0 {
			t.Fatalf("no call of %s found; the guard reads nothing", builder)
		}
		for _, r := range refs[builder] {
			if !r.inOwner && (r.decl == "" || !t577ReachedFromOwner(r.decl, refs, map[string]bool{})) {
				t.Errorf("%s: %s builds a degraded row outside keepListed / describeTally.one", fset.Position(r.pos), r.decl)
			}
		}
	}
}

// A degraded row's finding already states that its describe call failed: no
// Wave-2 pass, run the way dispatch runs it, sends that call again for the
// row. Every list type, with each call its fetch makes refused in turn, is
// enriched over the rows that refusal left degraded.
func TestT577Dev_Wave2NeverRepeatsTheDescribeADegradedRowStates(t *testing.T) {
	_, cache := buildVisibilityTypeCache(t)
	for _, w := range awsclient.AllWave2() {
		fetch := resource.GetPaginatedFetcher(w.ShortName)
		if fetch == nil {
			continue
		}
		clean := &t576Injector{}
		clients := demo.NewServiceClients()
		t576WrapClients(clients, clean)
		_, _ = fetch(context.Background(), clients, "")
		ops := map[string]bool{}
		for _, c := range clean.calls {
			ops[c.Op] = true
		}
		for op := range ops {
			inj := &t576Injector{fail: t576DenyOp(op)}
			clients := demo.NewServiceClients()
			t576WrapClients(clients, inj)
			page, _ := fetch(context.Background(), clients, "")
			var degraded []resource.Resource
			for _, r := range page.Resources {
				if r.Fields[awsclient.DegradedFindingField] != "" {
					degraded = append(degraded, r)
				}
			}
			if len(degraded) == 0 {
				continue
			}
			run := t576Enrich(t, w.Enricher.Fn, page.Resources, t576ReadsOf(cache, w.Enricher.Reads), nil)
			for _, c := range run.calls {
				if c.Op != op {
					continue
				}
				for _, r := range degraded {
					if t576Names(t576Leaves(c.In), r) {
						t.Errorf("%s: wave 2 sends %s again for %s, whose degraded row states it failed", w.ShortName, op, r.ID)
					}
				}
			}
		}
	}
}

// Nobody could describe a degraded row, so no pivot on it answers a proven
// zero: every related pivot of every demo row whose describe failed answers
// unknown, a lower bound, or what it found.
func TestT577Dev_DegradedRowPivotsNeverAnswerAProvenZero(t *testing.T) {
	byType, cache := buildVisibilityTypeCache(t)
	keys := map[string]struct{}{}
	for k := range cache {
		keys[k] = struct{}{}
	}
	clients := demo.NewServiceClients()
	seen := 0
	for short, rows := range byType {
		for _, r := range rows {
			if r.Fields[awsclient.DegradedFindingField] == "" {
				continue
			}
			seen++
			for _, def := range resource.GetRelated(short) {
				op := runtime.DetailOperation{ID: 1, ResourceType: short, Resource: r, Clients: clients}
				got := runtime.RunRelatedDef(context.Background(), op, cache, keys, def).Result
				if got.State() == domain.RelatedResolved && got.Count() == 0 && !got.Truncated() {
					t.Errorf("%s %s: %s answers a proven 0", short, r.ID, def.DisplayName)
				}
			}
		}
	}
	if seen == 0 {
		t.Fatal("the demo bench holds no degraded row; the guard reads nothing")
	}
}

// Every reader of a degraded row keys on DegradedFindingField: every fetcher,
// top-level or child, with each call it makes refused in turn, returns no row
// whose details finding is missing from that field.
func TestT577Dev_EveryDegradedRowRecordsItsCode(t *testing.T) {
	for _, u := range t577FetchUnits(t) {
		clean := &t576Injector{}
		clients := demo.NewServiceClients()
		t576WrapClients(clients, clean)
		_, _ = u.fetch(clients)
		ops := map[string]bool{"": true}
		for _, c := range clean.calls {
			ops[c.Op] = true
		}
		for op := range ops {
			inj := &t576Injector{fail: t576DenyOp(op)}
			clients := demo.NewServiceClients()
			t576WrapClients(clients, inj)
			rows, _ := u.fetch(clients)
			for _, r := range rows {
				for _, f := range r.Findings {
					code := string(f.Code)
					if (strings.HasSuffix(code, ".warn.details_denied") || strings.HasSuffix(code, ".warn.details_unavailable")) && r.Fields[awsclient.DegradedFindingField] != code {
						t.Errorf("%s with %q refused: %s carries %s but records %q", u.name, op, r.ID, code, r.Fields[awsclient.DegradedFindingField])
					}
				}
			}
		}
	}
}

func TestT577Dev_DemoDeniedAgreementsAreListed(t *testing.T) {
	result, err := awsclient.FetchTransferAgreements(context.Background(), demo.NewServiceClients().Transfer, fixtures.WarnTransferDetailsDeniedID, "")
	if err == nil || !strings.Contains(err.Error(), "DescribeAgreement") {
		t.Errorf("error = %v, want the refused DescribeAgreement reported", err)
	}
	byID := map[string]resource.Resource{}
	for _, r := range result.Resources {
		byID[r.ID] = r
	}
	for id, status := range map[string]string{fixtures.AgreementDeniedActiveID: "ACTIVE", fixtures.AgreementDeniedInactiveID: "INACTIVE"} {
		r, ok := byID[id]
		if !ok {
			t.Errorf("agreement %s is not listed", id)
			continue
		}
		if got := r.Fields[awsclient.DegradedFindingField]; got != string(awsclient.DetailsDeniedCode("transfer_agreements")) {
			t.Errorf("%s: degraded finding = %q, want details denied", id, got)
		}
		if !strings.Contains(strings.ToUpper(fmt.Sprint(r.Fields)), status) {
			t.Errorf("%s: fields %v do not carry the listed status %s", id, r.Fields, status)
		}
	}
	inactive := byID[fixtures.AgreementDeniedInactiveID]
	if !slices.ContainsFunc(inactive.Findings, func(f domain.Finding) bool { return f.Phrase == "inactive: partner traffic rejected" }) {
		t.Errorf("the inactive denied agreement lost its warning: %+v", inactive.Findings)
	}
}
