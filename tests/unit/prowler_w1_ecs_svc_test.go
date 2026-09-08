package unit

// prowler_w1_ecs_svc_test.go — behavioural pins for ecs-svc.public-ip
// (batch w1).
//
// A service with AssignPublicIp=ENABLED gives every task it launches a
// routable address, so the tasks are reachable from the internet as soon as
// a security group allows it. The signal comes from the DescribeServices
// response EnrichECSServices already reads — no extra API call.

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo/fakes"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

const pw1ECSSvcCodePublicIP = domain.FindingCode("ecs-svc.public-ip")

// pw1ECSSvcFake serves DescribeServices for EnrichECSServices.
type pw1ECSSvcFake struct {
	awsclient.ECSAPI
	services map[string]ecstypes.Service // service name → service
	err      error
}

func (f *pw1ECSSvcFake) DescribeServices(_ context.Context, in *ecs.DescribeServicesInput, _ ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	var out []ecstypes.Service
	for _, name := range in.Services {
		if svc, ok := f.services[name]; ok {
			out = append(out, svc)
		}
	}
	return &ecs.DescribeServicesOutput{Services: out}, nil
}

// pw1ECSService builds a healthy running service; assignPublicIP controls the
// awsvpc network configuration. Pass "" for a service with no
// NetworkConfiguration at all (the EC2 launch-type shape).
func pw1ECSService(name string, assignPublicIP ecstypes.AssignPublicIp) ecstypes.Service {
	svc := ecstypes.Service{
		ServiceName:  aws.String(name),
		ServiceArn:   aws.String("arn:aws:ecs:us-east-1:123456789012:service/acme-cluster/" + name),
		Status:       aws.String("ACTIVE"),
		DesiredCount: 2,
		RunningCount: 2,
		LaunchType:   ecstypes.LaunchTypeFargate,
	}
	if assignPublicIP != "" {
		svc.NetworkConfiguration = &ecstypes.NetworkConfiguration{
			AwsvpcConfiguration: &ecstypes.AwsVpcConfiguration{
				Subnets:        []string{"subnet-0aaaa1111bbbb2222"},
				SecurityGroups: []string{"sg-0aaaa1111bbbb2222"},
				AssignPublicIp: assignPublicIP,
			},
		}
	}
	return svc
}

func pw1ECSSvcResource(name string) resource.Resource {
	return resource.Resource{
		ID:   name,
		Name: name,
		Fields: map[string]string{
			"service_name": name,
			"cluster":      "acme-cluster",
			"status":       "ACTIVE",
		},
	}
}

func pw1EnrichECSSvc(t *testing.T, fake *pw1ECSSvcFake, names ...string) awsclient.IssueEnricherResult {
	t.Helper()
	rs := make([]resource.Resource, 0, len(names))
	for _, n := range names {
		rs = append(rs, pw1ECSSvcResource(n))
	}
	res, err := awsclient.EnrichECSServices(context.Background(), &awsclient.ServiceClients{ECS: fake}, rs, nil)
	if err != nil && fake.err == nil {
		t.Fatalf("EnrichECSServices: %v", err)
	}
	return res
}

// TestECSSvc_PublicIP_Enabled pins the warning and the row naming the setting
// the operator has to flip.
func TestECSSvc_PublicIP_Enabled(t *testing.T) {
	const name = "acme-public-api"
	fake := &pw1ECSSvcFake{services: map[string]ecstypes.Service{
		name: pw1ECSService(name, ecstypes.AssignPublicIpEnabled),
	}}
	res := pw1EnrichECSSvc(t, fake, name)
	pw1RequireFinding(t, res.Findings[name], pw1ECSSvcCodePublicIP,
		"tasks get public IPs", domain.SevWarn, "wave2")
	pw1RequireRow(t, pw1Rows(res, name, pw1ECSSvcCodePublicIP), "Public address assignment", "enabled")
}

// TestECSSvc_PublicIP_DisabledIsHealthy pins the negative case.
func TestECSSvc_PublicIP_DisabledIsHealthy(t *testing.T) {
	const name = "acme-private-api"
	fake := &pw1ECSSvcFake{services: map[string]ecstypes.Service{
		name: pw1ECSService(name, ecstypes.AssignPublicIpDisabled),
	}}
	res := pw1EnrichECSSvc(t, fake, name)
	pw1RequireNoFinding(t, res.Findings[name], pw1ECSSvcCodePublicIP)
}

// TestECSSvc_PublicIP_NoNetworkConfigurationIsSilent pins that an EC2
// launch-type service, which carries no awsvpc block at all, emits nothing —
// the absent block is not a disabled setting to report.
func TestECSSvc_PublicIP_NoNetworkConfigurationIsSilent(t *testing.T) {
	const name = "acme-bridge-worker"
	fake := &pw1ECSSvcFake{services: map[string]ecstypes.Service{
		name: pw1ECSService(name, ""),
	}}
	res := pw1EnrichECSSvc(t, fake, name)
	pw1RequireNoFinding(t, res.Findings[name], pw1ECSSvcCodePublicIP)
}

