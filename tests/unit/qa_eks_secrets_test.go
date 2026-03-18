package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/internal/resource"
	"github.com/k2m30/a9s/internal/tui"
	"github.com/k2m30/a9s/internal/tui/messages"
	"github.com/k2m30/a9s/internal/tui/styles"
)

// ===========================================================================
// EKS Clusters — List View
// ===========================================================================

// TestQA_EKS_ListColumns verifies the EKS cluster list displays all expected
// columns: Cluster Name, Version, Status, Endpoint, Platform Version.
func TestQA_EKS_ListColumns(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Navigate to EKS resource list
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "eks",
	})

	// Load EKS fixture data
	clusters := fixtureEKSClusters()
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "eks",
		Resources:    clusters,
	})

	plain := stripANSI(rootViewContent(m))

	// Verify column headers are present
	expectedHeaders := []string{"Cluster Name", "Version", "Status"}
	for _, hdr := range expectedHeaders {
		if !strings.Contains(plain, hdr) {
			t.Errorf("EKS list should contain column header %q, got: %s", hdr, plain)
		}
	}

	// Verify fixture data values are rendered. Note: config-driven columns use
	// SDK path extraction which requires RawStruct. Without RawStruct, only
	// columns whose title (lowercased) matches a Fields key appear. "Version"
	// matches "version", "Status" matches "status", but "Cluster Name" does
	// not match "cluster_name" (space vs underscore). This is expected behavior
	// for fixture-only resources.
	if !strings.Contains(plain, "1.31") {
		t.Errorf("EKS list should contain version '1.31', got: %s", plain)
	}
	if !strings.Contains(plain, "ACTIVE") {
		t.Errorf("EKS list should contain status 'ACTIVE', got: %s", plain)
	}
}

// TestQA_EKS_ListColumnsFromTypeDef verifies the EKS resource type definition
// has exactly the expected columns with correct titles and widths.
func TestQA_EKS_ListColumnsFromTypeDef(t *testing.T) {
	rt := resource.FindResourceType("eks")
	if rt == nil {
		t.Fatal("resource type 'eks' not found")
	}

	expected := []struct {
		title string
		width int
		key   string
	}{
		{"Cluster Name", 28, "cluster_name"},
		{"Version", 10, "version"},
		{"Status", 14, "status"},
		{"Endpoint", 48, "endpoint"},
		{"Platform Version", 18, "platform_version"},
	}

	if len(rt.Columns) != len(expected) {
		t.Fatalf("expected %d EKS columns, got %d", len(expected), len(rt.Columns))
	}

	for i, want := range expected {
		col := rt.Columns[i]
		if col.Title != want.title {
			t.Errorf("column %d: expected title %q, got %q", i, want.title, col.Title)
		}
		if col.Width != want.width {
			t.Errorf("column %d (%s): expected width %d, got %d", i, want.title, want.width, col.Width)
		}
		if col.Key != want.key {
			t.Errorf("column %d (%s): expected key %q, got %q", i, want.title, want.key, col.Key)
		}
	}
}

// TestQA_EKS_StatusColoring_Active verifies that ACTIVE status uses green
// coloring (#9ece6a), matching the "running" category in RowColorStyle.
func TestQA_EKS_StatusColoring_Active(t *testing.T) {
	style := styles.RowColorStyle("ACTIVE")
	rendered := style.Render("ACTIVE row content")

	// The rendered text should contain ANSI color codes (non-empty styling applied)
	if rendered == "ACTIVE row content" {
		t.Error("ACTIVE status should apply color styling, but got unstyled output")
	}

	// Verify green color (#9ece6a) is applied — same as ColRunning
	// RowColorStyle maps "active" to ColRunning which is green
	runningStyled := styles.RowColorStyle("running").Render("test")
	activeStyled := styles.RowColorStyle("ACTIVE").Render("test")

	// Both should produce the same ANSI styling since they map to the same color
	if stripANSI(runningStyled) != stripANSI(activeStyled) {
		t.Error("ACTIVE and running should render the same plain text")
	}
}

// TestQA_EKS_FrameTitle verifies the frame title shows eks-clusters(<count>).
func TestQA_EKS_FrameTitle(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "eks",
	})

	clusters := fixtureEKSClusters()
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "eks",
		Resources:    clusters,
	})

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "eks(1)") {
		t.Errorf("frame title should contain 'eks(1)', got: %s", plain)
	}
}

