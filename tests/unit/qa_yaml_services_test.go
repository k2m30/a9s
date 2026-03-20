package unit

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/internal/resource"
)

// ===========================================================================
// YAML fixture builders — return []resource.Resource with Fields map populated
// Suffixed with ForYAML to avoid name collisions with other test fixtures.
// ===========================================================================

// 1. Lambda
func fixtureLambdaForYAML() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "my-api-handler",
			Name:   "my-api-handler",
			Status: "",
			Fields: map[string]string{
				"function_name": "my-api-handler",
				"runtime":       "nodejs20.x",
				"memory":        "256",
				"timeout":       "30",
				"handler":       "index.handler",
				"last_modified": "2026-03-01T12:00:00Z",
			},
		},
		{
			ID:     "data-processor",
			Name:   "data-processor",
			Status: "",
			Fields: map[string]string{
				"function_name": "data-processor",
				"runtime":       "python3.12",
				"memory":        "1024",
				"timeout":       "300",
				"handler":       "app.lambda_handler",
				"last_modified": "2026-02-15T08:30:00Z",
			},
		},
	}
}

// 2. Alarm
func fixtureAlarmForYAML() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "HighCPUAlarm",
			Name:   "HighCPUAlarm",
			Status: "OK",
			Fields: map[string]string{
				"alarm_name":  "HighCPUAlarm",
				"state":       "OK",
				"metric_name": "CPUUtilization",
				"namespace":   "AWS/EC2",
				"threshold":   "80",
			},
		},
		{
			ID:     "DiskSpaceLow",
			Name:   "DiskSpaceLow",
			Status: "ALARM",
			Fields: map[string]string{
				"alarm_name":  "DiskSpaceLow",
				"state":       "ALARM",
				"metric_name": "DiskSpaceUtilization",
				"namespace":   "CWAgent",
				"threshold":   "90",
			},
		},
	}
}

// 3. SNS Topic
func fixtureSNSTopicForYAML() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "arn:aws:sns:us-east-1:123456789012:alerts",
			Name:   "alerts",
			Status: "",
			Fields: map[string]string{
				"display_name": "alerts",
				"topic_arn":    "arn:aws:sns:us-east-1:123456789012:alerts",
			},
		},
	}
}

// 4. SQS
func fixtureSQSForYAML() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "order-processing",
			Name:   "order-processing",
			Status: "",
			Fields: map[string]string{
				"queue_name":         "order-processing",
				"approx_messages":    "142",
				"approx_not_visible": "5",
				"delay_seconds":      "0",
				"queue_url":          "https://sqs.us-east-1.amazonaws.com/123456789012/order-processing",
			},
		},
	}
}

// 5. ELB
func fixtureELBForYAML() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/api-lb/50dc6c495c0c9188",
			Name:   "api-lb",
			Status: "active",
			Fields: map[string]string{
				"name":     "api-lb",
				"dns_name": "api-lb-123456789.us-east-1.elb.amazonaws.com",
				"type":     "application",
				"scheme":   "internet-facing",
				"state":    "active",
				"vpc_id":   "vpc-0abc1234",
			},
		},
	}
}

// 6. Target Group
func fixtureTGForYAML() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/api-targets/50dc6c495c0c9188",
			Name:   "api-targets",
			Status: "",
			Fields: map[string]string{
				"target_group_name": "api-targets",
				"port":              "8080",
				"protocol":          "HTTP",
				"vpc_id":            "vpc-0abc1234",
				"target_type":       "instance",
				"health_check_path": "/health",
			},
		},
	}
}

// 7. ECS Cluster
func fixtureECSClusterForYAML() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "arn:aws:ecs:us-east-1:123456789012:cluster/prod-cluster",
			Name:   "prod-cluster",
			Status: "ACTIVE",
			Fields: map[string]string{
				"cluster_name":   "prod-cluster",
				"status":         "ACTIVE",
				"running_tasks":  "12",
				"pending_tasks":  "0",
				"services_count": "4",
			},
		},
	}
}

// 8. ECS Service
func fixtureECSSvcForYAML() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "arn:aws:ecs:us-east-1:123456789012:service/prod-cluster/api-service",
			Name:   "api-service",
			Status: "ACTIVE",
			Fields: map[string]string{
				"service_name":  "api-service",
				"cluster":       "prod-cluster",
				"status":        "ACTIVE",
				"desired_count": "3",
				"running_count": "3",
				"launch_type":   "FARGATE",
			},
		},
	}
}

