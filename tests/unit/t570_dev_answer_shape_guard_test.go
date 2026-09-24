package unit_test

import (
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

// t570IdentityFields are the Fields keys a row carries its own identity
// under, or the keys its describe and list calls need beside it (an ECS
// service's cluster, a node group's cluster, a rule's bus); every other key a
// checker reads is a relation the fetcher recorded.
var t570IdentityFields = map[string]bool{
	"arn": true, "name": true, "id": true, "stream_arn": true, "topic_arn": true,
	"load_balancer_arn": true, "target_group_arn": true, "certificate_arn": true,
	"role_name": true, "nodegroup_name": true, "uri": true, "cluster": true,
	"cluster_name": true, "service_name": true, "dns_name": true, "domain_name": true,
	"event_bus": true,
}

// t570ResourceParams are the parameters of fn typed as the source row.
func t570ResourceParams(pkg *packages.Package, fn *ast.FuncDecl) map[types.Object]bool {
	out := map[types.Object]bool{}
	for _, field := range fn.Type.Params.List {
		for _, name := range field.Names {
			obj := pkg.TypesInfo.Defs[name]
			if obj == nil {
				continue
			}
			if named, ok := types.Unalias(obj.Type()).(*types.Named); ok && named.Obj().Name() == "Resource" &&
				named.Obj().Pkg() != nil && strings.HasSuffix(named.Obj().Pkg().Path(), "/core/domain") {
				out[obj] = true
			}
		}
	}
	return out
}

// t570ReadsIdentity reports whether e reads the source row's identity: its
// ID, Name or an identity Fields key, directly or through a local tainted
// by one.
func t570ReadsIdentity(pkg *packages.Package, e ast.Node, params, tainted map[types.Object]bool) bool {
	found := false
	ast.Inspect(e, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.SelectorExpr:
			if id, ok := x.X.(*ast.Ident); ok && params[pkg.TypesInfo.Uses[id]] && (x.Sel.Name == "ID" || x.Sel.Name == "Name") {
				found = true
			}
		case *ast.IndexExpr:
			sel, ok := x.X.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Fields" {
				return true
			}
			id, ok := sel.X.(*ast.Ident)
			if !ok || !params[pkg.TypesInfo.Uses[id]] {
				return true
			}
			if lit, ok := x.Index.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				if key, err := strconv.Unquote(lit.Value); err == nil && t570IdentityFields[key] {
					found = true
				}
			}
		case *ast.Ident:
			if tainted[pkg.TypesInfo.Uses[x]] {
				found = true
			}
		}
		return !found
	})
	return found
}

// t570ParsesRelation reports whether e derives a relation from the row's
// identity rather than restating it: a call into core/aws itself
// (logGroupOwner, a read the identity keys), or a parse at a marker the call
// names. What it yields is a relation, read empty when the row records none.
func t570ParsesRelation(pkg *packages.Package, e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	var callee *ast.Ident
	switch f := call.Fun.(type) {
	case *ast.Ident:
		callee = f
	case *ast.IndexExpr:
		callee, _ = f.X.(*ast.Ident)
	}
	if callee != nil {
		if obj := pkg.TypesInfo.Uses[callee]; obj != nil && obj.Pkg() == pkg.Types {
			return true
		}
	}
	for _, arg := range call.Args {
		if lit, ok := arg.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			return true
		}
	}
	return false
}

// t570TestsEmpty reports whether cond asks if an identity value is empty:
// `x == ""` on a value read from the row's identity.
func t570TestsEmpty(pkg *packages.Package, cond ast.Expr, params, tainted map[types.Object]bool) bool {
	found := false
	ast.Inspect(cond, func(n ast.Node) bool {
		be, ok := n.(*ast.BinaryExpr)
		if !ok || be.Op != token.EQL {
			return true
		}
		for _, pair := range [][2]ast.Expr{{be.X, be.Y}, {be.Y, be.X}} {
			if lit, ok := pair[1].(*ast.BasicLit); ok && lit.Value == `""` && t570ReadsIdentity(pkg, pair[0], params, tainted) {
				found = true
			}
		}
		return !found
	})
	return found
}

