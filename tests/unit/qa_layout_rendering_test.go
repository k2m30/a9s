package unit

import (
	"fmt"
	"strings"
	"testing"

	"github.com/k2m30/a9s/internal/app"
	awsclient "github.com/k2m30/a9s/internal/aws"
	"github.com/k2m30/a9s/internal/resource"
	"github.com/k2m30/a9s/internal/views"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newTestState returns an AppState configured with the given profile/region
// and terminal dimensions, with no AWS connection required.
func newTestState(profile, region string, width, height int) app.AppState {
	if profile == "" {
		profile = "test-profile"
	}
	if region == "" {
		region = "us-east-1"
	}
	s := app.AppState{
		CurrentView:   app.MainMenuView,
		ActiveProfile: profile,
		ActiveRegion:  region,
		Breadcrumbs:   []string{"main"},
		Keys:          app.DefaultKeyMap(),
		Width:         width,
		Height:        height,
	}
	return s
}

// countNonEmptyLines counts lines in the rendered output, trimming trailing
// blank lines (the terminal would not show them).
func countNonEmptyLines(content string) int {
	lines := strings.Split(content, "\n")
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return len(lines)
}

// lastNonEmptyLine returns the last non-blank line in the content.
func lastNonEmptyLine(content string) string {
	lines := strings.Split(content, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			return lines[i]
		}
	}
	return ""
}

// firstNonEmptyLine returns the first non-blank line in the content.
func firstNonEmptyLine(content string) string {
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) != "" {
			return line
		}
	}
	return ""
}

// makeEC2Resources creates n EC2 resources with all fields populated.
func makeEC2Resources(n int) []resource.Resource {
	res := make([]resource.Resource, n)
	for i := range res {
		id := fmt.Sprintf("i-%012d", i)
		name := fmt.Sprintf("server-%03d", i)
		res[i] = resource.Resource{
			ID: id, Name: name, Status: "running",
			Fields: map[string]string{
				"instance_id": id,
				"name":        name,
				"state":       "running",
				"type":        "t3.medium",
				"private_ip":  fmt.Sprintf("10.0.%d.%d", i/256, i%256),
				"public_ip":   fmt.Sprintf("54.%d.%d.%d", i/65536, (i/256)%256, i%256),
				"launch_time": "2026-01-15T10:30:00Z",
			},
			DetailData: map[string]string{"Instance ID": id, "Name": name, "State": "running"},
			RawJSON:    fmt.Sprintf(`{"InstanceId":"%s","Name":"%s"}`, id, name),
		}
	}
	return res
}

// makeS3Buckets creates n S3 bucket resources.
func makeS3Buckets(n int) []resource.Resource {
	res := make([]resource.Resource, n)
	for i := range res {
		name := fmt.Sprintf("bucket-%03d", i)
		res[i] = resource.Resource{
			ID: name, Name: name,
			Fields: map[string]string{
				"name":          name,
				"creation_date": "2026-01-01T00:00:00Z",
			},
			DetailData: map[string]string{"Bucket Name": name},
		}
	}
	return res
}

// makeS3Objects creates n S3 object resources (inside-bucket view).
func makeS3Objects(n int) []resource.Resource {
	res := make([]resource.Resource, n)
	for i := range res {
		key := fmt.Sprintf("folder/file-%03d.txt", i)
		res[i] = resource.Resource{
			ID: key, Name: key,
			Fields: map[string]string{
				"key":           key,
				"size":          "1024",
				"last_modified": "2026-02-10T12:00:00Z",
				"storage_class": "STANDARD",
			},
		}
	}
	return res
}

// makeRDSResources creates n RDS resources with all fields.
func makeRDSResources(n int) []resource.Resource {
	res := make([]resource.Resource, n)
	for i := range res {
		id := fmt.Sprintf("mydb-%03d", i)
		res[i] = resource.Resource{
			ID: id, Name: id, Status: "available",
			Fields: map[string]string{
				"db_identifier": id,
				"engine":        "postgres",
				"engine_version": "15.4",
				"status":        "available",
				"class":         "db.r6g.large",
				"endpoint":      fmt.Sprintf("%s.abc.us-east-1.rds.amazonaws.com", id),
				"multi_az":      "true",
			},
			DetailData: map[string]string{"DB Identifier": id},
			RawJSON:    fmt.Sprintf(`{"DBIdentifier":"%s"}`, id),
		}
	}
	return res
}

// makeRedisResources creates n Redis resources with all fields.
func makeRedisResources(n int) []resource.Resource {
	res := make([]resource.Resource, n)
	for i := range res {
		id := fmt.Sprintf("redis-%03d", i)
		res[i] = resource.Resource{
			ID: id, Name: id, Status: "available",
			Fields: map[string]string{
				"cluster_id":     id,
				"engine_version": "7.0",
				"node_type":      "cache.r6g.large",
				"status":         "available",
				"nodes":          "3",
				"endpoint":       fmt.Sprintf("%s.abc.use1.cache.amazonaws.com", id),
			},
			DetailData: map[string]string{"Cluster ID": id},
			RawJSON:    fmt.Sprintf(`{"ClusterId":"%s"}`, id),
		}
	}
	return res
}

// makeDocDBResources creates n DocumentDB resources with all fields.
func makeDocDBResources(n int) []resource.Resource {
	res := make([]resource.Resource, n)
	for i := range res {
		id := fmt.Sprintf("docdb-%03d", i)
		res[i] = resource.Resource{
			ID: id, Name: id, Status: "available",
			Fields: map[string]string{
				"cluster_id":     id,
				"engine_version": "6.0",
				"status":         "available",
				"instances":      "2",
				"endpoint":       fmt.Sprintf("%s.cluster-abc.us-east-1.docdb.amazonaws.com", id),
			},
			DetailData: map[string]string{"Cluster ID": id},
			RawJSON:    fmt.Sprintf(`{"ClusterId":"%s"}`, id),
		}
	}
	return res
}

// makeEKSResources creates n EKS resources with all fields.
func makeEKSResources(n int) []resource.Resource {
	res := make([]resource.Resource, n)
	for i := range res {
		name := fmt.Sprintf("eks-%03d", i)
		res[i] = resource.Resource{
			ID: name, Name: name, Status: "ACTIVE",
			Fields: map[string]string{
				"cluster_name":     name,
				"version":          "1.29",
				"status":           "ACTIVE",
				"endpoint":         fmt.Sprintf("https://%s.eks.us-east-1.amazonaws.com", name),
				"platform_version": "eks.5",
			},
			DetailData: map[string]string{"Cluster Name": name},
			RawJSON:    fmt.Sprintf(`{"ClusterName":"%s"}`, name),
		}
	}
	return res
}

// makeSecretsResources creates n Secrets Manager resources with all fields.
func makeSecretsResources(n int) []resource.Resource {
	res := make([]resource.Resource, n)
	for i := range res {
		name := fmt.Sprintf("prod/secret-%03d", i)
		res[i] = resource.Resource{
			ID: name, Name: name,
			Fields: map[string]string{
				"secret_name":      name,
				"description":      fmt.Sprintf("Secret number %d", i),
				"last_accessed":    "2026-03-14",
				"last_changed":     "2026-03-10",
				"rotation_enabled": "false",
			},
			DetailData: map[string]string{"Secret Name": name},
			RawJSON:    fmt.Sprintf(`{"SecretName":"%s"}`, name),
		}
	}
	return res
}

// resourceFactory maps resource type short names to factory functions.
var resourceFactory = map[string]func(int) []resource.Resource{
	"ec2":     makeEC2Resources,
	"s3":      makeS3Buckets,
	"rds":     makeRDSResources,
	"redis":   makeRedisResources,
	"docdb":   makeDocDBResources,
	"eks":     makeEKSResources,
	"secrets": makeSecretsResources,
}

// allResourceShortNames returns the short names of all resource types.
func allResourceShortNames() []string {
	return []string{"s3", "ec2", "rds", "redis", "docdb", "eks", "secrets"}
}

// ===========================================================================
// 1. Output height never exceeds terminal height (EVERY view)
// ===========================================================================

func TestQALayout_OutputHeight_MainMenu(t *testing.T) {
	for _, h := range []int{10, 20, 40, 80} {
		t.Run(fmt.Sprintf("Height%d", h), func(t *testing.T) {
			state := newTestState("", "", 80, h)
			state.CurrentView = app.MainMenuView
			state.Breadcrumbs = []string{"main"}

			view := state.View()
			lines := countNonEmptyLines(view.Content)
			if lines > h {
				t.Errorf("MainMenu: output has %d lines but terminal height is %d", lines, h)
			}
		})
	}
}