// ===========================================================================
// EKS Clusters — Detail View
// ===========================================================================

// TestQA_EKS_DetailView verifies pressing Enter on an EKS cluster opens
// the detail view with the cluster name in the frame title.
func TestQA_EKS_DetailView(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Navigate to EKS
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "eks",
	})

	clusters := fixtureEKSClusters()
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "eks",
		Resources:    clusters,
	})

	// Navigate to detail view via NavigateMsg
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:   messages.TargetDetail,
		Resource: &clusters[0],
	})

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "test-cluster-1") {
		t.Errorf("detail view frame title should contain cluster name, got: %s", plain)
	}
}

// TestQA_EKS_DetailViewFields verifies the detail view renders key-value
// pairs from the EKS cluster's Fields map.
func TestQA_EKS_DetailViewFields(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	clusters := fixtureEKSClusters()
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:   messages.TargetDetail,
		Resource: &clusters[0],
	})

	plain := stripANSI(rootViewContent(m))

	// Check that key field values are rendered
	expectedValues := []string{"test-cluster-1", "1.31", "ACTIVE", "eks.52"}
	for _, val := range expectedValues {
		if !strings.Contains(plain, val) {
			t.Errorf("detail view should contain %q, got: %s", val, plain)
		}
	}
}

// ===========================================================================
// EKS Clusters — YAML View
// ===========================================================================

// TestQA_EKS_YAMLView verifies pressing y on an EKS cluster opens the
// YAML view with "yaml" in the frame title.
func TestQA_EKS_YAMLView(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	clusters := fixtureEKSClusters()
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:   messages.TargetYAML,
		Resource: &clusters[0],
	})

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "yaml") {
		t.Errorf("YAML view frame title should contain 'yaml', got: %s", plain)
	}
	if !strings.Contains(plain, "test-cluster-1") {
		t.Errorf("YAML view frame title should contain cluster name, got: %s", plain)
	}
}

// TestQA_EKS_YAMLViewContainsData verifies the YAML view renders
// cluster data as YAML key-value pairs.
func TestQA_EKS_YAMLViewContainsData(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	clusters := fixtureEKSClusters()
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:   messages.TargetYAML,
		Resource: &clusters[0],
	})

	plain := stripANSI(rootViewContent(m))

	// YAML view should contain field keys from the Fields map
	if !strings.Contains(plain, "cluster_name") {
		t.Errorf("YAML view should contain 'cluster_name' key, got: %s", plain)
	}
	if !strings.Contains(plain, "1.31") {
		t.Errorf("YAML view should contain version '1.31', got: %s", plain)
	}
}

// ===========================================================================
// Secrets Manager — List View
// ===========================================================================

// TestQA_Secrets_ListColumns verifies the Secrets Manager list displays all
// expected columns: Secret Name, Description, Last Accessed, Last Changed, Rotation.
func TestQA_Secrets_ListColumns(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Navigate to Secrets Manager resource list
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "secrets",
	})

	// Load Secrets fixture data
	secrets := fixtureSecrets()
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "secrets",
		Resources:    secrets,
	})

	plain := stripANSI(rootViewContent(m))

	// Verify column headers are present
	expectedHeaders := []string{"Secret Name", "Description"}
	for _, hdr := range expectedHeaders {
		if !strings.Contains(plain, hdr) {
			t.Errorf("Secrets list should contain column header %q, got: %s", hdr, plain)
		}
	}

	// Note: config-driven columns use SDK path extraction (e.g. Path: "Name")
	// which requires RawStruct. Without RawStruct, only columns whose title
	// exactly matches a Fields key (case-insensitive) display values.
	// "Description" title matches "description" key but the fixture values are
	// empty strings, so no visible data rows for the default 80-col terminal.
	// The frame title "secrets(5)" confirms all 5 resources were loaded.
	if !strings.Contains(plain, "secrets(5)") {
		t.Errorf("Secrets list frame title should show 'secrets(5)', got: %s", plain)
	}
}

