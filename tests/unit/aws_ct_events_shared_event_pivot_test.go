// aws_ct_events_shared_event_pivot_test.go — the "CT events by SharedEventId"
// self-pivot and the field it filters on.
//
// CloudTrail's LookupEvents accepts exactly eight LookupAttributeKey values:
// EventId, EventName, ReadOnly, Username, ResourceType, ResourceName,
// EventSource, AccessKeyId. "SharedEventId" is not one of them, so a filter
// that reaches the API under that key fails the call and the pivot renders an
// error instead of the sibling events. The shared event id is carried on the
// built row instead and matched locally.
package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// validLookupAttributeKeys is the closed set CloudTrail accepts. Any other
// key sent as a LookupAttribute makes LookupEvents fail outright.
var validLookupAttributeKeys = map[string]bool{
	"EventId": true, "EventName": true, "ReadOnly": true, "Username": true,
	"ResourceType": true, "ResourceName": true, "EventSource": true, "AccessKeyId": true,
}

// sharedEventIDPivot returns the checker registered under the
// "CT events by SharedEventId" display name.
func sharedEventIDPivot(t *testing.T) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("ct-events") {
		if def.DisplayName == "CT events by SharedEventId" {
			if def.Checker == nil {
				t.Fatal("the SharedEventId pivot is registered with a nil checker")
			}
			return def.Checker
		}
	}
	t.Fatal("no ct-events related entry named \"CT events by SharedEventId\"")
	return nil
}

// crossAccountEventRow builds the source row the pivot runs on: a
// cross-account CloudTrail event whose JSON blob is eventJSON.
func crossAccountEventRow(eventID, eventJSON string) resource.Resource {
	return resource.Resource{
		ID:   eventID,
		Name: "AssumeRole",
		Fields: map[string]string{
			"_ct.cross_account": "true",
		},
		RawStruct: cloudtrailtypes.Event{
			EventId:         aws.String(eventID),
			EventName:       aws.String("AssumeRole"),
			CloudTrailEvent: aws.String(eventJSON),
		},
	}
}

// TestCTSharedEventIdPivot_FiltersLocallyOnTheSharedEventField pins that the
// pivot hands navigation a LOCAL filter key. Every key without the local
// prefix is forwarded verbatim as a LookupAttributeKey, and "SharedEventId"
// is not a key the API knows.
func TestCTSharedEventIdPivot_FiltersLocallyOnTheSharedEventField(t *testing.T) {
	const sharedID = "shared-9f2c41b8-0a7e-4d33-9c10-2b6f5ae10001"

	res := crossAccountEventRow("ct-event-abc123", `{
		"eventName": "AssumeRole",
		"sharedEventID": "`+sharedID+`",
		"recipientAccountId": "123456789012",
		"userIdentity": {"type": "AssumedRole", "accountId": "210987654321"}
	}`)

	result := sharedEventIDPivot(t)(context.Background(), nil, res, resource.ResourceCache{})

	if result.State() != domain.RelatedDeferred {
		t.Fatalf("State = %v, want %v (the pivot drills in via a filtered fetch)", result.State(), domain.RelatedDeferred)
	}
	got := result.FetchFilter()
	want := map[string]string{"_localfield.shared_event_id": sharedID}
	if len(got) != len(want) {
		t.Fatalf("FetchFilter = %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("FetchFilter[%q] = %q, want %q (whole filter: %v)", k, got[k], v, got)
		}
	}
	for k := range got {
		if validLookupAttributeKeys[k] {
			continue
		}
		if k == "_localfield.shared_event_id" {
			continue
		}
		t.Errorf("FetchFilter key %q is neither a valid CloudTrail LookupAttributeKey nor a local-field key", k)
	}
}

// TestCTSharedEventIdPivot_FallsBackToTheEventIdServerKey pins the fallback
// for a cross-account event whose JSON carries no sharedEventID: the pivot
// scopes by the event's own id under "EventId", which IS a key the API
// accepts.
func TestCTSharedEventIdPivot_FallsBackToTheEventIdServerKey(t *testing.T) {
	const eventID = "ct-event-def456"

	res := crossAccountEventRow(eventID, `{
		"eventName": "AssumeRole",
		"recipientAccountId": "123456789012",
		"userIdentity": {"type": "AssumedRole", "accountId": "210987654321"}
	}`)

	result := sharedEventIDPivot(t)(context.Background(), nil, res, resource.ResourceCache{})

	if result.State() != domain.RelatedDeferred {
		t.Fatalf("State = %v, want %v", result.State(), domain.RelatedDeferred)
	}
	got := result.FetchFilter()
	if len(got) != 1 || got["EventId"] != eventID {
		t.Errorf("FetchFilter = %v, want map[EventId:%s]", got, eventID)
	}
}

