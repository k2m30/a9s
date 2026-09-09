// Command readonlycheck is the AST-based read-only gate for a9s.
//
// a9s never issues write API calls to AWS. This checker loads core/aws/...
// and core/runtime/... with full type information and flags every call
// expression whose selector name matches a write-verb prefix. Because it
// operates on the parsed AST rather than source text, comments, strings,
// and formatting are structurally irrelevant — the whole class of lexical
// bypasses (dot-newline calls, block comments splitting a call, verbs
// embedded in string literals or comments, interface method declarations
// that merely look like calls) is eliminated by construction rather than
// special-cased.
package main

import (
	"fmt"
	"go/ast"
	"go/types"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

const sdkServicePkgPrefix = "github.com/aws/aws-sdk-go-v2/service/"

// writeVerbRe matches a write-verb prefix followed by the rest of a
// camel-case AWS API method name (e.g. DeleteBucket, PutObject).
var writeVerbRe = regexp.MustCompile(`^(?:Create|Delete|Update|Put|Modify|Terminate|Stop|Reboot|Execute|Send|Publish|Remove|Start|Cancel|Attach|Detach|Associate|Disassociate|Register|Deregister|Enable|Disable|Restore|Invoke|Revoke|Authorize)[A-Z][A-Za-z0-9]*$`)

// exemptNames are exact method-name matches for local helpers that are not
// AWS API calls despite matching writeVerbRe (see scripts/verify-readonly.sh
// history for why each one exists).
var exemptNames = map[string]bool{
	"CreateServiceClients": true,
	"ExecuteTaskAt":        true,
	// Session-state mutators (in-memory maps on session.Session), moved into
	// scanned core/runtime, where RefreshListEnrichment owns the list-refresh
	// mutation list.
	"DeleteEnrichmentRan":          true,
	"DeleteEnrichmentTruncatedIDs": true,
}

func isWriteVerbCall(name string) bool {
	return name == "RunInstances" || writeVerbRe.MatchString(name)
}

// excludedFile mirrors the file-level exclusions in scripts/verify-readonly.sh:
// interface declaration files (SDK method signatures look like calls) and the
// handful of files that construct/describe clients rather than call them.
func excludedFile(path string) bool {
	base := filepath.Base(path)
	if strings.HasSuffix(base, "interfaces.go") {
		return true
	}
	switch base {
	case "errors.go", "client.go", "profile.go", "regions.go":
		return true
	}
	return false
}

// isSDKReceiver reports whether the value selected on (sel.X) has a named
// type declared in a github.com/aws/aws-sdk-go-v2/service/* package. This is
// the cheap, definitely-a-write-call case: no exemption can apply to it.
func isSDKReceiver(info *types.Info, x ast.Expr) bool {
	tv, ok := info.Types[x]
	if !ok || tv.Type == nil {
		return false
	}
	t := tv.Type
	if ptr, isPtr := t.(*types.Pointer); isPtr {
		t = ptr.Elem()
	}
	named, ok := t.(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	if obj == nil || obj.Pkg() == nil {
		return false
	}
	return strings.HasPrefix(obj.Pkg().Path(), sdkServicePkgPrefix)
}

func main() {
	hits, err := run()
	if err != nil {
		fmt.Fprintln(os.Stderr, "readonlycheck:", err)
		os.Exit(1)
	}
	if len(hits) > 0 {
		sort.Strings(hits)
		for _, h := range hits {
			fmt.Println(h)
		}
		fmt.Println("FAIL: Write API calls detected!")
		os.Exit(1)
	}
	fmt.Println("PASS: All API calls are read-only")
}

func run() ([]string, error) {
	cfg := &packages.Config{
		Mode: packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedFiles,
	}
	pkgs, err := packages.Load(cfg, "./core/aws/...", "./core/runtime/...")
	if err != nil {
		return nil, fmt.Errorf("load packages: %w", err)
	}
	if n := packages.PrintErrors(pkgs); n > 0 {
		return nil, fmt.Errorf("%d package error(s)", n)
	}

	var hits []string
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			pos := pkg.Fset.Position(file.Pos())
			if excludedFile(pos.Filename) {
				continue
			}
			ast.Inspect(file, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				name := sel.Sel.Name
				if !isWriteVerbCall(name) {
					return true
				}
				if !isSDKReceiver(pkg.TypesInfo, sel.X) && exemptNames[name] {
					return true
				}
				callPos := pkg.Fset.Position(call.Pos())
				hits = append(hits, fmt.Sprintf("%s:%d: %s", callPos.Filename, callPos.Line, name))
				return true
			})
		}
	}
	return hits, nil
}