// TestECSSvc_PublicIP_CoexistsWithDeploymentFailure pins independence: the
// posture finding and the existing deployment-failure finding are two
// entries on the same service, each with its own code and rows.
func TestECSSvc_PublicIP_CoexistsWithDeploymentFailure(t *testing.T) {
	const name = "acme-broken-public"
	svc := pw1ECSService(name, ecstypes.AssignPublicIpEnabled)
	svc.Deployments = []ecstypes.Deployment{{
		RolloutState:       ecstypes.DeploymentRolloutStateFailed,
		RolloutStateReason: aws.String("ECS deployment circuit breaker: task failed to start."),
	}}
	fake := &pw1ECSSvcFake{services: map[string]ecstypes.Service{name: svc}}

	res := pw1EnrichECSSvc(t, fake, name)
	pw1RequireFinding(t, res.Findings[name], pw1ECSSvcCodePublicIP,
		"tasks get public IPs", domain.SevWarn, "wave2")
	if _, ok := pw1FindFinding(res.Findings[name], domain.FindingCode("ecs-svc.deployment-failed")); !ok {
		t.Errorf("deployment-failed finding lost alongside the posture finding: %+v", res.Findings[name])
	}
	if len(pw1Rows(res, name, pw1ECSSvcCodePublicIP)) == 0 {
		t.Errorf("posture rows crowded out by the deployment-failure rows")
	}
}

// TestECSSvc_PublicIP_BatchErrorMarksEveryServiceInTheBatch pins the
// partial-failure contract: a DescribeServices error leaves each service of
// that batch in TruncatedIDs so the row renders "?" instead of a clean
// healthy state.
func TestECSSvc_PublicIP_BatchErrorMarksEveryServiceInTheBatch(t *testing.T) {
	fake := &pw1ECSSvcFake{err: errors.New("AccessDeniedException: ecs:DescribeServices")}
	res := pw1EnrichECSSvc(t, fake, "acme-a", "acme-b")
	for _, name := range []string{"acme-a", "acme-b"} {
		if !res.TruncatedIDs[name] {
			t.Errorf("TruncatedIDs missing %q after a failed DescribeServices batch", name)
		}
		pw1RequireNoFinding(t, res.Findings[name], pw1ECSSvcCodePublicIP)
	}
}

// TestECSSvc_PublicIP_HealthyNeighbourStillEvaluated pins that one service
// missing from the DescribeServices response does not stop the others from
// being evaluated.
func TestECSSvc_PublicIP_HealthyNeighbourStillEvaluated(t *testing.T) {
	fake := &pw1ECSSvcFake{services: map[string]ecstypes.Service{
		"acme-present": pw1ECSService("acme-present", ecstypes.AssignPublicIpEnabled),
	}}
	res := pw1EnrichECSSvc(t, fake, "acme-absent", "acme-present")
	pw1RequireFinding(t, res.Findings["acme-present"], pw1ECSSvcCodePublicIP,
		"tasks get public IPs", domain.SevWarn, "wave2")
	pw1RequireNoFinding(t, res.Findings["acme-absent"], pw1ECSSvcCodePublicIP)
}

// TestECSSvc_DemoBench_OnlyWitnessAssignsPublicIPs pins the demo fixture
// contract for this signal.
func TestECSSvc_DemoBench_OnlyWitnessAssignsPublicIPs(t *testing.T) {
	fake := fakes.NewECS()
	out, err := awsclient.FetchECSServicesPage(context.Background(), fake, fake, fake, "")
	if err != nil {
		t.Fatalf("FetchECSServicesPage(demo): %v", err)
	}
	res, err := awsclient.EnrichECSServices(context.Background(),
		&awsclient.ServiceClients{ECS: fakes.NewECS()}, out.Resources, nil)
	if err != nil {
		t.Fatalf("EnrichECSServices(demo): %v", err)
	}
	var carriers []string
	for id, fs := range res.Findings {
		if _, ok := pw1FindFinding(fs, pw1ECSSvcCodePublicIP); ok {
			carriers = append(carriers, id)
		}
	}
	pw1RequireOnlyWitness(t, pw1ECSSvcCodePublicIP, fixtures.ECSServicePublicIP, carriers)
}

// TestECSSvc_PublicIP_InactiveServiceIsSilent pins common contract rule 4 on
// ecs-svc: a service being drained or already inactive launches no more tasks,
// so its address assignment is not an open posture item.
func TestECSSvc_PublicIP_InactiveServiceIsSilent(t *testing.T) {
	for _, status := range []string{"INACTIVE", "DRAINING"} {
		name := "acme-gone-" + strings.ToLower(status)
		svc := pw1ECSService(name, ecstypes.AssignPublicIpEnabled)
		svc.Status = aws.String(status)
		fake := &pw1ECSSvcFake{services: map[string]ecstypes.Service{name: svc}}

		r := pw1ECSSvcResource(name)
		r.Fields["status"] = status
		res, err := awsclient.EnrichECSServices(context.Background(),
			&awsclient.ServiceClients{ECS: fake}, []resource.Resource{r}, nil)
		if err != nil {
			t.Fatalf("EnrichECSServices: %v", err)
		}
		if _, ok := pw1FindFinding(res.Findings[name], pw1ECSSvcCodePublicIP); ok {
			t.Errorf("status %q: ecs-svc.public-ip emitted for a service that launches no tasks", status)
		}
	}
}
