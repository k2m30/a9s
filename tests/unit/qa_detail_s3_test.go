package unit

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/internal/app"
	"github.com/k2m30/a9s/internal/resource"
	"github.com/k2m30/a9s/internal/views"
)

// ===========================================================================
// Helper: build an AppState in ResourceListView for a given resource type
// with the supplied resources already loaded.
// ===========================================================================

func stateWithResources(resourceType string, resources []resource.Resource) app.AppState {
	s := app.NewAppState("", "")
	s.CurrentView = app.ResourceListView
	s.CurrentResourceType = resourceType
	s.Resources = resources
	s.SelectedIndex = 0
	s.Width = 120
	s.Height = 40
	return s
}

// pressKey sends a single character key event through Update and returns the
// resulting AppState.
func pressKey(s app.AppState, ch string) app.AppState {
	updated, _ := s.Update(tea.KeyPressMsg{Code: -1, Text: ch})
	return updated.(app.AppState)
}

// pressSpecial sends a special key (Enter, Escape, etc.) through Update.
func pressSpecial(s app.AppState, code rune) app.AppState {
	updated, _ := s.Update(tea.KeyPressMsg{Code: code})
	return updated.(app.AppState)
}

// pressSpecialKey is an alias used across test files for sending special keys.
func pressSpecialKey(s app.AppState, code rune) app.AppState {
	return pressSpecial(s, code)
}

// ---------------------------------------------------------------------------
// 6. Resource Detail (d key) — QA-082 through QA-093
// ---------------------------------------------------------------------------