// t570Returns reports whether block returns a call to one of names as a
// statement of its own.
func t570Returns(block *ast.BlockStmt, names ...string) bool {
	for _, st := range block.List {
		ret, ok := st.(*ast.ReturnStmt)
		if !ok || len(ret.Results) != 1 {
			continue
		}
		call, ok := ret.Results[0].(*ast.CallExpr)
		if !ok {
			continue
		}
		if id, ok := call.Fun.(*ast.Ident); ok {
			for _, n := range names {
				if id.Name == n {
					return true
				}
			}
		}
	}
	return false
}

func t570EachFunc(t *testing.T, visit func(pkg *packages.Package, file string, fn *ast.FuncDecl)) {
	pkg := t570LoadAWS(t)
	for _, f := range pkg.Syntax {
		path := pkg.Fset.Position(f.Pos()).Filename
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		for _, decl := range f.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Body != nil {
				visit(pkg, path, fn)
			}
		}
	}
}

// A checker that has no id, ARN or name for its own row cannot ask AWS
// anything about it, so it has read nothing: its answer is unknown, never a
// proven 0. foundNone is for a relation field read empty on a readable row.
func TestT570_MissingRowIdentityIsNotAProvenZero(t *testing.T) {
	var found []string
	t570EachFunc(t, func(pkg *packages.Package, path string, fn *ast.FuncDecl) {
		params := t570ResourceParams(pkg, fn)
		if len(params) == 0 {
			return
		}
		tainted := map[types.Object]bool{}
		for changed := true; changed; {
			changed = false
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				as, ok := n.(*ast.AssignStmt)
				if !ok || len(as.Lhs) != len(as.Rhs) {
					return true
				}
				for i, lhs := range as.Lhs {
					id, ok := lhs.(*ast.Ident)
					if !ok {
						continue
					}
					obj := pkg.TypesInfo.Defs[id]
					if obj == nil {
						obj = pkg.TypesInfo.Uses[id]
					}
					if obj != nil && !tainted[obj] && types.Identical(obj.Type(), types.Typ[types.String]) && !t570ParsesRelation(pkg, as.Rhs[i]) && t570ReadsIdentity(pkg, as.Rhs[i], params, tainted) {
						tainted[obj] = true
						changed = true
					}
				}
				return true
			})
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			ifs, ok := n.(*ast.IfStmt)
			if ok && t570Returns(ifs.Body, "foundNone") && t570TestsEmpty(pkg, ifs.Cond, params, tainted) {
				found = append(found, filepath.Base(path)+":"+strconv.Itoa(pkg.Fset.Position(ifs.Pos()).Line)+" "+fn.Name.Name)
			}
			return true
		})
	})
	sort.Strings(found)
	for _, f := range found {
		t.Errorf("%s answers a proven 0 when the row's own identity is missing; that row was not read (rowKeyMissing)", f)
	}
}

