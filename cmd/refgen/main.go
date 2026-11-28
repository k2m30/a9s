package main

import (
	"fmt"
	"path"
	"reflect"
	"slices"
	"strings"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/fieldpath"
)

func main() {
	awsclient.Install()

	fmt.Println("# views_reference.yaml")
	fmt.Println("# Generated from AWS SDK Go v2 struct reflection")
	fmt.Println("# Use these paths in your ~/.a9s/views/ configuration files")
	fmt.Println()

	children := slices.SortedFunc(slices.Values(catalog.AllChildren()), func(a, b catalog.ResourceTypeDef) int {
		return strings.Compare(a.ShortName, b.ShortName)
	})
	for _, td := range slices.Concat(catalog.All(), children) {
		var paths, names []string
		for _, rt := range td.RowTypes {
			names = append(names, typeName(rt))
			for _, p := range structPaths(rt) {
				if !slices.Contains(paths, p) {
					paths = append(paths, p)
				}
			}
		}
		fmt.Printf("%s:  # %s\n", td.ShortName, strings.Join(names, ", "))
		for _, p := range paths {
			fmt.Printf("  - %s\n", p)
		}
		fmt.Println()
	}
}

// typeName names rt with the service its package belongs to, so the docdb
// and rds DBCluster of one type read apart: "docdb/types.DBCluster".
func typeName(rt reflect.Type) string {
	for rt.Kind() == reflect.Pointer {
		rt = rt.Elem()
	}
	return path.Base(path.Dir(rt.PkgPath())) + "/" + path.Base(rt.PkgPath()) + "." + rt.Name()
}

// structPaths is every path of rt, a field promoted through an anonymous
// embed named as the promotion reads it ("Name", not "WorkGroupSummary.Name"):
// fieldpath.ExtractValue resolves both, and the wrapper's name is a9s's own.
func structPaths(rt reflect.Type, active ...reflect.Type) []string {
	for rt.Kind() == reflect.Pointer {
		rt = rt.Elem()
	}
	var promoted, own []string
	embedded := map[string]bool{}
	for f := range rt.Fields() {
		if et := f.Type; f.Anonymous && strings.Split(f.Tag.Get("json"), ",")[0] == "" {
			for et.Kind() == reflect.Pointer {
				et = et.Elem()
			}
			if et.Kind() == reflect.Struct && et != rt && !slices.Contains(active, et) {
				embedded[f.Name] = true
				promoted = append(promoted, structPaths(et, slices.Concat(active, []reflect.Type{rt})...)...)
			}
		}
	}
	for _, p := range fieldpath.EnumeratePaths(rt, "") {
		first, _, _ := strings.Cut(p, ".")
		if !embedded[strings.TrimSuffix(first, "[]")] {
			own = append(own, p)
		}
	}
	return append(own, promoted...)
}