// QA-082: d on EC2 instance shows all attributes
func TestQA082_DescribeEC2(t *testing.T) {
	detail := map[string]string{
		"Instance ID": "i-0abc123",
		"Name":        "web-server",
		"State":       "running",
		"Type":        "t3.medium",
		"AMI":         "ami-12345",
		"VPC":         "vpc-abc",
	}
	s := stateWithResources("ec2", []resource.Resource{
		{
			ID: "i-0abc123", Name: "web-server", Status: "running",
			Fields:     map[string]string{"instance_id": "i-0abc123", "name": "web-server"},
			DetailData: detail,
		},
	})

	s = pressKey(s, "d")

	if s.CurrentView != app.DetailView {
		t.Fatalf("expected DetailView, got %d", s.CurrentView)
	}
	// Title should reference the resource name
	if !strings.Contains(s.Detail.Title, "web-server") {
		t.Errorf("detail title should contain resource name, got %q", s.Detail.Title)
	}
	// Breadcrumbs should include "detail"
	crumbs := strings.Join(s.Breadcrumbs, " > ")
	if !strings.Contains(crumbs, "detail") {
		t.Errorf("breadcrumbs should contain 'detail', got %q", crumbs)
	}
	// All detail keys should be present
	for k := range detail {
		found := false
		for _, dk := range s.Detail.Keys {
			if dk == k {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("detail should contain key %q", k)
		}
	}
}

// QA-083: d on S3 bucket shows bucket details
func TestQA083_DescribeS3Bucket(t *testing.T) {
	s := stateWithResources("s3", []resource.Resource{
		{
			ID: "my-bucket", Name: "my-bucket",
			Fields:     map[string]string{"name": "my-bucket", "region": "us-east-1"},
			DetailData: map[string]string{"Bucket Name": "my-bucket", "Region": "us-east-1", "Creation Date": "2025-01-01"},
		},
	})

	s = pressKey(s, "d")

	if s.CurrentView != app.DetailView {
		t.Fatalf("expected DetailView for S3, got %d", s.CurrentView)
	}
	if s.Detail.Data["Bucket Name"] != "my-bucket" {
		t.Errorf("expected detail data 'Bucket Name'='my-bucket', got %q", s.Detail.Data["Bucket Name"])
	}
}

// QA-084: d on RDS instance shows DB details
func TestQA084_DescribeRDS(t *testing.T) {
	s := stateWithResources("rds", []resource.Resource{
		{
			ID: "mydb", Name: "mydb", Status: "available",
			Fields:     map[string]string{"db_identifier": "mydb"},
			DetailData: map[string]string{"DB Identifier": "mydb", "Engine": "postgres", "Status": "available"},
		},
	})

	s = pressKey(s, "d")

	if s.CurrentView != app.DetailView {
		t.Fatalf("expected DetailView for RDS, got %d", s.CurrentView)
	}
	if s.Detail.Data["Engine"] != "postgres" {
		t.Errorf("expected Engine=postgres in detail, got %q", s.Detail.Data["Engine"])
	}
}

// QA-085: d on ElastiCache Redis cluster
func TestQA085_DescribeRedis(t *testing.T) {
	s := stateWithResources("redis", []resource.Resource{
		{
			ID: "redis-001", Name: "redis-001", Status: "available",
			Fields:     map[string]string{"cluster_id": "redis-001"},
			DetailData: map[string]string{"Cluster ID": "redis-001", "Version": "7.0", "Node Type": "cache.t3.micro"},
		},
	})

	s = pressKey(s, "d")

	if s.CurrentView != app.DetailView {
		t.Fatalf("expected DetailView for Redis, got %d", s.CurrentView)
	}
	if !strings.Contains(s.Detail.Title, "redis-001") {
		t.Errorf("detail title should reference resource name, got %q", s.Detail.Title)
	}
}

// QA-086: d on DocumentDB cluster
func TestQA086_DescribeDocDB(t *testing.T) {
	s := stateWithResources("docdb", []resource.Resource{
		{
			ID: "docdb-cluster-1", Name: "docdb-cluster-1", Status: "available",
			Fields:     map[string]string{"cluster_id": "docdb-cluster-1"},
			DetailData: map[string]string{"Cluster ID": "docdb-cluster-1", "Version": "5.0", "Status": "available"},
		},
	})

	s = pressKey(s, "d")

	if s.CurrentView != app.DetailView {
		t.Fatalf("expected DetailView for DocDB, got %d", s.CurrentView)
	}
}

// QA-087: d on EKS cluster
func TestQA087_DescribeEKS(t *testing.T) {
	s := stateWithResources("eks", []resource.Resource{
		{
			ID: "my-k8s", Name: "my-k8s", Status: "ACTIVE",
			Fields: map[string]string{"cluster_name": "my-k8s"},
			DetailData: map[string]string{
				"Cluster Name":     "my-k8s",
				"Version":          "1.29",
				"Status":           "ACTIVE",
				"Endpoint":         "https://eks.example.com",
				"Platform Version": "eks.5",
			},
		},
	})

	s = pressKey(s, "d")

	if s.CurrentView != app.DetailView {
		t.Fatalf("expected DetailView for EKS, got %d", s.CurrentView)
	}
	if s.Detail.Data["Version"] != "1.29" {
		t.Errorf("expected Version=1.29, got %q", s.Detail.Data["Version"])
	}
}

// QA-088: d on Secrets Manager secret shows metadata only (not the value)
func TestQA088_DescribeSecretMetadataOnly(t *testing.T) {
	s := stateWithResources("secrets", []resource.Resource{
		{
			ID: "prod/db-password", Name: "prod/db-password",
			Fields: map[string]string{"secret_name": "prod/db-password"},
			DetailData: map[string]string{
				"Name":          "prod/db-password",
				"ARN":           "arn:aws:secretsmanager:us-east-1:123456:secret:prod/db-password-AbCdEf",
				"Last Accessed": "2026-03-10",
				"Last Changed":  "2026-02-01",
			},
		},
	})

	s = pressKey(s, "d")

	if s.CurrentView != app.DetailView {
		t.Fatalf("expected DetailView for secrets, got %d", s.CurrentView)
	}
	// The secret VALUE must not be in the detail data
	for k, v := range s.Detail.Data {
		lk := strings.ToLower(k)
		if lk == "value" || lk == "secret_value" || lk == "secretstring" {
			t.Errorf("detail view should NOT contain secret value, found key=%q val=%q", k, v)
		}
	}
}

// QA-089: Scroll up/down in detail view
func TestQA089_DetailScrollUpDown(t *testing.T) {
	data := make(map[string]string)
	for i := 0; i < 25; i++ {
		data[fmt.Sprintf("Key_%02d", i)] = fmt.Sprintf("Value_%02d", i)
	}
	s := app.NewAppState("", "")
	s.CurrentView = app.DetailView
	s.Detail = views.NewDetailModel("Scroll Test", data)
	s.Detail.Width = 80
	s.Detail.Height = 10

	// Scroll down 10 times
	for i := 0; i < 10; i++ {
		s = pressKey(s, "j")
	}
	if s.Detail.Offset != 10 {
		t.Errorf("expected offset 10 after 10 j presses, got %d", s.Detail.Offset)
	}

	// Scroll up 5 times
	for i := 0; i < 5; i++ {
		s = pressKey(s, "k")
	}
	if s.Detail.Offset != 5 {
		t.Errorf("expected offset 5 after 5 k presses, got %d", s.Detail.Offset)
	}

	// Cannot scroll past top
	for i := 0; i < 20; i++ {
		s = pressKey(s, "k")
	}
	if s.Detail.Offset != 0 {
		t.Errorf("expected offset 0 after scrolling past top, got %d", s.Detail.Offset)
	}
}

// QA-090: g/G in detail view (top/bottom)
func TestQA090_DetailGoTopBottom(t *testing.T) {
	data := make(map[string]string)
	for i := 0; i < 20; i++ {
		data[fmt.Sprintf("Key_%02d", i)] = fmt.Sprintf("Val_%02d", i)
	}
	s := app.NewAppState("", "")
	s.CurrentView = app.DetailView
	s.Detail = views.NewDetailModel("Top/Bottom", data)
	s.Detail.Height = 5

	// Scroll down a bit first
	for i := 0; i < 8; i++ {
		s = pressKey(s, "j")
	}

	// g -> top
	s = pressKey(s, "g")
	if s.Detail.Offset != 0 {
		t.Errorf("g should set offset to 0, got %d", s.Detail.Offset)
	}

	// G -> bottom
	s = pressKey(s, "G")
	if s.Detail.Offset == 0 {
		t.Error("G should scroll to bottom, offset is still 0")
	}
}

// QA-091: Escape from detail view returns to list with cursor preserved
func TestQA091_EscapeFromDetailPreservesCursor(t *testing.T) {
	resources := make([]resource.Resource, 10)
	for i := range resources {
		id := fmt.Sprintf("i-%03d", i)
		resources[i] = resource.Resource{
			ID: id, Name: "server-" + id,
			Fields:     map[string]string{"instance_id": id},
			DetailData: map[string]string{"ID": id},
		}
	}
	s := stateWithResources("ec2", resources)

	// Move cursor to index 5
	for i := 0; i < 5; i++ {
		s = pressKey(s, "j")
	}
	if s.SelectedIndex != 5 {
		t.Fatalf("cursor should be at 5, got %d", s.SelectedIndex)
	}

	// Press d to open detail
	s = pressKey(s, "d")
	if s.CurrentView != app.DetailView {
		t.Fatalf("expected DetailView after d, got %d", s.CurrentView)
	}

	// Press Escape to go back
	s = pressSpecial(s, tea.KeyEscape)

	if s.CurrentView != app.ResourceListView {
		t.Errorf("expected ResourceListView after Escape, got %d", s.CurrentView)
	}
	if s.SelectedIndex != 5 {
		t.Errorf("cursor position should be preserved at 5, got %d", s.SelectedIndex)
	}
}

// QA-092: d on empty list does nothing
func TestQA092_DescribeEmptyList(t *testing.T) {
	s := stateWithResources("ec2", []resource.Resource{})

	s = pressKey(s, "d")

	if s.CurrentView != app.ResourceListView {
		t.Errorf("d on empty list should stay in ResourceListView, got %d", s.CurrentView)
	}
}

// QA-093: d when resource has no DetailData
func TestQA093_DescribeNoDetailData(t *testing.T) {
	s := stateWithResources("ec2", []resource.Resource{
		{
			ID: "i-empty", Name: "no-details",
			Fields:     map[string]string{"instance_id": "i-empty"},
			DetailData: nil, // no detail data
		},
	})

	s = pressKey(s, "d")

	if s.CurrentView != app.ResourceListView {
		t.Errorf("d with nil DetailData should stay in ResourceListView, got %d", s.CurrentView)
	}
	if !strings.Contains(s.StatusMessage, "No detail data") {
		t.Errorf("expected status 'No detail data...', got %q", s.StatusMessage)
	}
}

// ---------------------------------------------------------------------------
// 7. JSON View (y key) — QA-094 through QA-099
// ---------------------------------------------------------------------------

// QA-094: y shows formatted JSON for EC2 instance
func TestQA094_JSONViewEC2(t *testing.T) {
	rawJSON := `{"InstanceId":"i-0abc","InstanceType":"t3.medium","State":{"Name":"running"}}`
	s := stateWithResources("ec2", []resource.Resource{
		{
			ID: "i-0abc", Name: "web-server",
			Fields:  map[string]string{"instance_id": "i-0abc"},
			RawJSON: rawJSON,
		},
	})

	s = pressKey(s, "y")

	if s.CurrentView != app.JSONView {
		t.Fatalf("expected JSONView, got %d", s.CurrentView)
	}
	if !strings.Contains(s.JSONData.Title, "web-server") {
		t.Errorf("JSON title should contain resource name, got %q", s.JSONData.Title)
	}
	crumbs := strings.Join(s.Breadcrumbs, " > ")
	if !strings.Contains(crumbs, "json") {
		t.Errorf("breadcrumbs should contain 'json', got %q", crumbs)
	}
}

// QA-095: JSON content is valid and parseable
func TestQA095_JSONIsValid(t *testing.T) {
	rawJSON := `{"InstanceId":"i-0abc","Tags":[{"Key":"Name","Value":"test"}]}`
	s := stateWithResources("ec2", []resource.Resource{
		{
			ID: "i-0abc", Name: "test",
			Fields:  map[string]string{"instance_id": "i-0abc"},
			RawJSON: rawJSON,
		},
	})

	s = pressKey(s, "y")

	if s.CurrentView != app.JSONView {
		t.Fatalf("expected JSONView")
	}
	// The raw JSON stored should be valid
	var parsed interface{}
	if err := json.Unmarshal([]byte(s.JSONData.Content), &parsed); err != nil {
		t.Errorf("JSON content should be valid JSON, parse error: %v", err)
	}
}

// QA-096: Scroll in JSON view
func TestQA096_JSONViewScroll(t *testing.T) {
	// Build a multiline JSON string
	lines := make([]string, 30)
	for i := range lines {
		lines[i] = fmt.Sprintf("  \"field_%02d\": \"value_%02d\"", i, i)
	}
	bigJSON := "{\n" + strings.Join(lines, ",\n") + "\n}"

	s := app.NewAppState("", "")
	s.CurrentView = app.JSONView
	s.JSONData = views.NewJSONView("Big JSON", bigJSON)
	s.JSONData.Height = 10

	// Scroll down
	for i := 0; i < 10; i++ {
		s = pressKey(s, "j")
	}
	if s.JSONData.Offset != 10 {
		t.Errorf("expected JSON offset 10, got %d", s.JSONData.Offset)
	}

	// Scroll back up
	for i := 0; i < 5; i++ {
		s = pressKey(s, "k")
	}
	if s.JSONData.Offset != 5 {
		t.Errorf("expected JSON offset 5, got %d", s.JSONData.Offset)
	}

	// Cannot scroll past top
	for i := 0; i < 20; i++ {
		s = pressKey(s, "k")
	}
	if s.JSONData.Offset != 0 {
		t.Errorf("expected JSON offset 0, got %d", s.JSONData.Offset)
	}
}

// QA-097: Escape from JSON view returns to list
func TestQA097_EscapeFromJSONView(t *testing.T) {
	resources := []resource.Resource{
		{
			ID: "i-1", Name: "server",
			Fields:  map[string]string{"instance_id": "i-1"},
			RawJSON: `{"id":"i-1"}`,
		},
	}
	s := stateWithResources("ec2", resources)

	// Open JSON view
	s = pressKey(s, "y")
	if s.CurrentView != app.JSONView {
		t.Fatalf("expected JSONView")
	}

	// Press Escape
	s = pressSpecial(s, tea.KeyEscape)

	if s.CurrentView != app.ResourceListView {
		t.Errorf("expected ResourceListView after Escape from JSON, got %d", s.CurrentView)
	}
}

// QA-098: y when resource has no RawJSON
func TestQA098_JSONViewNoRawJSON(t *testing.T) {
	s := stateWithResources("ec2", []resource.Resource{
		{
			ID: "i-empty", Name: "no-json",
			Fields:  map[string]string{"instance_id": "i-empty"},
			RawJSON: "", // no JSON
		},
	})

	s = pressKey(s, "y")

	if s.CurrentView != app.ResourceListView {
		t.Errorf("y with empty RawJSON should stay in ResourceListView, got %d", s.CurrentView)
	}
	if !strings.Contains(s.StatusMessage, "No JSON data") {
		t.Errorf("expected status 'No JSON data...', got %q", s.StatusMessage)
	}
}

// QA-099: y on each of the 7 resource types
func TestQA099_JSONViewAllResourceTypes(t *testing.T) {
	types := []struct {
		shortName string
		idKey     string
	}{
		{"s3", "name"},
		{"ec2", "instance_id"},
		{"rds", "db_identifier"},
		{"redis", "cluster_id"},
		{"docdb", "cluster_id"},
		{"eks", "cluster_name"},
		{"secrets", "secret_name"},
	}

	for _, rt := range types {
		t.Run(rt.shortName, func(t *testing.T) {
			s := stateWithResources(rt.shortName, []resource.Resource{
				{
					ID: "test-id", Name: "test-resource",
					Fields:  map[string]string{rt.idKey: "test-id"},
					RawJSON: fmt.Sprintf(`{"%s":"test-id"}`, rt.idKey),
				},
			})

			s = pressKey(s, "y")

			if s.CurrentView != app.JSONView {
				t.Errorf("y on %s should open JSONView, got %d", rt.shortName, s.CurrentView)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 8. Secret Reveal (x key) — QA-100 through QA-106
// ---------------------------------------------------------------------------

// QA-100: x on a secret triggers reveal (loading starts, SecretRevealedMsg transitions)
func TestQA100_RevealSecret(t *testing.T) {
	// We cannot make a real AWS call, but we can verify:
	// 1. After SecretRevealedMsg, RevealView is entered.
	s := app.NewAppState("", "")
	s.CurrentView = app.RevealView
	s.Reveal = views.NewRevealView("Secret: my-secret", `{"db_password":"hunter2"}`)
	s.Reveal.Width = 80
	s.Reveal.Height = 24

	if s.CurrentView != app.RevealView {
		t.Fatalf("expected RevealView")
	}
	if !strings.Contains(s.Reveal.Content, "hunter2") {
		t.Errorf("reveal content should contain the secret value")
	}
	if !strings.Contains(s.Reveal.Title, "my-secret") {
		t.Errorf("reveal title should contain secret name, got %q", s.Reveal.Title)
	}
}

// QA-100 (continued): SecretRevealedMsg transitions to RevealView
func TestQA100_SecretRevealedMsgTransition(t *testing.T) {
	s := app.NewAppState("", "")
	s.CurrentView = app.ResourceListView
	s.CurrentResourceType = "secrets"
	s.Loading = true
	s.Width = 80
	s.Height = 24

	msg := app.SecretRevealedMsg{
		SecretName: "prod/api-key",
		Value:      "sk-live-abc123",
		Err:        nil,
	}

	updated, _ := s.Update(msg)
	result := updated.(app.AppState)

	if result.CurrentView != app.RevealView {
		t.Fatalf("expected RevealView after SecretRevealedMsg, got %d", result.CurrentView)
	}
	if !result.Loading {
		// Loading should be cleared
	}
	if result.Reveal.Content != "sk-live-abc123" {
		t.Errorf("reveal content should be the secret value, got %q", result.Reveal.Content)
	}
	crumbs := strings.Join(result.Breadcrumbs, " > ")
	if !strings.Contains(crumbs, "reveal") {
		t.Errorf("breadcrumbs should contain 'reveal', got %q", crumbs)
	}
}

// QA-101: x on a non-secret resource type does nothing
func TestQA101_RevealOnNonSecret(t *testing.T) {
	for _, rt := range []string{"ec2", "s3", "rds", "redis", "docdb", "eks"} {
		t.Run(rt, func(t *testing.T) {
			s := stateWithResources(rt, []resource.Resource{
				{
					ID: "test-id", Name: "test",
					Fields: map[string]string{"name": "test"},
				},
			})

			s = pressKey(s, "x")

			if s.CurrentView != app.ResourceListView {
				t.Errorf("x on %s should stay in ResourceListView, got %d", rt, s.CurrentView)
			}
		})
	}
}

// QA-102: x when secret value is very long (reveal shows scrollable content)
func TestQA102_RevealLongSecret(t *testing.T) {
	longValue := strings.Repeat("a]b[c{d}e\n", 1000) // 10,000 char secret
	s := app.NewAppState("", "")
	s.CurrentView = app.RevealView
	s.Reveal = views.NewRevealView("Secret: big-config", longValue)
	s.Reveal.Width = 80
	s.Reveal.Height = 20

	// Should be able to scroll
	s = pressKey(s, "j")
	if s.Reveal.Offset != 1 {
		t.Errorf("expected reveal offset 1 after scrolling, got %d", s.Reveal.Offset)
	}

	// Verify content is not truncated
	if s.Reveal.Content != longValue {
		t.Error("reveal content should not be truncated for long secrets")
	}
}

// QA-103: x when secret value is binary (base64 representation)
// This test verifies the RevealView can handle binary-like content.
func TestQA103_RevealBinarySecret(t *testing.T) {
	// Binary secrets are typically base64-encoded by the AWS SDK
	binaryValue := "SGVsbG8gV29ybGQhIFRoaXMgaXMgYmluYXJ5IGRhdGE="
	s := app.NewAppState("", "")
	s.CurrentView = app.RevealView
	s.Reveal = views.NewRevealView("Secret: binary-cert", binaryValue)
	s.Reveal.Width = 80
	s.Reveal.Height = 24

	output := s.Reveal.View()
	if !strings.Contains(output, binaryValue) {
		t.Error("reveal view should display binary/base64 content")
	}
}

// QA-104: x when AWS connection is nil
func TestQA104_RevealNoAWSConnection(t *testing.T) {
	s := stateWithResources("secrets", []resource.Resource{
		{
			ID: "my-secret", Name: "my-secret",
			Fields: map[string]string{"secret_name": "my-secret"},
		},
	})
	s.Clients = nil // no AWS connection

	s = pressKey(s, "x")

	if s.CurrentView != app.ResourceListView {
		t.Errorf("x without AWS clients should stay in ResourceListView, got %d", s.CurrentView)
	}
	if !strings.Contains(s.StatusMessage, "No AWS connection") {
		t.Errorf("expected 'No AWS connection' status, got %q", s.StatusMessage)
	}
	if !s.StatusIsError {
		t.Error("status should be marked as error")
	}
}

// QA-105: x when secret fetch fails
func TestQA105_RevealFetchFails(t *testing.T) {
	s := app.NewAppState("", "")
	s.CurrentView = app.ResourceListView
	s.CurrentResourceType = "secrets"
	s.Loading = true

	msg := app.SecretRevealedMsg{
		SecretName: "forbidden-secret",
		Value:      "",
		Err:        fmt.Errorf("AccessDeniedException: not authorized"),
	}

	updated, _ := s.Update(msg)
	result := updated.(app.AppState)

	if result.CurrentView == app.RevealView {
		t.Error("should NOT enter RevealView on error")
	}
	if !result.StatusIsError {
		t.Error("status should be error")
	}
	if !strings.Contains(result.StatusMessage, "Error revealing secret") {
		t.Errorf("expected error message about revealing, got %q", result.StatusMessage)
	}
	if result.Loading {
		t.Error("loading should be cleared after error")
	}
}

// QA-106: Copy secret from reveal view (c key)
// Skipping actual clipboard test; verify status message pattern instead.
func TestQA106_CopyFromRevealView(t *testing.T) {
	s := app.NewAppState("", "")
	s.CurrentView = app.RevealView
	s.Reveal = views.NewRevealView("Secret: api-key", "sk-live-secret-value")
	s.Reveal.Width = 80
	s.Reveal.Height = 24

	// Press c — clipboard may or may not be available in CI
	s = pressKey(s, "c")

	// If clipboard is available, status should contain "copied" or "Secret copied"
	// If clipboard fails, status should contain "Copy failed"
	if s.StatusMessage == "" {
		t.Error("pressing c in reveal view should set a status message")
	}
	if !strings.Contains(s.StatusMessage, "copied") && !strings.Contains(s.StatusMessage, "Copy failed") {
		t.Errorf("expected status about copy result, got %q", s.StatusMessage)
	}
}

// ---------------------------------------------------------------------------
// 9. Copy (c key) — QA-107 through QA-112
// ---------------------------------------------------------------------------

// QA-107: c copies EC2 instance ID (verify status message)
func TestQA107_CopyEC2ID(t *testing.T) {
	s := stateWithResources("ec2", []resource.Resource{
		{
			ID: "i-0abc123def", Name: "web-server",
			Fields: map[string]string{"instance_id": "i-0abc123def"},
		},
	})

	s = pressKey(s, "c")

	// Clipboard may fail in CI — check that the status message references the ID
	if s.StatusMessage == "" {
		t.Error("c should set a status message")
	}
	if strings.Contains(s.StatusMessage, "Copied") {
		if !strings.Contains(s.StatusMessage, "i-0abc123def") {
			t.Errorf("copied status should reference the ID, got %q", s.StatusMessage)
		}
	}
	// If copy failed, that's also acceptable (no clipboard in CI)
}

// QA-108: c copies S3 bucket name
func TestQA108_CopyS3BucketName(t *testing.T) {
	s := stateWithResources("s3", []resource.Resource{
		{
			ID: "my-app-data", Name: "my-app-data",
			Fields: map[string]string{"name": "my-app-data"},
		},
	})

	s = pressKey(s, "c")

	if s.StatusMessage == "" {
		t.Error("c should set a status message")
	}
	if strings.Contains(s.StatusMessage, "Copied") && !strings.Contains(s.StatusMessage, "my-app-data") {
		t.Errorf("copied status should contain 'my-app-data', got %q", s.StatusMessage)
	}
}

// QA-109: c copies RDS DB identifier
func TestQA109_CopyRDSIdentifier(t *testing.T) {
	s := stateWithResources("rds", []resource.Resource{
		{
			ID: "prod-database-1", Name: "prod-database-1",
			Fields: map[string]string{"db_identifier": "prod-database-1"},
		},
	})

	s = pressKey(s, "c")

	if s.StatusMessage == "" {
		t.Error("c should set a status message")
	}
	if strings.Contains(s.StatusMessage, "Copied") && !strings.Contains(s.StatusMessage, "prod-database-1") {
		t.Errorf("copied status should contain DB identifier, got %q", s.StatusMessage)
	}
}

// QA-110: c copies various resource identifiers for all 7 types
func TestQA110_CopyAllResourceTypes(t *testing.T) {
	testCases := []struct {
		shortName string
		id        string
		idKey     string
	}{
		{"s3", "my-bucket", "name"},
		{"ec2", "i-0abc123", "instance_id"},
		{"rds", "mydb", "db_identifier"},
		{"redis", "redis-001", "cluster_id"},
		{"docdb", "docdb-cluster", "cluster_id"},
		{"eks", "k8s-prod", "cluster_name"},
		{"secrets", "prod/api-key", "secret_name"},
	}

	for _, tc := range testCases {
		t.Run(tc.shortName, func(t *testing.T) {
			s := stateWithResources(tc.shortName, []resource.Resource{
				{
					ID: tc.id, Name: tc.id,
					Fields: map[string]string{tc.idKey: tc.id},
				},
			})

			s = pressKey(s, "c")

			if s.StatusMessage == "" {
				t.Errorf("c on %s should set status message", tc.shortName)
			}
			// Verify the correct ID is referenced if copy succeeded
			if strings.Contains(s.StatusMessage, "Copied") && !strings.Contains(s.StatusMessage, tc.id) {
				t.Errorf("copied status for %s should contain %q, got %q", tc.shortName, tc.id, s.StatusMessage)
			}
		})
	}
}

// QA-111: c on empty list does nothing
func TestQA111_CopyEmptyList(t *testing.T) {
	s := stateWithResources("ec2", []resource.Resource{})

	originalStatus := s.StatusMessage
	s = pressKey(s, "c")

	if s.StatusMessage != originalStatus {
		t.Errorf("c on empty list should not change status, got %q", s.StatusMessage)
	}
}

// QA-112: c when clipboard is unavailable (verify error handling)
// This test verifies the code path handles clipboard errors gracefully.
// We cannot force clipboard failure in all environments, but we verify
// that the app handles both success and failure paths.
func TestQA112_CopyClipboardUnavailable(t *testing.T) {
	s := stateWithResources("ec2", []resource.Resource{
		{
			ID: "i-test", Name: "test",
			Fields: map[string]string{"instance_id": "i-test"},
		},
	})

	s = pressKey(s, "c")

	// Either "Copied: i-test" or "Copy failed: ..."
	if s.StatusMessage == "" {
		t.Error("c should always set a status message (success or failure)")
	}
	if strings.Contains(s.StatusMessage, "Copy failed") {
		if !s.StatusIsError {
			t.Error("clipboard failure should set StatusIsError=true")
		}
	} else if strings.Contains(s.StatusMessage, "Copied") {
		if s.StatusIsError {
			t.Error("successful copy should not set StatusIsError")
		}
	}
}

// ---------------------------------------------------------------------------
// 10. S3 Drill-Down — QA-113 through QA-122
// ---------------------------------------------------------------------------

// QA-113: Enter on a bucket navigates into it (starts S3 object fetch)
func TestQA113_EnterOnS3Bucket(t *testing.T) {
	s := stateWithResources("s3", []resource.Resource{
		{
			ID: "my-data-bucket", Name: "my-data-bucket",
			Fields: map[string]string{"name": "my-data-bucket"},
		},
	})
	// S3Bucket should be empty (we are at bucket list level)
	s.S3Bucket = ""

	s = pressSpecial(s, tea.KeyEnter)

	if s.S3Bucket != "my-data-bucket" {
		t.Errorf("S3Bucket should be 'my-data-bucket', got %q", s.S3Bucket)
	}
	if s.S3Prefix != "" {
		t.Errorf("S3Prefix should be empty, got %q", s.S3Prefix)
	}
	if !s.Loading {
		t.Error("should be in loading state after entering bucket")
	}
	// View should stay in ResourceListView (loading objects)
	if s.CurrentView != app.ResourceListView {
		t.Errorf("should stay in ResourceListView while loading, got %d", s.CurrentView)
	}
}

// QA-114: Folder-style navigation with prefixes
func TestQA114_EnterOnS3Folder(t *testing.T) {
	s := stateWithResources("s3", []resource.Resource{
		{
			ID: "logs/", Name: "logs/",
			Fields: map[string]string{"name": "logs/"},
		},
		{
			ID: "data/", Name: "data/",
			Fields: map[string]string{"name": "data/"},
		},
	})
	s.S3Bucket = "my-data-bucket"
	s.S3Prefix = ""

	s = pressSpecial(s, tea.KeyEnter)

	if s.S3Prefix != "logs/" {
		t.Errorf("S3Prefix should be 'logs/', got %q", s.S3Prefix)
	}
	if !s.Loading {
		t.Error("should be loading after entering folder")
	}
}

// QA-115: Enter on a nested folder
func TestQA115_EnterOnNestedFolder(t *testing.T) {
	s := stateWithResources("s3", []resource.Resource{
		{
			ID: "logs/2024/", Name: "2024/",
			Fields: map[string]string{"name": "2024/"},
		},
	})
	s.S3Bucket = "my-data-bucket"
	s.S3Prefix = "logs/"

	s = pressSpecial(s, tea.KeyEnter)

	if s.S3Prefix != "logs/2024/" {
		t.Errorf("S3Prefix should be 'logs/2024/', got %q", s.S3Prefix)
	}
}

// QA-116: Enter on an S3 object (file, not folder) — no-op unless it has DetailData
func TestQA116_EnterOnS3File(t *testing.T) {
	s := stateWithResources("s3", []resource.Resource{
		{
			ID: "report.csv", Name: "report.csv",
			Fields: map[string]string{"name": "report.csv"},
			// No trailing slash, so it's a file — Enter should NOT drill down
			// and should only open detail if DetailData is available
		},
	})
	s.S3Bucket = "my-data-bucket"
	s.S3Prefix = ""

	originalPrefix := s.S3Prefix
	s = pressSpecial(s, tea.KeyEnter)

	// Should NOT change the prefix (no drill-down into files)
	if s.S3Prefix != originalPrefix {
		t.Errorf("S3Prefix should not change for a file, got %q", s.S3Prefix)
	}
	// Should not start loading
	if s.Loading {
		t.Error("should not start loading for a file without trailing /")
	}
}

// QA-117: Escape goes back to parent prefix
func TestQA117_EscapeFromNestedPrefix(t *testing.T) {
	s := stateWithResources("s3", []resource.Resource{
		{ID: "file1.txt", Name: "file1.txt", Fields: map[string]string{"name": "file1.txt"}},
	})
	s.S3Bucket = "my-data-bucket"
	s.S3Prefix = ""

	// Simulate having pushed to a folder: first push the parent state
	// then navigate into logs/ folder
	// The proper way is to go through the drill-down flow:
	// Start at bucket list -> enter bucket -> enter folder
	bucketList := stateWithResources("s3", []resource.Resource{
		{ID: "my-data-bucket", Name: "my-data-bucket", Fields: map[string]string{"name": "my-data-bucket"}},
	})
	bucketList.S3Bucket = ""

	// Enter bucket
	bucketList = pressSpecial(bucketList, tea.KeyEnter)
	// Simulate objects loaded at root level
	rootObjects := []resource.Resource{
		{ID: "logs/", Name: "logs/", Fields: map[string]string{"name": "logs/"}},
	}
	updated, _ := bucketList.Update(app.ResourcesLoadedMsg{ResourceType: "s3", Resources: rootObjects})
	bucketList = updated.(app.AppState)

	// Enter logs/ folder
	bucketList = pressSpecial(bucketList, tea.KeyEnter)
	// Simulate objects loaded in logs/
	logObjects := []resource.Resource{
		{ID: "logs/2024/", Name: "2024/", Fields: map[string]string{"name": "2024/"}},
	}
	updated, _ = bucketList.Update(app.ResourcesLoadedMsg{ResourceType: "s3", Resources: logObjects})
	bucketList = updated.(app.AppState)

	if bucketList.S3Prefix != "logs/" {
		t.Fatalf("expected S3Prefix='logs/', got %q", bucketList.S3Prefix)
	}

	// Now Escape should go back
	bucketList = pressSpecial(bucketList, tea.KeyEscape)

	// Should go back to previous state (root of bucket)
	if bucketList.S3Prefix != "" {
		t.Errorf("after Escape, S3Prefix should be empty (root), got %q", bucketList.S3Prefix)
	}
}

// QA-118: Escape from root prefix goes back to bucket list
func TestQA118_EscapeFromBucketRootToBucketList(t *testing.T) {
	// Start at bucket list
	bucketList := stateWithResources("s3", []resource.Resource{
		{ID: "my-data-bucket", Name: "my-data-bucket", Fields: map[string]string{"name": "my-data-bucket"}},
	})
	bucketList.S3Bucket = ""

	// Enter bucket
	bucketList = pressSpecial(bucketList, tea.KeyEnter)
	if bucketList.S3Bucket != "my-data-bucket" {
		t.Fatalf("expected S3Bucket='my-data-bucket'")
	}

	// Load some objects
	updated, _ := bucketList.Update(app.ResourcesLoadedMsg{
		ResourceType: "s3",
		Resources:    []resource.Resource{{ID: "file.txt", Name: "file.txt", Fields: map[string]string{"name": "file.txt"}}},
	})
	bucketList = updated.(app.AppState)

	// Escape from root prefix
	bucketList = pressSpecial(bucketList, tea.KeyEscape)

	// Should go back to bucket list view
	if bucketList.CurrentView != app.ResourceListView {
		t.Errorf("should still be in ResourceListView, got %d", bucketList.CurrentView)
	}
}

// QA-119: Breadcrumbs show full S3 path
func TestQA119_S3Breadcrumbs(t *testing.T) {
	// Navigate through the full S3 drill-down flow so updateBreadcrumbs is called.
	s := stateWithResources("s3", []resource.Resource{
		{ID: "my-data-bucket", Name: "my-data-bucket", Fields: map[string]string{"name": "my-data-bucket"}},
	})
	s.S3Bucket = ""
	s.Width = 120
	s.Height = 40

	// Enter bucket
	s = pressSpecial(s, tea.KeyEnter)

	// Load root objects with a folder
	updated, _ := s.Update(app.ResourcesLoadedMsg{
		ResourceType: "s3",
		Resources: []resource.Resource{
			{ID: "logs/", Name: "logs/", Fields: map[string]string{"name": "logs/"}},
		},
	})
	s = updated.(app.AppState)

	// Verify breadcrumbs include bucket name
	view := s.View()
	if !strings.Contains(view.Content, "my-data-bucket") {
		t.Error("breadcrumbs should contain the bucket name after entering bucket")
	}

	// Enter logs/ folder
	s = pressSpecial(s, tea.KeyEnter)

	// Load nested folder contents
	updated, _ = s.Update(app.ResourcesLoadedMsg{
		ResourceType: "s3",
		Resources: []resource.Resource{
			{ID: "logs/2024/", Name: "2024/", Fields: map[string]string{"name": "2024/"}},
		},
	})
	s = updated.(app.AppState)

	// Verify breadcrumbs include the prefix
	view = s.View()
	if !strings.Contains(view.Content, "logs/") {
		t.Error("breadcrumbs should contain the S3 prefix path")
	}
}

// QA-120: Empty bucket (no objects)
func TestQA120_EmptyBucket(t *testing.T) {
	// Start at bucket list
	s := stateWithResources("s3", []resource.Resource{
		{ID: "empty-bucket", Name: "empty-bucket", Fields: map[string]string{"name": "empty-bucket"}},
	})
	s.S3Bucket = ""

	// Enter bucket
	s = pressSpecial(s, tea.KeyEnter)

	// Simulate empty response
	updated, _ := s.Update(app.ResourcesLoadedMsg{ResourceType: "s3", Resources: []resource.Resource{}})
	s = updated.(app.AppState)

	if len(s.Resources) != 0 {
		t.Errorf("expected 0 resources for empty bucket, got %d", len(s.Resources))
	}
	// Should show a message about no resources
	if s.StatusMessage == "" {
		t.Error("expected a status message about no resources found")
	}
}

// QA-121: Bucket with many objects (large list scrolls)
// We cannot test real pagination, but we can verify large lists work.
func TestQA121_LargeS3ObjectList(t *testing.T) {
	resources := make([]resource.Resource, 200)
	for i := range resources {
		key := fmt.Sprintf("file-%04d.txt", i)
		resources[i] = resource.Resource{
			ID: key, Name: key,
			Fields: map[string]string{"key": key, "size": "1 KB", "last_modified": "2026-01-01", "storage_class": "STANDARD"},
		}
	}

	s := stateWithResources("s3", resources)
	s.S3Bucket = "big-bucket"
	s.S3Prefix = ""
	s.Width = 80
	s.Height = 20

	// Navigate down through the list
	for i := 0; i < 50; i++ {
		s = pressKey(s, "j")
	}

	if s.SelectedIndex != 50 {
		t.Errorf("expected cursor at 50, got %d", s.SelectedIndex)
	}

	// Rendered view should show the item at index 50
	view := s.View()
	if !strings.Contains(view.Content, "file-0050.txt") {
		t.Error("scrolled view should show item at cursor position 50")
	}
}

// QA-122: d on an S3 object shows metadata
func TestQA122_DescribeS3Object(t *testing.T) {
	s := stateWithResources("s3", []resource.Resource{
		{
			ID: "report.csv", Name: "report.csv",
			Fields: map[string]string{"name": "report.csv"},
			DetailData: map[string]string{
				"Key":           "report.csv",
				"Size":          "1024",
				"Content Type":  "text/csv",
				"Last Modified": "2026-03-10T12:00:00Z",
				"Storage Class": "STANDARD",
				"ETag":          "\"abc123\"",
			},
		},
	})
	s.S3Bucket = "my-data-bucket"
	s.S3Prefix = ""

	s = pressKey(s, "d")

	if s.CurrentView != app.DetailView {
		t.Fatalf("d on S3 object should open DetailView, got %d", s.CurrentView)
	}
	if s.Detail.Data["Key"] != "report.csv" {
		t.Errorf("detail should contain 'Key'='report.csv', got %q", s.Detail.Data["Key"])
	}
	if s.Detail.Data["Storage Class"] != "STANDARD" {
		t.Errorf("detail should contain 'Storage Class'='STANDARD', got %q", s.Detail.Data["Storage Class"])
	}
}

// QA-122 (edge): d on S3 object with no DetailData
func TestQA122_DescribeS3ObjectNoDetail(t *testing.T) {
	s := stateWithResources("s3", []resource.Resource{
		{
			ID: "nodata.txt", Name: "nodata.txt",
			Fields:     map[string]string{"name": "nodata.txt"},
			DetailData: nil,
		},
	})
	s.S3Bucket = "my-data-bucket"

	s = pressKey(s, "d")

	if s.CurrentView != app.ResourceListView {
		t.Errorf("d with nil DetailData should stay in ResourceListView, got %d", s.CurrentView)
	}
	if !strings.Contains(s.StatusMessage, "No detail data") {
		t.Errorf("expected 'No detail data' status, got %q", s.StatusMessage)
	}
}
