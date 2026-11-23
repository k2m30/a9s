package unit_test

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
)

// t577NotFound answers every call of op that names exactly one of rows with
// the service saying the item is gone: what a per-item describe answers for a
// resource deleted after the list call. A call naming several rows is answered
// from the fakes; a batch call reports a deleted item by leaving it out, which
// the clean run already covers.
func t577NotFound(op string, rows []resource.Resource) func(service, o string, in any) error {
	return func(service, o string, in any) error {
		if o != op {
			return nil
		}
		named := 0
		for _, r := range rows {
			if t576Names(t576Leaves(in), r) {
				named++
			}
		}
		if named != 1 {
			return nil
		}
		return t576APIError(service, o, "ResourceNotFoundException", "Requested resource not found")
	}
}

// A resource the service reports gone was deleted after the list call: it is
// no row at all, never a degraded "details unavailable" row and never a
// failure the page reports. Every fetcher, top-level or child, with each
// per-item operation it calls answered not-found in turn, returns no degraded
// row the clean run did not already carry.
func TestT577Dev_GoneIsNeverADegradedRow(t *testing.T) {
	for _, u := range t577FetchUnits(t) {
		clean := &t576Injector{}
		clients := demo.NewServiceClients()
		t576WrapClients(clients, clean)
		rows, _ := u.fetch(clients)
		witness := map[string]bool{}
		for _, r := range rows {
			if r.Fields[awsclient.DegradedFindingField] != "" {
				witness[r.ID] = true
			}
		}
		ops := map[string]bool{}
		for _, c := range clean.calls {
			ops[c.Op] = true
		}
		for op := range ops {
			inj := &t576Injector{fail: t577NotFound(op, rows)}
			clients := demo.NewServiceClients()
			t576WrapClients(clients, inj)
			got, _ := u.fetch(clients)
			for _, r := range got {
				if code := r.Fields[awsclient.DegradedFindingField]; code != "" && !witness[r.ID] {
					t.Errorf("%s: %s answered not-found made %s a %s row", u.name, op, r.ID, code)
				}
			}
		}
	}
}

// t577RefuseHead keys the refusal on head's request, never on arrival: a
// fetcher that fans its calls out issues them in no fixed order.
func t577RefuseHead(op string, head t576Call) func(service, o string, in any) error {
	want := t576Leaves(head.In)
	done := false
	return func(service, o string, in any) error {
		if o != op || done || !slices.Equal(t576Leaves(in), want) {
			return nil
		}
		done = true
		return t576Denied(service, o)
	}
}

// t577ChainOf is the first call of op and every later call of op that
// continues it: a request naming nothing the chain did not already send or
// receive, which is how a page token travels from one answer to the next
// request.
func t577ChainOf(calls []t576Call, op string) []t576Call {
	var chain []t576Call
	known := map[string]bool{}
	for _, c := range calls {
		if c.Op != op {
			continue
		}
		in := t576Leaves(c.In)
		if len(chain) > 0 {
			continues := true
			for _, l := range in {
				if !known[l] {
					continues = false
					break
				}
			}
			if !continues {
				continue
			}
		}
		chain = append(chain, c)
		for _, l := range in {
			known[l] = true
		}
		for _, l := range t576Leaves(c.Out) {
			known[l] = true
		}
	}
	return chain
}

// One refused call costs the rows that call was for, never the page: every
// fetcher that calls an operation for more than one item still returns every
// row the first of those calls, and the pages that continue it, did not name
// when only that first call is refused.
func TestT577Dev_OneRefusedCallNeverEmptiesThePage(t *testing.T) {
	for _, u := range t577FetchUnits(t) {
		clean := &t576Injector{}
		clients := demo.NewServiceClients()
		t576WrapClients(clients, clean)
		rows, _ := u.fetch(clients)
		count := map[string]int{}
		for _, c := range clean.calls {
			count[c.Op]++
		}
		for op, n := range count {
			chain := t577ChainOf(clean.calls, op)
			if len(chain) == n {
				continue
			}
			var owned []string
			for _, c := range chain {
				owned = append(owned, t576Leaves(c.In)...)
				owned = append(owned, t576Leaves(c.Out)...)
			}
			inj := &t576Injector{fail: t577RefuseHead(op, chain[0])}
			clients := demo.NewServiceClients()
			t576WrapClients(clients, inj)
			got, _ := u.fetch(clients)
			kept := map[string]bool{}
			for _, r := range got {
				kept[r.ID] = true
			}
			for _, r := range rows {
				if !kept[r.ID] && !t576Names(owned, r) {
					t.Errorf("%s: refusing the first %s call lost %s, which that call did not name", u.name, op, r.ID)
				}
			}
		}
	}
}

