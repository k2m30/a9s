package main

import (
	"go/ast"
	"go/types"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

// boundedReads are the pageAll calls that stop before the last page, keyed
// "<enclosing function> <sdk package>.<Operation>". A read may stop early only
// when what the snapshot records from it cannot change with more pages: the
// service answers newest first and the app reads the same first page.
var boundedReads = map[string]string{
	"captureLatestScalingActivity autoscaling.DescribeScalingActivities": "activities are newest first; the app reads MaxRecords 1 (core/aws/asg_issue_enrichment.go)",
	"captureLTDefaultVersion ec2.DescribeLaunchTemplateVersions":         "the request names one version ($Default), so the answer holds at most one",
	"captureSFNLastExecution sfn.ListExecutions":                         "executions are newest first; the app reads MaxResults 1 (core/aws/sfn_issue_enrichment.go)",
	"captureLatestBuild codebuild.ListBuildsForProject":                  "build ids are newest first; the app reads the first page and its first id (core/aws/cb_issue_enrichment.go)",
	"captureCFN cloudformation.DescribeStackEvents":                      "events are newest first; the app scans the first page (core/aws/cfn_issue_enrichment.go)",
	"captureLogs cloudwatchlogs.DescribeLogStreams":                      "ordered by LastEventTime descending; the app reads Limit 1 (core/aws/logs_issue_enrichment.go)",
	"captureGlue glue.GetJobRuns":                                        "runs are newest first; the app reads MaxResults 1 (core/aws/glue_issue_enrichment.go)",
}

func loadCollector(t *testing.T) *packages.Package {
	t.Helper()
	cfg := &packages.Config{Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo}
	pkgs, err := packages.Load(cfg, "github.com/k2m30/a9s/v3/cmd/snapshot")
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) != 1 || len(pkgs[0].Errors) > 0 {
		t.Fatalf("load collector: %d packages, errors %v", len(pkgs), pkgs[0].Errors)
	}
	return pkgs[0]
}

// sdkClientMethod names an SDK client method a selector denotes, as
// "<sdk package>.<Method>", or "" when it denotes something else.
func sdkClientMethod(info *types.Info, sel *ast.SelectorExpr) string {
	s, ok := info.Selections[sel]
	if !ok || (s.Kind() != types.MethodVal && s.Kind() != types.MethodExpr) {
		return ""
	}
	recv := s.Recv()
	if p, isPtr := recv.(*types.Pointer); isPtr {
		recv = p.Elem()
	}
	n, ok := recv.(*types.Named)
	if !ok || n.Obj().Name() != "Client" || n.Obj().Pkg() == nil || !strings.HasPrefix(n.Obj().Pkg().Path(), "github.com/aws/aws-sdk-go-v2/service/") {
		return ""
	}
	return n.Obj().Pkg().Name() + "." + sel.Sel.Name
}

func isCollectorPageAll(info *types.Info, call *ast.CallExpr) bool {
	fun := call.Fun
	switch f := fun.(type) {
	case *ast.IndexExpr:
		fun = f.X
	case *ast.IndexListExpr:
		fun = f.X
	}
	id, ok := fun.(*ast.Ident)
	if !ok {
		return false
	}
	obj, ok := info.Uses[id].(*types.Func)
	return ok && obj.Name() == "pageAll" && obj.Pkg() != nil && obj.Pkg().Path() == "github.com/k2m30/a9s/v3/cmd/snapshot"
}

// A pageAll call reads every page unless its bound says otherwise; each
// bounded call is a declared choice with the reason more pages cannot change
// what the snapshot records, and every declared choice is a call that exists.
func TestT11a_EveryBoundedReadIsDeclared(t *testing.T) {
	pkg := loadCollector(t)
	allPagesObj := pkg.Types.Scope().Lookup("allPages")
	onePageObj := pkg.Types.Scope().Lookup("onePage")
	if allPagesObj == nil || onePageObj == nil {
		t.Fatal("pagination.go declares no allPages or onePage bound")
	}
	found := map[string]int{}
	for _, file := range pkg.Syntax {
		if strings.HasSuffix(pkg.Fset.Position(file.Pos()).Filename, "_test.go") {
			continue
		}
		for _, decl := range file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok || !isCollectorPageAll(pkg.TypesInfo, call) || len(call.Args) == 0 {
					return true
				}
				bound, _ := call.Args[len(call.Args)-1].(*ast.Ident)
				if bound != nil && pkg.TypesInfo.Uses[bound] == allPagesObj {
					return true
				}
				var ops []string
				for _, a := range call.Args {
					ast.Inspect(a, func(m ast.Node) bool {
						if sel, ok := m.(*ast.SelectorExpr); ok {
							if op := sdkClientMethod(pkg.TypesInfo, sel); op != "" {
								ops = append(ops, op)
							}
						}
						return true
					})
				}
				pos := pkg.Fset.Position(call.Pos())
				for _, op := range ops {
					key := fd.Name.Name + " " + op
					found[key]++
					if boundedReads[key] == "" {
						t.Errorf("%s:%d %s reads a bounded number of pages with no declared reason", filepath.Base(pos.Filename), pos.Line, key)
					}
					if found[key] > 1 {
						t.Errorf("%s:%d %s is a second bounded call under one declaration", filepath.Base(pos.Filename), pos.Line, key)
					}
				}
				if bound == nil || pkg.TypesInfo.Uses[bound] != onePageObj {
					t.Errorf("%s:%d a bounded read reads the first page only (onePage), the page every declared reason covers", filepath.Base(pos.Filename), pos.Line)
				}
				if len(ops) == 0 {
					t.Errorf("%s:%d bounded pageAll call names no SDK operation", filepath.Base(pos.Filename), pos.Line)
				}
				return true
			})
		}
	}
	for key := range boundedReads {
		if found[key] == 0 {
			t.Errorf("declared bounded read %q matches no bounded pageAll call", key)
		}
	}
}

