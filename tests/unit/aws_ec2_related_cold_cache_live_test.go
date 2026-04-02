package unit

import (
	"context"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func demoRelatedEC2Resource(t *testing.T) resource.Resource {
	t.Helper()
	resources, ok := demo.GetResources("ec2")
	if !ok || len(resources) == 0 {
		t.Fatal("demo ec2 fixtures missing")
	}
	return resources[0]
}

func demoRelatedClients() *awsclient.ServiceClients {
	cfg := demo.NewDemoAWSConfig()
	return awsclient.CreateServiceClients(cfg)
}

func TestEC2RelatedCheckers_FetchLiveDataOnColdCache(t *testing.T) {
	clients := demoRelatedClients()
	instance := demoRelatedEC2Resource(t)
	cache := resource.ResourceCache{}

	for _, target := range []string{"tg", "asg", "ebs-snap"} {
		checker := ec2CheckerByTarget(t, target)
		got := checker(context.Background(), clients, instance, cache)
		if got.Err != nil {
			t.Fatalf("%s checker returned unexpected error on cold cache: %v", target, got.Err)
		}
		if got.Count < 0 {
			t.Fatalf("%s checker should resolve from live fetch on cold cache; got %+v", target, got)
		}
		if got.Count == 0 {
			t.Fatalf("%s checker should find related resources for demo ec2 fixture on cold cache; got %+v", target, got)
		}
	}
}

func TestEC2RelatedCheckers_NodeGroupsAndCloudTrailResolveOnColdCache(t *testing.T) {
	clients := demoRelatedClients()
	instance := demoRelatedEC2Resource(t)
	cache := resource.ResourceCache{}

	for _, target := range []string{"ng", "ct-events"} {
		checker := ec2CheckerByTarget(t, target)
		got := checker(context.Background(), clients, instance, cache)
		if got.Err != nil {
			t.Fatalf("%s checker returned unexpected error on cold cache: %v", target, got.Err)
		}
		if got.Count < 0 {
			t.Fatalf("%s checker should not stay unknown on cold cache with live clients; got %+v", target, got)
		}
	}
}

func TestEC2RelatedCheckers_AlarmResolvesOnColdCache(t *testing.T) {
	clients := demoRelatedClients()
	instance := demoRelatedEC2Resource(t)

	checker := ec2CheckerByTarget(t, "alarm")
	got := checker(context.Background(), clients, instance, resource.ResourceCache{})
	if got.Err != nil {
		t.Fatalf("alarm checker returned unexpected error on cold cache: %v", got.Err)
	}
	if got.Count < 0 {
		t.Fatalf("alarm checker should resolve on cold cache with live clients; got %+v", got)
	}
}

func TestEC2RelatedCheckers_EIPResolvesOnColdCache(t *testing.T) {
	clients := demoRelatedClients()
	instance := demoRelatedEC2Resource(t)

	checker := ec2CheckerByTarget(t, "eip")
	got := checker(context.Background(), clients, instance, resource.ResourceCache{})
	if got.Err != nil {
		t.Fatalf("eip checker returned unexpected error on cold cache: %v", got.Err)
	}
	if got.Count < 0 {
		t.Fatalf("eip checker should resolve on cold cache with live clients; got %+v", got)
	}
}