var t577AnswerRootRe = regexp.MustCompile(`(?i)out(put)?$`)

// t577HollowAnswerDeciders are the calls and literals a branch uses to say
// what a missing field means for the row: a mark, a recorded short answer, or
// the finding the absence itself is.
var t577HollowAnswerDeciders = map[string]bool{
	"UnusableAnswerErr": true,
	"setWave2Finding":   true,
}

func t577SelectorRoot(e ast.Expr) (string, bool) {
	for {
		switch x := e.(type) {
		case *ast.SelectorExpr:
			e = x.X
		case *ast.Ident:
			return x.Name, true
		default:
			return "", false
		}
	}
}

// t577HollowAnswerBranch reports whether cond tests a field of an SDK answer
// for nil without an error term — the error-collapsed shape is the unrecorded
// answer gate's.
func t577HollowAnswerBranch(cond ast.Expr) bool {
	hollow := false
	errTerm := false
	ast.Inspect(cond, func(n ast.Node) bool {
		b, ok := n.(*ast.BinaryExpr)
		if !ok {
			return true
		}
		y, isID := b.Y.(*ast.Ident)
		if !isID || y.Name != "nil" {
			return true
		}
		if x, isX := b.X.(*ast.Ident); isX && b.Op == token.NEQ && strings.Contains(strings.ToLower(x.Name), "err") {
			errTerm = true
		}
		if sel, isSel := b.X.(*ast.SelectorExpr); isSel && b.Op == token.EQL {
			if root, ok := t577SelectorRoot(sel); ok && t577AnswerRootRe.MatchString(root) {
				hollow = true
			}
		}
		return true
	})
	return hollow && !errTerm
}

// t577BodyDecides reports whether body records the short answer or states
// the finding the absence is.
func t577BodyDecides(body *ast.BlockStmt) bool {
	if bodyRecords(body) {
		return true
	}
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		var name string
		switch x := n.(type) {
		case *ast.CompositeLit:
			if id, ok := x.Type.(*ast.Ident); ok {
				name = id.Name
			}
		case *ast.CallExpr:
			if id, ok := x.Fun.(*ast.Ident); ok {
				name = id.Name
			}
		}
		if t577HollowAnswerDeciders[name] {
			found = true
		}
		return true
	})
	return found
}

// t577BareExit reports whether body only leaves — a return of zero values, a
// continue or a break — so the branch states nothing about the row it read.
func t577BareExit(body *ast.BlockStmt) bool {
	if len(body.List) == 0 {
		return false
	}
	for _, s := range body.List {
		switch x := s.(type) {
		case *ast.BranchStmt:
		case *ast.ReturnStmt:
			for _, r := range x.Results {
				switch v := r.(type) {
				case *ast.Ident:
				case *ast.BasicLit:
				case *ast.CompositeLit:
					if len(v.Elts) > 0 {
						return false
					}
				default:
					return false
				}
			}
		default:
			return false
		}
	}
	return true
}

