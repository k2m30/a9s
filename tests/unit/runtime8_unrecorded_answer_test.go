// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// runtime8_unrecorded_answer_test.go — a failed call and an answer without the
// field are two facts, and `if err != nil || out == nil` is one branch.
//
// A call that refused and a call that answered without the field the caller
// needs both leave a9s knowing nothing about the row. Collapsed into one
// condition whose body returns, breaks or continues, neither is recorded: the
// row goes on to be rendered as if it had been looked at.
//
// The gate below finds the shape. It does not decide any site — some of those
// arms legitimately owe the row nothing, and which is which is a per-site
// judgement. What it refuses is leaving the judgement unwritten: a site either
// records through the recorder, or says on the line above why it does not.
package unit_test

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// unrecordedReasonMarker is the form this file pins for "decided, and the
// answer is that this arm owes the row nothing". A comment, because that is
// what the arm has to carry — a sentence saying why silence is right here —
// and because the repo already states cap and race reasons this way.
const unrecordedReasonMarker = "// no finding:"

// unrecordedRecorders are the calls that record something about the row: a
// mark, a coverage flag, or a failure the aggregate will carry.
var unrecordedRecorders = map[string]bool{
	"MarkSkipped":           true,
	"MarkUnusable":          true,
	"markUninspected":       true,
	"SetTruncated":          true,
	"FailedCall":            true,
	"FailedCallInRegion":    true,
	"FailedOnPage":          true,
	"UnusableAnswer":        true,
	"MarkInformationalOnly": true,
}

// unrecordedAnswerViolations reports every `if err != nil || <the answer is
// missing something>` in file whose body records nothing and whose preceding
// line does not say why it need not.
func unrecordedAnswerViolations(fset *token.FileSet, fileName string, src []byte, file *ast.File) (violations []string, shapes int) {
	lines := strings.Split(string(src), "\n")
	ast.Inspect(file, func(n ast.Node) bool {
		ifStmt, ok := n.(*ast.IfStmt)
		if !ok {
			return true
		}
		errName, missing, is := errOrMissingAnswer(ifStmt.Cond)
		if !is {
			return true
		}
		shapes++
		if bodyRecords(ifStmt.Body) {
			return true
		}
		line := fset.Position(ifStmt.Pos()).Line
		if line >= 2 && strings.HasPrefix(strings.TrimSpace(lines[line-2]), unrecordedReasonMarker) {
			return true
		}
		violations = append(violations, fmt.Sprintf(
			"%s:%d `%s != nil || %s` records neither the refusal nor the missing field, and says nothing about why the row is owed neither",
			fileName, line, errName, missing))
		return true
	})
	return violations, shapes
}

// errOrMissingAnswer reports whether cond is `<err> != nil || …` with at least
// one other term that is a nil or emptiness test on the answer, and names both
// halves for the message.
func errOrMissingAnswer(cond ast.Expr) (errName, missing string, is bool) {
	var terms []ast.Expr
	var flatten func(ast.Expr)
	flatten = func(e ast.Expr) {
		if b, ok := e.(*ast.BinaryExpr); ok && b.Op == token.LOR {
			flatten(b.X)
			flatten(b.Y)
			return
		}
		terms = append(terms, e)
	}
	flatten(cond)
	if len(terms) < 2 {
		return "", "", false
	}
	for _, term := range terms {
		b, ok := term.(*ast.BinaryExpr)
		if !ok || b.Op != token.NEQ {
			continue
		}
		x, xok := b.X.(*ast.Ident)
		y, yok := b.Y.(*ast.Ident)
		if xok && yok && y.Name == "nil" && strings.Contains(strings.ToLower(x.Name), "err") {
			errName = x.Name
		}
	}
	if errName == "" {
		return "", "", false
	}
	for _, term := range terms {
		b, ok := term.(*ast.BinaryExpr)
		if !ok || b.Op != token.EQL {
			continue
		}
		if id, isID := b.Y.(*ast.Ident); isID && id.Name == "nil" {
			return errName, exprText(b.X) + " == nil", true
		}
		if lit, isLit := b.Y.(*ast.BasicLit); isLit && lit.Value == "0" {
			return errName, exprText(b.X) + " == 0", true
		}
	}
	return "", "", false
}

