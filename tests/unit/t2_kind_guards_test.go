package unit_test

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"golang.org/x/tools/go/packages"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/tests/unit"
)

// t2KindTypes is every type whose findings depend on what kind of resource a
// row is: the types the spec names and the types whose enrichers gate a pass
// on a kind field.
var t2KindTypes = []string{
	"dbi", "glue", "ssm", "logs", "elb", "kms", "ses",
	"role", "apigw", "dbi-snap", "dbc-snap",
}

// t2KindGated is the floor of kind-gated findings: each one applies to some
// kinds of its type and not to others, per the AWS documentation the row tests
// cite.
var t2KindGated = map[string][]domain.FindingCode{
	"glue": {t2GlueLoggingOff, t2GlueNoSecConfig, t2GlueSecConfigWeak},
	"ssm":  {t2SSMStale},
	"dbi":  {t2DBIDeletionProtOff, t2DBISingleAZ, t2DBINoBackups},
	"logs": {t2LogsNoMetricFilters},
}

func t2Classifier(td catalog.ResourceTypeDef) (func(domain.Resource) string, []string, error) {
	v := reflect.ValueOf(td)
	kf := v.FieldByName("Kind")
	if !kf.IsValid() {
		return nil, nil, fmt.Errorf("ResourceTypeDef has no Kind field")
	}
	if kf.Kind() != reflect.Func || kf.IsNil() {
		return nil, nil, fmt.Errorf("%s declares no Kind classifier", td.ShortName)
	}
	fn, ok := kf.Interface().(func(domain.Resource) string)
	if !ok {
		return nil, nil, fmt.Errorf("%s Kind is %s, want func(domain.Resource) string", td.ShortName, kf.Type())
	}
	kinds := t2Strings(v.FieldByName("Kinds"))
	if len(kinds) == 0 {
		return nil, nil, fmt.Errorf("%s declares no Kinds value set", td.ShortName)
	}
	return fn, kinds, nil
}

func t2Strings(v reflect.Value) []string {
	if !v.IsValid() || v.Kind() != reflect.Slice {
		return nil
	}
	out := make([]string, v.Len())
	for i := range out {
		out[i] = v.Index(i).String()
	}
	return out
}

func t2KindDecl(def catalog.FindingDef) (only, not []string) {
	v := reflect.ValueOf(def)
	return t2Strings(v.FieldByName("OnlyKinds")), t2Strings(v.FieldByName("NotKinds"))
}

func t2AppliesTo(kinds, only, not []string) []string {
	var out []string
	for _, k := range kinds {
		if (len(only) == 0 || slices.Contains(only, k)) && !slices.Contains(not, k) {
			out = append(out, k)
		}
	}
	return out
}

func t2Type(t *testing.T, short string) catalog.ResourceTypeDef {
	t.Helper()
	td := resource.FindResourceType(short)
	if td == nil {
		t.Fatalf("%s is not registered", short)
	}
	return *td
}

// Every kind a classifier can return has a demo row, and on the demo bench a
// kind-gated finding is absent from every row of an excluded kind and present
// on at least one row of an included kind.
func TestT2Guard_KindClassifiersAndGatedFindingsOnTheDemoBench(t *testing.T) {
	byType, cache := buildVisibilityTypeCache(t)
	clients := demo.NewServiceClients()
	gated := 0
	for _, short := range t2KindTypes {
		td := t2Type(t, short)
		kindOf, kinds, err := t2Classifier(td)
		if err != nil {
			t.Errorf("%s: %v", short, err)
			continue
		}
		rows := mergeWave2Findings(t, td, byType[short], cache, clients)
		byKind := map[string][]resource.Resource{}
		for _, r := range rows {
			k := kindOf(domain.Resource(r))
			if !slices.Contains(kinds, k) {
				t.Errorf("%s/%s: classifier returned %q, outside the declared kinds %v", short, r.ID, k, kinds)
			}
			byKind[k] = append(byKind[k], r)
		}
		for _, k := range kinds {
			if len(byKind[k]) == 0 {
				t.Errorf("%s: no demo row of kind %q", short, k)
			}
		}
		for _, def := range td.Findings {
			only, not := t2KindDecl(def)
			if len(only) == 0 && len(not) == 0 {
				continue
			}
			gated++
			applies := t2AppliesTo(kinds, only, not)
			present := false
			for k, rs := range byKind {
				for _, r := range rs {
					has := t1aHas(r.Findings, def.Code)
					if has && !slices.Contains(applies, k) {
						t.Errorf("%s/%s of kind %q carries %s, declared for %v", short, r.ID, k, def.Code, applies)
					}
					present = present || has
				}
			}
			if !present {
				t.Errorf("%s: no demo row of an included kind %v carries %s", short, applies, def.Code)
			}
		}
	}
	for short, codes := range t2KindGated {
		td := t2Type(t, short)
		for _, code := range codes {
			def, ok := t2Def(td, code)
			if !ok {
				t.Errorf("%s: %s is not declared", short, code)
				continue
			}
			if only, not := t2KindDecl(def); len(only) == 0 && len(not) == 0 {
				t.Errorf("%s: %s declares neither OnlyKinds nor NotKinds", short, code)
			}
		}
	}
	if gated == 0 {
		t.Error("no finding declares OnlyKinds or NotKinds")
	}
}

