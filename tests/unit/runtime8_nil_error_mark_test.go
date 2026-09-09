// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// runtime8_nil_error_mark_test.go — an answer with no error is not a failure.
//
// MarkSkipped reads the refused call off the error it is handed. Handed a nil
// error it has nothing to read, so the row records an unnamed check and the
// aggregate says "no reason given" — a9s telling the operator it does not know
// why, when it does: the service answered and left out the field. That is
// MarkUnusable's case, and it has a cause to state.
//
// The gate below finds the class rather than the one site: a MarkSkipped whose
// error argument is reachable nil, which in this tree is the `if err != nil ||
// <something else>` shape — the branch is entered for the something-else with
// err still nil.
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
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// nilErrorMarkViolations reports every MarkSkipped call in file whose error
// argument can be nil where it is called: the literal nil, or an identifier
// the enclosing branch condition admits as nil.
func nilErrorMarkViolations(fset *token.FileSet, fileName string, file *ast.File) (violations []string, calls int) {
	// guards maps a node to the conditions in force inside it, collected on
	// the way down so a call knows what its enclosing ifs established.
	var stack []ast.Expr
	var walk func(n ast.Node)
	walk = func(n ast.Node) {
		if n == nil {
			return
		}
		switch v := n.(type) {
		case *ast.IfStmt:
			stack = append(stack, v.Cond)
			walk(v.Body)
			stack = stack[:len(stack)-1]
			walk(v.Else)
			return
		case *ast.CallExpr:
			if name, ok := v.Fun.(*ast.Ident); ok && name.Name == "MarkSkipped" && len(v.Args) == 4 {
				calls++
				if why := nilErrorReason(v.Args[3], stack); why != "" {
					violations = append(violations, fmt.Sprintf("%s:%d MarkSkipped %s",
						fileName, fset.Position(v.Pos()).Line, why))
				}
			}
		}
		ast.Inspect(n, func(c ast.Node) bool {
			if c == n {
				return true
			}
			switch c.(type) {
			case *ast.IfStmt, *ast.CallExpr:
				walk(c)
				return false
			}
			return true
		})
	}
	for _, decl := range file.Decls {
		walk(decl)
	}
	return violations, calls
}

// nilErrorReason returns why arg can be nil under the given enclosing
// conditions, or "" when it cannot.
func nilErrorReason(arg ast.Expr, conds []ast.Expr) string {
	if id, ok := arg.(*ast.Ident); ok && id.Name == "nil" {
		return "is handed a literal nil error, so the row records no check and the failure reads " +
			"\"no reason given\" — an answer with no error is MarkUnusable's"
	}
	name, ok := arg.(*ast.Ident)
	if !ok {
		return ""
	}
	for _, cond := range conds {
		if other, admits := orBranchAdmitsNil(cond, name.Name); admits {
			return fmt.Sprintf("is reached from `%s != nil || %s`, so it runs with %s still nil whenever "+
				"the other arm is what was true — that arm is an answer without a field, which is "+
				"MarkUnusable's", name.Name, other, name.Name)
		}
	}
	return ""
}

// orBranchAdmitsNil reports whether cond is an || chain containing
// `<name> != nil` beside at least one other term, and names one other term.
func orBranchAdmitsNil(cond ast.Expr, name string) (other string, admits bool) {
	var terms []ast.Expr
	var flatten func(e ast.Expr)
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
		return "", false
	}
	found := false
	for _, term := range terms {
		b, ok := term.(*ast.BinaryExpr)
		if !ok || b.Op != token.NEQ {
			continue
		}
		x, xok := b.X.(*ast.Ident)
		y, yok := b.Y.(*ast.Ident)
		if xok && yok && x.Name == name && y.Name == "nil" {
			found = true
		}
	}
	if !found {
		return "", false
	}
	for _, term := range terms {
		b, ok := term.(*ast.BinaryExpr)
		if ok && b.Op == token.NEQ {
			if x, xok := b.X.(*ast.Ident); xok && x.Name == name {
				continue
			}
		}
		return exprText(term), true
	}
	return "", false
}

