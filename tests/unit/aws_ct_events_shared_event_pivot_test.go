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