func t2Def(td catalog.ResourceTypeDef, code domain.FindingCode) (catalog.FindingDef, bool) {
	i := slices.IndexFunc(td.Findings, func(d catalog.FindingDef) bool { return d.Code == code })
	if i < 0 {
		return catalog.FindingDef{}, false
	}
	return td.Findings[i], true
}

// The demo bench carries the kinds the rows above turn on — a Spark streaming
// job, a Ray job, a Multi-AZ DB cluster member and a log group of every
// non-standard class — and each renders through the app's own list and detail
// path: the Status cell leads with its top finding, the row colour is that
// finding's, and no Attention value restates the phrase or shows a Go bool or
// an SDK enum token. The Ray job carries a gated finding's defect (no security
// configuration) without the finding, so a selector that ignores the kind
// fails here.
func TestT2Demo_KindWitnessesRenderThroughTheApp(t *testing.T) {
	byType, cache := buildVisibilityTypeCache(t)
	clients := demo.NewServiceClients()

	glueTD := t2Type(t, "glue")
	glueRows := mergeWave2Findings(t, glueTD, byType["glue"], cache, clients)
	sawRay := false
	for _, want := range []string{"gluestreaming", "glueray"} {
		i := slices.IndexFunc(glueRows, func(r resource.Resource) bool {
			job, ok := r.RawStruct.(gluetypes.Job)
			return ok && job.Command != nil && job.Command.Name != nil && *job.Command.Name == want
		})
		if i < 0 {
			t.Errorf("no demo glue job with Command.Name %q", want)
			continue
		}
		r := glueRows[i]
		if want == "glueray" {
			sawRay = true
			job, _ := r.RawStruct.(gluetypes.Job)
			if job.SecurityConfiguration != nil {
				t.Errorf("demo Ray job %s names a security configuration; it must name none so the gate can fail on it", r.ID)
			}
			for _, code := range []domain.FindingCode{t2GlueNoSecConfig, t2GlueSecConfigWeak, t2GlueLoggingOff} {
				if t1aHas(r.Findings, code) {
					t.Errorf("demo Ray job %s carries %s", r.ID, code)
				}
			}
		}
		if len(r.Findings) > 0 {
			t563AssertWitnessRendered(t, glueTD, glueRows, r)
		}
	}
	if !sawRay {
		t.Error("the demo bench has no Ray job for the security-configuration gate to fail on")
	}

	dbiTD := t2Type(t, "dbi")
	dbiRows := mergeWave2Findings(t, dbiTD, byType["dbi"], cache, clients)
	i := slices.IndexFunc(dbiRows, func(r resource.Resource) bool {
		db, ok := r.RawStruct.(rdstypes.DBInstance)
		if !ok || db.DBClusterIdentifier == nil || db.Engine == nil {
			return false
		}
		return *db.Engine == "mysql" || *db.Engine == "postgres"
	})
	if i < 0 {
		t.Error("no demo dbi row is a Multi-AZ DB cluster member (mysql/postgres with DBClusterIdentifier)")
	} else {
		for _, code := range []domain.FindingCode{t2DBIDeletionProtOff, t2DBISingleAZ, t2DBINoBackups} {
			if t1aHas(dbiRows[i].Findings, code) {
				t.Errorf("demo Multi-AZ DB cluster member %s carries %s", dbiRows[i].ID, code)
			}
		}
		if len(dbiRows[i].Findings) > 0 {
			t563AssertWitnessRendered(t, dbiTD, dbiRows, dbiRows[i])
		}
	}

	logsTD := t2Type(t, "logs")
	logsRows := mergeWave2Findings(t, logsTD, byType["logs"], cache, clients)
	for _, class := range []cwlogstypes.LogGroupClass{cwlogstypes.LogGroupClassInfrequentAccess, cwlogstypes.LogGroupClassDelivery} {
		i := slices.IndexFunc(logsRows, func(r resource.Resource) bool {
			lg, ok := r.RawStruct.(cwlogstypes.LogGroup)
			return ok && lg.LogGroupClass == class
		})
		if i < 0 {
			t.Errorf("no demo log group of class %s", class)
			continue
		}
		if t1aHas(logsRows[i].Findings, t2LogsNoMetricFilters) {
			t.Errorf("demo %s log group %s carries %s", class, logsRows[i].ID, t2LogsNoMetricFilters)
		}
		if len(logsRows[i].Findings) > 0 {
			t563AssertWitnessRendered(t, logsTD, logsRows, logsRows[i])
		}
	}
}