func TestQALayout_OutputHeight_ResourceList(t *testing.T) {
	for _, rtName := range allResourceShortNames() {
		for _, h := range []int{10, 20, 40, 80} {
			t.Run(fmt.Sprintf("%s_Height%d", rtName, h), func(t *testing.T) {
				state := newTestState("", "", 120, h)
				state.CurrentView = app.ResourceListView
				state.CurrentResourceType = rtName
				state.Breadcrumbs = []string{"main", rtName}
				factory := resourceFactory[rtName]
				state.Resources = factory(50)

				view := state.View()
				lines := countNonEmptyLines(view.Content)
				if lines > h {
					t.Errorf("ResourceList(%s): output has %d lines but terminal height is %d",
						rtName, lines, h)
				}
			})
		}
	}
}

func TestQALayout_OutputHeight_DetailView(t *testing.T) {
	for _, h := range []int{10, 20, 40, 80} {
		t.Run(fmt.Sprintf("Height%d", h), func(t *testing.T) {
			data := make(map[string]string)
			for i := 0; i < 100; i++ {
				data[fmt.Sprintf("Key%03d", i)] = fmt.Sprintf("Value%03d", i)
			}
			state := newTestState("", "", 80, h)
			state.CurrentView = app.DetailView
			state.CurrentResourceType = "ec2"
			state.Breadcrumbs = []string{"main", "EC2 Instances", "detail"}
			state.Detail = views.NewDetailModel("Test Resource - Detail", data)
			state.Detail.Width = 80
			state.Detail.Height = h

			view := state.View()
			lines := countNonEmptyLines(view.Content)
			if lines > h {
				t.Errorf("DetailView: output has %d lines but terminal height is %d", lines, h)
			}
		})
	}
}

func TestQALayout_OutputHeight_JSONView(t *testing.T) {
	for _, h := range []int{10, 20, 40, 80} {
		t.Run(fmt.Sprintf("Height%d", h), func(t *testing.T) {
			jsonLines := make([]string, 500)
			for i := range jsonLines {
				jsonLines[i] = fmt.Sprintf(`  "key%d": "value%d",`, i, i)
			}
			jsonContent := "{\n" + strings.Join(jsonLines, "\n") + "\n}"

			state := newTestState("", "", 80, h)
			state.CurrentView = app.JSONView
			state.CurrentResourceType = "ec2"
			state.Breadcrumbs = []string{"main", "EC2 Instances", "json"}
			state.JSONData = views.NewJSONView("Test - JSON", jsonContent)
			state.JSONData.Width = 80
			state.JSONData.Height = h

			view := state.View()
			lines := countNonEmptyLines(view.Content)
			if lines > h {
				t.Errorf("JSONView: output has %d lines but terminal height is %d", lines, h)
			}
		})
	}
}

func TestQALayout_OutputHeight_RevealView(t *testing.T) {
	for _, h := range []int{10, 20, 40, 80} {
		t.Run(fmt.Sprintf("Height%d", h), func(t *testing.T) {
			longSecret := strings.Repeat("secret-line\n", 200)
			state := newTestState("", "", 80, h)
			state.CurrentView = app.RevealView
			state.CurrentResourceType = "secrets"
			state.Breadcrumbs = []string{"main", "Secrets Manager", "reveal"}
			state.Reveal = views.NewRevealView("Secret: my-secret", longSecret)
			state.Reveal.Width = 80
			state.Reveal.Height = h

			view := state.View()
			lines := countNonEmptyLines(view.Content)
			if lines > h {
				t.Errorf("RevealView: output has %d lines but terminal height is %d", lines, h)
			}
		})
	}
}

func TestQALayout_OutputHeight_ProfileSelect(t *testing.T) {
	for _, h := range []int{10, 20, 40, 80} {
		t.Run(fmt.Sprintf("Height%d", h), func(t *testing.T) {
			profiles := make([]string, 30)
			for i := range profiles {
				profiles[i] = fmt.Sprintf("profile-%02d", i)
			}
			state := newTestState("", "", 80, h)
			state.CurrentView = app.ProfileSelectView
			state.Breadcrumbs = []string{"main", "profile"}
			state.ProfileSelector = views.NewProfileSelect(profiles, "profile-00")

			view := state.View()
			lines := countNonEmptyLines(view.Content)
			if lines > h {
				t.Errorf("ProfileSelect: output has %d lines but terminal height is %d", lines, h)
			}
		})
	}
}

func TestQALayout_OutputHeight_RegionSelect(t *testing.T) {
	for _, h := range []int{10, 20, 40, 80} {
		t.Run(fmt.Sprintf("Height%d", h), func(t *testing.T) {
			regions := awsclient.AllRegions()
			state := newTestState("", "", 80, h)
			state.CurrentView = app.RegionSelectView
			state.Breadcrumbs = []string{"main", "region"}
			state.RegionSelector = views.NewRegionSelect(regions, "us-east-1")

			view := state.View()
			lines := countNonEmptyLines(view.Content)
			if lines > h {
				t.Errorf("RegionSelect: output has %d lines but terminal height is %d", lines, h)
			}
		})
	}
}

// ===========================================================================
// 2. Status bar always visible (EVERY view, EVERY mode)
// ===========================================================================

func TestQALayout_StatusBar_NormalMode_AllViews(t *testing.T) {
	tests := []struct {
		name  string
		setup func() app.AppState
	}{
		{
			name: "MainMenu",
			setup: func() app.AppState {
				s := newTestState("", "", 80, 24)
				s.CurrentView = app.MainMenuView
				return s
			},
		},
		{
			name: "ResourceList_EC2",
			setup: func() app.AppState {
				s := newTestState("", "", 120, 24)
				s.CurrentView = app.ResourceListView
				s.CurrentResourceType = "ec2"
				s.Breadcrumbs = []string{"main", "EC2 Instances"}
				s.Resources = makeEC2Resources(5)
				return s
			},
		},
		{
			name: "DetailView",
			setup: func() app.AppState {
				s := newTestState("", "", 80, 24)
				s.CurrentView = app.DetailView
				s.CurrentResourceType = "ec2"
				s.Breadcrumbs = []string{"main", "EC2 Instances", "detail"}
				s.Detail = views.NewDetailModel("Test", map[string]string{"Key": "Val"})
				s.Detail.Width = 80
				s.Detail.Height = 20
				return s
			},
		},
		{
			name: "JSONView",
			setup: func() app.AppState {
				s := newTestState("", "", 80, 24)
				s.CurrentView = app.JSONView
				s.CurrentResourceType = "ec2"
				s.Breadcrumbs = []string{"main", "EC2 Instances", "json"}
				s.JSONData = views.NewJSONView("Test", `{"key":"value"}`)
				s.JSONData.Width = 80
				s.JSONData.Height = 20
				return s
			},
		},
		{
			name: "RevealView",
			setup: func() app.AppState {
				s := newTestState("", "", 80, 24)
				s.CurrentView = app.RevealView
				s.CurrentResourceType = "secrets"
				s.Breadcrumbs = []string{"main", "Secrets Manager", "reveal"}
				s.Reveal = views.NewRevealView("Secret", "my-secret-value")
				s.Reveal.Width = 80
				s.Reveal.Height = 20
				return s
			},
		},
		{
			name: "ProfileSelect",
			setup: func() app.AppState {
				s := newTestState("", "", 80, 24)
				s.CurrentView = app.ProfileSelectView
				s.Breadcrumbs = []string{"main", "profile"}
				s.ProfileSelector = views.NewProfileSelect([]string{"default", "staging"}, "default")
				return s
			},
		},
		{
			name: "RegionSelect",
			setup: func() app.AppState {
				s := newTestState("", "", 80, 24)
				s.CurrentView = app.RegionSelectView
				s.Breadcrumbs = []string{"main", "region"}
				s.RegionSelector = views.NewRegionSelect(awsclient.AllRegions(), "us-east-1")
				return s
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			state := tc.setup()
			view := state.View()
			last := lastNonEmptyLine(view.Content)
			if !strings.Contains(last, "Ready") && !strings.Contains(last, "help") && !strings.Contains(last, "quit") && !strings.Contains(last, "command") && !strings.Contains(last, "filter") {
				t.Errorf("Status bar not visible in %s. Last line: %q", tc.name, last)
			}
		})
	}
}

func TestQALayout_StatusBar_CommandMode(t *testing.T) {
	for _, rtName := range allResourceShortNames() {
		t.Run(rtName, func(t *testing.T) {
			state := newTestState("", "", 120, 24)
			state.CurrentView = app.ResourceListView
			state.CurrentResourceType = rtName
			state.Breadcrumbs = []string{"main", rtName}
			state.Resources = resourceFactory[rtName](5)
			state.CommandMode = true
			state.CommandText = "test"

			view := state.View()
			last := lastNonEmptyLine(view.Content)
			if !strings.Contains(last, ":test") {
				t.Errorf("Command mode status bar should show ':test', last line: %q", last)
			}
		})
	}
}

