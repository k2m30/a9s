package unit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
)

// The account and Region exposure settings are read by one owner, once per
// account and Region; an enricher that calls one of them itself is a second
// reading the owner's cache and failure marks never see.
var t4AccountSettingCalls = []string{
	"GetSnapshotBlockPublicAccessState",
	"DescribeVpcBlockPublicAccessOptions",
	"DescribeVpcBlockPublicAccessExclusions",
	"GetEbsEncryptionByDefault",
}

// S3 Control GetPublicAccessBlock shares its method name with the bucket-level
// call, so it is recognised by its input type.
const t4S3ControlImport = "github.com/aws/aws-sdk-go-v2/service/s3control"

func TestT4_AccountSettingReadsHaveOneOwner(t *testing.T) {
	dir := filepath.Join("..", "..", "core", "aws")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	readers := map[string][]string{}
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		s3control := ""
		for _, imp := range f.Imports {
			if path, _ := strconv.Unquote(imp.Path.Value); path == t4S3ControlImport {
				s3control = "s3control"
				if imp.Name != nil {
					s3control = imp.Name.Name
				}
			}
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				sel, ok := n.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				read := ""
				if x, ok := sel.X.(*ast.Ident); ok && s3control != "" && x.Name == s3control && sel.Sel.Name == "GetPublicAccessBlockInput" {
					read = "s3control.GetPublicAccessBlock"
				}
				if slices.Contains(t4AccountSettingCalls, sel.Sel.Name) {
					read = sel.Sel.Name
				}
				if read != "" && !slices.Contains(readers[read], name) {
					readers[read] = append(readers[read], name)
				}
				return true
			})
		}
	}

	var owners []string
	for _, read := range append([]string{"s3control.GetPublicAccessBlock"}, t4AccountSettingCalls...) {
		files := readers[read]
		if len(files) == 0 {
			t.Errorf("no function in core/aws reads %s", read)
		}
		for _, f := range files {
			if !slices.Contains(owners, f) {
				owners = append(owners, f)
			}
		}
	}
	if len(owners) > 1 {
		t.Errorf("account settings are read in %d files %v (per read: %v), want one owner", len(owners), owners, readers)
	}
}

func TestT4_LT_EBSEncryptionByDefaultIsReadOncePerRegion(t *testing.T) {
	const off = "lt-0a1b2c3d4e5f6t401"
	version := t581LTVersion(off, t581AMIDefault, &ec2types.LaunchTemplateInstanceMetadataOptions{HttpTokens: ec2types.LaunchTemplateHttpTokensStateRequired})
	version.LaunchTemplateData.BlockDeviceMappings = []ec2types.LaunchTemplateBlockDeviceMapping{t581UnencryptedRoot()}
	fake := &t581EBSDefaultEC2{t581LTEC2Fake: &t581LTEC2Fake{
		templates: []ec2types.LaunchTemplate{t581LT(off, "acme-"+off)},
		versions:  map[string]ec2types.LaunchTemplateVersion{off: version},
	}}
	clients := &awsclient.ServiceClients{EC2: fake}
	for range 2 {
		rows, _ := t581LoadAndEnrich(t, "lt", clients, t581LoadedAMICache())
		if !hasCode(t581Row(t, rows, off).Findings, "lt.warn.unencrypted") {
			t.Errorf("%s findings = %v, want lt.warn.unencrypted: the Region default is off", off, t581Row(t, rows, off).Findings)
		}
	}
	if fake.calls != 1 {
		t.Errorf("GetEbsEncryptionByDefault calls = %d over two passes, want 1", fake.calls)
	}
}