func t2GeneratedFindings(t *testing.T, short string) ([]string, map[domain.FindingCode][]string) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "..", "docs", "resources", short+".md"))
	if err != nil {
		t.Fatalf("%s: %v", short, err)
	}
	text := string(body)
	start := strings.Index(text, "<!-- BEGIN GENERATED: findings -->")
	end := strings.Index(text, "<!-- END GENERATED: findings -->")
	if start < 0 || end < start {
		t.Fatalf("docs/resources/%s.md has no generated findings block", short)
	}
	var header []string
	rows := map[domain.FindingCode][]string{}
	for _, line := range strings.Split(text[start:end], "\n") {
		if !strings.HasPrefix(line, "|") || strings.HasPrefix(line, "| ---") {
			continue
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		if header == nil {
			header = cells
			continue
		}
		rows[domain.FindingCode(cells[0])] = cells
	}
	return header, rows
}

// The generated resource doc states, for every kind-gated finding, the kinds
// it applies to, read from the declaration; a finding that declares nothing
// is not listed with kinds it never names.
func TestT2Docs_KindGatedFindingsShowAppliesTo(t *testing.T) {
	checked := 0
	for _, td := range slices.Concat(catalog.All(), catalog.AllChildren()) {
		kinds := []string(nil)
		if _, k, err := t2Classifier(td); err == nil {
			kinds = k
		}
		for _, def := range td.Findings {
			only, not := t2KindDecl(def)
			if len(only) == 0 && len(not) == 0 {
				continue
			}
			checked++
			header, rows := t2GeneratedFindings(t, td.ShortName)
			col := slices.Index(header, "Applies to")
			if col < 0 {
				t.Errorf("docs/resources/%s.md findings table has no \"Applies to\" column; header %v", td.ShortName, header)
				continue
			}
			row, ok := rows[def.Code]
			if !ok || col >= len(row) {
				t.Errorf("docs/resources/%s.md lists no row for %s", td.ShortName, def.Code)
				continue
			}
			named := slices.Concat(only, not)
			if len(kinds) > 0 {
				named = t2AppliesTo(kinds, only, not)
			}
			for _, k := range named {
				if !strings.Contains(row[col], k) {
					t.Errorf("docs/resources/%s.md %s Applies to = %q, want it to name %q", td.ShortName, def.Code, row[col], k)
				}
			}
		}
	}
	for short, codes := range t2KindGated {
		header, rows := t2GeneratedFindings(t, short)
		col := slices.Index(header, "Applies to")
		for _, code := range codes {
			if col < 0 || col >= len(rows[code]) || rows[code][col] == "" {
				t.Errorf("docs/resources/%s.md: %s shows no Applies to", short, code)
			}
		}
	}
	if checked == 0 {
		t.Error("no finding declares OnlyKinds or NotKinds, so no Applies to was checked")
	}
}

