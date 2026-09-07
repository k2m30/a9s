package unit

// cites_wave1_row_owner_test.go pins who owns a Wave-1 finding's supporting
// rows. addWave1Rows keys the rows by a FindingCode, and the detail view reads
// them back by the code of the finding it is rendering, so rows filed under a
// code the resource does not carry are stored and never shown. The way that
// happens is deriving the owning code a second time, beside the predicate that
// already decided it: the two derivations agree until a branch is added to one
// of them.

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
)

// TestCitesWave1RowsCodeIsNotDerivedTwice walks every addWave1Rows call in
// core/aws and requires the owning code to be named, not computed. A constant
// or the code of the finding that built the rows is a name; a call expression
// is a second derivation of a fact the finding predicate has already decided,
// and nothing keeps the two in step.
func TestCitesWave1RowsCodeIsNotDerivedTwice(t *testing.T) {
	dir := filepath.Join(projectRoot(t), "core", "aws")
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, nil, 0)
	if err != nil {
		t.Fatalf("cannot parse core/aws: %v", err)
	}

	var offenders []string
	for _, pkg := range pkgs {
		for name, file := range pkg.Files {
			if strings.HasSuffix(name, "wave1_rows.go") {
				continue
			}
			ast.Inspect(file, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				ident, ok := call.Fun.(*ast.Ident)
				if !ok || ident.Name != "addWave1Rows" || len(call.Args) < 2 {
					return true
				}
				if _, derived := call.Args[1].(*ast.CallExpr); derived {
					pos := fset.Position(call.Args[1].Pos())
					offenders = append(offenders, filepath.Base(pos.Filename)+":"+
						strings.TrimPrefix(pos.String(), pos.Filename+":"))
				}
				return true
			})
		}
	}
	sort.Strings(offenders)

	if len(offenders) > 0 {
		t.Errorf("%d addWave1Rows call(s) compute the owning code instead of naming it: %v. "+
			"Pass the code of the finding that built the rows, and delete the helper that derives it — "+
			"one predicate decides which finding a condition raises, and the rows belong to that finding",
			len(offenders), offenders)
	}
}

// eksClusterWithIssues drives one cluster through the real fetcher and returns
// the resource it built.
func eksClusterWithIssues(t *testing.T, status ekstypes.ClusterStatus, issueCodes ...ekstypes.NodegroupIssueCode) ([]domain.Finding, map[domain.FindingCode]domain.AttentionDetail) {
	t.Helper()

	issues := make([]ekstypes.ClusterIssue, 0, len(issueCodes))
	for _, code := range issueCodes {
		issues = append(issues, ekstypes.ClusterIssue{
			Code:        ekstypes.ClusterIssueCode(code),
			Message:     aws.String("reported by AWS"),
			ResourceIds: []string{"subnet-0aaa111111111111a"},
		})
	}

	const name = "acme-cites"
	clients := &awsclient.ServiceClients{EKS: newMockEKSFull(
		&mockEKSListClustersClient{output: &eks.ListClustersOutput{Clusters: []string{name}}},
		&mockEKSDescribeClusterClient{outputs: map[string]*eks.DescribeClusterOutput{
			name: {Cluster: &ekstypes.Cluster{
				Name:            aws.String(name),
				Arn:             aws.String("arn:aws:eks:us-east-1:123456789012:cluster/" + name),
				Version:         aws.String("1.29"),
				Status:          status,
				Endpoint:        aws.String("https://ABCDEF1234567890.gr7.us-east-1.eks.amazonaws.com"),
				PlatformVersion: aws.String("eks.5"),
				Health:          &ekstypes.ClusterHealth{Issues: issues},
			}},
		}},
		nil, nil,
	)}

	result, err := awsclient.FetchEKSClustersPage(context.Background(), clients, "")
	if err != nil {
		t.Fatalf("status %s: %v", status, err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("status %s: expected one cluster, got %d", status, len(result.Resources))
	}
	return result.Resources[0].Findings, result.Resources[0].AttentionDetails
}

// TestCitesEksIssueRowsBelongToTheirFinding drives every cluster status the
// fetcher accepts, each with reported health issues, and requires the rows AWS
// reported to sit under a finding the cluster carries. A row under any other
// code is written and never read: buildAttentionEntries looks rows up by the
// code of the finding it is rendering.
func TestCitesEksIssueRowsBelongToTheirFinding(t *testing.T) {
	statuses := []ekstypes.ClusterStatus{
		ekstypes.ClusterStatusActive,
		ekstypes.ClusterStatusCreating,
		ekstypes.ClusterStatusUpdating,
		ekstypes.ClusterStatusDeleting,
		ekstypes.ClusterStatusPending,
		ekstypes.ClusterStatusFailed,
	}

	withRows := 0
	for _, status := range statuses {
		findings, details := eksClusterWithIssues(t, status, "InsufficientFreeAddresses", "AccessDenied")
		if len(details) == 0 {
			continue
		}
		withRows++

		carried := map[domain.FindingCode]bool{}
		var names []string
		for _, f := range findings {
			carried[f.Code] = true
			names = append(names, string(f.Code))
		}
		sort.Strings(names)

		for code, detail := range details {
			if !carried[code] {
				t.Errorf("status %s: %d issue row(s) are filed under %q, which this cluster carries no finding for. "+
					"The detail view reads rows by the code of the finding it renders, so these are never shown. "+
					"Findings on the cluster: %v", status, len(detail.Rows), code, names)
			}
		}
	}

	if withRows == 0 {
		t.Error("no cluster status produced issue rows, so this pin checked nothing — " +
			"the health issues AWS reported are being dropped before they reach the resource")
	}
}