// 9. ECS Task
func fixtureECSTaskForYAML() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "a1b2c3d4e5f6",
			Name:   "a1b2c3d4e5f6",
			Status: "RUNNING",
			Fields: map[string]string{
				"task_id":         "a1b2c3d4e5f6",
				"cluster":         "prod-cluster",
				"status":          "RUNNING",
				"task_definition": "api-service:42",
				"launch_type":     "FARGATE",
				"cpu":             "256",
				"memory":          "512",
			},
		},
	}
}

// 10. CloudFormation Stack
func fixtureCFNForYAML() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "arn:aws:cloudformation:us-east-1:123456789012:stack/prod-infra/a1b2c3d4",
			Name:   "prod-infra",
			Status: "CREATE_COMPLETE",
			Fields: map[string]string{
				"stack_name":   "prod-infra",
				"status":       "CREATE_COMPLETE",
				"creation_time": "2026-01-15T10:00:00Z",
				"last_updated": "2026-03-01T14:30:00Z",
				"description":  "Production infrastructure stack",
			},
		},
	}
}

// 11. IAM Role
func fixtureRoleForYAML() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "AROA1234567890EXAMPLE",
			Name:   "lambda-execution-role",
			Status: "",
			Fields: map[string]string{
				"role_name":   "lambda-execution-role",
				"role_id":     "AROA1234567890EXAMPLE",
				"path":        "/service-role/",
				"create_date": "2025-06-01T09:00:00Z",
				"description": "Role for Lambda function execution",
			},
		},
	}
}

// 12. CloudWatch Log Groups
func fixtureLogsForYAML() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "/aws/lambda/my-api-handler",
			Name:   "/aws/lambda/my-api-handler",
			Status: "",
			Fields: map[string]string{
				"log_group_name": "/aws/lambda/my-api-handler",
				"stored_bytes":   "1048576",
				"retention_days": "30",
				"creation_time":  "1706745600000",
			},
		},
	}
}

// 13. SSM Parameters
func fixtureSSMForYAML() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "/app/prod/db-password",
			Name:   "/app/prod/db-password",
			Status: "",
			Fields: map[string]string{
				"name":          "/app/prod/db-password",
				"type":          "SecureString",
				"version":       "3",
				"last_modified": "2026-02-28T16:00:00Z",
				"description":   "Production database password",
			},
		},
	}
}

// 14. DynamoDB Tables
func fixtureDDBForYAML() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "user-sessions",
			Name:   "user-sessions",
			Status: "ACTIVE",
			Fields: map[string]string{
				"table_name":   "user-sessions",
				"status":       "ACTIVE",
				"item_count":   "1500000",
				"size_bytes":   "524288000",
				"billing_mode": "PAY_PER_REQUEST",
			},
		},
	}
}

// 15. ACM Certificates
func fixtureACMForYAML() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "arn:aws:acm:us-east-1:123456789012:certificate/a1b2c3d4-5678-90ab-cdef-EXAMPLE",
			Name:   "example.com",
			Status: "ISSUED",
			Fields: map[string]string{
				"domain_name": "example.com",
				"status":      "ISSUED",
				"type":        "AMAZON_ISSUED",
				"not_after":   "2027-03-20T00:00:00Z",
				"in_use":      "Yes",
			},
		},
	}
}

// 16. Auto Scaling Groups
func fixtureASGForYAML() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "prod-api-asg",
			Name:   "prod-api-asg",
			Status: "",
			Fields: map[string]string{
				"asg_name":  "prod-api-asg",
				"min_size":  "2",
				"max_size":  "10",
				"desired":   "4",
				"instances": "4",
				"status":    "InService",
			},
		},
	}
}

// 17. IAM Users
func fixtureIAMUserForYAML() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "AIDA1234567890EXAMPLE",
			Name:   "deploy-bot",
			Status: "",
			Fields: map[string]string{
				"user_name":          "deploy-bot",
				"user_id":            "AIDA1234567890EXAMPLE",
				"path":               "/",
				"create_date":        "2025-01-15T12:00:00Z",
				"password_last_used": "2026-03-19T08:00:00Z",
			},
		},
	}
}

// 18. IAM Groups
func fixtureIAMGroupForYAML() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "AGPA1234567890EXAMPLE",
			Name:   "developers",
			Status: "",
			Fields: map[string]string{
				"group_name":  "developers",
				"group_id":    "AGPA1234567890EXAMPLE",
				"path":        "/",
				"create_date": "2024-06-01T09:00:00Z",
				"arn":         "arn:aws:iam::123456789012:group/developers",
			},
		},
	}
}

