package unit

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/internal/resource"
)

// ===========================================================================
// YAML fixture builders for v2.2.0 resource types
// ===========================================================================

func fixtureCFs() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "E1A2B3C4D5E6F7",
			Name:   "E1A2B3C4D5E6F7",
			Status: "Deployed",
			Fields: map[string]string{
				"distribution_id": "E1A2B3C4D5E6F7",
				"domain_name":     "d1234abcdef.cloudfront.net",
				"status":          "Deployed",
				"enabled":         "true",
				"aliases":         "cdn.example.com",
				"price_class":     "PriceClass_All",
			},
		},
	}
}

func fixtureR53s() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "/hostedzone/Z1234567890ABC",
			Name:   "example.com.",
			Status: "",
			Fields: map[string]string{
				"zone_id":      "/hostedzone/Z1234567890ABC",
				"name":         "example.com.",
				"record_count": "42",
				"private_zone": "false",
				"comment":      "Production hosted zone",
			},
		},
	}
}

func fixtureAPIGWs() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "abc123def4",
			Name:   "prod-api",
			Status: "",
			Fields: map[string]string{
				"api_id":      "abc123def4",
				"name":        "prod-api",
				"protocol":    "HTTP",
				"endpoint":    "https://abc123def4.execute-api.us-east-1.amazonaws.com",
				"description": "Production REST API",
			},
		},
	}
}

func fixtureECRs() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "my-app",
			Name:   "my-app",
			Status: "",
			Fields: map[string]string{
				"repository_name": "my-app",
				"uri":             "123456789012.dkr.ecr.us-east-1.amazonaws.com/my-app",
				"tag_mutability":  "IMMUTABLE",
				"scan_on_push":    "true",
				"created_at":      "2025-06-15 10:30:00",
			},
		},
	}
}

func fixtureEFSs() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "fs-0abc1234def56789a",
			Name:   "prod-efs",
			Status: "available",
			Fields: map[string]string{
				"file_system_id":   "fs-0abc1234def56789a",
				"name":             "prod-efs",
				"life_cycle_state": "available",
				"performance_mode": "generalPurpose",
				"encrypted":        "true",
				"mount_targets":    "3",
			},
		},
	}
}

func fixtureEBRules() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "daily-backup-rule",
			Name:   "daily-backup-rule",
			Status: "ENABLED",
			Fields: map[string]string{
				"name":        "daily-backup-rule",
				"state":       "ENABLED",
				"description": "Daily backup trigger",
				"event_bus":   "default",
				"schedule":    "cron(0 2 * * ? *)",
			},
		},
	}
}

func fixtureSFNs() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "order-processing",
			Name:   "order-processing",
			Status: "",
			Fields: map[string]string{
				"name":          "order-processing",
				"type":          "STANDARD",
				"arn":           "arn:aws:states:us-east-1:123456789012:stateMachine:order-processing",
				"creation_date": "2025-06-15 10:30:00",
			},
		},
	}
}

func fixturePipelines() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "deploy-pipeline",
			Name:   "deploy-pipeline",
			Status: "",
			Fields: map[string]string{
				"name":          "deploy-pipeline",
				"pipeline_type": "V2",
				"version":       "3",
				"created":       "2025-06-15 10:30:00",
				"updated":       "2025-06-15 10:30:00",
			},
		},
	}
}

func fixtureKinesisStreams() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "events-stream",
			Name:   "events-stream",
			Status: "ACTIVE",
			Fields: map[string]string{
				"stream_name":   "events-stream",
				"status":        "ACTIVE",
				"stream_mode":   "ON_DEMAND",
				"creation_time": "2025-06-15 10:30:00",
			},
		},
	}
}

func fixtureWAFs() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "a1b2c3d4-5678-90ab-cdef-EXAMPLE11111",
			Name:   "prod-waf-acl",
			Status: "",
			Fields: map[string]string{
				"name":        "prod-waf-acl",
				"id":          "a1b2c3d4-5678-90ab-cdef-EXAMPLE11111",
				"description": "Production WAF rules",
			},
		},
	}
}

