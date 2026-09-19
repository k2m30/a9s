package unit_test

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/iam"

	awspkg "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/internal/tui"

	tea "charm.land/bubbletea/v2"
)

func TestModel_HasAppContext(t *testing.T) {
	t.Parallel()
	m := newBlessedModel(t, "", "")
	ctx := m.AppContext()
	if ctx == nil {
		t.Fatal("AppContext() returned nil — Model must own an app-level context")
	}
	if err := ctx.Err(); err != nil {
		t.Fatalf("AppContext().Err() = %v, want nil (context should be live on construction)", err)
	}
}

func TestModel_QuitCancelsAppContext(t *testing.T) {
	t.Parallel()
	m := newBlessedModel(t, "", "")

	// Capture AppContext before the Update so we can observe the same context object.
	ctx := m.AppContext()
	if ctx == nil {
		t.Fatal("AppContext() returned nil before quit")
	}

	updated, _ := m.Update(tea.QuitMsg{})
	m2, ok := updated.(tui.Model)
	if !ok {
		t.Fatalf("Update returned type %T, want tui.Model", updated)
	}

	// Either the original ctx was cancelled, or the model's new context is cancelled.
	// Both are acceptable signals that the quit path cancels the app context.
	afterCtx := m2.AppContext()
	if afterCtx == nil {
		t.Fatal("AppContext() returned nil after quit")
	}

	if ctx.Err() != context.Canceled && afterCtx.Err() != context.Canceled {
		t.Fatalf(
			"neither the pre-quit context (err=%v) nor the post-quit context (err=%v) "+
				"is context.Canceled — QuitMsg must cancel the app context",
			ctx.Err(), afterCtx.Err(),
		)
	}
}

func TestFetchersUseContextNotBackground(t *testing.T) {
	t.Parallel()
	files := []string{
		"core/runtime/fetchers.go",
		"internal/tui/fetch_adapter.go",
		"internal/tui/runtime_adapter_related.go",
		"core/aws/iam_policies_related.go",
		"core/aws/iam_roles_related.go",
		"core/aws/iam_users_related.go",
		"core/aws/iam_groups_related.go",
		"core/aws/client.go",
	}

	root := findModuleRoot(t)

	pattern := regexp.MustCompile(`context\.Background\(\)`)
	totalViolations := 0
	var report strings.Builder

	for _, rel := range files {
		full := filepath.Join(root, rel)
		data, err := os.ReadFile(full)
		if err != nil {
			t.Errorf("cannot read %s: %v", rel, err)
			continue
		}
		matches := pattern.FindAllIndex(data, -1)
		if len(matches) > 0 {
			totalViolations += len(matches)
			report.WriteString("\n  ")
			report.WriteString(rel)
			report.WriteString(": ")
			report.WriteString(strings.Repeat("context.Background() ", len(matches)))
			report.WriteString("(")
			report.WriteString(itoa(len(matches)))
			report.WriteString(" occurrence(s))")
		}
	}

	if totalViolations > 0 {
		t.Errorf(
			"found %d context.Background() call(s) in fetch/related files — "+
				"all fetchers must use a derived context from the app context, not context.Background():%s",
			totalViolations, report.String(),
		)
	}
}

func findModuleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find go.mod starting from %s", dir)
		}
		dir = parent
	}
}

// itoa converts a small non-negative int to its decimal string without importing strconv
// at the package level (keeps the import list clean).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := [20]byte{}
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[pos:])
}

// cancelObservingIAMClient implements IAMListEntitiesForPolicyAPI and records
// the context it was called with.
type cancelObservingIAMClient struct {
	gotCtx context.Context
	calls  int
}

func (c *cancelObservingIAMClient) ListEntitiesForPolicy(
	ctx context.Context,
	_ *iam.ListEntitiesForPolicyInput,
	_ ...func(*iam.Options),
) (*iam.ListEntitiesForPolicyOutput, error) {
	c.gotCtx = ctx
	c.calls++
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &iam.ListEntitiesForPolicyOutput{}, nil
}

func TestIAMRelatedChecker_RespectsCancelledContext_PolicyRole(t *testing.T) {
	testPolicyCheckerRespectsCancelledContext(t, "role")
}

func TestIAMRelatedChecker_RespectsCancelledContext_PolicyUser(t *testing.T) {
	testPolicyCheckerRespectsCancelledContext(t, "iam-user")
}

func TestIAMRelatedChecker_RespectsCancelledContext_PolicyGroup(t *testing.T) {
	testPolicyCheckerRespectsCancelledContext(t, "iam-group")
}