// t2AgeLiteral is a number with a time unit, as "over 365 days", "(>90d)" or
// "last 24h" spell it.
var t2AgeLiteral = regexp.MustCompile(`\d+\s*(d|h|days?|hours?|weeks?|months?|years?)\b`)

// An age cut-off in a phrase is a9s policy, so the number lives once, in the
// constant the check compares against, and reaches the text through a slot.
// The Kinesis phrase states the 24-hour retention AWS fixes as the minimum and
// default, not a cut-off of a9s's.
func TestT2Guard_NoAgeLiteralInAPhrase(t *testing.T) {
	awsFixed := map[domain.FindingCode]bool{"kinesis.min-retention": true}
	var hits []string
	defs := 0
	for _, td := range slices.Concat(catalog.All(), catalog.AllChildren()) {
		for _, def := range td.Findings {
			defs++
			if awsFixed[def.Code] {
				continue
			}
			if m := t2AgeLiteral.FindString(def.Phrase); m != "" {
				hits = append(hits, fmt.Sprintf("%s %q (%q)", def.Code, def.Phrase, m))
			}
		}
	}
	if defs < 300 {
		t.Fatalf("scanned %d finding definitions; the catalog did not load", defs)
	}
	slices.Sort(hits)
	if len(hits) > 0 {
		t.Errorf("%d phrases state an age cut-off as a literal:\n  %s", len(hits), strings.Join(hits, "\n  "))
	}
}

var t2KindFields = map[string][]string{
	"github.com/aws/aws-sdk-go-v2/service/glue/types.JobCommand":                     {"Name"},
	"github.com/aws/aws-sdk-go-v2/service/ssm/types.ParameterMetadata":               {"Type"},
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types.LogGroup":             {"LogGroupClass"},
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types.LoadBalancer": {"Type"},
	"github.com/aws/aws-sdk-go-v2/service/kms/types.KeyMetadata":                     {"KeySpec", "Origin", "KeyManager", "CustomerMasterKeySpec"},
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types.IdentityInfo":                  {"IdentityType"},
	"github.com/aws/aws-sdk-go-v2/service/sesv2.GetEmailIdentityOutput":              {"IdentityType"},
	"github.com/aws/aws-sdk-go-v2/service/rds/types.DBInstance":                      {"DBClusterIdentifier", "ReadReplicaSourceDBInstanceIdentifier"},
	"github.com/aws/aws-sdk-go-v2/service/rds/types.DBSnapshot":                      {"SnapshotType"},
	"github.com/aws/aws-sdk-go-v2/service/rds/types.DBClusterSnapshot":               {"SnapshotType"},
	"github.com/aws/aws-sdk-go-v2/service/docdb/types.DBClusterSnapshot":             {"SnapshotType"},
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types.Api":                    {"ProtocolType"},
	"github.com/aws/aws-sdk-go-v2/service/iam/types.Role":                            {"Path"},
}

// t2KindValues are AWS kind values spelled distinctly enough that a constant
// equal to one, compared against anything, is a kind test.
var t2KindValues = []string{
	"glueetl", "gluestreaming", "pythonshell", "glueray",
	"String", "StringList", "SecureString",
	"STANDARD", "INFREQUENT_ACCESS", "ESSENTIALS", "DELIVERY",
	"EMAIL_ADDRESS", "MANAGED_DOMAIN",
}

// t2PresenceKinds are the kind fields whose presence is the kind: a DB
// instance with a DBClusterIdentifier is a cluster member, one with a
// replication source is a read replica. A nil test on any other kind field
// only guards its read.
var t2PresenceKinds = []string{"DBClusterIdentifier", "ReadReplicaSourceDBInstanceIdentifier"}

// t2KindFieldKeys are the Fields keys a type's fetcher stores its kind under,
// with the file prefixes of that type; other types use the same key for other
// facts (an SNS subscription's protocol).
var t2KindFieldKeys = map[string][]string{
	"command":         {"glue"},
	"identity_type":   {"ses"},
	"protocol":        {"apigw"},
	"log_group_class": {"logs", "cwlogs"},
}

func t2TypeName(tp types.Type) string {
	tp = types.Unalias(tp)
	if ptr, ok := tp.(*types.Pointer); ok {
		tp = types.Unalias(ptr.Elem())
	}
	return types.TypeString(tp, nil)
}

