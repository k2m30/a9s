package unit_test

// A type without a StubCreator never
// auto-opens a synthetic detail: handleResourcesLoaded's
// `td.StubCreator != nil` guard, driven through the real dispatch chain (root
// Model.Update -> handleRelatedNavigate -> handleResourcesLoaded).
//
// "asg" registers neither FetchByIDs nor a StubCreator (only "ami" registers
// one — core/aws/catalog_compute.go), so a TargetID cache-miss followed by an
// empty ResourcesLoaded must leave the operator on the related list/flash,
// never synthesize a detail.

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

func TestRelatedNavigate_NoStubCreator_EmptyResult_NoDetailAutoOpen(t *testing.T) {
	m := newRelatedDemoModel(t)

	ec2Res := resource.Resource{
		ID:     "i-0a1b2c3d4e5f60001",
		Name:   "web-prod-01",
		Fields: map[string]string{"instance_id": "i-0a1b2c3d4e5f60001"},
	}
	m = navigateToEC2DetailRelated(t, m, ec2Res)

	m, _ = relatedApplyMsg(m, messages.RelatedNavigate{
		TargetType:     "asg",
		SourceResource: ec2Res,
		TargetID:       "asg-does-not-exist",
	})

	m2, cmd := relatedApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceByID,
		ResourceType: "asg",
		Resources:    []resource.Resource{},
	})
	m = m2
	m = applyRelatedFollowUp(m, cmd)

	view := stripAnsi(relatedViewContent(m))
	if strings.Contains(view, "detail -- asg-does-not-exist") {
		t.Fatalf("asg has no StubCreator; an empty TargetID miss must NOT auto-open a synthetic detail; got:\n%s", view)
	}
}