func TestQALayout_StatusBar_FilterMode(t *testing.T) {
	state := newTestState("", "", 120, 24)
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "ec2"
	state.Breadcrumbs = []string{"main", "EC2 Instances"}
	state.Resources = makeEC2Resources(10)
	state.FilterMode = true
	state.Filter = "server"
	state.FilteredResources = state.Resources

	view := state.View()
	last := lastNonEmptyLine(view.Content)
	if !strings.Contains(last, "/server") {
		t.Errorf("Filter mode status bar should show '/server', last line: %q", last)
	}
}

func TestQALayout_StatusBar_ErrorMode(t *testing.T) {
	state := newTestState("", "", 80, 24)
	state.CurrentView = app.MainMenuView
	state.StatusMessage = "Connection failed: timeout"
	state.StatusIsError = true

	view := state.View()
	last := lastNonEmptyLine(view.Content)
	if !strings.Contains(last, "Connection failed") {
		t.Errorf("Error status bar should show error message, last line: %q", last)
	}
}

func TestQALayout_StatusBar_LoadingMode(t *testing.T) {
	state := newTestState("", "", 120, 24)
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "ec2"
	state.Breadcrumbs = []string{"main", "EC2 Instances"}
	state.Loading = true
	state.Resources = nil

	view := state.View()
	content := view.Content
	// Loading indicator should appear somewhere in output (header or content)
	if !strings.Contains(strings.ToLower(content), "loading") {
		t.Error("Loading state should show 'loading' somewhere in the output")
	}
}

// ===========================================================================
// 3. Header always visible (EVERY view)
// ===========================================================================

func TestQALayout_Header_Visible_AllViews(t *testing.T) {
	viewSetups := map[string]func() app.AppState{
		"MainMenu": func() app.AppState {
			return newTestState("myprofile", "eu-west-1", 80, 24)
		},
		"ResourceList": func() app.AppState {
			s := newTestState("myprofile", "eu-west-1", 120, 24)
			s.CurrentView = app.ResourceListView
			s.CurrentResourceType = "ec2"
			s.Breadcrumbs = []string{"main", "EC2 Instances"}
			s.Resources = makeEC2Resources(3)
			return s
		},
		"DetailView": func() app.AppState {
			s := newTestState("myprofile", "eu-west-1", 80, 24)
			s.CurrentView = app.DetailView
			s.CurrentResourceType = "ec2"
			s.Breadcrumbs = []string{"main", "EC2 Instances", "detail"}
			s.Detail = views.NewDetailModel("Test", map[string]string{"A": "B"})
			s.Detail.Width = 80
			s.Detail.Height = 20
			return s
		},
		"JSONView": func() app.AppState {
			s := newTestState("myprofile", "eu-west-1", 80, 24)
			s.CurrentView = app.JSONView
			s.CurrentResourceType = "ec2"
			s.Breadcrumbs = []string{"main", "EC2 Instances", "json"}
			s.JSONData = views.NewJSONView("Test", `{"a":1}`)
			s.JSONData.Width = 80
			s.JSONData.Height = 20
			return s
		},
		"RevealView": func() app.AppState {
			s := newTestState("myprofile", "eu-west-1", 80, 24)
			s.CurrentView = app.RevealView
			s.CurrentResourceType = "secrets"
			s.Breadcrumbs = []string{"main", "Secrets Manager", "reveal"}
			s.Reveal = views.NewRevealView("Secret", "val")
			s.Reveal.Width = 80
			s.Reveal.Height = 20
			return s
		},
		"ProfileSelect": func() app.AppState {
			s := newTestState("myprofile", "eu-west-1", 80, 24)
			s.CurrentView = app.ProfileSelectView
			s.Breadcrumbs = []string{"main", "profile"}
			s.ProfileSelector = views.NewProfileSelect([]string{"default"}, "default")
			return s
		},
		"RegionSelect": func() app.AppState {
			s := newTestState("myprofile", "eu-west-1", 80, 24)
			s.CurrentView = app.RegionSelectView
			s.Breadcrumbs = []string{"main", "region"}
			s.RegionSelector = views.NewRegionSelect(awsclient.AllRegions(), "eu-west-1")
			return s
		},
	}

	for viewName, setup := range viewSetups {
		t.Run(viewName, func(t *testing.T) {
			state := setup()
			view := state.View()
			first := firstNonEmptyLine(view.Content)
			if !strings.Contains(first, "myprofile") {
				t.Errorf("%s header should contain profile name 'myprofile', got: %q", viewName, first)
			}
			if !strings.Contains(first, "eu-west-1") {
				t.Errorf("%s header should contain region 'eu-west-1', got: %q", viewName, first)
			}
		})
	}
}

// ===========================================================================
// 4. Breadcrumbs always visible (EVERY view)
// ===========================================================================

func TestQALayout_Breadcrumbs_Visible_AllViews(t *testing.T) {
	tests := []struct {
		name            string
		setup           func() app.AppState
		expectedSegment string
	}{
		{
			name: "MainMenu",
			setup: func() app.AppState {
				return newTestState("", "", 80, 24)
			},
			expectedSegment: "main",
		},
		{
			name: "ResourceList_EC2",
			setup: func() app.AppState {
				s := newTestState("", "", 120, 24)
				s.CurrentView = app.ResourceListView
				s.CurrentResourceType = "ec2"
				s.Breadcrumbs = []string{"main", "EC2 Instances"}
				s.Resources = makeEC2Resources(3)
				return s
			},
			expectedSegment: "EC2 Instances",
		},
		{
			name: "DetailView",
			setup: func() app.AppState {
				s := newTestState("", "", 80, 24)
				s.CurrentView = app.DetailView
				s.CurrentResourceType = "ec2"
				s.Breadcrumbs = []string{"main", "EC2 Instances", "detail"}
				s.Detail = views.NewDetailModel("Test", map[string]string{"A": "B"})
				s.Detail.Width = 80
				s.Detail.Height = 20
				return s
			},
			expectedSegment: "detail",
		},
		{
			name: "JSONView",
			setup: func() app.AppState {
				s := newTestState("", "", 80, 24)
				s.CurrentView = app.JSONView
				s.CurrentResourceType = "ec2"
				s.Breadcrumbs = []string{"main", "EC2 Instances", "json"}
				s.JSONData = views.NewJSONView("Test", `{"a":1}`)
				s.JSONData.Width = 80
				s.JSONData.Height = 20
				return s
			},
			expectedSegment: "json",
		},
		{
			name: "RevealView",
			setup: func() app.AppState {
				s := newTestState("", "", 80, 24)
				s.CurrentView = app.RevealView
				s.CurrentResourceType = "secrets"
				s.Breadcrumbs = []string{"main", "Secrets Manager", "reveal"}
				s.Reveal = views.NewRevealView("Secret", "val")
				s.Reveal.Width = 80
				s.Reveal.Height = 20
				return s
			},
			expectedSegment: "reveal",
		},
		{
			name: "ProfileSelect",
			setup: func() app.AppState {
				s := newTestState("", "", 80, 24)
				s.CurrentView = app.ProfileSelectView
				s.Breadcrumbs = []string{"main", "profile"}
				s.ProfileSelector = views.NewProfileSelect([]string{"default"}, "default")
				return s
			},
			expectedSegment: "profile",
		},
		{
			name: "RegionSelect",
			setup: func() app.AppState {
				s := newTestState("", "", 80, 24)
				s.CurrentView = app.RegionSelectView
				s.Breadcrumbs = []string{"main", "region"}
				s.RegionSelector = views.NewRegionSelect(awsclient.AllRegions(), "us-east-1")
				return s
			},
			expectedSegment: "region",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			state := tc.setup()
			view := state.View()
			lines := strings.Split(view.Content, "\n")
			// Breadcrumbs should be in the first few lines (line 1 or 2, after header)
			found := false
			for i := 0; i < len(lines) && i < 5; i++ {
				if strings.Contains(lines[i], tc.expectedSegment) && strings.Contains(lines[i], "main") {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Breadcrumbs should contain %q in the first few lines of %s view",
					tc.expectedSegment, tc.name)
			}
		})
	}
}

// ===========================================================================
// 5. Column headers visible for EVERY resource type
// ===========================================================================

func TestQALayout_ColumnHeaders_AllResourceTypes(t *testing.T) {
	for _, rtName := range allResourceShortNames() {
		t.Run(rtName, func(t *testing.T) {
			rt := resource.FindResourceType(rtName)
			if rt == nil {
				t.Fatalf("Unknown resource type: %s", rtName)
			}

			state := newTestState("", "", 200, 30)
			state.CurrentView = app.ResourceListView
			state.CurrentResourceType = rtName
			state.Breadcrumbs = []string{"main", rt.Name}
			factory := resourceFactory[rtName]
			state.Resources = factory(3)

			view := state.View()
			content := view.Content

			for _, col := range rt.Columns {
				if !strings.Contains(content, col.Title) {
					t.Errorf("Resource type %s: column header %q not found in rendered output",
						rtName, col.Title)
				}
			}
		})
	}
}

