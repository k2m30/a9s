package unit

import (
	"go/ast"
	"go/types"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"golang.org/x/tools/go/packages"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
)

// Every narrow interface a checker asserts off a ServiceClients field must be
// satisfied by the client CreateServiceClients puts in that field. A wrapper
// (the in-flight coalescing ones) that exposes only the aggregate makes the
// assertion fail on a live session, and the check it guards silently never
// runs — while every unit test, whose fake embeds the aggregate, passes.
func TestT562_EveryAssertedClientInterfaceHoldsOnTheLiveClients(t *testing.T) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo,
		Dir:  filepath.Join("..", ".."),
	}
	pkgs, err := packages.Load(cfg, pagingGuardAWSPkg)
	if err != nil {
		t.Fatalf("load %s: %v", pagingGuardAWSPkg, err)
	}
	if len(pkgs) != 1 || len(pkgs[0].Errors) > 0 {
		t.Fatalf("load %s: %d packages, errors %v", pagingGuardAWSPkg, len(pkgs), pkgs[0].Errors)
	}
	pkg := pkgs[0]

	live := reflect.ValueOf(awsclient.CreateServiceClients(aws.Config{
		Region:      "us-east-1",
		Credentials: credentials.NewStaticCredentialsProvider("AKIAIOSFODNN7EXAMPLE", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", ""),
	})).Elem()

	// An assertion is matched by the static type of what it asserts on, so
	// clients.S3.(X), c.InRegion(r).S3.(X) and s3For(ctx, b).(X) all count.
	clientFields := map[string]types.Type{}
	sc := pkg.Types.Scope().Lookup("ServiceClients").Type().Underlying().(*types.Struct)
	for i := range sc.NumFields() {
		if f := sc.Field(i); f.Exported() && types.IsInterface(f.Type()) {
			clientFields[f.Name()] = f.Type()
		}
	}

	type pair struct{ field, iface string }
	sites := map[pair][]string{}
	ifaces := map[pair]types.Type{}
	for _, file := range pkg.Syntax {
		path := pkg.Fset.Position(file.Pos()).Filename
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		ast.Inspect(file, func(n ast.Node) bool {
			ta, ok := n.(*ast.TypeAssertExpr)
			if !ok || ta.Type == nil {
				return true
			}
			field := t562FieldOfType(clientFields, pkg.TypesInfo.TypeOf(ta.X))
			if field == "" {
				return true
			}
			typ := pkg.TypesInfo.TypeOf(ta.Type)
			p := pair{field, types.TypeString(typ, types.RelativeTo(pkg.Types))}
			ifaces[p] = typ
			pos := pkg.Fset.Position(ta.Pos())
			sites[p] = append(sites[p], filepath.Base(pos.Filename)+":"+strconv.Itoa(pos.Line))
			return true
		})
	}
	if len(sites) < 50 {
		t.Fatalf("found %d (field, interface) assertions on ServiceClients fields, want the ~90 core/aws makes; the scan lost its target", len(sites))
	}

	var failing []string
	for p, at := range sites {
		iface, ok := ifaces[p].Underlying().(*types.Interface)
		if !ok {
			t.Fatalf("%s is asserted off clients.%s but is not an interface", p.iface, p.field)
		}
		fv := live.FieldByName(p.field)
		if !fv.IsValid() || fv.IsNil() {
			failing = append(failing, p.field+"/"+p.iface+" (field unset on the live clients) at "+strings.Join(at, ", "))
			continue
		}
		dyn := fv.Elem().Type()
		var missing []string
		for i := range iface.NumMethods() {
			if _, has := dyn.MethodByName(iface.Method(i).Name()); !has {
				missing = append(missing, iface.Method(i).Name())
			}
		}
		if len(missing) > 0 {
			failing = append(failing, p.field+"/"+p.iface+": "+dyn.String()+" lacks "+strings.Join(missing, ", ")+" at "+strings.Join(at, ", "))
		}
	}
	sort.Strings(failing)
	for _, f := range failing {
		t.Errorf("%s", f)
	}
}

func t562FieldOfType(fields map[string]types.Type, typ types.Type) string {
	if typ == nil {
		return ""
	}
	for name, ft := range fields {
		if types.Identical(ft, typ) {
			return name
		}
	}
	return ""
}
