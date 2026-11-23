package unit

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// A stopped task has no health left to judge: its row carries the stopped
// state and nothing about a health check that ran before it exited.
func TestT2Dev_StoppedTaskShowsOnlyItsState(t *testing.T) {
	td := catalog.FindAny("ecs-task")
	if td == nil || td.Fetcher == nil {
		t.Fatal("ecs-task has no catalog Fetcher")
	}
	const id = "0aaaa1111bbbb2222cccc3333dddd4499"
	task := pw1Task(id, pw1TaskDefARN)
	task.LastStatus = aws.String("STOPPED")
	task.DesiredStatus = aws.String("STOPPED")
	task.HealthStatus = ecstypes.HealthStatusUnhealthy
	fake := &pw1ECSTaskFake{tasks: map[string]ecstypes.Task{id: task}, defs: map[string]ecstypes.TaskDefinition{pw1TaskDefARN: pw1TaskDef(pw1TaskDefARN, pw1HealthyContainer("app"))}}
	page, err := td.Fetcher(context.Background(), &awsclient.ServiceClients{ECS: fake}, "")
	if err != nil {
		t.Fatalf("ecs-task fetcher: %v", err)
	}
	r := pw1ResourceByID(t, page.Resources, id)
	if !hasCode(r.Findings, awsclient.CodeECSTaskStateStopped) || hasCode(r.Findings, awsclient.CodeECSTaskHealthUnhealthy) {
		t.Errorf("stopped task with health UNHEALTHY: findings %v, want the stopped state alone", r.Findings)
	}
}

// An Application Load Balancer logs the HTTP requests its listeners serve, so
// access logs off on one with an HTTP listener is reported, and the same
// setting on a balancer with no logged listener is not.
func TestT2Dev_ALBAccessLogsJudgedByItsListeners(t *testing.T) {
	alb := w3ELBRes("acme-alb-http", "application")
	idle := w3ELBRes("acme-alb-idle", "application")
	fake := w3NewELBFake()
	for _, r := range []resource.Resource{alb, idle} {
		fake.attrs[r.Fields["load_balancer_arn"]] = w3ELBAttrs(map[string]string{"access_logs.s3.enabled": "false"})
	}
	fake.listeners[alb.Fields["load_balancer_arn"]] = []elbtypes.Listener{w3Listener("http", 80, elbtypes.ProtocolEnumHttp, "", w3ForwardAction())}
	res := w3RunELBEnrich(t, fake, alb, idle)
	const code = domain.FindingCode("elb.misconfigured")
	if !hasCode(res.Findings[alb.ID], code) {
		t.Fatalf("%s: ALB with an HTTP listener and access logs off raised no %s", alb.ID, code)
	}
	w2AssertRow(t, res.AttentionDetails[alb.ID][code].Rows, "Access Logs", "disabled")
	if hasCode(res.Findings[idle.ID], code) {
		t.Errorf("%s: a balancer with no listener raised %s for logs nothing would write", idle.ID, code)
	}
}