// ===========================================================================
// 6. Column VALUES visible for EVERY resource type
// ===========================================================================

func TestQALayout_ColumnValues_AllResourceTypes(t *testing.T) {
	for _, rtName := range allResourceShortNames() {
		t.Run(rtName, func(t *testing.T) {
			rt := resource.FindResourceType(rtName)
			if rt == nil {
				t.Fatalf("Unknown resource type: %s", rtName)
			}

			state := newTestState("", "", 200, 30)
			state.CurrentView = app.ResourceListView
			state.CurrentResourceType = rtName
			state.Breadcrumbs = []string{"main", rt.Name}
			factory := resourceFactory[rtName]
			resources := factory(1)
			state.Resources = resources

			view := state.View()
			content := view.Content

			for _, col := range rt.Columns {
				val := resources[0].Fields[col.Key]
				if val == "" {
					t.Errorf("Resource type %s: test resource has empty field for column %q (key=%q) — fix test helper",
						rtName, col.Title, col.Key)
					continue
				}
				// Value might be truncated to 40 chars max, so check prefix
				checkVal := val
				if len(checkVal) > 40 {
					checkVal = checkVal[:39]
				}
				if !strings.Contains(content, checkVal) {
					t.Errorf("Resource type %s: field value %q (key=%q) not found in rendered output",
						rtName, checkVal, col.Key)
				}
			}
		})
	}
}

// ===========================================================================
// 7. S3 inside-bucket uses object columns, not bucket columns
// ===========================================================================

func TestQALayout_S3InsideBucket_ObjectColumns(t *testing.T) {
	state := newTestState("", "", 200, 30)
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "s3"
	state.S3Bucket = "my-test-bucket"
	state.S3Prefix = ""
	state.Breadcrumbs = []string{"main", "S3 Buckets", "my-test-bucket"}
	state.Resources = makeS3Objects(5)

	view := state.View()
	content := view.Content

	// Object columns should be visible
	objectCols := resource.S3ObjectColumns()
	for _, col := range objectCols {
		if !strings.Contains(content, col.Title) {
			t.Errorf("S3 inside-bucket should show object column %q, not found", col.Title)
		}
	}

	// Bucket-specific columns should NOT appear as headers
	// "Bucket Name" is from S3 bucket columns, "Key" is from object columns
	if !strings.Contains(content, "Key") {
		t.Error("S3 inside-bucket should show 'Key' column header")
	}
	if !strings.Contains(content, "Size") {
		t.Error("S3 inside-bucket should show 'Size' column header")
	}
}

func TestQALayout_S3InsideBucket_ObjectValues(t *testing.T) {
	state := newTestState("", "", 200, 30)
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "s3"
	state.S3Bucket = "my-test-bucket"
	state.S3Prefix = "folder/"
	state.Breadcrumbs = []string{"main", "S3 Buckets", "my-test-bucket", "folder/"}
	objects := makeS3Objects(3)
	state.Resources = objects

	view := state.View()
	content := view.Content

	// Object field values should be visible
	if !strings.Contains(content, "folder/file-000.txt") {
		t.Error("S3 object key 'folder/file-000.txt' should be visible")
	}
	if !strings.Contains(content, "1024") {
		t.Error("S3 object size '1024' should be visible")
	}
	if !strings.Contains(content, "STANDARD") {
		t.Error("S3 object storage class 'STANDARD' should be visible")
	}
}

func TestQALayout_S3InsideBucket_TitleShowsBucketName(t *testing.T) {
	state := newTestState("", "", 200, 30)
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "s3"
	state.S3Bucket = "my-data-bucket"
	state.S3Prefix = ""
	state.Breadcrumbs = []string{"main", "S3 Buckets", "my-data-bucket"}
	state.Resources = makeS3Objects(2)

	view := state.View()
	content := view.Content

	if !strings.Contains(content, "my-data-bucket") {
		t.Error("S3 inside-bucket view should show bucket name in title")
	}
}

// ===========================================================================
// 8. Cursor indicator visible
// ===========================================================================

func TestQALayout_CursorIndicator_MainMenu(t *testing.T) {
	state := newTestState("", "", 80, 24)
	state.CurrentView = app.MainMenuView
	state.SelectedIndex = 2

	view := state.View()
	if !strings.Contains(view.Content, "> ") {
		t.Error("MainMenu should show '> ' cursor indicator for selected item")
	}
}

func TestQALayout_CursorIndicator_ResourceList(t *testing.T) {
	for _, rtName := range allResourceShortNames() {
		t.Run(rtName, func(t *testing.T) {
			state := newTestState("", "", 200, 30)
			state.CurrentView = app.ResourceListView
			state.CurrentResourceType = rtName
			state.Breadcrumbs = []string{"main", rtName}
			factory := resourceFactory[rtName]
			state.Resources = factory(5)
			state.SelectedIndex = 2

			view := state.View()
			if !strings.Contains(view.Content, "> ") {
				t.Errorf("ResourceList(%s) should show '> ' cursor indicator", rtName)
			}
		})
	}
}

func TestQALayout_CursorIndicator_ProfileSelect(t *testing.T) {
	state := newTestState("", "", 80, 24)
	state.CurrentView = app.ProfileSelectView
	state.Breadcrumbs = []string{"main", "profile"}
	state.ProfileSelector = views.NewProfileSelect(
		[]string{"default", "staging", "prod"}, "default",
	)

	view := state.View()
	if !strings.Contains(view.Content, "> ") {
		t.Error("ProfileSelect should show '> ' cursor indicator")
	}
}

func TestQALayout_CursorIndicator_RegionSelect(t *testing.T) {
	state := newTestState("", "", 80, 40)
	state.CurrentView = app.RegionSelectView
	state.Breadcrumbs = []string{"main", "region"}
	state.RegionSelector = views.NewRegionSelect(awsclient.AllRegions(), "us-east-1")

	view := state.View()
	if !strings.Contains(view.Content, "> ") {
		t.Error("RegionSelect should show '> ' cursor indicator")
	}
}

func TestQALayout_CursorIndicator_CorrectRow(t *testing.T) {
	state := newTestState("", "", 200, 30)
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "ec2"
	state.Breadcrumbs = []string{"main", "EC2 Instances"}
	state.Resources = makeEC2Resources(10)
	state.SelectedIndex = 3

	view := state.View()
	lines := strings.Split(view.Content, "\n")
	// Find the line with the cursor
	cursorLine := ""
	for _, line := range lines {
		if strings.Contains(line, "> ") && strings.Contains(line, "server-003") {
			cursorLine = line
			break
		}
	}
	if cursorLine == "" {
		t.Error("Cursor '> ' should be on the row for server-003 (index 3)")
	}
}

// ===========================================================================
// 9. Empty state messages
// ===========================================================================

func TestQALayout_EmptyState_ResourceList_NoResources(t *testing.T) {
	for _, rtName := range allResourceShortNames() {
		t.Run(rtName, func(t *testing.T) {
			state := newTestState("", "", 120, 24)
			state.CurrentView = app.ResourceListView
			state.CurrentResourceType = rtName
			state.Breadcrumbs = []string{"main", rtName}
			state.Resources = nil

			view := state.View()
			content := strings.ToLower(view.Content)
			if !strings.Contains(content, "no ") {
				t.Errorf("Empty ResourceList(%s) should show 'No ...' message, got: %s",
					rtName, view.Content)
			}
		})
	}
}

func TestQALayout_EmptyState_ResourceList_FilterNoMatch(t *testing.T) {
	state := newTestState("", "", 120, 24)
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "ec2"
	state.Breadcrumbs = []string{"main", "EC2 Instances"}
	state.Resources = makeEC2Resources(5)
	state.Filter = "nonexistent"
	state.FilteredResources = []resource.Resource{}

	view := state.View()
	content := strings.ToLower(view.Content)
	if !strings.Contains(content, "no ") || !strings.Contains(content, "filter") ||
		!strings.Contains(content, "nonexistent") {
		t.Errorf("Filtered empty list should show 'No ... matching filter: nonexistent', got: %s",
			view.Content)
	}
}

func TestQALayout_EmptyState_DetailView_NoData(t *testing.T) {
	state := newTestState("", "", 80, 24)
	state.CurrentView = app.DetailView
	state.CurrentResourceType = "ec2"
	state.Breadcrumbs = []string{"main", "EC2 Instances", "detail"}
	state.Detail = views.NewDetailModel("Empty Detail", map[string]string{})
	state.Detail.Width = 80
	state.Detail.Height = 20

	view := state.View()
	content := strings.ToLower(view.Content)
	if !strings.Contains(content, "no detail") {
		t.Error("DetailView with no data should show 'No details available'")
	}
}

