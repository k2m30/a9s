package unit_test

import (
	"go/ast"
	"go/types"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

// Readers that are not pivots: the fetcher renders the launch source on the
// row, the enricher judges a launch configuration's posture, and the
// attachment enricher turns a failed or stuck state into a finding.
var (
	t570ASGNonPivotReaders      = []string{"asg.go:FetchAutoScalingGroupsPage", "asg_issue_enrichment.go:asgLaunchConfigurationPosture"}
	t570TGWStateNonPivotReaders = []string{"tgw_issue_enrichment.go:EnrichTGWAttachments"}
)

// t570ResourceDocPivots reads the pivots a docs/resources/<type>.md lists in
// its related-panel section: every "### `<target>`" block of section 2 that
// does not declare the pivot unimplemented.
func t570ResourceDocPivots(t *testing.T, path string) (pivots []string, ok bool) {
	t.Helper()
	raw, err := os.ReadFile(path) //nolint:gosec // paths come from a glob inside the repo
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	inSection := false
	target := ""
	var body strings.Builder
	flush := func() {
		if target != "" && !strings.Contains(body.String(), "pivot not implemented") {
			pivots = append(pivots, target)
		}
		target = ""
		body.Reset()
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "## ") {
			flush()
			inSection = strings.HasPrefix(line, "## 2.")
			ok = ok || inSection
			continue
		}
		if !inSection {
			continue
		}
		if rest, found := strings.CutPrefix(line, "### `"); found {
			flush()
			target, _, _ = strings.Cut(rest, "`")
			continue
		}
		body.WriteString(line)
		body.WriteByte('\n')
	}
	flush()
	return pivots, ok
}

// t570ContractPivots reads the pivots docs/related-resources.md registers per
// type: the "- **`<target>`**" bullets of each "### `<type>`" section, up to
// its "Explicitly excluded" list.
func t570ContractPivots(t *testing.T) map[string]map[string]bool {
	t.Helper()
	path := filepath.Join("..", "..", "docs", "related-resources.md")
	raw, err := os.ReadFile(path) //nolint:gosec // fixed path inside the repo
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	out := map[string]map[string]bool{}
	source := ""
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "## ") {
			source = ""
			continue
		}
		if rest, ok := strings.CutPrefix(line, "### `"); ok {
			source, _, _ = strings.Cut(rest, "`")
			out[source] = map[string]bool{}
			continue
		}
		if source == "" {
			continue
		}
		if strings.HasPrefix(line, "Explicitly excluded") {
			source = ""
			continue
		}
		rest, ok := strings.CutPrefix(line, "- **`")
		if !ok {
			continue
		}
		if target, _, found := strings.Cut(rest, "`**"); found {
			out[source][target] = true
		}
	}
	return out
}

// The two contract documents name the same pivots for every type:
// docs/related-resources.md is the contract and docs/resources/<type>.md
// explains each of its pivots, so a pivot one lists and the other does not
// (or declares unimplemented) is two contracts for one panel.
func TestT570_ContractDocsListTheSamePivots(t *testing.T) {
	bullets := t570ContractPivots(t)
	paths, err := filepath.Glob(filepath.Join("..", "..", "docs", "resources", "*.md"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no resource docs found: %v", err)
	}
	checked := 0
	for _, path := range paths {
		typ := strings.TrimSuffix(filepath.Base(path), ".md")
		contract, listed := bullets[typ]
		if !listed {
			continue
		}
		docPivots, hasSection := t570ResourceDocPivots(t, path)
		if !hasSection {
			continue
		}
		checked++
		want := map[string]bool{}
		for target := range contract {
			want[target] = true
		}
		got := map[string]bool{}
		for _, p := range docPivots {
			got[p] = true
		}
		var onlyContract, onlyDoc []string
		for target := range want {
			if target != "ct-events" && !got[target] {
				onlyContract = append(onlyContract, target)
			}
		}
		for target := range got {
			if target != "ct-events" && !want[target] {
				onlyDoc = append(onlyDoc, target)
			}
		}
		sort.Strings(onlyContract)
		sort.Strings(onlyDoc)
		if len(onlyContract) > 0 || len(onlyDoc) > 0 {
			t.Errorf("%s: docs/related-resources.md lists %v that docs/resources/%s.md does not implement; docs/resources/%s.md lists %v that the contract does not",
				typ, onlyContract, typ, typ, onlyDoc)
		}
	}
	if checked < 50 {
		t.Fatalf("compared %d types, want every resource doc with a related-panel section", checked)
	}
}

const t570AWSPkg = "github.com/k2m30/a9s/v3/core/aws"

func t570LoadAWS(t *testing.T) *packages.Package {
	t.Helper()
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo,
		Dir:  filepath.Join("..", ".."),
	}
	pkgs, err := packages.Load(cfg, t570AWSPkg)
	if err != nil || len(pkgs) != 1 || len(pkgs[0].Errors) > 0 {
		t.Fatalf("load %s: %v %v", t570AWSPkg, err, pkgs)
	}
	return pkgs[0]
}