// TestCTSharedEventIdPivot_SameAccountEventOffersNothing is the negative
// half: a single-account event has no sibling in another account, so the
// pivot resolves to a proven zero rather than a filter.
func TestCTSharedEventIdPivot_SameAccountEventOffersNothing(t *testing.T) {
	res := crossAccountEventRow("ct-event-ghi789", `{"sharedEventID": "shared-should-not-be-used"}`)
	res.Fields["_ct.cross_account"] = "false"

	result := sharedEventIDPivot(t)(context.Background(), nil, res, resource.ResourceCache{})

	if result.State() != domain.RelatedResolved || result.Count() != 0 {
		t.Errorf("State/Count = %v/%d, want %v/0 for a single-account event",
			result.State(), result.Count(), domain.RelatedResolved)
	}
	if len(result.FetchFilter()) != 0 {
		t.Errorf("FetchFilter = %v, want empty for a single-account event", result.FetchFilter())
	}
}

// ---------------------------------------------------------------------------
// The drill itself: the field the local filter reads must exist on the row.
// ---------------------------------------------------------------------------

// recordingLookupEvents captures the LookupEvents input so a test can assert
// what actually reached the API, and answers with a fixed page.
type recordingLookupEvents struct {
	page     *cloudtrail.LookupEventsOutput
	captured *cloudtrail.LookupEventsInput
}

func (r *recordingLookupEvents) LookupEvents(_ context.Context, params *cloudtrail.LookupEventsInput, _ ...func(*cloudtrail.Options)) (*cloudtrail.LookupEventsOutput, error) {
	r.captured = params
	return r.page, nil
}

var _ awsclient.CloudTrailLookupEventsAPI = (*recordingLookupEvents)(nil)

// TestFetchCTEventsFiltered_SharedEventIdNeverReachesTheAPI pins the drill
// end to end: the local filter is stripped before the call, so LookupEvents
// sees no attribute at all, and the page is narrowed afterwards to the events
// whose shared-event-id field matches.
func TestFetchCTEventsFiltered_SharedEventIdNeverReachesTheAPI(t *testing.T) {
	const sharedID = "shared-9f2c41b8-0a7e-4d33-9c10-2b6f5ae10001"
	const otherSharedID = "shared-11112222-3333-4444-5555-666677778888"

	api := &recordingLookupEvents{
		page: &cloudtrail.LookupEventsOutput{
			Events: []cloudtrailtypes.Event{
				{
					EventId:         aws.String("ct-event-match"),
					EventName:       aws.String("AssumeRole"),
					CloudTrailEvent: aws.String(`{"eventName":"AssumeRole","sharedEventID":"` + sharedID + `"}`),
				},
				{
					EventId:         aws.String("ct-event-other"),
					EventName:       aws.String("AssumeRole"),
					CloudTrailEvent: aws.String(`{"eventName":"AssumeRole","sharedEventID":"` + otherSharedID + `"}`),
				},
			},
		},
	}

	out, err := awsclient.FetchCloudTrailEventsPageFiltered(
		context.Background(), api,
		map[string]string{"_localfield.shared_event_id": sharedID}, "")
	if err != nil {
		t.Fatalf("FetchCloudTrailEventsPageFiltered: %v", err)
	}

	if api.captured == nil {
		t.Fatal("LookupEvents was never called")
	}
	for _, attr := range api.captured.LookupAttributes {
		if !validLookupAttributeKeys[string(attr.AttributeKey)] {
			t.Errorf("LookupEvents received AttributeKey %q, which CloudTrail does not accept",
				string(attr.AttributeKey))
		}
	}
	if len(api.captured.LookupAttributes) != 0 {
		t.Errorf("LookupAttributes = %v, want none (the shared event id is a local field)",
			api.captured.LookupAttributes)
	}

	if len(out.Resources) != 1 {
		t.Fatalf("got %d rows, want 1 (only the matching shared event id)", len(out.Resources))
	}
	if out.Resources[0].ID != "ct-event-match" {
		t.Errorf("drilled row = %q, want %q", out.Resources[0].ID, "ct-event-match")
	}
	if got := out.Resources[0].Fields["shared_event_id"]; got != sharedID {
		t.Errorf("Fields[\"shared_event_id\"] = %q, want %q", got, sharedID)
	}
}

// TestFetchCTEventsFiltered_EventWithoutSharedIDCarriesAnEmptyField is the
// negative half: an event whose JSON has no sharedEventID carries the field
// empty, so it can never be folded into another event's shared-id drill.
func TestFetchCTEventsFiltered_EventWithoutSharedIDCarriesAnEmptyField(t *testing.T) {
	api := &recordingLookupEvents{
		page: &cloudtrail.LookupEventsOutput{
			Events: []cloudtrailtypes.Event{
				{
					EventId:         aws.String("ct-event-plain"),
					EventName:       aws.String("DescribeInstances"),
					CloudTrailEvent: aws.String(`{"eventName":"DescribeInstances"}`),
				},
			},
		},
	}

	out, err := awsclient.FetchCloudTrailEventsPageFiltered(context.Background(), api, nil, "")
	if err != nil {
		t.Fatalf("FetchCloudTrailEventsPageFiltered: %v", err)
	}
	if len(out.Resources) != 1 {
		t.Fatalf("got %d rows, want 1", len(out.Resources))
	}
	if got := out.Resources[0].Fields["shared_event_id"]; got != "" {
		t.Errorf("Fields[\"shared_event_id\"] = %q, want \"\" for an event without one", got)
	}
}