// A read that failed before anything was found carries its error to the
// flash: ReadFailed, which reads a missing client as unknown on its own.
// NotRead under a failed call drops the error.
func TestT570_FailedReadKeepsItsError(t *testing.T) {
	var found []string
	t570EachFunc(t, func(pkg *packages.Package, path string, fn *ast.FuncDecl) {
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			ifs, ok := n.(*ast.IfStmt)
			if !ok || !t570Returns(ifs.Body, "NotRead") {
				return true
			}
			failed := false
			ast.Inspect(ifs.Cond, func(c ast.Node) bool {
				be, ok := c.(*ast.BinaryExpr)
				if !ok || be.Op != token.NEQ {
					return true
				}
				if id, ok := be.Y.(*ast.Ident); !ok || id.Name != "nil" {
					return true
				}
				if tv, ok := pkg.TypesInfo.Types[be.X]; ok && types.Identical(tv.Type, types.Universe.Lookup("error").Type()) {
					failed = true
				}
				return true
			})
			if failed {
				found = append(found, filepath.Base(path)+":"+strconv.Itoa(pkg.Fset.Position(ifs.Pos()).Line)+" "+fn.Name.Name)
			}
			return true
		})
		// An error assigned and not yet tested when a later branch answers
		// NotRead is dropped the same way.
		errType := types.Universe.Lookup("error").Type()
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			block, ok := n.(*ast.BlockStmt)
			if !ok {
				return true
			}
			pending := map[types.Object]bool{}
			for _, st := range block.List {
				switch x := st.(type) {
				case *ast.AssignStmt:
					for _, lhs := range x.Lhs {
						id, ok := lhs.(*ast.Ident)
						if !ok {
							continue
						}
						obj := pkg.TypesInfo.Defs[id]
						if obj == nil {
							obj = pkg.TypesInfo.Uses[id]
						}
						if obj != nil && types.Identical(obj.Type(), errType) {
							pending[obj] = true
						}
					}
				case *ast.IfStmt:
					if len(pending) > 0 && t570Returns(x.Body, "NotRead") {
						mentions := false
						ast.Inspect(x, func(c ast.Node) bool {
							if id, ok := c.(*ast.Ident); ok && pending[pkg.TypesInfo.Uses[id]] {
								mentions = true
							}
							return true
						})
						if !mentions {
							found = append(found, filepath.Base(path)+":"+strconv.Itoa(pkg.Fset.Position(x.Pos()).Line)+" "+fn.Name.Name+" (error not tested)")
						}
					}
					ast.Inspect(x, func(c ast.Node) bool {
						if id, ok := c.(*ast.Ident); ok {
							delete(pending, pkg.TypesInfo.Uses[id])
						}
						return true
					})
				default:
					ast.Inspect(st, func(c ast.Node) bool {
						if id, ok := c.(*ast.Ident); ok {
							delete(pending, pkg.TypesInfo.Uses[id])
						}
						return true
					})
				}
			}
			return true
		})
	})
	sort.Strings(found)
	for _, f := range found {
		t.Errorf("%s answers NotRead for a failed call; the error is lost (ReadFailed)", f)
	}
}

// t570StateField is a state or status field AWS keeps reporting after the
// link it names stopped carrying anything, and the one predicate a related
// checker reads it through.
type t570StateField struct {
	pkg, typ, field string
	owners          []string
}

var t570StateFields = []t570StateField{
	{"github.com/aws/aws-sdk-go-v2/service/ec2/types", "Instance", "State", []string{"ec2InstanceLive"}},
	{"github.com/aws/aws-sdk-go-v2/service/autoscaling/types", "Instance", "LifecycleState", []string{"asgMembers"}},
	{"github.com/aws/aws-sdk-go-v2/service/ec2/types", "NatGateway", "State", []string{"natLiveAddresses"}},
	{"github.com/aws/aws-sdk-go-v2/service/ec2/types", "NatGatewayAddress", "Status", []string{"natLiveAddresses"}},
	{"github.com/aws/aws-sdk-go-v2/service/ec2/types", "TransitGatewayAttachment", "State", []string{"tgwAttachmentLive"}},
	{"github.com/aws/aws-sdk-go-v2/service/ec2/types", "TransitGatewayVpcAttachment", "State", []string{"tgwAttachmentLive"}},
	{"github.com/aws/aws-sdk-go-v2/service/dynamodb/types", "KinesisDataStreamDestination", "DestinationStatus", []string{"ddbDestinationLive"}},
	{"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types", "ExportTask", "Status", []string{"exportTaskWrites"}},
	{"github.com/aws/aws-sdk-go-v2/service/ecs/types", "ContainerInstance", "Status", []string{"ecsContainerInstanceMember"}},
	{"github.com/aws/aws-sdk-go-v2/service/sns/types", "Subscription", "SubscriptionArn", []string{"snsSubCarries"}},
	{"github.com/aws/aws-sdk-go-v2/service/ses/types", "ReceiptRule", "Enabled", []string{"sesActiveRulesFor"}},
	{"github.com/aws/aws-sdk-go-v2/service/sesv2/types", "EventDestination", "Enabled", []string{"sesEventDestinations"}},
	{"github.com/aws/aws-sdk-go-v2/service/opensearch/types", "LogPublishingOption", "Enabled", []string{"checkOpenSearchLogs"}},
	{"github.com/aws/aws-sdk-go-v2/service/elasticache/types", "NotificationConfiguration", "TopicStatus", []string{"checkRedisSNS"}},
	{"github.com/aws/aws-sdk-go-v2/service/mwaa/types", "ModuleLoggingConfiguration", "Enabled", []string{"checkMWAALogs"}},
	{"github.com/aws/aws-sdk-go-v2/service/codebuild/types", "CloudWatchLogsConfig", "Status", []string{"checkCbLogs"}},
	{"github.com/aws/aws-sdk-go-v2/service/codebuild/types", "S3LogsConfig", "Status", []string{"checkCbS3"}},
	{"github.com/aws/aws-sdk-go-v2/service/ecs/types", "ExecuteCommandConfiguration", "Logging", []string{"checkECSLogs"}},
}