// pageMember reports whether an output member resumes a list: a *string
// whose name is a page token or starts with Next or LastEvaluated (CloudFront
// NextMarker, Route 53 NextRecordName, DynamoDB LastEvaluatedTableName, Logs
// NextForwardToken).
func pageMember(f *types.Var) bool {
	p, ok := f.Type().(*types.Pointer)
	if !ok {
		return false
	}
	if b, ok := p.Elem().(*types.Basic); !ok || b.Kind() != types.String {
		return false
	}
	return isPageMember(f.Name())
}

// resumesAList reports whether an SDK output type carries a page member, at
// the top level or in a struct it holds (CloudFront's DistributionList).
func resumesAList(t types.Type, depth int) bool {
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	n, ok := t.(*types.Named)
	if !ok || n.Obj().Pkg() == nil || !strings.HasPrefix(n.Obj().Pkg().Path(), "github.com/aws/aws-sdk-go-v2/service/") {
		return false
	}
	st, ok := n.Underlying().(*types.Struct)
	if !ok {
		return false
	}
	for i := range st.NumFields() {
		f := st.Field(i)
		if pageMember(f) || (depth > 0 && resumesAList(f.Type(), depth-1)) {
			return true
		}
	}
	return false
}

// Every SDK client method whose answer can name a further page is called by
// a pageAll closure that passes the closure's own input to it, so the token
// pageAll sets is the one sent; a call anywhere else, a method value or a
// method expression, or a closure that builds its own input reads one page.
func TestT11a_EveryTokenBearingCallIsMadeByPageAll(t *testing.T) {
	pkg := loadCollector(t)
	walked := map[*ast.SelectorExpr]bool{}
	for _, file := range pkg.Syntax {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || !isCollectorPageAll(pkg.TypesInfo, call) {
				return true
			}
			for _, a := range call.Args {
				lit, ok := a.(*ast.FuncLit)
				if !ok || lit.Type.Params == nil || len(lit.Type.Params.List) < 2 || len(lit.Type.Params.List[1].Names) == 0 {
					continue
				}
				in := pkg.TypesInfo.Defs[lit.Type.Params.List[1].Names[0]]
				ast.Inspect(lit.Body, func(m ast.Node) bool {
					inner, ok := m.(*ast.CallExpr)
					if !ok {
						return true
					}
					sel, ok := inner.Fun.(*ast.SelectorExpr)
					if !ok {
						return true
					}
					if slices.ContainsFunc(inner.Args, func(e ast.Expr) bool {
						id, ok := e.(*ast.Ident)
						return ok && pkg.TypesInfo.Uses[id] == in
					}) {
						walked[sel] = true
					}
					return true
				})
			}
			return true
		})
	}

	for _, file := range pkg.Syntax {
		if strings.HasSuffix(pkg.Fset.Position(file.Pos()).Filename, "_test.go") {
			continue
		}
		ast.Inspect(file, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			op := sdkClientMethod(pkg.TypesInfo, sel)
			if op == "" || walked[sel] {
				return true
			}
			sig, ok := pkg.TypesInfo.Selections[sel].Type().(*types.Signature)
			if !ok || sig.Results().Len() == 0 || !resumesAList(sig.Results().At(0).Type(), 1) {
				return true
			}
			pos := pkg.Fset.Position(sel.Pos())
			t.Errorf("%s:%s %s can answer with a further page and is not called by a pageAll closure with its input", filepath.Base(pos.Filename), strconv.Itoa(pos.Line), op)
			return true
		})
	}
}