// bodyRecords reports whether body calls one of the recorders.
func bodyRecords(body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fn := call.Fun.(type) {
		case *ast.Ident:
			if unrecordedRecorders[fn.Name] {
				found = true
			}
		case *ast.SelectorExpr:
			if unrecordedRecorders[fn.Sel.Name] {
				found = true
			}
		}
		return true
	})
	return found
}

// TestUnrecordedAnswer_EveryCollapsedFailureIsDecided walks core/aws.
func TestUnrecordedAnswer_EveryCollapsedFailureIsDecided(t *testing.T) {
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
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
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
		found, n := unrecordedAnswerViolations(fset, "core/aws/"+name, src, file)
		shapes += n
		violations = append(violations, found...)
	}
	if shapes < 10 {
		t.Fatalf("only %d collapsed-failure branches found under core/aws; the gate is not looking at the tree it thinks it is", shapes)
	}
	if len(violations) > 0 {
		sort.Strings(violations)
		t.Errorf("%d of %d collapsed failure branches are undecided — each either records through the recorder "+
			"or carries a %q line saying why the row is owed nothing:\n  %s",
			len(violations), shapes, unrecordedReasonMarker, strings.Join(violations, "\n  "))
	}
}

// unrecordedRDS answers DescribeDBClusterSnapshotAttributes with an output
// that carries no attributes result and no error — the service replied, and
// the reply is missing the field the enricher reads.
type unrecordedRDS struct {
	awsclient.RDSAPI
}

func (unrecordedRDS) DescribeDBClusterSnapshotAttributes(_ context.Context, _ *rds.DescribeDBClusterSnapshotAttributesInput, _ ...func(*rds.Options)) (*rds.DescribeDBClusterSnapshotAttributesOutput, error) {
	return &rds.DescribeDBClusterSnapshotAttributesOutput{}, nil
}

// TestUnrecordedAnswer_DBCSnapAttributesWithoutTheField is the first sample
// site. A cluster snapshot whose share attributes could not be read is not a
// snapshot that is known to be private — but the arm returns an empty list
// with no error, which is exactly what "read them, none of them shares with
// all" looks like.
func TestUnrecordedAnswer_DBCSnapAttributesWithoutTheField(t *testing.T) {
	entry, ok := awsclient.Wave2EnricherFor("dbc-snap")
	if !ok {
		t.Fatal("dbc-snap has no registered Wave 2 enricher, so this pin has nothing to drive")
	}
	const snapID = "example-cluster-snapshot-1"
	snap := resource.Resource{
		ID:   snapID,
		Name: snapID,
		Type: "dbc-snap",
		Fields: map[string]string{
			"snapshot_id": snapID,
			"cluster_id":  "example-cluster",
			"type":        "manual",
		},
		RawStruct: rdsClusterSnapshotFixture(snapID),
	}

	clients := &awsclient.ServiceClients{RDS: unrecordedRDS{}}
	result, _ := entry.Fn(context.Background(), clients, []resource.Resource{snap}, nil)

	check, marked := result.TruncatedIDs[snapID]
	if !marked {
		t.Fatalf("the snapshot whose share attributes came back without the attributes result is not marked "+
			"uninspected, so it renders as read-and-not-shared: %v", result.TruncatedIDs)
	}
	if !strings.Contains(check, "DescribeDBClusterSnapshotAttributes") {
		t.Errorf("the row's check is %q — it names nothing, because the arm that swallowed the answer "+
			"returned a nil error; the call that answered without the field is "+
			"DescribeDBClusterSnapshotAttributes", check)
	}
}

// rdsClusterSnapshotFixture is the raw struct a dbc-snap row carries for an
// Aurora snapshot, which is what routes the enricher to the RDS branch.
func rdsClusterSnapshotFixture(id string) any {
	return rdstypes.DBClusterSnapshot{
		DBClusterSnapshotIdentifier: aws.String(id),
		DBClusterIdentifier:         aws.String("example-cluster"),
		SnapshotType:                aws.String("manual"),
	}
}