// testPolicyCheckerRespectsCancelledContext is the shared body for all three
// policy-checker cancellation tests. It installs cancelObservingIAMClient via the
// test seam, then calls the checker with a pre-cancelled context.
func testPolicyCheckerRespectsCancelledContext(t *testing.T, targetType string) {
	t.Helper()

	mock := &cancelObservingIAMClient{}
	restore := awspkg.SetIAMListEntitiesAPIForTest(mock)
	defer restore()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res := resource.Resource{
		ID:   "arn:aws:iam::111122223333:policy/test-policy",
		Name: "test-policy",
		Fields: map[string]string{
			"arn": "arn:aws:iam::111122223333:policy/test-policy",
		},
	}

	checker := checkerByTarget(t, "policy", targetType)
	result := checker(ctx, &awspkg.ServiceClients{}, res, resource.ResourceCache{})

	if mock.calls > 0 {
		if mock.gotCtx == nil {
			t.Fatalf("policy→%s: mock was called but gotCtx is nil", targetType)
		}
		if mock.gotCtx.Err() != context.Canceled {
			t.Errorf(
				"policy→%s: mock received context with Err()=%v, want context.Canceled — "+
					"checker is calling context.Background() instead of threading ctx",
				targetType, mock.gotCtx.Err(),
			)
		}
	}

	// Whether or not the mock was called, the result must indicate failure
	// (State: RelatedError) because the context was cancelled before anything
	// could succeed — checkPolicyRole/User/Group all route a non-nil API error
	// (including a propagated context.Canceled) through resource.ErrorRelated.
	if result.State() != domain.RelatedError {
		t.Errorf(
			"policy→%s: State=%v after pre-cancelled ctx, want RelatedError — "+
				"checker must propagate context cancellation as an error",
			targetType, result.State(),
		)
	}
}

// Role, user and group checkers have no injectable IAM client, so these tests
// pass nil clients: the nil-clients guard answers unknown (-1) before any API
// call.

func TestIAMRelatedChecker_RespectsCancelledContext_RolePolicy(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res := resource.Resource{ID: "my-role", Name: "my-role"}

	checker := checkerByTarget(t, "role", "policy")
	result := checker(ctx, nil, res, resource.ResourceCache{})
	if result.State() != domain.RelatedUnknown {
		t.Errorf("role→policy with nil clients: Count=%d, want -1", result.Count())
	}
}

func TestIAMRelatedChecker_RespectsCancelledContext_UserGroup(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res := resource.Resource{ID: "alice", Name: "alice"}
	checker := checkerByTarget(t, "iam-user", "iam-group")
	result := checker(ctx, nil, res, resource.ResourceCache{})
	if result.State() != domain.RelatedUnknown {
		t.Errorf("iam-user→iam-group with nil clients: Count=%d, want -1", result.Count())
	}
}

func TestIAMRelatedChecker_RespectsCancelledContext_UserPolicy(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res := resource.Resource{ID: "alice", Name: "alice"}
	checker := checkerByTarget(t, "iam-user", "policy")
	result := checker(ctx, nil, res, resource.ResourceCache{})
	if result.State() != domain.RelatedUnknown {
		t.Errorf("iam-user→policy with nil clients: Count=%d, want -1", result.Count())
	}
}

func TestIAMRelatedChecker_RespectsCancelledContext_GroupUser(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res := resource.Resource{ID: "eng-team", Name: "eng-team"}
	checker := checkerByTarget(t, "iam-group", "iam-user")
	result := checker(ctx, nil, res, resource.ResourceCache{})
	if result.State() != domain.RelatedUnknown {
		t.Errorf("iam-group→iam-user with nil clients: Count=%d, want -1", result.Count())
	}
}

func TestIAMRelatedChecker_RespectsCancelledContext_GroupPolicy(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res := resource.Resource{ID: "eng-team", Name: "eng-team"}
	checker := checkerByTarget(t, "iam-group", "policy")
	result := checker(ctx, nil, res, resource.ResourceCache{})
	if result.State() != domain.RelatedUnknown {
		t.Errorf("iam-group→policy with nil clients: Count=%d, want -1", result.Count())
	}
}

func TestModel_Cancel_ZeroValue_DoesNotPanic(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Cancel() on zero-value Model panicked: %v", r)
		}
	}()
	var m tui.Model
	m.Cancel()
}

func TestModel_Cancel_CancelsAppContext(t *testing.T) {
	t.Parallel()
	m := newBlessedModel(t, "", "")

	ctx := m.AppContext()
	if ctx == nil {
		t.Fatal("AppContext() returned nil before Cancel()")
	}

	m.Cancel()

	select {
	case <-ctx.Done():
	default:
		t.Error("AppContext().Done() not closed after Cancel() — appCancel was not invoked")
	}
}