// ===========================================================================
// 1. Lambda — YAML Tests
// ===========================================================================

func TestQA_YAML_Lambda_ViewContainsFields(t *testing.T) {
	lambdas := fixtureLambdaForYAML()
	for _, fn := range lambdas {
		out := yamlView(t, fn, 120, 40)
		for k, v := range fn.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("Lambda YAML for %q missing key %q", fn.ID, k)
			}
			if v != "" && !strings.Contains(out, v) {
				t.Errorf("Lambda YAML for %q missing value %q", fn.ID, v)
			}
		}
	}
}

func TestQA_YAML_Lambda_FrameTitle(t *testing.T) {
	lambdas := fixtureLambdaForYAML()
	m := yamlModel(lambdas[0], 120, 40)
	title := m.FrameTitle()
	if !strings.Contains(title, "yaml") {
		t.Errorf("Lambda FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_Lambda_RawContentUncolored(t *testing.T) {
	lambdas := fixtureLambdaForYAML()
	m := yamlModel(lambdas[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("Lambda RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// 2. Alarm — YAML Tests
// ===========================================================================

func TestQA_YAML_Alarm_ViewContainsFields(t *testing.T) {
	alarms := fixtureAlarmForYAML()
	for _, a := range alarms {
		out := yamlView(t, a, 120, 40)
		for k, v := range a.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("Alarm YAML for %q missing key %q", a.ID, k)
			}
			if v != "" && !strings.Contains(out, v) {
				t.Errorf("Alarm YAML for %q missing value %q", a.ID, v)
			}
		}
	}
}

func TestQA_YAML_Alarm_FrameTitle(t *testing.T) {
	alarms := fixtureAlarmForYAML()
	m := yamlModel(alarms[0], 120, 40)
	title := m.FrameTitle()
	if !strings.Contains(title, "yaml") {
		t.Errorf("Alarm FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_Alarm_RawContentUncolored(t *testing.T) {
	alarms := fixtureAlarmForYAML()
	m := yamlModel(alarms[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("Alarm RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// 3. SNS Topic — YAML Tests
// ===========================================================================

func TestQA_YAML_SNSTopic_ViewContainsFields(t *testing.T) {
	topics := fixtureSNSTopicForYAML()
	for _, tp := range topics {
		out := yamlView(t, tp, 120, 40)
		for k, v := range tp.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("SNS Topic YAML for %q missing key %q", tp.ID, k)
			}
			if v != "" && !strings.Contains(out, v) {
				t.Errorf("SNS Topic YAML for %q missing value %q", tp.ID, v)
			}
		}
	}
}

func TestQA_YAML_SNSTopic_FrameTitle(t *testing.T) {
	topics := fixtureSNSTopicForYAML()
	m := yamlModel(topics[0], 120, 40)
	title := m.FrameTitle()
	if !strings.Contains(title, "yaml") {
		t.Errorf("SNSTopic FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_SNSTopic_RawContentUncolored(t *testing.T) {
	topics := fixtureSNSTopicForYAML()
	m := yamlModel(topics[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("SNSTopic RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// 4. SQS — YAML Tests
// ===========================================================================

func TestQA_YAML_SQS_ViewContainsFields(t *testing.T) {
	queues := fixtureSQSForYAML()
	for _, q := range queues {
		out := yamlView(t, q, 120, 40)
		for k, v := range q.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("SQS YAML for %q missing key %q", q.ID, k)
			}
			if v != "" && !strings.Contains(out, v) {
				t.Errorf("SQS YAML for %q missing value %q", q.ID, v)
			}
		}
	}
}

func TestQA_YAML_SQS_FrameTitle(t *testing.T) {
	queues := fixtureSQSForYAML()
	m := yamlModel(queues[0], 120, 40)
	title := m.FrameTitle()
	if !strings.Contains(title, "yaml") {
		t.Errorf("SQS FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_SQS_RawContentUncolored(t *testing.T) {
	queues := fixtureSQSForYAML()
	m := yamlModel(queues[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("SQS RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// 5. ELB — YAML Tests
// ===========================================================================

func TestQA_YAML_ELB_ViewContainsFields(t *testing.T) {
	elbs := fixtureELBForYAML()
	for _, lb := range elbs {
		out := yamlView(t, lb, 120, 40)
		for k, v := range lb.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("ELB YAML for %q missing key %q", lb.ID, k)
			}
			if v != "" && !strings.Contains(out, v) {
				t.Errorf("ELB YAML for %q missing value %q", lb.ID, v)
			}
		}
	}
}

func TestQA_YAML_ELB_FrameTitle(t *testing.T) {
	elbs := fixtureELBForYAML()
	m := yamlModel(elbs[0], 120, 40)
	title := m.FrameTitle()
	if !strings.Contains(title, "yaml") {
		t.Errorf("ELB FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_ELB_RawContentUncolored(t *testing.T) {
	elbs := fixtureELBForYAML()
	m := yamlModel(elbs[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("ELB RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// 6. Target Group — YAML Tests
// ===========================================================================

func TestQA_YAML_TG_ViewContainsFields(t *testing.T) {
	tgs := fixtureTGForYAML()
	for _, tg := range tgs {
		out := yamlView(t, tg, 120, 40)
		for k, v := range tg.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("TG YAML for %q missing key %q", tg.ID, k)
			}
			if v != "" && !strings.Contains(out, v) {
				t.Errorf("TG YAML for %q missing value %q", tg.ID, v)
			}
		}
	}
}

func TestQA_YAML_TG_FrameTitle(t *testing.T) {
	tgs := fixtureTGForYAML()
	m := yamlModel(tgs[0], 120, 40)
	title := m.FrameTitle()
	if !strings.Contains(title, "yaml") {
		t.Errorf("TG FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_TG_RawContentUncolored(t *testing.T) {
	tgs := fixtureTGForYAML()
	m := yamlModel(tgs[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("TG RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// 7. ECS Cluster — YAML Tests
// ===========================================================================

func TestQA_YAML_ECSCluster_ViewContainsFields(t *testing.T) {
	clusters := fixtureECSClusterForYAML()
	for _, c := range clusters {
		out := yamlView(t, c, 120, 40)
		for k, v := range c.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("ECSCluster YAML for %q missing key %q", c.ID, k)
			}
			if v != "" && !strings.Contains(out, v) {
				t.Errorf("ECSCluster YAML for %q missing value %q", c.ID, v)
			}
		}
	}
}

func TestQA_YAML_ECSCluster_FrameTitle(t *testing.T) {
	clusters := fixtureECSClusterForYAML()
	m := yamlModel(clusters[0], 120, 40)
	title := m.FrameTitle()
	if !strings.Contains(title, "yaml") {
		t.Errorf("ECSCluster FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_ECSCluster_RawContentUncolored(t *testing.T) {
	clusters := fixtureECSClusterForYAML()
	m := yamlModel(clusters[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("ECSCluster RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// 8. ECS Service — YAML Tests
// ===========================================================================

func TestQA_YAML_ECSSvc_ViewContainsFields(t *testing.T) {
	svcs := fixtureECSSvcForYAML()
	for _, s := range svcs {
		out := yamlView(t, s, 120, 40)
		for k, v := range s.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("ECSSvc YAML for %q missing key %q", s.ID, k)
			}
			if v != "" && !strings.Contains(out, v) {
				t.Errorf("ECSSvc YAML for %q missing value %q", s.ID, v)
			}
		}
	}
}

func TestQA_YAML_ECSSvc_FrameTitle(t *testing.T) {
	svcs := fixtureECSSvcForYAML()
	m := yamlModel(svcs[0], 120, 40)
	title := m.FrameTitle()
	if !strings.Contains(title, "yaml") {
		t.Errorf("ECSSvc FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_ECSSvc_RawContentUncolored(t *testing.T) {
	svcs := fixtureECSSvcForYAML()
	m := yamlModel(svcs[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("ECSSvc RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// 9. ECS Task — YAML Tests
// ===========================================================================

func TestQA_YAML_ECSTask_ViewContainsFields(t *testing.T) {
	tasks := fixtureECSTaskForYAML()
	for _, tk := range tasks {
		out := yamlView(t, tk, 120, 40)
		for k, v := range tk.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("ECSTask YAML for %q missing key %q", tk.ID, k)
			}
			if v != "" && !strings.Contains(out, v) {
				t.Errorf("ECSTask YAML for %q missing value %q", tk.ID, v)
			}
		}
	}
}

func TestQA_YAML_ECSTask_FrameTitle(t *testing.T) {
	tasks := fixtureECSTaskForYAML()
	m := yamlModel(tasks[0], 120, 40)
	title := m.FrameTitle()
	if !strings.Contains(title, "yaml") {
		t.Errorf("ECSTask FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_ECSTask_RawContentUncolored(t *testing.T) {
	tasks := fixtureECSTaskForYAML()
	m := yamlModel(tasks[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("ECSTask RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// 10. CloudFormation Stack — YAML Tests
// ===========================================================================

func TestQA_YAML_CFN_ViewContainsFields(t *testing.T) {
	stacks := fixtureCFNForYAML()
	for _, s := range stacks {
		out := yamlView(t, s, 120, 40)
		for k, v := range s.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("CFN YAML for %q missing key %q", s.ID, k)
			}
			if v != "" && !strings.Contains(out, v) {
				t.Errorf("CFN YAML for %q missing value %q", s.ID, v)
			}
		}
	}
}

func TestQA_YAML_CFN_FrameTitle(t *testing.T) {
	stacks := fixtureCFNForYAML()
	m := yamlModel(stacks[0], 120, 40)
	title := m.FrameTitle()
	if !strings.Contains(title, "yaml") {
		t.Errorf("CFN FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_CFN_RawContentUncolored(t *testing.T) {
	stacks := fixtureCFNForYAML()
	m := yamlModel(stacks[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("CFN RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// 11. IAM Role — YAML Tests
// ===========================================================================

func TestQA_YAML_Role_ViewContainsFields(t *testing.T) {
	roles := fixtureRoleForYAML()
	for _, r := range roles {
		out := yamlView(t, r, 120, 40)
		for k, v := range r.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("Role YAML for %q missing key %q", r.ID, k)
			}
			if v != "" && !strings.Contains(out, v) {
				t.Errorf("Role YAML for %q missing value %q", r.ID, v)
			}
		}
	}
}

func TestQA_YAML_Role_FrameTitle(t *testing.T) {
	roles := fixtureRoleForYAML()
	m := yamlModel(roles[0], 120, 40)
	title := m.FrameTitle()
	if !strings.Contains(title, "yaml") {
		t.Errorf("Role FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_Role_RawContentUncolored(t *testing.T) {
	roles := fixtureRoleForYAML()
	m := yamlModel(roles[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("Role RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// 12. CloudWatch Log Groups — YAML Tests
// ===========================================================================

func TestQA_YAML_Logs_ViewContainsFields(t *testing.T) {
	logs := fixtureLogsForYAML()
	for _, lg := range logs {
		out := yamlView(t, lg, 120, 40)
		for k, v := range lg.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("Logs YAML for %q missing key %q", lg.ID, k)
			}
			if v != "" && !strings.Contains(out, v) {
				t.Errorf("Logs YAML for %q missing value %q", lg.ID, v)
			}
		}
	}
}

func TestQA_YAML_Logs_FrameTitle(t *testing.T) {
	logs := fixtureLogsForYAML()
	m := yamlModel(logs[0], 120, 40)
	title := m.FrameTitle()
	if !strings.Contains(title, "yaml") {
		t.Errorf("Logs FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_Logs_RawContentUncolored(t *testing.T) {
	logs := fixtureLogsForYAML()
	m := yamlModel(logs[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("Logs RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// 13. SSM Parameters — YAML Tests
// ===========================================================================

func TestQA_YAML_SSM_ViewContainsFields(t *testing.T) {
	params := fixtureSSMForYAML()
	for _, p := range params {
		out := yamlView(t, p, 120, 40)
		for k, v := range p.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("SSM YAML for %q missing key %q", p.ID, k)
			}
			if v != "" && !strings.Contains(out, v) {
				t.Errorf("SSM YAML for %q missing value %q", p.ID, v)
			}
		}
	}
}

func TestQA_YAML_SSM_FrameTitle(t *testing.T) {
	params := fixtureSSMForYAML()
	m := yamlModel(params[0], 120, 40)
	title := m.FrameTitle()
	if !strings.Contains(title, "yaml") {
		t.Errorf("SSM FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_SSM_RawContentUncolored(t *testing.T) {
	params := fixtureSSMForYAML()
	m := yamlModel(params[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("SSM RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// 14. DynamoDB Tables — YAML Tests
// ===========================================================================

func TestQA_YAML_DDB_ViewContainsFields(t *testing.T) {
	tables := fixtureDDBForYAML()
	for _, tb := range tables {
		out := yamlView(t, tb, 120, 40)
		for k, v := range tb.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("DDB YAML for %q missing key %q", tb.ID, k)
			}
			if v != "" && !strings.Contains(out, v) {
				t.Errorf("DDB YAML for %q missing value %q", tb.ID, v)
			}
		}
	}
}

func TestQA_YAML_DDB_FrameTitle(t *testing.T) {
	tables := fixtureDDBForYAML()
	m := yamlModel(tables[0], 120, 40)
	title := m.FrameTitle()
	if !strings.Contains(title, "yaml") {
		t.Errorf("DDB FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_DDB_RawContentUncolored(t *testing.T) {
	tables := fixtureDDBForYAML()
	m := yamlModel(tables[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("DDB RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// 15. ACM Certificates — YAML Tests
// ===========================================================================

func TestQA_YAML_ACM_ViewContainsFields(t *testing.T) {
	certs := fixtureACMForYAML()
	for _, c := range certs {
		out := yamlView(t, c, 120, 40)
		for k, v := range c.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("ACM YAML for %q missing key %q", c.ID, k)
			}
			if v != "" && !strings.Contains(out, v) {
				t.Errorf("ACM YAML for %q missing value %q", c.ID, v)
			}
		}
	}
}

func TestQA_YAML_ACM_FrameTitle(t *testing.T) {
	certs := fixtureACMForYAML()
	m := yamlModel(certs[0], 120, 40)
	title := m.FrameTitle()
	if !strings.Contains(title, "yaml") {
		t.Errorf("ACM FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_ACM_RawContentUncolored(t *testing.T) {
	certs := fixtureACMForYAML()
	m := yamlModel(certs[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("ACM RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// 16. Auto Scaling Groups — YAML Tests
// ===========================================================================

func TestQA_YAML_ASG_ViewContainsFields(t *testing.T) {
	asgs := fixtureASGForYAML()
	for _, a := range asgs {
		out := yamlView(t, a, 120, 40)
		for k, v := range a.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("ASG YAML for %q missing key %q", a.ID, k)
			}
			if v != "" && !strings.Contains(out, v) {
				t.Errorf("ASG YAML for %q missing value %q", a.ID, v)
			}
		}
	}
}

func TestQA_YAML_ASG_FrameTitle(t *testing.T) {
	asgs := fixtureASGForYAML()
	m := yamlModel(asgs[0], 120, 40)
	title := m.FrameTitle()
	if !strings.Contains(title, "yaml") {
		t.Errorf("ASG FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_ASG_RawContentUncolored(t *testing.T) {
	asgs := fixtureASGForYAML()
	m := yamlModel(asgs[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("ASG RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// 17. IAM Users — YAML Tests
// ===========================================================================

func TestQA_YAML_IAMUser_ViewContainsFields(t *testing.T) {
	users := fixtureIAMUserForYAML()
	for _, u := range users {
		out := yamlView(t, u, 120, 40)
		for k, v := range u.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("IAMUser YAML for %q missing key %q", u.ID, k)
			}
			if v != "" && !strings.Contains(out, v) {
				t.Errorf("IAMUser YAML for %q missing value %q", u.ID, v)
			}
		}
	}
}

func TestQA_YAML_IAMUser_FrameTitle(t *testing.T) {
	users := fixtureIAMUserForYAML()
	m := yamlModel(users[0], 120, 40)
	title := m.FrameTitle()
	if !strings.Contains(title, "yaml") {
		t.Errorf("IAMUser FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_IAMUser_RawContentUncolored(t *testing.T) {
	users := fixtureIAMUserForYAML()
	m := yamlModel(users[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("IAMUser RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// 18. IAM Groups — YAML Tests
// ===========================================================================

func TestQA_YAML_IAMGroup_ViewContainsFields(t *testing.T) {
	groups := fixtureIAMGroupForYAML()
	for _, g := range groups {
		out := yamlView(t, g, 120, 40)
		for k, v := range g.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("IAMGroup YAML for %q missing key %q", g.ID, k)
			}
			if v != "" && !strings.Contains(out, v) {
				t.Errorf("IAMGroup YAML for %q missing value %q", g.ID, v)
			}
		}
	}
}

func TestQA_YAML_IAMGroup_FrameTitle(t *testing.T) {
	groups := fixtureIAMGroupForYAML()
	m := yamlModel(groups[0], 120, 40)
	title := m.FrameTitle()
	if !strings.Contains(title, "yaml") {
		t.Errorf("IAMGroup FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_IAMGroup_RawContentUncolored(t *testing.T) {
	groups := fixtureIAMGroupForYAML()
	m := yamlModel(groups[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("IAMGroup RawContent() contains ANSI codes, expected plain YAML")
	}
}
