package unit

// Tests for CloudTrailFake.LookupEvents + matchesLookupAttributes
// (internal/demo/fakes/cloudtrail.go:29-88).
//
// matchesLookupAttributes has 6 attribute-key branches:
//   Username, EventName, EventSource, ResourceName, ResourceType, AccessKeyId
// with AND logic across multiple attributes.
//
// Fixture facts used below:
//   - "bob.smith" is Username on events: DeleteBucket, ApiCallRateInsight
//   - "CreateBucket" is EventName on 1 event (Username=nil)
//   - "s3.amazonaws.com" is EventSource on: CreateBucket, DeleteBucket, VpcEndpointAccess, PutObject×3, PutBucketPolicy, GetObject
//   - ResourceName "webapp-assets-prod" appears on CreateBucket, DeleteBucket, and PutObject(bob)
//   - ResourceType "AWS::S3::Bucket" appears on CreateBucket, DeleteBucket, and PutObject(bob)
//   - AND: bob.smith + s3.amazonaws.com → only DeleteBucket (1 event)
//   - "nonexistent-user" → empty

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"

	"github.com/k2m30/a9s/v3/internal/demo/fakes"
)

func newCTFake() *fakes.CloudTrailFake {
	return fakes.NewCloudTrail()
}

func lookupEvents(t *testing.T, f *fakes.CloudTrailFake, attrs []cloudtrailtypes.LookupAttribute) []cloudtrailtypes.Event {
	t.Helper()
	out, err := f.LookupEvents(context.Background(), &cloudtrail.LookupEventsInput{
		LookupAttributes: attrs,
	})
	if err != nil {
		t.Fatalf("LookupEvents error: %v", err)
	}
	return out.Events
}

// TestCTFake_LookupEvents_NoFilter verifies that empty LookupAttributes returns all events.
func TestCTFake_LookupEvents_NoFilter(t *testing.T) {
	f := newCTFake()
	events := lookupEvents(t, f, nil)
	if len(events) == 0 {
		t.Error("expected events with no filter, got zero")
	}
}

// TestCTFake_LookupEvents_UsernameFilter verifies the Username branch:
// "bob.smith" appears as Username on exactly 2 events (DeleteBucket, ApiCallRateInsight).
func TestCTFake_LookupEvents_UsernameFilter(t *testing.T) {
	f := newCTFake()
	events := lookupEvents(t, f, []cloudtrailtypes.LookupAttribute{
		{AttributeKey: cloudtrailtypes.LookupAttributeKeyUsername, AttributeValue: aws.String("bob.smith")},
	})
	if len(events) != 2 {
		t.Errorf("Username=bob.smith: got %d events, want 2 (DeleteBucket + ApiCallRateInsight)", len(events))
	}
	for _, e := range events {
		if aws.ToString(e.Username) != "bob.smith" {
			t.Errorf("event %q has Username=%q, want %q", aws.ToString(e.EventName), aws.ToString(e.Username), "bob.smith")
		}
	}
}

// TestCTFake_LookupEvents_EventNameFilter verifies the EventName branch:
// "CreateBucket" appears as EventName on exactly 1 event.
func TestCTFake_LookupEvents_EventNameFilter(t *testing.T) {
	f := newCTFake()
	events := lookupEvents(t, f, []cloudtrailtypes.LookupAttribute{
		{AttributeKey: cloudtrailtypes.LookupAttributeKeyEventName, AttributeValue: aws.String("CreateBucket")},
	})
	if len(events) != 1 {
		t.Errorf("EventName=CreateBucket: got %d events, want 1", len(events))
	}
	if len(events) == 1 && aws.ToString(events[0].EventName) != "CreateBucket" {
		t.Errorf("EventName = %q, want %q", aws.ToString(events[0].EventName), "CreateBucket")
	}
}