// An enricher that reads an answer and finds the field it needs missing has
// learned nothing about the row: it records the short answer, states the
// finding the absence is, or says on the line above why the row is owed
// nothing. A bare return renders the row as inspected and healthy.
func TestT577Dev_EveryHollowAnswerInAnEnricherIsDecided(t *testing.T) {
	awsDir := filepath.Join("..", "..", "core", "aws")
	entries, err := os.ReadDir(awsDir)
	if err != nil {
		t.Fatalf("read core/aws: %v", err)
	}
	fset := token.NewFileSet()
	var violations []string
	shapes := 0
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, "_issue_enrichment.go") && !strings.HasSuffix(name, "_detail_enrichment.go") {
			continue
		}
		path := filepath.Join(awsDir, name)
		src, rerr := os.ReadFile(path)
		if rerr != nil {
			t.Fatalf("read %s: %v", name, rerr)
		}
		file, perr := parser.ParseFile(fset, path, src, parser.ParseComments)
		if perr != nil {
			t.Fatalf("parse %s: %v", name, perr)
		}
		lines := strings.Split(string(src), "\n")
		ast.Inspect(file, func(n ast.Node) bool {
			ifStmt, ok := n.(*ast.IfStmt)
			if !ok || !t577HollowAnswerBranch(ifStmt.Cond) {
				return true
			}
			shapes++
			if !t577BareExit(ifStmt.Body) {
				return true
			}
			line := fset.Position(ifStmt.Pos()).Line
			if t577BodyDecides(ifStmt.Body) || (line >= 2 && strings.HasPrefix(strings.TrimSpace(lines[line-2]), unrecordedReasonMarker)) {
				return true
			}
			violations = append(violations, "core/aws/"+name+":"+strings.TrimSpace(lines[line-1]))
			return true
		})
	}
	if shapes < 5 {
		t.Fatalf("only %d hollow-answer branches found in the enrichers; the guard is not looking at the tree it thinks it is", shapes)
	}
	if len(violations) > 0 {
		sort.Strings(violations)
		t.Errorf("%d of %d hollow-answer branches say nothing about the row:\n  %s", len(violations), shapes, strings.Join(violations, "\n  "))
	}
}

func t577DemoPage(t *testing.T, shortName string, inj *t576Injector) ([]resource.Resource, error) {
	t.Helper()
	fetch := resource.GetPaginatedFetcher(shortName)
	if fetch == nil {
		t.Fatalf("no paginated fetcher for %s", shortName)
	}
	clients := demo.NewServiceClients()
	t576WrapClients(clients, inj)
	res, err := fetch(context.Background(), clients, "")
	return res.Resources, err
}

// A workgroup whose configuration read was refused shows it: the row carries
// the details-denied finding, its result location and cost cap are marked
// unread, and the page reports the refusals.
func TestT577Dev_RefusedWorkgroupReadIsReported(t *testing.T) {
	rows, err := t577DemoPage(t, "athena", &t576Injector{fail: t576DenyOp("GetWorkGroup")})
	if len(rows) == 0 {
		t.Fatal("no athena rows")
	}
	for _, r := range rows {
		if r.Fields[awsclient.DegradedFindingField] != string(awsclient.DetailsDeniedCode("athena")) || r.Fields["result_config_unread"] != "true" {
			t.Errorf("%s fields = %v, want the details-denied finding and the unread marker", r.ID, r.Fields)
		}
	}
	if err == nil || !strings.Contains(err.Error(), "athena: GetWorkGroup failed for") {
		t.Errorf("err = %v, want the GetWorkGroup refusals reported", err)
	}
}

// ClientException from DescribeTaskDefinition says the definition could not
// be read — not permitted, or deleted while a task still runs it — never that
// it has no volumes, roles or secrets: every task row whose definition it
// names is marked unjoined, and the page counts the failures against the
// tasks that asked.
func TestT577Dev_UnreadTaskDefinitionMarksTheTask(t *testing.T) {
	refuse := func(service, o string, _ any) error {
		if o == "DescribeTaskDefinition" {
			return t576APIError(service, o, "ClientException", "Unable to describe task definition.")
		}
		return nil
	}
	rows, err := t577DemoPage(t, "ecs-task", &t576Injector{fail: refuse})
	joined := 0
	for _, r := range rows {
		if r.Fields["task_definition"] == "" {
			continue
		}
		joined++
		if r.Fields["task_def_join_error"] != "true" {
			t.Errorf("%s: definition %s unread, yet task_def_join_error is unset", r.ID, r.Fields["task_definition"])
		}
	}
	if joined == 0 {
		t.Fatal("no ecs-task row names a task definition")
	}
	want := "DescribeTaskDefinition failed for " + strconv.Itoa(joined) + " of " + strconv.Itoa(joined) + " "
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("err = %v, want %q", err, want)
	}
}