// exprText renders an expression back to source for the failure message.
func exprText(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.BinaryExpr:
		return exprText(v.X) + " " + v.Op.String() + " " + exprText(v.Y)
	case *ast.SelectorExpr:
		return exprText(v.X) + "." + v.Sel.Name
	case *ast.Ident:
		return v.Name
	case *ast.CallExpr:
		return exprText(v.Fun) + "(…)"
	}
	return "…"
}

// TestNilErrorMarkGate_NoRecordedRowIsMarkedWithoutAReason walks core/aws.
func TestNilErrorMarkGate_NoRecordedRowIsMarkedWithoutAReason(t *testing.T) {
	awsDir := filepath.Join("..", "..", "core", "aws")
	entries, err := os.ReadDir(awsDir)
	if err != nil {
		t.Fatalf("read core/aws: %v", err)
	}
	fset := token.NewFileSet()
	var violations []string
	total := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, perr := parser.ParseFile(fset, filepath.Join(awsDir, name), nil, 0)
		if perr != nil {
			t.Fatalf("parse %s: %v", name, perr)
		}
		found, n := nilErrorMarkViolations(fset, "core/aws/"+name, file)
		total += n
		violations = append(violations, found...)
	}
	if total < 50 {
		t.Fatalf("only %d MarkSkipped calls found under core/aws; the gate is not looking at the tree it thinks it is", total)
	}
	if len(violations) > 0 {
		sort.Strings(violations)
		t.Errorf("%d mark site(s) can record a row with no reason:\n  %s", len(violations), strings.Join(violations, "\n  "))
	}
}

// nilErrorECS answers DescribeTasks with one running task and then answers
// DescribeTaskDefinition with an output that carries no TaskDefinition and no
// error — the shape the row is about: the service replied, and the reply is
// missing the field the enricher needs.
type nilErrorECS struct {
	awsclient.ECSAPI
	taskID string
}

func (f *nilErrorECS) DescribeTasks(_ context.Context, _ *ecs.DescribeTasksInput, _ ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error) {
	return &ecs.DescribeTasksOutput{Tasks: []ecstypes.Task{{
		TaskArn:           aws.String("arn:aws:ecs:us-east-1:123456789012:task/example-cluster/" + f.taskID),
		TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/example:7"),
		LastStatus:        aws.String("RUNNING"),
	}}}, nil
}

func (f *nilErrorECS) DescribeTaskDefinition(_ context.Context, _ *ecs.DescribeTaskDefinitionInput, _ ...func(*ecs.Options)) (*ecs.DescribeTaskDefinitionOutput, error) {
	return &ecs.DescribeTaskDefinitionOutput{}, nil
}

// TestECSTaskRow_MarkedWithoutAnErrorStillNamesItsReason is the behavioural
// half. The row is uninspected either way; what it must not do is say it does
// not know why.
func TestECSTaskRow_MarkedWithoutAnErrorStillNamesItsReason(t *testing.T) {
	const taskID = "abc12345678901234567890123456789012"
	clients := &awsclient.ServiceClients{ECS: &nilErrorECS{taskID: taskID}}
	resources := []resource.Resource{{
		ID:   taskID,
		Name: taskID,
		Fields: map[string]string{
			"cluster": "arn:aws:ecs:us-east-1:123456789012:cluster/example-cluster",
			"task_id": taskID,
		},
	}}

	result, err := awsclient.EnrichECSTasks(context.Background(), clients, resources, nil)

	check, marked := result.TruncatedIDs[taskID]
	if !marked {
		t.Fatalf("the task whose definition came back without its TaskDefinition is not marked uninspected: %v", result.TruncatedIDs)
	}
	if !strings.Contains(check, "DescribeTaskDefinition") {
		t.Errorf("the row's check is %q — it names nothing, because the reason was handed to the recorder "+
			"as a nil error; the call that answered without the field is DescribeTaskDefinition", check)
	}
	if err != nil && strings.Contains(err.Error(), "no reason given") {
		t.Errorf("the aggregate reads %q — a9s knows why this row was skipped and says it does not", err)
	}
}