// A related checker reads whether a subscription, attachment, member,
// destination or export still carries anything through that state's one
// predicate, so every pivot over the same state agrees; reading the field
// anywhere else in a related file is a second answer to the same question.
func TestT570_RelatedStateFieldsHaveOnePredicate(t *testing.T) {
	var found []string
	t570EachFunc(t, func(pkg *packages.Package, path string, fn *ast.FuncDecl) {
		if ok, _ := filepath.Match("*_related*.go", filepath.Base(path)); !ok {
			return
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			s := pkg.TypesInfo.Selections[sel]
			if s == nil || s.Kind() != types.FieldVal {
				return true
			}
			recv := s.Recv()
			if p, isPtr := recv.(*types.Pointer); isPtr {
				recv = p.Elem()
			}
			named, ok := types.Unalias(recv).(*types.Named)
			if !ok || named.Obj().Pkg() == nil {
				return true
			}
			for _, f := range t570StateFields {
				if named.Obj().Pkg().Path() == f.pkg && named.Obj().Name() == f.typ && sel.Sel.Name == f.field && !slices.Contains(f.owners, fn.Name.Name) {
					found = append(found, filepath.Base(path)+":"+strconv.Itoa(pkg.Fset.Position(sel.Pos()).Line)+" "+fn.Name.Name+" reads "+f.typ+"."+f.field+" outside "+strings.Join(f.owners, ", "))
				}
			}
			return true
		})
	})
	sort.Strings(found)
	for _, f := range found {
		t.Error(f)
	}
}

// t570InClosure reports whether pos lies inside a function literal of body.
func t570InClosure(body *ast.BlockStmt, pos token.Pos) bool {
	in := false
	ast.Inspect(body, func(n ast.Node) bool {
		if lit, ok := n.(*ast.FuncLit); ok && lit.Pos() <= pos && pos < lit.End() {
			in = true
		}
		return !in
	})
	return in
}