// t570FieldReaders returns every top-level function of core/aws (tests
// excluded) that selects one of fields on the named SDK struct, as
// "file.go:func".
func t570FieldReaders(t *testing.T, pkg *packages.Package, structPkg, structName string, fields ...string) []string {
	t.Helper()
	want := map[string]bool{}
	for _, f := range fields {
		want[f] = true
	}
	seen := map[string]bool{}
	for _, file := range pkg.Syntax {
		path := pkg.Fset.Position(file.Pos()).Filename
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				sel, ok := n.(*ast.SelectorExpr)
				if !ok || !want[sel.Sel.Name] {
					return true
				}
				s, ok := pkg.TypesInfo.Selections[sel]
				if !ok || s.Kind() != types.FieldVal {
					return true
				}
				recv := s.Recv()
				if p, isPtr := recv.(*types.Pointer); isPtr {
					recv = p.Elem()
				}
				named, ok := recv.(*types.Named)
				if !ok || named.Obj().Pkg() == nil || named.Obj().Pkg().Path() != structPkg || named.Obj().Name() != structName {
					return true
				}
				seen[filepath.Base(path)+":"+fn.Name.Name] = true
				return true
			})
		}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func t570Without(readers []string, allowed ...string) []string {
	var out []string
	for _, r := range readers {
		if !slices.Contains(allowed, r) {
			out = append(out, r)
		}
	}
	return out
}

// One classifier decides what an ECS task definition's secret references
// name: container Secrets[] and a log driver's SecretOptions. A second
// function reading either field is a second classification that can drift.
func TestT570_ECSSecretReferences_HaveOneClassifier(t *testing.T) {
	pkg := t570LoadAWS(t)
	readers := append(
		t570FieldReaders(t, pkg, "github.com/aws/aws-sdk-go-v2/service/ecs/types", "ContainerDefinition", "Secrets"),
		t570FieldReaders(t, pkg, "github.com/aws/aws-sdk-go-v2/service/ecs/types", "LogConfiguration", "SecretOptions")...)
	readers = slices.Compact(slices.Sorted(slices.Values(readers)))
	if len(readers) != 1 {
		t.Errorf("ECS Secrets/SecretOptions are read by %d functions, want exactly the one classifier: %v", len(readers), readers)
	}
}

// One function returns an ASG's launch sources (launch template, launch
// configuration, mixed-instances policy with its overrides); every pivot
// reads them through it. The fetcher's display rows and the launch-config
// enricher are not pivots.
func TestT570_ASGLaunchSources_HaveOneOwner(t *testing.T) {
	pkg := t570LoadAWS(t)
	readers := t570FieldReaders(t, pkg, "github.com/aws/aws-sdk-go-v2/service/autoscaling/types", "AutoScalingGroup",
		"LaunchTemplate", "LaunchConfigurationName", "MixedInstancesPolicy")
	readers = t570Without(readers, t570ASGNonPivotReaders...)
	if len(readers) != 1 {
		t.Errorf("ASG launch fields are read by %d functions outside the fetcher and enricher, want exactly the one launch-source function: %v", len(readers), readers)
	}
}