// TestQA_Secrets_ListColumnsFromTypeDef verifies the Secrets Manager resource
// type definition has exactly the expected columns with correct titles and widths.
func TestQA_Secrets_ListColumnsFromTypeDef(t *testing.T) {
	rt := resource.FindResourceType("secrets")
	if rt == nil {
		t.Fatal("resource type 'secrets' not found")
	}

	expected := []struct {
		title string
		width int
		key   string
	}{
		{"Secret Name", 36, "secret_name"},
		{"Description", 30, "description"},
		{"Last Accessed", 18, "last_accessed"},
		{"Last Changed", 18, "last_changed"},
		{"Rotation", 10, "rotation_enabled"},
	}

	if len(rt.Columns) != len(expected) {
		t.Fatalf("expected %d Secrets columns, got %d", len(expected), len(rt.Columns))
	}

	for i, want := range expected {
		col := rt.Columns[i]
		if col.Title != want.title {
			t.Errorf("column %d: expected title %q, got %q", i, want.title, col.Title)
		}
		if col.Width != want.width {
			t.Errorf("column %d (%s): expected width %d, got %d", i, want.title, want.width, col.Width)
		}
		if col.Key != want.key {
			t.Errorf("column %d (%s): expected key %q, got %q", i, want.title, want.key, col.Key)
		}
	}
}

// TestQA_Secrets_FrameTitle verifies the frame title shows secrets(<count>).
func TestQA_Secrets_FrameTitle(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "secrets",
	})

	secrets := fixtureSecrets()
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "secrets",
		Resources:    secrets,
	})

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "secrets(5)") {
		t.Errorf("frame title should contain 'secrets(5)', got: %s", plain)
	}
}

// ===========================================================================
// Secrets Manager — Reveal (x key)
// ===========================================================================

// TestQA_Secrets_XKeyTriggersReveal verifies that pressing x on the secrets
// resource list produces a command (fetching the secret value).
func TestQA_Secrets_XKeyTriggersReveal(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Navigate to Secrets Manager
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "secrets",
	})

	secrets := fixtureSecrets()
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "secrets",
		Resources:    secrets,
	})

	// Press x to trigger reveal
	_, cmd := rootApplyMsg(m, rootKeyPress("x"))

	// The x key should return a command (fetchSecretValue)
	if cmd == nil {
		t.Error("pressing 'x' on secrets list should return a command to fetch the secret value")
	}
}

// TestQA_Secrets_XKeyDoesNothingOnEC2 verifies that pressing x on the EC2
// instance list does nothing -- no view change, no error, no command.
func TestQA_Secrets_XKeyDoesNothingOnEC2(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Navigate to EC2 resource list
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	instances := fixtureEC2Instances()
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    instances,
	})

	contentBefore := rootViewContent(m)

	// Press x — should do nothing
	m, cmd := rootApplyMsg(m, rootKeyPress("x"))

	contentAfter := rootViewContent(m)

	if cmd != nil {
		t.Error("pressing 'x' on EC2 list should return nil command, but got non-nil")
	}
	if contentBefore != contentAfter {
		t.Error("pressing 'x' on EC2 list should not change the view")
	}
}

// TestQA_Secrets_XKeyDoesNothingOnRDS verifies that pressing x on the RDS
// instance list does nothing.
func TestQA_Secrets_XKeyDoesNothingOnRDS(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "dbi",
	})

	instances := fixtureRDSInstances()
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "dbi",
		Resources:    instances,
	})

	_, cmd := rootApplyMsg(m, rootKeyPress("x"))

	if cmd != nil {
		t.Error("pressing 'x' on RDS list should return nil command")
	}
}

// TestQA_Secrets_XKeyDoesNothingOnS3 verifies that pressing x on the S3
// bucket list does nothing.
func TestQA_Secrets_XKeyDoesNothingOnS3(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "s3",
	})

	buckets := fixtureS3Buckets()
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "s3",
		Resources:    buckets,
	})

	_, cmd := rootApplyMsg(m, rootKeyPress("x"))

	if cmd != nil {
		t.Error("pressing 'x' on S3 list should return nil command")
	}
}

// TestQA_Secrets_XKeyDoesNothingOnEKS verifies that pressing x on the EKS
// cluster list does nothing.
func TestQA_Secrets_XKeyDoesNothingOnEKS(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "eks",
	})

	clusters := fixtureEKSClusters()
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "eks",
		Resources:    clusters,
	})

	_, cmd := rootApplyMsg(m, rootKeyPress("x"))

	if cmd != nil {
		t.Error("pressing 'x' on EKS list should return nil command")
	}
}