// A place already read keeps its rows when a second place fails: the checker
// folds both into relatedAnswer (joinReads, unreadBy), so the rows it found
// stand as a lower bound and the failure reaches the flash. Answering
// NotRead, ReadFailed or keyMissing after ids were collected drops them; a
// failed call that only sets a partial flag drops its error.
func TestT570_ASecondPlaceFailingKeepsTheFirst(t *testing.T) {
	var found []string
	errType := types.Universe.Lookup("error").Type()
	t570EachFunc(t, func(pkg *packages.Package, path string, fn *ast.FuncDecl) {
		// The ids a checker answers with are the slices it appends to and
		// then hands to an answer; an input it builds for a call is not.
		answered := map[types.Object]bool{}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			ret, ok := n.(*ast.ReturnStmt)
			if !ok || t570InClosure(fn.Body, ret.Pos()) {
				return true
			}
			for _, r := range ret.Results {
				if call, ok := r.(*ast.CallExpr); ok {
					if id, ok := call.Fun.(*ast.Ident); ok && (id.Name == "NotRead" || id.Name == "ReadFailed" || id.Name == "keyMissing") {
						continue
					}
				}
				ast.Inspect(r, func(c ast.Node) bool {
					if id, ok := c.(*ast.Ident); ok && pkg.TypesInfo.Uses[id] != nil {
						answered[pkg.TypesInfo.Uses[id]] = true
					}
					return true
				})
			}
			return true
		})
		var firstAppend token.Pos
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			as, ok := n.(*ast.AssignStmt)
			if !ok || len(as.Rhs) != 1 || t570InClosure(fn.Body, as.Pos()) {
				return true
			}
			call, ok := as.Rhs[0].(*ast.CallExpr)
			if id, isID := as.Lhs[0].(*ast.Ident); ok && isID {
				if f, isIdent := call.Fun.(*ast.Ident); isIdent && f.Name == "append" && answered[pkg.TypesInfo.Uses[id]] && (firstAppend == token.NoPos || as.Pos() < firstAppend) {
					firstAppend = as.Pos()
				}
			}
			return true
		})
		// A branch taken only when no id was collected loses none.
		var noneFound []*ast.IfStmt
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			ifs, ok := n.(*ast.IfStmt)
			if !ok {
				return true
			}
			ast.Inspect(ifs.Cond, func(c ast.Node) bool {
				be, ok := c.(*ast.BinaryExpr)
				if !ok || be.Op != token.EQL {
					return true
				}
				if call, ok := be.X.(*ast.CallExpr); ok && len(call.Args) == 1 {
					if f, ok := call.Fun.(*ast.Ident); ok && f.Name == "len" {
						if id, ok := call.Args[0].(*ast.Ident); ok && answered[pkg.TypesInfo.Uses[id]] {
							noneFound = append(noneFound, ifs)
						}
					}
				}
				return true
			})
			return true
		})
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.ReturnStmt:
				if firstAppend == token.NoPos || x.Pos() < firstAppend || t570InClosure(fn.Body, x.Pos()) || len(x.Results) != 1 {
					return true
				}
				if slices.ContainsFunc(noneFound, func(ifs *ast.IfStmt) bool { return ifs.Body.Pos() <= x.Pos() && x.Pos() < ifs.Body.End() }) {
					return true
				}
				if call, ok := x.Results[0].(*ast.CallExpr); ok {
					if id, ok := call.Fun.(*ast.Ident); ok && (id.Name == "NotRead" || id.Name == "ReadFailed" || id.Name == "keyMissing") {
						found = append(found, filepath.Base(path)+":"+strconv.Itoa(pkg.Fset.Position(x.Pos()).Line)+" "+fn.Name.Name+" answers "+id.Name+" after collecting ids")
					}
				}
			case *ast.IfStmt:
				var errVar types.Object
				ast.Inspect(x.Cond, func(c ast.Node) bool {
					be, ok := c.(*ast.BinaryExpr)
					if !ok || be.Op != token.NEQ {
						return true
					}
					if id, ok := be.X.(*ast.Ident); ok {
						if obj := pkg.TypesInfo.Uses[id]; obj != nil && types.Identical(obj.Type(), errType) {
							errVar = obj
						}
					}
					return true
				})
				if errVar == nil {
					return true
				}
				setsPartial, carries := false, false
				ast.Inspect(x.Body, func(c ast.Node) bool {
					if as, ok := c.(*ast.AssignStmt); ok && len(as.Lhs) == 1 {
						if id, ok := as.Lhs[0].(*ast.Ident); ok && strings.Contains(strings.ToLower(id.Name), "partial") {
							if v, ok := as.Rhs[0].(*ast.Ident); ok && v.Name == "true" {
								setsPartial = true
							}
						}
					}
					if id, ok := c.(*ast.Ident); ok && pkg.TypesInfo.Uses[id] == errVar {
						carries = true
					}
					return true
				})
				if setsPartial && !carries {
					found = append(found, filepath.Base(path)+":"+strconv.Itoa(pkg.Fset.Position(x.Pos()).Line)+" "+fn.Name.Name+" marks a failed call partial and drops its error")
				}
			}
			return true
		})
	})
	sort.Strings(found)
	for _, f := range found {
		t.Error(f)
	}
}