func t2IsNil(e ast.Expr) bool {
	id, ok := e.(*ast.Ident)
	return ok && id.Name == "nil"
}

func t2LoadCoreAWS(t *testing.T) []*packages.Package {
	t.Helper()
	cfg := &packages.Config{Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo, Dir: filepath.Join("..", "..")}
	pkgs, err := unit.LoadPackagesOnce(cfg, "./core/aws")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	files := 0
	for _, p := range pkgs {
		for _, e := range p.Errors {
			t.Fatalf("%s: %v", p.PkgPath, e)
		}
		files += len(p.Syntax)
	}
	if files < 300 {
		t.Fatalf("loaded %d files; the load lost core/aws", files)
	}
	return pkgs
}

func t2Site(p *packages.Package, pos token.Pos) string {
	at := p.Fset.Position(pos)
	return fmt.Sprintf("%s:%d", filepath.Base(at.Filename), at.Line)
}

// t2ClassifierBodies returns the source ranges of every function a catalog
// literal names as its Kind classifier: a function literal in place, or the
// declaration of a function it names.
func t2ClassifierBodies(p *packages.Package) [][2]token.Pos {
	named := map[types.Object]bool{}
	var out [][2]token.Pos
	for _, f := range p.Syntax {
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}
			tv, ok := p.TypesInfo.Types[lit]
			if !ok || t2TypeName(tv.Type) != "github.com/k2m30/a9s/v3/core/catalog.ResourceTypeDef" {
				return true
			}
			for _, el := range lit.Elts {
				kv, ok := el.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				if key, ok := kv.Key.(*ast.Ident); !ok || key.Name != "Kind" {
					continue
				}
				switch v := kv.Value.(type) {
				case *ast.FuncLit:
					out = append(out, [2]token.Pos{v.Pos(), v.End()})
				case *ast.Ident:
					if obj := p.TypesInfo.Uses[v]; obj != nil {
						named[obj] = true
					}
				}
			}
			return true
		})
	}
	for _, f := range p.Syntax {
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && named[p.TypesInfo.Defs[fd.Name]] {
				out = append(out, [2]token.Pos{fd.Pos(), fd.End()})
			}
		}
	}
	return out
}

// t2KindTestIn reports why operand e of a test in file is a kind test, or "".
func t2KindTestIn(p *packages.Package, e ast.Expr, file string, againstNil bool) string {
	found := ""
	ast.Inspect(e, func(n ast.Node) bool {
		if found != "" {
			return false
		}
		x, ok := n.(ast.Expr)
		if !ok {
			return true
		}
		if tv, ok := p.TypesInfo.Types[x]; ok && tv.Value != nil && tv.Value.Kind() == constant.String {
			if v := constant.StringVal(tv.Value); slices.Contains(t2KindValues, v) {
				found = "value " + v
			}
			return false
		}
		switch x := x.(type) {
		case *ast.SelectorExpr:
			sel, ok := p.TypesInfo.Selections[x]
			if !ok || sel.Kind() != types.FieldVal {
				return true
			}
			if !slices.Contains(t2KindFields[t2TypeName(sel.Recv())], x.Sel.Name) {
				return true
			}
			if againstNil && !slices.Contains(t2PresenceKinds, x.Sel.Name) {
				return true
			}
			found = "field " + x.Sel.Name
		case *ast.IndexExpr:
			sel, ok := x.X.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Fields" {
				return true
			}
			tv, ok := p.TypesInfo.Types[x.Index]
			if !ok || tv.Value == nil || tv.Value.Kind() != constant.String {
				return true
			}
			key := constant.StringVal(tv.Value)
			if slices.ContainsFunc(t2KindFieldKeys[key], func(prefix string) bool { return strings.HasPrefix(file, prefix) }) {
				found = "Fields[" + key + "]"
			}
		}
		return true
	})
	return found
}