func fixtureGlueJobs() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "etl-daily-job",
			Name:   "etl-daily-job",
			Status: "",
			Fields: map[string]string{
				"job_name":      "etl-daily-job",
				"glue_version":  "4.0",
				"worker_type":   "G.2X",
				"num_workers":   "10",
				"last_modified": "2025-06-15 10:30:00",
			},
		},
	}
}

func fixtureEBs() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "e-abc1234def",
			Name:   "prod-api-env",
			Status: "Ready",
			Fields: map[string]string{
				"environment_name": "prod-api-env",
				"application_name": "my-web-app",
				"status":           "Ready",
				"health":           "Green",
				"version_label":    "v1.2.3",
			},
		},
	}
}

func fixtureSESIdentities() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "user@example.com",
			Name:   "user@example.com",
			Status: "",
			Fields: map[string]string{
				"identity_name":       "user@example.com",
				"identity_type":       "EMAIL_ADDRESS",
				"sending_enabled":     "true",
				"verification_status": "SUCCESS",
			},
		},
	}
}

func fixtureRedshifts() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "analytics-cluster",
			Name:   "analytics-cluster",
			Status: "available",
			Fields: map[string]string{
				"cluster_id": "analytics-cluster",
				"status":     "available",
				"node_type":  "dc2.large",
				"num_nodes":  "4",
				"db_name":    "analytics_db",
				"endpoint":   "analytics-cluster.abc123.us-east-1.redshift.amazonaws.com",
			},
		},
	}
}

func fixtureTrails() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "org-trail",
			Name:   "org-trail",
			Status: "",
			Fields: map[string]string{
				"trail_name":   "org-trail",
				"s3_bucket":    "cloudtrail-logs-bucket",
				"home_region":  "us-east-1",
				"multi_region": "true",
			},
		},
	}
}

func fixtureAthenas() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "analytics-wg",
			Name:   "analytics-wg",
			Status: "ENABLED",
			Fields: map[string]string{
				"workgroup_name": "analytics-wg",
				"state":          "ENABLED",
				"description":    "Analytics workgroup",
				"engine_version": "Athena engine version 3",
			},
		},
	}
}

func fixtureCodeArtifacts() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "shared-libs",
			Name:   "shared-libs",
			Status: "",
			Fields: map[string]string{
				"repo_name":    "shared-libs",
				"domain_name":  "my-domain",
				"domain_owner": "123456789012",
				"description":  "Shared libraries repository",
			},
		},
	}
}

func fixtureCBs() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "build-project",
			Name:   "build-project",
			Status: "",
			Fields: map[string]string{
				"name":          "build-project",
				"source_type":   "CODECOMMIT",
				"description":   "CI build project",
				"last_modified": "2025-06-15T10:30:00Z",
			},
		},
	}
}

func fixtureOpenSearches() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "search-prod",
			Name:   "search-prod",
			Status: "",
			Fields: map[string]string{
				"domain_name":    "search-prod",
				"engine_version": "OpenSearch_2.11",
				"instance_type":  "r6g.large.search",
				"instance_count": "3",
				"endpoint":       "search-prod-abc123.us-east-1.es.amazonaws.com",
			},
		},
	}
}

func fixtureKMSKeys() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "12345678-1234-1234-1234-123456789012",
			Name:   "alias/prod-key",
			Status: "Enabled",
			Fields: map[string]string{
				"key_id":      "12345678-1234-1234-1234-123456789012",
				"alias":       "alias/prod-key",
				"status":      "Enabled",
				"description": "Production encryption key",
			},
		},
	}
}

func fixtureMSKs() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "events-kafka",
			Name:   "events-kafka",
			Status: "ACTIVE",
			Fields: map[string]string{
				"cluster_name": "events-kafka",
				"cluster_type": "PROVISIONED",
				"state":        "ACTIVE",
				"version":      "K3AEGXETSR30VB",
			},
		},
	}
}