func TestQALayout_EmptyState_JSONView_NoContent(t *testing.T) {
	state := newTestState("", "", 80, 24)
	state.CurrentView = app.JSONView
	state.CurrentResourceType = "ec2"
	state.Breadcrumbs = []string{"main", "EC2 Instances", "json"}
	state.JSONData = views.NewJSONView("Empty JSON", "")
	state.JSONData.Width = 80
	state.JSONData.Height = 20

	view := state.View()
	content := strings.ToLower(view.Content)
	if !strings.Contains(content, "no json") {
		t.Error("JSONView with empty content should show 'No JSON content available'")
	}
}

func TestQALayout_EmptyState_RevealView_NoContent(t *testing.T) {
	state := newTestState("", "", 80, 24)
	state.CurrentView = app.RevealView
	state.CurrentResourceType = "secrets"
	state.Breadcrumbs = []string{"main", "Secrets Manager", "reveal"}
	state.Reveal = views.NewRevealView("Empty Reveal", "")
	state.Reveal.Width = 80
	state.Reveal.Height = 20

	view := state.View()
	content := strings.ToLower(view.Content)
	if !strings.Contains(content, "no content") {
		t.Error("RevealView with empty content should show 'No content available'")
	}
}

func TestQALayout_EmptyState_Loading(t *testing.T) {
	state := newTestState("", "", 120, 24)
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "ec2"
	state.Breadcrumbs = []string{"main", "EC2 Instances"}
	state.Loading = true
	state.Resources = nil

	view := state.View()
	content := strings.ToLower(view.Content)
	if !strings.Contains(content, "loading") {
		t.Error("ResourceList in loading state should show 'Loading...' message")
	}
}

// ===========================================================================
// 10. Terminal size edge cases
// ===========================================================================

func TestQALayout_EdgeCase_Width1Height1_NoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Panic with Width=1, Height=1: %v", r)
		}
	}()

	state := newTestState("", "", 1, 1)
	state.CurrentView = app.MainMenuView
	_ = state.View()
}

func TestQALayout_EdgeCase_Width0Height0_NoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Panic with Width=0, Height=0: %v", r)
		}
	}()

	state := newTestState("", "", 0, 0)
	state.CurrentView = app.MainMenuView
	_ = state.View()
}

func TestQALayout_EdgeCase_LargeTerminal(t *testing.T) {
	state := newTestState("", "", 300, 100)
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "ec2"
	state.Breadcrumbs = []string{"main", "EC2 Instances"}
	state.Resources = makeEC2Resources(20)

	view := state.View()
	if view.Content == "" {
		t.Error("Large terminal (300x100) should render content")
	}
	lines := countNonEmptyLines(view.Content)
	if lines > 100 {
		t.Errorf("Large terminal: output has %d lines but height is 100", lines)
	}
}

func TestQALayout_EdgeCase_NarrowTerminal(t *testing.T) {
	state := newTestState("", "", 40, 24)
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "ec2"
	state.Breadcrumbs = []string{"main", "EC2 Instances"}
	state.Resources = makeEC2Resources(5)

	view := state.View()
	if view.Content == "" {
		t.Error("Narrow terminal (40 wide) should still render content")
	}
	// Content should be present, even if truncated
	if !strings.Contains(view.Content, "server-000") && !strings.Contains(view.Content, "i-0") {
		t.Error("Narrow terminal should still show some resource data")
	}
}

func TestQALayout_EdgeCase_ResizeSmaller(t *testing.T) {
	// Start at 80x24
	state := newTestState("", "", 80, 24)
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "ec2"
	state.Breadcrumbs = []string{"main", "EC2 Instances"}
	state.Resources = makeEC2Resources(50)

	view1 := state.View()
	lines1 := countNonEmptyLines(view1.Content)
	if lines1 > 24 {
		t.Errorf("Before resize: output has %d lines but height is 24", lines1)
	}

	// Resize to 40x10
	state.Width = 40
	state.Height = 10

	view2 := state.View()
	lines2 := countNonEmptyLines(view2.Content)
	if lines2 > 10 {
		t.Errorf("After resize to 40x10: output has %d lines but height is 10", lines2)
	}
}

func TestQALayout_EdgeCase_AllViews_Width1Height1_NoPanic(t *testing.T) {
	viewSetups := []struct {
		name  string
		setup func() app.AppState
	}{
		{"MainMenu", func() app.AppState {
			return newTestState("", "", 1, 1)
		}},
		{"ResourceList", func() app.AppState {
			s := newTestState("", "", 1, 1)
			s.CurrentView = app.ResourceListView
			s.CurrentResourceType = "ec2"
			s.Breadcrumbs = []string{"main", "EC2 Instances"}
			s.Resources = makeEC2Resources(3)
			return s
		}},
		{"DetailView", func() app.AppState {
			s := newTestState("", "", 1, 1)
			s.CurrentView = app.DetailView
			s.CurrentResourceType = "ec2"
			s.Breadcrumbs = []string{"main", "EC2 Instances", "detail"}
			s.Detail = views.NewDetailModel("T", map[string]string{"K": "V"})
			s.Detail.Width = 1
			s.Detail.Height = 1
			return s
		}},
		{"JSONView", func() app.AppState {
			s := newTestState("", "", 1, 1)
			s.CurrentView = app.JSONView
			s.CurrentResourceType = "ec2"
			s.Breadcrumbs = []string{"main", "EC2 Instances", "json"}
			s.JSONData = views.NewJSONView("T", `{"a":1}`)
			s.JSONData.Width = 1
			s.JSONData.Height = 1
			return s
		}},
		{"RevealView", func() app.AppState {
			s := newTestState("", "", 1, 1)
			s.CurrentView = app.RevealView
			s.CurrentResourceType = "secrets"
			s.Breadcrumbs = []string{"main", "Secrets Manager", "reveal"}
			s.Reveal = views.NewRevealView("T", "val")
			s.Reveal.Width = 1
			s.Reveal.Height = 1
			return s
		}},
		{"ProfileSelect", func() app.AppState {
			s := newTestState("", "", 1, 1)
			s.CurrentView = app.ProfileSelectView
			s.Breadcrumbs = []string{"main", "profile"}
			s.ProfileSelector = views.NewProfileSelect([]string{"default"}, "default")
			return s
		}},
		{"RegionSelect", func() app.AppState {
			s := newTestState("", "", 1, 1)
			s.CurrentView = app.RegionSelectView
			s.Breadcrumbs = []string{"main", "region"}
			s.RegionSelector = views.NewRegionSelect(awsclient.AllRegions(), "us-east-1")
			return s
		}},
	}

	for _, tc := range viewSetups {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Panic in %s with Width=1, Height=1: %v", tc.name, r)
				}
			}()
			state := tc.setup()
			_ = state.View()
		})
	}
}

func TestQALayout_EdgeCase_AllViews_Width0Height0_NoPanic(t *testing.T) {
	viewSetups := []struct {
		name  string
		setup func() app.AppState
	}{
		{"MainMenu", func() app.AppState {
			return newTestState("", "", 0, 0)
		}},
		{"ResourceList", func() app.AppState {
			s := newTestState("", "", 0, 0)
			s.CurrentView = app.ResourceListView
			s.CurrentResourceType = "ec2"
			s.Breadcrumbs = []string{"main", "EC2 Instances"}
			s.Resources = makeEC2Resources(3)
			return s
		}},
		{"DetailView", func() app.AppState {
			s := newTestState("", "", 0, 0)
			s.CurrentView = app.DetailView
			s.CurrentResourceType = "ec2"
			s.Breadcrumbs = []string{"main", "EC2 Instances", "detail"}
			s.Detail = views.NewDetailModel("T", map[string]string{"K": "V"})
			return s
		}},
		{"JSONView", func() app.AppState {
			s := newTestState("", "", 0, 0)
			s.CurrentView = app.JSONView
			s.CurrentResourceType = "ec2"
			s.Breadcrumbs = []string{"main", "EC2 Instances", "json"}
			s.JSONData = views.NewJSONView("T", `{"a":1}`)
			return s
		}},
		{"RevealView", func() app.AppState {
			s := newTestState("", "", 0, 0)
			s.CurrentView = app.RevealView
			s.CurrentResourceType = "secrets"
			s.Breadcrumbs = []string{"main", "Secrets Manager", "reveal"}
			s.Reveal = views.NewRevealView("T", "val")
			return s
		}},
		{"ProfileSelect", func() app.AppState {
			s := newTestState("", "", 0, 0)
			s.CurrentView = app.ProfileSelectView
			s.Breadcrumbs = []string{"main", "profile"}
			s.ProfileSelector = views.NewProfileSelect([]string{"default"}, "default")
			return s
		}},
		{"RegionSelect", func() app.AppState {
			s := newTestState("", "", 0, 0)
			s.CurrentView = app.RegionSelectView
			s.Breadcrumbs = []string{"main", "region"}
			s.RegionSelector = views.NewRegionSelect(awsclient.AllRegions(), "us-east-1")
			return s
		}},
	}

	for _, tc := range viewSetups {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Panic in %s with Width=0, Height=0: %v", tc.name, r)
				}
			}()
			state := tc.setup()
			_ = state.View()
		})
	}
}