// The classifier is the one reader of a type's kind: outside the function a
// catalog literal names as Kind, no fetcher or enricher in core/aws compares,
// switches on or prefix-tests a kind field, a Fields entry holding a kind, or
// a kind value, and the single-AZ check takes no per-site SkipSingleAZ flag.
// The related-panel checkers are outside findings and are not scanned.
func TestT2Guard_NoKindTestOutsideTheClassifier(t *testing.T) {
	sites := map[string]bool{}
	for _, p := range t2LoadCoreAWS(t) {
		allowed := t2ClassifierBodies(p)
		inClassifier := func(pos token.Pos) bool {
			return slices.ContainsFunc(allowed, func(r [2]token.Pos) bool { return pos >= r[0] && pos < r[1] })
		}
		for _, f := range p.Syntax {
			file := filepath.Base(p.Fset.Position(f.Pos()).Filename)
			if strings.Contains(file, "_related") {
				continue
			}
			ast.Inspect(f, func(n ast.Node) bool {
				var operands []ast.Expr
				againstNil := false
				switch x := n.(type) {
				case *ast.Ident:
					if x.Name == "SkipSingleAZ" {
						sites[t2Site(p, x.Pos())+" (SkipSingleAZ)"] = true
					}
				case *ast.BinaryExpr:
					if x.Op == token.EQL || x.Op == token.NEQ {
						operands = []ast.Expr{x.X, x.Y}
						againstNil = t2IsNil(x.X) || t2IsNil(x.Y)
					}
				case *ast.SwitchStmt:
					if x.Tag != nil {
						operands = []ast.Expr{x.Tag}
					}
				case *ast.CaseClause:
					operands = x.List
				case *ast.CallExpr:
					if fn, ok := x.Fun.(*ast.SelectorExpr); ok {
						if pkg, ok := fn.X.(*ast.Ident); ok && (pkg.Name == "strings" || pkg.Name == "slices") {
							operands = x.Args
						}
					}
				}
				for _, e := range operands {
					if inClassifier(e.Pos()) {
						continue
					}
					if why := t2KindTestIn(p, e, file, againstNil); why != "" {
						sites[t2Site(p, e.Pos())+" ("+why+")"] = true
					}
				}
				return true
			})
		}
	}
	if len(sites) > 0 {
		list := slices.Sorted(maps.Keys(sites))
		t.Errorf("%d kind tests outside a Kind classifier:\n  %s", len(list), strings.Join(list, "\n  "))
	}
}

// A row's Findings are written by the Wave 1 emit door alone, so every Wave 1
// finding passes the one applicability rule; no fetcher in core/aws assigns
// the field or sets it in a Resource literal. The door is the file that holds
// addWave1Finding.
func TestT2Guard_NoFindingsAssignedOutsideTheEmitDoor(t *testing.T) {
	const resourceType = "github.com/k2m30/a9s/v3/core/domain.Resource"
	sites := map[string]bool{}
	for _, p := range t2LoadCoreAWS(t) {
		for _, f := range p.Syntax {
			door := slices.ContainsFunc(f.Decls, func(d ast.Decl) bool {
				fd, ok := d.(*ast.FuncDecl)
				return ok && fd.Recv == nil && fd.Name.Name == "addWave1Finding"
			})
			if door {
				continue
			}
			for _, d := range f.Decls {
				ast.Inspect(d, func(n ast.Node) bool {
					switch x := n.(type) {
					case *ast.AssignStmt:
						for _, lhs := range x.Lhs {
							sel, ok := lhs.(*ast.SelectorExpr)
							if !ok || sel.Sel.Name != "Findings" {
								continue
							}
							if s, ok := p.TypesInfo.Selections[sel]; ok && t2TypeName(s.Recv()) == resourceType {
								sites[t2Site(p, sel.Pos())] = true
							}
						}
					case *ast.CompositeLit:
						tv, ok := p.TypesInfo.Types[x]
						if !ok || t2TypeName(tv.Type) != resourceType {
							return true
						}
						for _, el := range x.Elts {
							if kv, ok := el.(*ast.KeyValueExpr); ok {
								if key, ok := kv.Key.(*ast.Ident); ok && key.Name == "Findings" {
									sites[t2Site(p, kv.Pos())] = true
								}
							}
						}
					}
					return true
				})
			}
		}
	}
	if len(sites) > 0 {
		list := slices.Sorted(maps.Keys(sites))
		t.Errorf("%d sites set a Resource's Findings outside addWave1Finding:\n  %s", len(list), strings.Join(list, "\n  "))
	}
}