func fixtureBackups() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "abc12345-1234-1234-1234-123456789012",
			Name:   "daily-backup-plan",
			Status: "",
			Fields: map[string]string{
				"plan_name":      "daily-backup-plan",
				"plan_id":        "abc12345-1234-1234-1234-123456789012",
				"creation_date":  "2025-06-15T10:30:00Z",
				"last_execution": "2025-06-15T10:30:00Z",
			},
		},
	}
}

// ===========================================================================
// 1. CF — YAML Tests
// ===========================================================================

func TestQA_YAML_CF_ViewContainsFields(t *testing.T) {
	for _, r := range fixtureCFs() {
		out := yamlView(t, r, 120, 40)
		for k, val := range r.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("CF YAML for %q missing key %q", r.ID, k)
			}
			if val != "" && !strings.Contains(out, val) {
				t.Errorf("CF YAML for %q missing value %q", r.ID, val)
			}
		}
	}
}

func TestQA_YAML_CF_FrameTitle(t *testing.T) {
	m := yamlModel(fixtureCFs()[0], 120, 40)
	if title := m.FrameTitle(); !strings.Contains(title, "yaml") {
		t.Errorf("CF FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_CF_RawContentUncolored(t *testing.T) {
	m := yamlModel(fixtureCFs()[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("CF RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// 2. R53 — YAML Tests
// ===========================================================================

func TestQA_YAML_R53_ViewContainsFields(t *testing.T) {
	for _, r := range fixtureR53s() {
		out := yamlView(t, r, 120, 40)
		for k, val := range r.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("R53 YAML for %q missing key %q", r.ID, k)
			}
			if val != "" && !strings.Contains(out, val) {
				t.Errorf("R53 YAML for %q missing value %q", r.ID, val)
			}
		}
	}
}

func TestQA_YAML_R53_FrameTitle(t *testing.T) {
	m := yamlModel(fixtureR53s()[0], 120, 40)
	if title := m.FrameTitle(); !strings.Contains(title, "yaml") {
		t.Errorf("R53 FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_R53_RawContentUncolored(t *testing.T) {
	m := yamlModel(fixtureR53s()[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("R53 RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// 3. APIGW — YAML Tests
// ===========================================================================

func TestQA_YAML_APIGW_ViewContainsFields(t *testing.T) {
	for _, r := range fixtureAPIGWs() {
		out := yamlView(t, r, 120, 40)
		for k, val := range r.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("APIGW YAML for %q missing key %q", r.ID, k)
			}
			if val != "" && !strings.Contains(out, val) {
				t.Errorf("APIGW YAML for %q missing value %q", r.ID, val)
			}
		}
	}
}

func TestQA_YAML_APIGW_FrameTitle(t *testing.T) {
	m := yamlModel(fixtureAPIGWs()[0], 120, 40)
	if title := m.FrameTitle(); !strings.Contains(title, "yaml") {
		t.Errorf("APIGW FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_APIGW_RawContentUncolored(t *testing.T) {
	m := yamlModel(fixtureAPIGWs()[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("APIGW RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// 4. ECR — YAML Tests
// ===========================================================================

func TestQA_YAML_ECR_ViewContainsFields(t *testing.T) {
	for _, r := range fixtureECRs() {
		out := yamlView(t, r, 120, 40)
		for k, val := range r.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("ECR YAML for %q missing key %q", r.ID, k)
			}
			if val != "" && !strings.Contains(out, val) {
				t.Errorf("ECR YAML for %q missing value %q", r.ID, val)
			}
		}
	}
}

func TestQA_YAML_ECR_FrameTitle(t *testing.T) {
	m := yamlModel(fixtureECRs()[0], 120, 40)
	if title := m.FrameTitle(); !strings.Contains(title, "yaml") {
		t.Errorf("ECR FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_ECR_RawContentUncolored(t *testing.T) {
	m := yamlModel(fixtureECRs()[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("ECR RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// 5. EFS — YAML Tests
// ===========================================================================

func TestQA_YAML_EFS_ViewContainsFields(t *testing.T) {
	for _, r := range fixtureEFSs() {
		out := yamlView(t, r, 120, 40)
		for k, val := range r.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("EFS YAML for %q missing key %q", r.ID, k)
			}
			if val != "" && !strings.Contains(out, val) {
				t.Errorf("EFS YAML for %q missing value %q", r.ID, val)
			}
		}
	}
}

func TestQA_YAML_EFS_FrameTitle(t *testing.T) {
	m := yamlModel(fixtureEFSs()[0], 120, 40)
	if title := m.FrameTitle(); !strings.Contains(title, "yaml") {
		t.Errorf("EFS FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_EFS_RawContentUncolored(t *testing.T) {
	m := yamlModel(fixtureEFSs()[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("EFS RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// 6. EBRule — YAML Tests
// ===========================================================================

func TestQA_YAML_EBRule_ViewContainsFields(t *testing.T) {
	for _, r := range fixtureEBRules() {
		out := yamlView(t, r, 120, 40)
		for k, val := range r.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("EBRule YAML for %q missing key %q", r.ID, k)
			}
			if val != "" && !strings.Contains(out, val) {
				t.Errorf("EBRule YAML for %q missing value %q", r.ID, val)
			}
		}
	}
}

func TestQA_YAML_EBRule_FrameTitle(t *testing.T) {
	m := yamlModel(fixtureEBRules()[0], 120, 40)
	if title := m.FrameTitle(); !strings.Contains(title, "yaml") {
		t.Errorf("EBRule FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_EBRule_RawContentUncolored(t *testing.T) {
	m := yamlModel(fixtureEBRules()[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("EBRule RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// 7. SFN — YAML Tests
// ===========================================================================

func TestQA_YAML_SFN_ViewContainsFields(t *testing.T) {
	for _, r := range fixtureSFNs() {
		out := yamlView(t, r, 120, 40)
		for k, val := range r.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("SFN YAML for %q missing key %q", r.ID, k)
			}
			if val != "" && !strings.Contains(out, val) {
				t.Errorf("SFN YAML for %q missing value %q", r.ID, val)
			}
		}
	}
}

func TestQA_YAML_SFN_FrameTitle(t *testing.T) {
	m := yamlModel(fixtureSFNs()[0], 120, 40)
	if title := m.FrameTitle(); !strings.Contains(title, "yaml") {
		t.Errorf("SFN FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_SFN_RawContentUncolored(t *testing.T) {
	m := yamlModel(fixtureSFNs()[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("SFN RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// 8. Pipeline — YAML Tests
// ===========================================================================

func TestQA_YAML_Pipeline_ViewContainsFields(t *testing.T) {
	for _, r := range fixturePipelines() {
		out := yamlView(t, r, 120, 40)
		for k, val := range r.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("Pipeline YAML for %q missing key %q", r.ID, k)
			}
			if val != "" && !strings.Contains(out, val) {
				t.Errorf("Pipeline YAML for %q missing value %q", r.ID, val)
			}
		}
	}
}

func TestQA_YAML_Pipeline_FrameTitle(t *testing.T) {
	m := yamlModel(fixturePipelines()[0], 120, 40)
	if title := m.FrameTitle(); !strings.Contains(title, "yaml") {
		t.Errorf("Pipeline FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_Pipeline_RawContentUncolored(t *testing.T) {
	m := yamlModel(fixturePipelines()[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("Pipeline RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// 9. Kinesis — YAML Tests
// ===========================================================================

func TestQA_YAML_Kinesis_ViewContainsFields(t *testing.T) {
	for _, r := range fixtureKinesisStreams() {
		out := yamlView(t, r, 120, 40)
		for k, val := range r.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("Kinesis YAML for %q missing key %q", r.ID, k)
			}
			if val != "" && !strings.Contains(out, val) {
				t.Errorf("Kinesis YAML for %q missing value %q", r.ID, val)
			}
		}
	}
}

func TestQA_YAML_Kinesis_FrameTitle(t *testing.T) {
	m := yamlModel(fixtureKinesisStreams()[0], 120, 40)
	if title := m.FrameTitle(); !strings.Contains(title, "yaml") {
		t.Errorf("Kinesis FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_Kinesis_RawContentUncolored(t *testing.T) {
	m := yamlModel(fixtureKinesisStreams()[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("Kinesis RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// 10. WAF — YAML Tests
// ===========================================================================

func TestQA_YAML_WAF_ViewContainsFields(t *testing.T) {
	for _, r := range fixtureWAFs() {
		out := yamlView(t, r, 120, 40)
		for k, val := range r.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("WAF YAML for %q missing key %q", r.ID, k)
			}
			if val != "" && !strings.Contains(out, val) {
				t.Errorf("WAF YAML for %q missing value %q", r.ID, val)
			}
		}
	}
}

func TestQA_YAML_WAF_FrameTitle(t *testing.T) {
	m := yamlModel(fixtureWAFs()[0], 120, 40)
	if title := m.FrameTitle(); !strings.Contains(title, "yaml") {
		t.Errorf("WAF FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_WAF_RawContentUncolored(t *testing.T) {
	m := yamlModel(fixtureWAFs()[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("WAF RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// 11. Glue — YAML Tests
// ===========================================================================

func TestQA_YAML_Glue_ViewContainsFields(t *testing.T) {
	for _, r := range fixtureGlueJobs() {
		out := yamlView(t, r, 120, 40)
		for k, val := range r.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("Glue YAML for %q missing key %q", r.ID, k)
			}
			if val != "" && !strings.Contains(out, val) {
				t.Errorf("Glue YAML for %q missing value %q", r.ID, val)
			}
		}
	}
}

func TestQA_YAML_Glue_FrameTitle(t *testing.T) {
	m := yamlModel(fixtureGlueJobs()[0], 120, 40)
	if title := m.FrameTitle(); !strings.Contains(title, "yaml") {
		t.Errorf("Glue FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_Glue_RawContentUncolored(t *testing.T) {
	m := yamlModel(fixtureGlueJobs()[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("Glue RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// 12. EB — YAML Tests
// ===========================================================================

func TestQA_YAML_EB_ViewContainsFields(t *testing.T) {
	for _, r := range fixtureEBs() {
		out := yamlView(t, r, 120, 40)
		for k, val := range r.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("EB YAML for %q missing key %q", r.ID, k)
			}
			if val != "" && !strings.Contains(out, val) {
				t.Errorf("EB YAML for %q missing value %q", r.ID, val)
			}
		}
	}
}

func TestQA_YAML_EB_FrameTitle(t *testing.T) {
	m := yamlModel(fixtureEBs()[0], 120, 40)
	if title := m.FrameTitle(); !strings.Contains(title, "yaml") {
		t.Errorf("EB FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_EB_RawContentUncolored(t *testing.T) {
	m := yamlModel(fixtureEBs()[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("EB RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// 13. SES — YAML Tests
// ===========================================================================

func TestQA_YAML_SES_ViewContainsFields(t *testing.T) {
	for _, r := range fixtureSESIdentities() {
		out := yamlView(t, r, 120, 40)
		for k, val := range r.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("SES YAML for %q missing key %q", r.ID, k)
			}
			if val != "" && !strings.Contains(out, val) {
				t.Errorf("SES YAML for %q missing value %q", r.ID, val)
			}
		}
	}
}

func TestQA_YAML_SES_FrameTitle(t *testing.T) {
	m := yamlModel(fixtureSESIdentities()[0], 120, 40)
	if title := m.FrameTitle(); !strings.Contains(title, "yaml") {
		t.Errorf("SES FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_SES_RawContentUncolored(t *testing.T) {
	m := yamlModel(fixtureSESIdentities()[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("SES RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// 14. Redshift — YAML Tests
// ===========================================================================

func TestQA_YAML_Redshift_ViewContainsFields(t *testing.T) {
	for _, r := range fixtureRedshifts() {
		out := yamlView(t, r, 120, 40)
		for k, val := range r.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("Redshift YAML for %q missing key %q", r.ID, k)
			}
			if val != "" && !strings.Contains(out, val) {
				t.Errorf("Redshift YAML for %q missing value %q", r.ID, val)
			}
		}
	}
}

func TestQA_YAML_Redshift_FrameTitle(t *testing.T) {
	m := yamlModel(fixtureRedshifts()[0], 120, 40)
	if title := m.FrameTitle(); !strings.Contains(title, "yaml") {
		t.Errorf("Redshift FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_Redshift_RawContentUncolored(t *testing.T) {
	m := yamlModel(fixtureRedshifts()[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("Redshift RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// 15. Trail — YAML Tests
// ===========================================================================

func TestQA_YAML_Trail_ViewContainsFields(t *testing.T) {
	for _, r := range fixtureTrails() {
		out := yamlView(t, r, 120, 40)
		for k, val := range r.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("Trail YAML for %q missing key %q", r.ID, k)
			}
			if val != "" && !strings.Contains(out, val) {
				t.Errorf("Trail YAML for %q missing value %q", r.ID, val)
			}
		}
	}
}

func TestQA_YAML_Trail_FrameTitle(t *testing.T) {
	m := yamlModel(fixtureTrails()[0], 120, 40)
	if title := m.FrameTitle(); !strings.Contains(title, "yaml") {
		t.Errorf("Trail FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_Trail_RawContentUncolored(t *testing.T) {
	m := yamlModel(fixtureTrails()[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("Trail RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// 16. Athena — YAML Tests
// ===========================================================================

func TestQA_YAML_Athena_ViewContainsFields(t *testing.T) {
	for _, r := range fixtureAthenas() {
		out := yamlView(t, r, 120, 40)
		for k, val := range r.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("Athena YAML for %q missing key %q", r.ID, k)
			}
			if val != "" && !strings.Contains(out, val) {
				t.Errorf("Athena YAML for %q missing value %q", r.ID, val)
			}
		}
	}
}

func TestQA_YAML_Athena_FrameTitle(t *testing.T) {
	m := yamlModel(fixtureAthenas()[0], 120, 40)
	if title := m.FrameTitle(); !strings.Contains(title, "yaml") {
		t.Errorf("Athena FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_Athena_RawContentUncolored(t *testing.T) {
	m := yamlModel(fixtureAthenas()[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("Athena RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// 17. CodeArtifact — YAML Tests
// ===========================================================================

func TestQA_YAML_CodeArtifact_ViewContainsFields(t *testing.T) {
	for _, r := range fixtureCodeArtifacts() {
		out := yamlView(t, r, 120, 40)
		for k, val := range r.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("CodeArtifact YAML for %q missing key %q", r.ID, k)
			}
			if val != "" && !strings.Contains(out, val) {
				t.Errorf("CodeArtifact YAML for %q missing value %q", r.ID, val)
			}
		}
	}
}

func TestQA_YAML_CodeArtifact_FrameTitle(t *testing.T) {
	m := yamlModel(fixtureCodeArtifacts()[0], 120, 40)
	if title := m.FrameTitle(); !strings.Contains(title, "yaml") {
		t.Errorf("CodeArtifact FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_CodeArtifact_RawContentUncolored(t *testing.T) {
	m := yamlModel(fixtureCodeArtifacts()[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("CodeArtifact RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// 18. CB — YAML Tests
// ===========================================================================

func TestQA_YAML_CB_ViewContainsFields(t *testing.T) {
	for _, r := range fixtureCBs() {
		out := yamlView(t, r, 120, 40)
		for k, val := range r.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("CB YAML for %q missing key %q", r.ID, k)
			}
			if val != "" && !strings.Contains(out, val) {
				t.Errorf("CB YAML for %q missing value %q", r.ID, val)
			}
		}
	}
}

func TestQA_YAML_CB_FrameTitle(t *testing.T) {
	m := yamlModel(fixtureCBs()[0], 120, 40)
	if title := m.FrameTitle(); !strings.Contains(title, "yaml") {
		t.Errorf("CB FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_CB_RawContentUncolored(t *testing.T) {
	m := yamlModel(fixtureCBs()[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("CB RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// 19. OpenSearch — YAML Tests
// ===========================================================================

func TestQA_YAML_OpenSearch_ViewContainsFields(t *testing.T) {
	for _, r := range fixtureOpenSearches() {
		out := yamlView(t, r, 120, 40)
		for k, val := range r.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("OpenSearch YAML for %q missing key %q", r.ID, k)
			}
			if val != "" && !strings.Contains(out, val) {
				t.Errorf("OpenSearch YAML for %q missing value %q", r.ID, val)
			}
		}
	}
}

func TestQA_YAML_OpenSearch_FrameTitle(t *testing.T) {
	m := yamlModel(fixtureOpenSearches()[0], 120, 40)
	if title := m.FrameTitle(); !strings.Contains(title, "yaml") {
		t.Errorf("OpenSearch FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_OpenSearch_RawContentUncolored(t *testing.T) {
	m := yamlModel(fixtureOpenSearches()[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("OpenSearch RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// 20. KMS — YAML Tests
// ===========================================================================

func TestQA_YAML_KMS_ViewContainsFields(t *testing.T) {
	for _, r := range fixtureKMSKeys() {
		out := yamlView(t, r, 120, 40)
		for k, val := range r.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("KMS YAML for %q missing key %q", r.ID, k)
			}
			if val != "" && !strings.Contains(out, val) {
				t.Errorf("KMS YAML for %q missing value %q", r.ID, val)
			}
		}
	}
}

func TestQA_YAML_KMS_FrameTitle(t *testing.T) {
	m := yamlModel(fixtureKMSKeys()[0], 120, 40)
	if title := m.FrameTitle(); !strings.Contains(title, "yaml") {
		t.Errorf("KMS FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_KMS_RawContentUncolored(t *testing.T) {
	m := yamlModel(fixtureKMSKeys()[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("KMS RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// 21. MSK — YAML Tests
// ===========================================================================

func TestQA_YAML_MSK_ViewContainsFields(t *testing.T) {
	for _, r := range fixtureMSKs() {
		out := yamlView(t, r, 120, 40)
		for k, val := range r.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("MSK YAML for %q missing key %q", r.ID, k)
			}
			if val != "" && !strings.Contains(out, val) {
				t.Errorf("MSK YAML for %q missing value %q", r.ID, val)
			}
		}
	}
}

func TestQA_YAML_MSK_FrameTitle(t *testing.T) {
	m := yamlModel(fixtureMSKs()[0], 120, 40)
	if title := m.FrameTitle(); !strings.Contains(title, "yaml") {
		t.Errorf("MSK FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_MSK_RawContentUncolored(t *testing.T) {
	m := yamlModel(fixtureMSKs()[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("MSK RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// 22. Backup — YAML Tests
// ===========================================================================

func TestQA_YAML_Backup_ViewContainsFields(t *testing.T) {
	for _, r := range fixtureBackups() {
		out := yamlView(t, r, 120, 40)
		for k, val := range r.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("Backup YAML for %q missing key %q", r.ID, k)
			}
			if val != "" && !strings.Contains(out, val) {
				t.Errorf("Backup YAML for %q missing value %q", r.ID, val)
			}
		}
	}
}

func TestQA_YAML_Backup_FrameTitle(t *testing.T) {
	m := yamlModel(fixtureBackups()[0], 120, 40)
	if title := m.FrameTitle(); !strings.Contains(title, "yaml") {
		t.Errorf("Backup FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_Backup_RawContentUncolored(t *testing.T) {
	m := yamlModel(fixtureBackups()[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("Backup RawContent() contains ANSI codes, expected plain YAML")
	}
}