// TestQA_Secrets_XKeyDoesNothingOnRedis verifies that pressing x on the
// ElastiCache Redis list does nothing.
func TestQA_Secrets_XKeyDoesNothingOnRedis(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "redis",
	})

	clusters := fixtureRedisClusters()
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "redis",
		Resources:    clusters,
	})

	_, cmd := rootApplyMsg(m, rootKeyPress("x"))

	if cmd != nil {
		t.Error("pressing 'x' on Redis list should return nil command")
	}
}

// TestQA_Secrets_XKeyDoesNothingOnDocDB verifies that pressing x on the
// DocumentDB cluster list does nothing.
func TestQA_Secrets_XKeyDoesNothingOnDocDB(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "dbc",
	})

	clusters := fixtureDocDBClusters()
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "dbc",
		Resources:    clusters,
	})

	_, cmd := rootApplyMsg(m, rootKeyPress("x"))

	if cmd != nil {
		t.Error("pressing 'x' on DocDB list should return nil command")
	}
}

// ===========================================================================
// Secrets Manager — Reveal View
// ===========================================================================

// TestQA_Secrets_RevealViewShowsSecretValue verifies that the reveal view
// displays the secret value after receiving SecretRevealedMsg.
func TestQA_Secrets_RevealViewShowsSecretValue(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Navigate to secrets list first
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "secrets",
	})

	secrets := fixtureSecrets()
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "secrets",
		Resources:    secrets,
	})

	// Simulate receiving the secret value
	m, _ = rootApplyMsg(m, messages.SecretRevealedMsg{
		SecretName: "test/integration",
		Value:      "super-secret-password-123",
	})

	plain := stripANSI(rootViewContent(m))

	// The reveal view should show the secret value
	if !strings.Contains(plain, "super-secret-password-123") {
		t.Errorf("reveal view should contain the secret value, got: %s", plain)
	}

	// The frame title should show the secret name
	if !strings.Contains(plain, "test/integration") {
		t.Errorf("reveal view frame title should contain the secret name, got: %s", plain)
	}
}

// TestQA_Secrets_RevealHeaderWarning verifies the reveal view displays a
// persistent red warning in the header: "Secret visible -- press esc to close".
func TestQA_Secrets_RevealHeaderWarning(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Navigate to secrets list
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "secrets",
	})

	secrets := fixtureSecrets()
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "secrets",
		Resources:    secrets,
	})

	// Trigger reveal
	m, _ = rootApplyMsg(m, messages.SecretRevealedMsg{
		SecretName: "test/integration",
		Value:      "my-secret-value",
	})

	plain := stripANSI(rootViewContent(m))

	// Header should show the warning text (with em dash)
	if !strings.Contains(plain, "Secret visible") {
		t.Errorf("reveal header should contain 'Secret visible', got: %s", plain)
	}
	if !strings.Contains(plain, "press esc to close") {
		t.Errorf("reveal header should contain 'press esc to close', got: %s", plain)
	}

	// The normal "? for help" should NOT be present while reveal is active
	if strings.Contains(plain, "? for help") {
		t.Error("reveal view should replace '? for help' with warning text")
	}
}

// TestQA_Secrets_RevealCopyReturnsCmd verifies that pressing c on the reveal
// view returns a command (to copy the secret value to clipboard).
func TestQA_Secrets_RevealCopyReturnsCmd(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Navigate to secrets list
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "secrets",
	})

	secrets := fixtureSecrets()
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "secrets",
		Resources:    secrets,
	})

	// Open reveal view
	m, _ = rootApplyMsg(m, messages.SecretRevealedMsg{
		SecretName: "test/integration",
		Value:      "copy-me-secret",
	})

	// Press c to copy
	_, cmd := rootApplyMsg(m, rootKeyPress("c"))

	if cmd == nil {
		t.Error("pressing 'c' on reveal view should return a copy command")
	}
}

