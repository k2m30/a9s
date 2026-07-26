package unit

// qa_sns_detail_attributes_test.go — coverage for the sns Detail view's new
// {Path: "Attributes"} field (core/config/defaults_messaging.go, mirroring
// sqs's {Path: "Attributes"}), which surfaces TopicEnriched.Attributes (set
// by the on-demand sns detail enricher, core/aws/sns_detail_enrichment.go)
// in the detail BODY — not just the YAML/JSON view. Mirrors the
// detail-render approach of tests/unit/qa_enrichment_stacked_views_test.go
// (navigate → load → enter detail → deliver enrichment → assert rendered
// content) and app_enrich_test.go's TestDetailView_EnrichResult_AcceptsMatchingID.

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/demo/fakes"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// TestSNSDetail_AttributesPath_RendersAfterEnrichment verifies that once the
// sns Detail view gained {Path: "Attributes"}, opening a topic's detail view
// renders TopicArn immediately (negative guard: no panic, no dependency on
// enrichment having landed yet), and after the on-demand detail enricher's
// result arrives (messages.EnrichDetailResult carrying a TopicEnriched
// RawStruct), the detail body renders the Attributes map's contents.
func TestSNSDetail_AttributesPath_RendersAfterEnrichment(t *testing.T) {
	m := tui.New("demo", "us-east-1",
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithNoCache(true),
		tui.WithProfileForTest(demo.DemoProfile),
		tui.WithRegionForTest(demo.DemoRegion))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 220, Height: 50})

	// Open the sns list.
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "sns",
	})

	snsClient := fakes.NewSNS()
	snsRes, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchSNSTopicsPage(context.Background(), snsClient, token)
	})
	if err != nil || len(snsRes) == 0 {
		t.Fatalf("demo sns fixtures missing (err=%v, len=%d)", err, len(snsRes))
	}

	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: "sns",
		Resources:    snsRes,
	})

	// Enter detail on the first topic.
	firstTopic := snsRes[0]
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: "sns",
		Resource:     &firstTopic,
	})

	// Negative guard: before enrichment arrives, the detail view must render
	// without error and show the TopicArn row.
	before := stripANSI(rootViewContent(m))
	if before == "" {
		t.Fatal("sns detail view must render before enrichment arrives")
	}
	if !strings.Contains(before, "TopicArn") {
		t.Errorf("sns detail view before enrichment must show a TopicArn row; got:\n%s", before)
	}

	topic, ok := firstTopic.RawStruct.(snstypes.Topic)
	if !ok {
		t.Fatalf("sns fixture RawStruct = %T, want snstypes.Topic", firstTopic.RawStruct)
	}

	// Deliver the on-demand detail enrichment result carrying a realistic
	// Attributes map, including one parsed-policy nested map value (mirroring
	// enrichSns's JSON-object attribute parsing, core/aws/sns_detail_enrichment.go).
	enrichedRes := firstTopic
	enrichedRes.RawStruct = awsclient.TopicEnriched{
		Topic: topic,
		Attributes: map[string]any{
			"DisplayName":            "order-events",
			"SubscriptionsConfirmed": "2",
			"SubscriptionsPending":   "0",
			"TopicArn":               *topic.TopicArn,
			"Policy": map[string]any{
				"Version": "2012-10-17",
				"Statement": []any{
					map[string]any{"Effect": "Allow", "Action": "SNS:Publish"},
				},
			},
		},
	}

	m, _ = rootApplyMsg(m, messages.EnrichDetailResult{
		ResourceType: "sns",
		ResourceID:   firstTopic.ID,
		EnrichedRes:  enrichedRes,
	})

	after := stripANSI(rootViewContent(m))
	if !strings.Contains(after, "DisplayName") {
		t.Errorf("sns detail body must render Attributes[DisplayName] after enrichment; got:\n%s", after)
	}
	if !strings.Contains(after, "SubscriptionsConfirmed") {
		t.Errorf("sns detail body must render Attributes[SubscriptionsConfirmed] after enrichment; got:\n%s", after)
	}
}