// ===========================================================================
// 11. Long content handling
// ===========================================================================

func TestQALayout_LongContent_ResourceWithLongName(t *testing.T) {
	longName := strings.Repeat("a", 200)
	state := newTestState("", "", 80, 24)
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "ec2"
	state.Breadcrumbs = []string{"main", "EC2 Instances"}
	state.Resources = []resource.Resource{
		{
			ID: "i-longname", Name: longName, Status: "running",
			Fields: map[string]string{
				"instance_id": "i-longname",
				"name":        longName,
				"state":       "running",
				"type":        "t3.medium",
				"private_ip":  "10.0.0.1",
				"public_ip":   "1.2.3.4",
				"launch_time": "2026-01-01",
			},
		},
	}

	view := state.View()
	// The long name should be truncated (max col width is 40), not wrapping
	lines := strings.Split(view.Content, "\n")
	for _, line := range lines {
		// Should not contain the full 200-char name — truncation should happen
		if strings.Contains(line, longName) {
			t.Error("200-char resource name should be truncated, not shown in full")
		}
	}
	// But some prefix should be visible
	// The padOrTruncate caps at maxColWidth=40, so first 39 chars + "..."
	if !strings.Contains(view.Content, strings.Repeat("a", 30)) {
		t.Error("Truncated long name should still show a visible prefix")
	}
}

func TestQALayout_LongContent_DetailView100Pairs(t *testing.T) {
	data := make(map[string]string)
	for i := 0; i < 100; i++ {
		data[fmt.Sprintf("Key%03d", i)] = fmt.Sprintf("Value%03d", i)
	}
	state := newTestState("", "", 80, 24)
	state.CurrentView = app.DetailView
	state.CurrentResourceType = "ec2"
	state.Breadcrumbs = []string{"main", "EC2 Instances", "detail"}
	state.Detail = views.NewDetailModel("Big Detail", data)
	state.Detail.Width = 80
	state.Detail.Height = 24

	view := state.View()
	lines := countNonEmptyLines(view.Content)
	// Should not exceed terminal height
	if lines > 24 {
		t.Errorf("DetailView with 100 pairs: output has %d lines but height is 24", lines)
	}
	// Should show at least some data
	if !strings.Contains(view.Content, "Key") {
		t.Error("DetailView with 100 pairs should show at least some key-value pairs")
	}
}

func TestQALayout_LongContent_JSONView500Lines(t *testing.T) {
	jsonLines := make([]string, 500)
	for i := range jsonLines {
		jsonLines[i] = fmt.Sprintf(`  "field%d": "value%d"`, i, i)
	}
	jsonContent := "{\n" + strings.Join(jsonLines, ",\n") + "\n}"

	state := newTestState("", "", 80, 24)
	state.CurrentView = app.JSONView
	state.CurrentResourceType = "ec2"
	state.Breadcrumbs = []string{"main", "EC2 Instances", "json"}
	state.JSONData = views.NewJSONView("Big JSON", jsonContent)
	state.JSONData.Width = 80
	state.JSONData.Height = 24

	view := state.View()
	lines := countNonEmptyLines(view.Content)
	if lines > 24 {
		t.Errorf("JSONView with 500 lines: output has %d lines but height is 24", lines)
	}
	// Should show some JSON content
	if !strings.Contains(view.Content, "field") {
		t.Error("JSONView with 500 lines should show at least some JSON content")
	}
}

func TestQALayout_LongContent_BreadcrumbsWith10Segments(t *testing.T) {
	state := newTestState("", "", 80, 24)
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "s3"
	state.Breadcrumbs = []string{
		"main", "S3 Buckets", "bucket-name", "folder1", "folder2",
		"folder3", "folder4", "folder5", "folder6", "folder7",
	}
	state.S3Bucket = "bucket-name"
	state.S3Prefix = "folder1/folder2/folder3/folder4/folder5/folder6/folder7/"
	state.Resources = makeS3Objects(3)

	view := state.View()
	lines := countNonEmptyLines(view.Content)
	if lines > 24 {
		t.Errorf("10-segment breadcrumbs: output has %d lines but height is 24", lines)
	}
	// Should still render something
	if view.Content == "" {
		t.Error("View with 10-segment breadcrumbs should not be empty")
	}
}

// ===========================================================================
// Additional: Height constraint with command/filter modes
// ===========================================================================

func TestQALayout_OutputHeight_CommandMode(t *testing.T) {
	for _, h := range []int{10, 20, 40} {
		t.Run(fmt.Sprintf("Height%d", h), func(t *testing.T) {
			state := newTestState("", "", 120, h)
			state.CurrentView = app.ResourceListView
			state.CurrentResourceType = "ec2"
			state.Breadcrumbs = []string{"main", "EC2 Instances"}
			state.Resources = makeEC2Resources(50)
			state.CommandMode = true
			state.CommandText = "rds"

			view := state.View()
			lines := countNonEmptyLines(view.Content)
			if lines > h {
				t.Errorf("CommandMode: output has %d lines but terminal height is %d", lines, h)
			}
		})
	}
}

func TestQALayout_OutputHeight_FilterMode(t *testing.T) {
	for _, h := range []int{10, 20, 40} {
		t.Run(fmt.Sprintf("Height%d", h), func(t *testing.T) {
			state := newTestState("", "", 120, h)
			state.CurrentView = app.ResourceListView
			state.CurrentResourceType = "ec2"
			state.Breadcrumbs = []string{"main", "EC2 Instances"}
			state.Resources = makeEC2Resources(50)
			state.FilterMode = true
			state.Filter = "server"
			state.FilteredResources = state.Resources

			view := state.View()
			lines := countNonEmptyLines(view.Content)
			if lines > h {
				t.Errorf("FilterMode: output has %d lines but terminal height is %d", lines, h)
			}
		})
	}
}

func TestQALayout_OutputHeight_ErrorStatus(t *testing.T) {
	for _, h := range []int{10, 20, 40} {
		t.Run(fmt.Sprintf("Height%d", h), func(t *testing.T) {
			state := newTestState("", "", 120, h)
			state.CurrentView = app.ResourceListView
			state.CurrentResourceType = "ec2"
			state.Breadcrumbs = []string{"main", "EC2 Instances"}
			state.Resources = makeEC2Resources(50)
			state.StatusMessage = "Error: credentials expired"
			state.StatusIsError = true

			view := state.View()
			lines := countNonEmptyLines(view.Content)
			if lines > h {
				t.Errorf("ErrorStatus: output has %d lines but terminal height is %d", lines, h)
			}
		})
	}
}

// ===========================================================================
// Cross-view consistency: header, breadcrumbs, status bar for all 7 resource
// types in ResourceListView
// ===========================================================================

func TestQALayout_ResourceListView_FullLayout_AllTypes(t *testing.T) {
	for _, rtName := range allResourceShortNames() {
		t.Run(rtName, func(t *testing.T) {
			rt := resource.FindResourceType(rtName)
			if rt == nil {
				t.Fatalf("Unknown resource type: %s", rtName)
			}

			state := newTestState("check-profile", "ap-southeast-1", 200, 30)
			state.CurrentView = app.ResourceListView
			state.CurrentResourceType = rtName
			state.Breadcrumbs = []string{"main", rt.Name}
			factory := resourceFactory[rtName]
			state.Resources = factory(10)

			view := state.View()
			content := view.Content

			// Header: profile + region
			first := firstNonEmptyLine(content)
			if !strings.Contains(first, "check-profile") {
				t.Errorf("%s: header missing profile, first line: %q", rtName, first)
			}
			if !strings.Contains(first, "ap-southeast-1") {
				t.Errorf("%s: header missing region, first line: %q", rtName, first)
			}

			// Breadcrumbs
			if !strings.Contains(content, rt.Name) {
				t.Errorf("%s: breadcrumbs should contain resource type name %q", rtName, rt.Name)
			}

			// Status bar
			last := lastNonEmptyLine(content)
			if !strings.Contains(last, "Ready") && !strings.Contains(last, "help") {
				t.Errorf("%s: status bar should show 'Ready', last line: %q", rtName, last)
			}

			// Height constraint
			lines := countNonEmptyLines(content)
			if lines > 30 {
				t.Errorf("%s: output has %d lines but height is 30", rtName, lines)
			}

			// Column headers
			for _, col := range rt.Columns {
				if !strings.Contains(content, col.Title) {
					t.Errorf("%s: missing column header %q", rtName, col.Title)
				}
			}

			// Cursor indicator
			if !strings.Contains(content, "> ") {
				t.Errorf("%s: missing cursor indicator '> '", rtName)
			}

			// At least one resource value visible
			firstRes := state.Resources[0]
			foundAnyValue := false
			for _, col := range rt.Columns {
				val := firstRes.Fields[col.Key]
				if val != "" {
					check := val
					if len(check) > 39 {
						check = check[:39]
					}
					if strings.Contains(content, check) {
						foundAnyValue = true
						break
					}
				}
			}
			if !foundAnyValue {
				t.Errorf("%s: no resource field values found in output", rtName)
			}
		})
	}
}