// One predicate per AWS state enum decides whether a transit-gateway
// attachment is live (TransitGatewayAttachmentState); the enricher that turns
// a state into a finding is not a liveness decision.
func TestT570_TGWAttachmentLiveness_HasOnePredicate(t *testing.T) {
	pkg := t570LoadAWS(t)
	const ec2types = "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	readers := append(
		t570FieldReaders(t, pkg, ec2types, "TransitGatewayAttachment", "State"),
		t570FieldReaders(t, pkg, ec2types, "TransitGatewayVpcAttachment", "State")...)
	readers = t570Without(slices.Compact(slices.Sorted(slices.Values(readers))), t570TGWStateNonPivotReaders...)
	if len(readers) != 1 {
		t.Errorf("attachment State is read by %d functions outside the enricher, want exactly the one liveness predicate: %v", len(readers), readers)
	}
}

// t570IsCacheType reports whether a package-level variable of type typ can
// hold answers across calls: a map, a sync.Map, or a struct guarding state
// with a mutex.
func t570IsCacheType(typ types.Type) bool {
	if p, ok := typ.(*types.Pointer); ok {
		typ = p.Elem()
	}
	if named, ok := typ.(*types.Named); ok && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == "sync" && named.Obj().Name() == "Map" {
		return true
	}
	switch u := typ.Underlying().(type) {
	case *types.Map:
		return true
	case *types.Struct:
		for i := range u.NumFields() {
			if named, ok := u.Field(i).Type().(*types.Named); ok && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == "sync" &&
				(named.Obj().Name() == "Mutex" || named.Obj().Name() == "RWMutex") {
				return true
			}
		}
	}
	return false
}

// A related checker's cached answers belong to the session: a package-level
// store outlives a profile or Region switch and every refresh, so the next
// account is shown the previous one's answer. A package-level map, sync.Map
// or mutex-guarded struct that a function writes (outside init) and a
// *_related*.go function reads is such a store.
func TestT570_RelatedCheckers_HoldNoPackageLevelCache(t *testing.T) {
	pkg := t570LoadAWS(t)
	caches := map[*types.Var]bool{}
	scope := pkg.Types.Scope()
	for _, name := range scope.Names() {
		if v, ok := scope.Lookup(name).(*types.Var); ok && t570IsCacheType(v.Type()) {
			caches[v] = true
		}
	}
	written := map[*types.Var]bool{}
	readByRelated := map[*types.Var]string{}
	for _, file := range pkg.Syntax {
		path := pkg.Fset.Position(file.Pos()).Filename
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		related, _ := filepath.Match("*_related*.go", filepath.Base(path))
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				id, ok := n.(*ast.Ident)
				if !ok {
					return true
				}
				v, ok := pkg.TypesInfo.Uses[id].(*types.Var)
				if !ok || !caches[v] {
					return true
				}
				if related {
					readByRelated[v] = filepath.Base(path) + ":" + fn.Name.Name
				}
				return true
			})
			if fn.Name.Name == "init" {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				var targets []ast.Expr
				switch x := n.(type) {
				case *ast.AssignStmt:
					targets = x.Lhs
				case *ast.IncDecStmt:
					targets = []ast.Expr{x.X}
				case *ast.CallExpr:
					if sel, ok := x.Fun.(*ast.SelectorExpr); ok {
						switch sel.Sel.Name {
						case "Store", "LoadOrStore", "LoadAndDelete", "Delete", "Swap", "CompareAndSwap", "Lock":
							targets = []ast.Expr{sel.X}
						}
					}
					if id, ok := x.Fun.(*ast.Ident); ok && id.Name == "delete" && len(x.Args) > 0 {
						targets = []ast.Expr{x.Args[0]}
					}
				}
				for _, target := range targets {
					if ix, ok := target.(*ast.IndexExpr); ok {
						target = ix.X
					}
					if id, ok := target.(*ast.Ident); ok {
						if v, ok := pkg.TypesInfo.Uses[id].(*types.Var); ok && caches[v] {
							written[v] = true
						}
					}
				}
				return true
			})
		}
	}
	var found []string
	for v, reader := range readByRelated {
		if written[v] {
			found = append(found, v.Name()+" (read by "+reader+")")
		}
	}
	sort.Strings(found)
	for _, f := range found {
		t.Errorf("related checker reads the package-level cache %s", f)
	}
}