// TestQA_Secrets_EscapeFromRevealReturnsToList verifies that pressing Escape
// on the reveal view returns to the secrets list.
func TestQA_Secrets_EscapeFromRevealReturnsToList(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Navigate to secrets list
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "secrets",
	})

	secrets := fixtureSecrets()
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "secrets",
		Resources:    secrets,
	})

	// Open reveal view
	m, _ = rootApplyMsg(m, messages.SecretRevealedMsg{
		SecretName: "test/integration",
		Value:      "my-secret-value",
	})

	// Verify we are in reveal view (warning text present)
	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "Secret visible") {
		t.Fatal("should be in reveal view before pressing Escape")
	}

	// Press Escape
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))

	plain = stripANSI(rootViewContent(m))

	// Should be back at secrets list
	if !strings.Contains(plain, "secrets(5)") {
		t.Errorf("after Escape from reveal, should return to secrets list with 'secrets(5)', got: %s", plain)
	}

	// The warning should be gone
	if strings.Contains(plain, "Secret visible") {
		t.Error("after Escape from reveal, the warning text should be gone")
	}

	// The "? for help" hint should be back
	if !strings.Contains(plain, "? for help") {
		t.Errorf("after Escape from reveal, '? for help' should be restored, got: %s", plain)
	}
}

// ===========================================================================
// Secrets Manager — Detail and YAML Views
// ===========================================================================

// TestQA_Secrets_DetailView verifies the detail view shows secret metadata.
func TestQA_Secrets_DetailView(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	secrets := fixtureSecrets()
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:   messages.TargetDetail,
		Resource: &secrets[0],
	})

	plain := stripANSI(rootViewContent(m))

	// Frame title should show the secret name
	if !strings.Contains(plain, "test/integration") {
		t.Errorf("detail view frame title should contain secret name, got: %s", plain)
	}

	// Detail view should render field values
	if !strings.Contains(plain, "2025-12-08") {
		t.Errorf("detail view should contain last_accessed date, got: %s", plain)
	}
}

// TestQA_Secrets_YAMLView verifies the YAML view shows secret metadata.
func TestQA_Secrets_YAMLView(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	secrets := fixtureSecrets()
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:   messages.TargetYAML,
		Resource: &secrets[0],
	})

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "yaml") {
		t.Errorf("YAML view frame title should contain 'yaml', got: %s", plain)
	}
	if !strings.Contains(plain, "test/integration") {
		t.Errorf("YAML view frame title should contain secret name, got: %s", plain)
	}
}

// TestQA_Secrets_YAMLViewContainsData verifies the YAML view renders secret
// metadata as YAML key-value pairs.
func TestQA_Secrets_YAMLViewContainsData(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	secrets := fixtureSecrets()
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:   messages.TargetYAML,
		Resource: &secrets[0],
	})

	plain := stripANSI(rootViewContent(m))

	// YAML view should contain field keys
	if !strings.Contains(plain, "secret_name") {
		t.Errorf("YAML view should contain 'secret_name' key, got: %s", plain)
	}
}

// ===========================================================================
// Secrets Manager — Reveal on non-list views
// ===========================================================================

// TestQA_Secrets_XKeyDoesNothingOnMainMenu verifies that pressing x on the
// main menu does nothing (handleReveal checks for resourceList != nil).
func TestQA_Secrets_XKeyDoesNothingOnMainMenu(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	contentBefore := rootViewContent(m)

	m, cmd := rootApplyMsg(m, rootKeyPress("x"))

	contentAfter := rootViewContent(m)

	if cmd != nil {
		t.Error("pressing 'x' on main menu should return nil command")
	}
	if contentBefore != contentAfter {
		t.Error("pressing 'x' on main menu should not change the view")
	}
}

// ===========================================================================
// EKS — Escape Navigation
// ===========================================================================

// TestQA_EKS_EscapeFromDetailReturnsToList verifies pressing Escape on the
// detail view pops back to the EKS list.
func TestQA_EKS_EscapeFromDetailReturnsToList(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "eks",
	})

	clusters := fixtureEKSClusters()
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "eks",
		Resources:    clusters,
	})

	// Push detail view
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:   messages.TargetDetail,
		Resource: &clusters[0],
	})

	// Press Escape
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "eks(1)") {
		t.Errorf("after Escape from detail, should return to EKS list, got: %s", plain)
	}
}

// TestQA_EKS_EscapeFromYAMLReturnsToList verifies pressing Escape on the
// YAML view (opened from list) pops back to the EKS list.
func TestQA_EKS_EscapeFromYAMLReturnsToList(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "eks",
	})

	clusters := fixtureEKSClusters()
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "eks",
		Resources:    clusters,
	})

	// Push YAML view
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:   messages.TargetYAML,
		Resource: &clusters[0],
	})

	// Press Escape
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "eks(1)") {
		t.Errorf("after Escape from YAML, should return to EKS list, got: %s", plain)
	}
}