// ===========================================================================
// Height constraint with varying resource counts
// ===========================================================================

func TestQALayout_OutputHeight_ResourceList_VariousCounts(t *testing.T) {
	counts := []int{0, 1, 5, 10, 50, 100, 200}
	for _, count := range counts {
		for _, h := range []int{10, 24, 40} {
			t.Run(fmt.Sprintf("Count%d_Height%d", count, h), func(t *testing.T) {
				state := newTestState("", "", 120, h)
				state.CurrentView = app.ResourceListView
				state.CurrentResourceType = "ec2"
				state.Breadcrumbs = []string{"main", "EC2 Instances"}
				if count > 0 {
					state.Resources = makeEC2Resources(count)
				}

				view := state.View()
				lines := countNonEmptyLines(view.Content)
				if lines > h {
					t.Errorf("Count=%d, Height=%d: output has %d lines", count, h, lines)
				}
			})
		}
	}
}

// ===========================================================================
// Status bar content correctness across modes
// ===========================================================================

func TestQALayout_StatusBar_EmptyFilter(t *testing.T) {
	state := newTestState("", "", 80, 24)
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "ec2"
	state.Breadcrumbs = []string{"main", "EC2 Instances"}
	state.Resources = makeEC2Resources(5)
	state.FilterMode = true
	state.Filter = ""

	view := state.View()
	last := lastNonEmptyLine(view.Content)
	// Should show "/" indicator even with empty filter
	if !strings.Contains(last, "/") {
		t.Errorf("Empty filter mode should show '/' in status bar, got: %q", last)
	}
}

func TestQALayout_StatusBar_CommandAutoComplete(t *testing.T) {
	state := newTestState("", "", 80, 24)
	state.CurrentView = app.MainMenuView
	state.CommandMode = true
	state.CommandText = "rd"

	view := state.View()
	last := lastNonEmptyLine(view.Content)
	// Should show ":rd" with possible "s" suggestion
	if !strings.Contains(last, ":rd") {
		t.Errorf("Command mode with 'rd' should show ':rd' in status bar, got: %q", last)
	}
}

// ===========================================================================
// Scroll position does not cause rendering overflow
// ===========================================================================

func TestQALayout_ScrolledResourceList_StillFitsHeight(t *testing.T) {
	state := newTestState("", "", 120, 15)
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "ec2"
	state.Breadcrumbs = []string{"main", "EC2 Instances"}
	state.Resources = makeEC2Resources(100)
	state.SelectedIndex = 80

	view := state.View()
	lines := countNonEmptyLines(view.Content)
	if lines > 15 {
		t.Errorf("Scrolled to index 80: output has %d lines but height is 15", lines)
	}
	// The selected item should be visible
	if !strings.Contains(view.Content, "server-080") {
		t.Error("Item at index 80 should be visible after scrolling")
	}
}

func TestQALayout_ScrolledResourceList_FirstItemNotVisible(t *testing.T) {
	state := newTestState("", "", 120, 15)
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "ec2"
	state.Breadcrumbs = []string{"main", "EC2 Instances"}
	state.Resources = makeEC2Resources(100)
	state.SelectedIndex = 80

	view := state.View()
	if strings.Contains(view.Content, "server-000") {
		t.Error("Item at index 0 should NOT be visible when scrolled to index 80")
	}
}

// ===========================================================================
// S3 bucket vs. object column correctness
// ===========================================================================

func TestQALayout_S3BucketList_UsesBucketColumns(t *testing.T) {
	state := newTestState("", "", 200, 30)
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "s3"
	state.S3Bucket = ""
	state.Breadcrumbs = []string{"main", "S3 Buckets"}
	state.Resources = makeS3Buckets(5)

	view := state.View()
	content := view.Content

	// Should use bucket columns
	if !strings.Contains(content, "Bucket Name") {
		t.Error("S3 bucket list should show 'Bucket Name' column header")
	}
	if !strings.Contains(content, "Creation Date") {
		t.Error("S3 bucket list should show 'Creation Date' column header")
	}
	// Should NOT show object columns
	if strings.Contains(content, "Storage Class") {
		t.Error("S3 bucket list should NOT show 'Storage Class' object column")
	}
}

// ===========================================================================
// Header contains loading indicator when loading
// ===========================================================================

func TestQALayout_Header_LoadingIndicator(t *testing.T) {
	state := newTestState("myprofile", "us-east-1", 120, 24)
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "ec2"
	state.Breadcrumbs = []string{"main", "EC2 Instances"}
	state.Loading = true
	state.Resources = nil

	view := state.View()
	first := firstNonEmptyLine(view.Content)
	if !strings.Contains(strings.ToLower(first), "loading") {
		t.Errorf("Header should show loading indicator when Loading=true, got: %q", first)
	}
}

// ===========================================================================
// Multiple resource types at minimum viable height
// ===========================================================================

func TestQALayout_MinimumViableHeight_AllResourceTypes(t *testing.T) {
	// Height=5 is very small: header(1) + breadcrumbs(1) + 2 content + status(1)
	for _, rtName := range allResourceShortNames() {
		t.Run(rtName, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Panic in %s at Height=5: %v", rtName, r)
				}
			}()

			state := newTestState("", "", 120, 5)
			state.CurrentView = app.ResourceListView
			state.CurrentResourceType = rtName
			state.Breadcrumbs = []string{"main", rtName}
			factory := resourceFactory[rtName]
			state.Resources = factory(20)

			view := state.View()
			lines := countNonEmptyLines(view.Content)
			if lines > 5 {
				t.Errorf("%s at Height=5: output has %d lines", rtName, lines)
			}
		})
	}
}

// ===========================================================================
// Profile and region selectors with many items
// ===========================================================================

func TestQALayout_ProfileSelect_ManyProfiles_FitsHeight(t *testing.T) {
	profiles := make([]string, 50)
	for i := range profiles {
		profiles[i] = fmt.Sprintf("profile-%03d", i)
	}
	state := newTestState("", "", 80, 20)
	state.CurrentView = app.ProfileSelectView
	state.Breadcrumbs = []string{"main", "profile"}
	state.ProfileSelector = views.NewProfileSelect(profiles, "profile-000")

	view := state.View()
	lines := countNonEmptyLines(view.Content)
	if lines > 20 {
		t.Errorf("ProfileSelect with 50 profiles: output has %d lines but height is 20", lines)
	}
}

func TestQALayout_RegionSelect_AllRegions_FitsHeight(t *testing.T) {
	regions := awsclient.AllRegions()
	state := newTestState("", "", 80, 20)
	state.CurrentView = app.RegionSelectView
	state.Breadcrumbs = []string{"main", "region"}
	state.RegionSelector = views.NewRegionSelect(regions, "us-east-1")

	view := state.View()
	lines := countNonEmptyLines(view.Content)
	if lines > 20 {
		t.Errorf("RegionSelect with %d regions: output has %d lines but height is 20",
			len(regions), lines)
	}
}

// ===========================================================================
// Separator line present in resource list
// ===========================================================================

func TestQALayout_ResourceList_SeparatorLine(t *testing.T) {
	for _, rtName := range allResourceShortNames() {
		t.Run(rtName, func(t *testing.T) {
			state := newTestState("", "", 200, 30)
			state.CurrentView = app.ResourceListView
			state.CurrentResourceType = rtName
			state.Breadcrumbs = []string{"main", rtName}
			factory := resourceFactory[rtName]
			state.Resources = factory(3)

			view := state.View()
			// The separator uses "─" (box-drawing horizontal)
			if !strings.Contains(view.Content, "─") {
				t.Errorf("ResourceList(%s) should have separator line with '─' characters", rtName)
			}
		})
	}
}

// ===========================================================================
// Resource count displayed in title
// ===========================================================================

func TestQALayout_ResourceList_CountInTitle(t *testing.T) {
	for _, rtName := range allResourceShortNames() {
		t.Run(rtName, func(t *testing.T) {
			state := newTestState("", "", 200, 30)
			state.CurrentView = app.ResourceListView
			state.CurrentResourceType = rtName
			state.Breadcrumbs = []string{"main", rtName}
			factory := resourceFactory[rtName]
			state.Resources = factory(7)

			view := state.View()
			if !strings.Contains(view.Content, "(7)") {
				t.Errorf("ResourceList(%s) title should show count (7)", rtName)
			}
		})
	}
}