// TestCTFake_LookupEvents_EventSourceFilter verifies the EventSource branch:
// "ec2.amazonaws.com" appears on at least 1 event (DescribeInstances, etc.).
func TestCTFake_LookupEvents_EventSourceFilter(t *testing.T) {
	f := newCTFake()
	events := lookupEvents(t, f, []cloudtrailtypes.LookupAttribute{
		{AttributeKey: cloudtrailtypes.LookupAttributeKeyEventSource, AttributeValue: aws.String("ec2.amazonaws.com")},
	})
	if len(events) == 0 {
		t.Error("EventSource=ec2.amazonaws.com: got 0 events, want >=1")
	}
	for _, e := range events {
		if aws.ToString(e.EventSource) != "ec2.amazonaws.com" {
			t.Errorf("event %q has EventSource=%q, want ec2.amazonaws.com", aws.ToString(e.EventName), aws.ToString(e.EventSource))
		}
	}
}

// TestCTFake_LookupEvents_ResourceNameFilter verifies the ResourceName branch:
// "webapp-assets-prod" is a ResourceName on 2 events (CreateBucket, DeleteBucket).
func TestCTFake_LookupEvents_ResourceNameFilter(t *testing.T) {
	f := newCTFake()
	events := lookupEvents(t, f, []cloudtrailtypes.LookupAttribute{
		{AttributeKey: cloudtrailtypes.LookupAttributeKeyResourceName, AttributeValue: aws.String("webapp-assets-prod")},
	})
	if len(events) != 3 {
		t.Errorf("ResourceName=webapp-assets-prod: got %d events, want 3 (CreateBucket + DeleteBucket + PutObject)", len(events))
	}
	for _, e := range events {
		found := false
		for _, r := range e.Resources {
			if aws.ToString(r.ResourceName) == "webapp-assets-prod" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("event %q passed ResourceName filter but has no resource named webapp-assets-prod", aws.ToString(e.EventName))
		}
	}
}

// TestCTFake_LookupEvents_ResourceTypeFilter verifies the ResourceType branch:
// "AWS::S3::Bucket" appears as ResourceType on at least 1 event.
func TestCTFake_LookupEvents_ResourceTypeFilter(t *testing.T) {
	f := newCTFake()
	events := lookupEvents(t, f, []cloudtrailtypes.LookupAttribute{
		{AttributeKey: cloudtrailtypes.LookupAttributeKeyResourceType, AttributeValue: aws.String("AWS::S3::Bucket")},
	})
	if len(events) == 0 {
		t.Error("ResourceType=AWS::S3::Bucket: got 0 events, want >=1")
	}
	for _, e := range events {
		found := false
		for _, r := range e.Resources {
			if aws.ToString(r.ResourceType) == "AWS::S3::Bucket" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("event %q passed ResourceType filter but has no AWS::S3::Bucket resource", aws.ToString(e.EventName))
		}
	}
}

// TestCTFake_LookupEvents_NonMatchingFilter verifies that a filter with no
// matching events returns an empty list.
func TestCTFake_LookupEvents_NonMatchingFilter(t *testing.T) {
	f := newCTFake()
	events := lookupEvents(t, f, []cloudtrailtypes.LookupAttribute{
		{AttributeKey: cloudtrailtypes.LookupAttributeKeyUsername, AttributeValue: aws.String("nonexistent-user-xyz")},
	})
	if len(events) != 0 {
		t.Errorf("Username=nonexistent-user-xyz: got %d events, want 0", len(events))
	}
}

// TestCTFake_LookupEvents_ANDLogic verifies that multiple attributes are ANDed:
// bob.smith + s3.amazonaws.com → only DeleteBucket (1 event).
// CreateBucket has s3.amazonaws.com but Username=nil, so it must NOT match.
func TestCTFake_LookupEvents_ANDLogic(t *testing.T) {
	f := newCTFake()
	events := lookupEvents(t, f, []cloudtrailtypes.LookupAttribute{
		{AttributeKey: cloudtrailtypes.LookupAttributeKeyUsername, AttributeValue: aws.String("bob.smith")},
		{AttributeKey: cloudtrailtypes.LookupAttributeKeyEventSource, AttributeValue: aws.String("s3.amazonaws.com")},
	})
	if len(events) != 1 {
		t.Errorf("Username=bob.smith AND EventSource=s3.amazonaws.com: got %d events, want 1 (DeleteBucket only)", len(events))
	}
	if len(events) == 1 {
		if aws.ToString(events[0].EventName) != "DeleteBucket" {
			t.Errorf("EventName = %q, want %q", aws.ToString(events[0].EventName), "DeleteBucket")
		}
	}
}