// ===========================================================================
// Verify that non-selected rows do NOT have cursor
// ===========================================================================

func TestQALayout_NonSelectedRows_NoCursor(t *testing.T) {
	state := newTestState("", "", 200, 30)
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "ec2"
	state.Breadcrumbs = []string{"main", "EC2 Instances"}
	state.Resources = makeEC2Resources(5)
	state.SelectedIndex = 0

	view := state.View()
	lines := strings.Split(view.Content, "\n")

	cursorCount := 0
	for _, line := range lines {
		// Check for "> " at the start of a data row (after optional ANSI)
		stripped := stripANSI(line)
		if strings.HasPrefix(stripped, "> ") {
			cursorCount++
		}
	}
	if cursorCount > 1 {
		t.Errorf("Only 1 row should have cursor '> ', found %d", cursorCount)
	}
}

// stripANSI removes ANSI escape sequences from a string for testing.
func stripANSI(s string) string {
	var result strings.Builder
	inEscape := false
	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEscape = false
			}
			continue
		}
		result.WriteRune(r)
	}
	return result.String()
}

// ===========================================================================
// Verify version string in header
// ===========================================================================

func TestQALayout_Header_ContainsVersion(t *testing.T) {
	state := newTestState("prof", "us-east-1", 80, 24)
	view := state.View()
	first := firstNonEmptyLine(view.Content)
	if !strings.Contains(first, "a9s") {
		t.Errorf("Header should contain 'a9s', got: %q", first)
	}
	if !strings.Contains(first, "v0.") {
		t.Errorf("Header should contain version string like 'v0.x', got: %q", first)
	}
}

// ===========================================================================
// Profile/Region selectors show active indicator
// ===========================================================================

func TestQALayout_ProfileSelect_ActiveIndicator(t *testing.T) {
	state := newTestState("", "", 80, 30)
	state.CurrentView = app.ProfileSelectView
	state.Breadcrumbs = []string{"main", "profile"}
	state.ProfileSelector = views.NewProfileSelect(
		[]string{"default", "staging", "prod"}, "staging",
	)

	view := state.View()
	// The active profile should be marked with "* "
	if !strings.Contains(view.Content, "* ") {
		t.Error("ProfileSelect should show '* ' indicator for active profile")
	}
	if !strings.Contains(view.Content, "staging") {
		t.Error("ProfileSelect should show the active profile name 'staging'")
	}
}

func TestQALayout_RegionSelect_ActiveIndicator(t *testing.T) {
	state := newTestState("", "", 80, 40)
	state.CurrentView = app.RegionSelectView
	state.Breadcrumbs = []string{"main", "region"}
	state.RegionSelector = views.NewRegionSelect(awsclient.AllRegions(), "eu-west-1")

	view := state.View()
	if !strings.Contains(view.Content, "* ") {
		t.Error("RegionSelect should show '* ' indicator for active region")
	}
	if !strings.Contains(view.Content, "eu-west-1") {
		t.Error("RegionSelect should show the active region 'eu-west-1'")
	}
}

// ===========================================================================
// Help instructions visible in MainMenu and selector views
// ===========================================================================

func TestQALayout_MainMenu_HelpInstructions(t *testing.T) {
	state := newTestState("", "", 80, 24)
	view := state.View()
	if !strings.Contains(view.Content, ":") || !strings.Contains(view.Content, "?") {
		t.Error("MainMenu should show help instructions mentioning ':' and '?'")
	}
}

func TestQALayout_ProfileSelect_Instructions(t *testing.T) {
	state := newTestState("", "", 80, 24)
	state.CurrentView = app.ProfileSelectView
	state.Breadcrumbs = []string{"main", "profile"}
	state.ProfileSelector = views.NewProfileSelect([]string{"default"}, "default")

	view := state.View()
	content := strings.ToLower(view.Content)
	if !strings.Contains(content, "enter") && !strings.Contains(content, "select") {
		t.Error("ProfileSelect should show instructions about Enter/select")
	}
}

func TestQALayout_RegionSelect_Instructions(t *testing.T) {
	state := newTestState("", "", 80, 40)
	state.CurrentView = app.RegionSelectView
	state.Breadcrumbs = []string{"main", "region"}
	state.RegionSelector = views.NewRegionSelect(awsclient.AllRegions(), "us-east-1")

	view := state.View()
	content := strings.ToLower(view.Content)
	if !strings.Contains(content, "enter") && !strings.Contains(content, "select") {
		t.Error("RegionSelect should show instructions about Enter/select")
	}
}

// ===========================================================================
// MainMenu shows all resource type names
// ===========================================================================

func TestQALayout_MainMenu_AllResourceTypesListed(t *testing.T) {
	state := newTestState("", "", 80, 40)
	view := state.View()
	allTypes := resource.AllResourceTypes()
	for _, rt := range allTypes {
		if !strings.Contains(view.Content, rt.Name) {
			t.Errorf("MainMenu should list resource type %q", rt.Name)
		}
	}
}

// ===========================================================================
// Region selector shows region codes and display names
// ===========================================================================

func TestQALayout_RegionSelect_ShowsCodesAndNames(t *testing.T) {
	regions := awsclient.AllRegions()
	state := newTestState("", "", 100, 60)
	state.CurrentView = app.RegionSelectView
	state.Breadcrumbs = []string{"main", "region"}
	state.RegionSelector = views.NewRegionSelect(regions, "us-east-1")

	view := state.View()
	// Check a few representative regions
	checkRegions := []struct{ code, name string }{
		{"us-east-1", "Virginia"},
		{"eu-west-1", "Ireland"},
		{"ap-northeast-1", "Tokyo"},
	}
	for _, r := range checkRegions {
		if !strings.Contains(view.Content, r.code) {
			t.Errorf("RegionSelect should show region code %q", r.code)
		}
		if !strings.Contains(view.Content, r.name) {
			t.Errorf("RegionSelect should show region display name containing %q", r.name)
		}
	}
}

// ===========================================================================
// Verify the "AWS Resources" title appears in MainMenu
// ===========================================================================

func TestQALayout_MainMenu_Title(t *testing.T) {
	state := newTestState("", "", 80, 24)
	view := state.View()
	if !strings.Contains(view.Content, "AWS Resources") {
		t.Error("MainMenu should show 'AWS Resources' title")
	}
}

// ===========================================================================
// DetailView title contains resource name
// ===========================================================================

func TestQALayout_DetailView_Title(t *testing.T) {
	state := newTestState("", "", 80, 24)
	state.CurrentView = app.DetailView
	state.CurrentResourceType = "ec2"
	state.Breadcrumbs = []string{"main", "EC2 Instances", "detail"}
	state.Detail = views.NewDetailModel("my-server - Detail", map[string]string{
		"Instance ID": "i-123",
		"Name":        "my-server",
	})
	state.Detail.Width = 80
	state.Detail.Height = 20

	view := state.View()
	if !strings.Contains(view.Content, "my-server") {
		t.Error("DetailView should show resource name in title")
	}
}

// ===========================================================================
// JSONView title contains resource name
// ===========================================================================

func TestQALayout_JSONView_Title(t *testing.T) {
	state := newTestState("", "", 80, 24)
	state.CurrentView = app.JSONView
	state.CurrentResourceType = "ec2"
	state.Breadcrumbs = []string{"main", "EC2 Instances", "json"}
	state.JSONData = views.NewJSONView("my-server - JSON", `{"InstanceId":"i-123"}`)
	state.JSONData.Width = 80
	state.JSONData.Height = 20

	view := state.View()
	if !strings.Contains(view.Content, "my-server") {
		t.Error("JSONView should show resource name in title")
	}
}

// ===========================================================================
// RevealView title contains secret name
// ===========================================================================

func TestQALayout_RevealView_Title(t *testing.T) {
	state := newTestState("", "", 80, 24)
	state.CurrentView = app.RevealView
	state.CurrentResourceType = "secrets"
	state.Breadcrumbs = []string{"main", "Secrets Manager", "reveal"}
	state.Reveal = views.NewRevealView("Secret: prod/db-password", "s3cr3t")
	state.Reveal.Width = 80
	state.Reveal.Height = 20

	view := state.View()
	if !strings.Contains(view.Content, "prod/db-password") {
		t.Error("RevealView should show secret name in title")
	}
}

// ===========================================================================
// Output stability: rendering same state twice gives identical output
// ===========================================================================

func TestQALayout_RenderIdempotent(t *testing.T) {
	state := newTestState("", "", 120, 24)
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "ec2"
	state.Breadcrumbs = []string{"main", "EC2 Instances"}
	state.Resources = makeEC2Resources(10)
	state.SelectedIndex = 3

	view1 := state.View()
	view2 := state.View()
	if view1.Content != view2.Content {
		t.Error("Rendering the same state twice should produce identical output")
	}
}
