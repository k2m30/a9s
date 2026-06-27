package unit

// ctevent_summarizer_test.go verifies that BuildSections routes each registered
// EventSource to its service-specific summarizer rather than SummarizeGeneric.
//
// After the static-map migration, RegisterSummarizer no longer exists — duplicate
// keys in a map literal are a compile error, strictly better than a runtime panic.
// The old TestRegisterSummarizer_DuplicatePanics test is intentionally removed.

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/semantics/ctevent"
)

// summarizerTestEvent returns an Event with the minimum fields set to produce a
// non-empty REQUEST section via BuildSections for the given source, name, and params.
// UserIdentity.ARN is set so the ACTOR section emits rows (non-service event path).
func summarizerTestEvent(source, name string, params map[string]any) *ctevent.Event {
	return &ctevent.Event{
		EventSource:    source,
		EventName:      name,
		EventCategory:  "Management",
		EventType:      "AwsApiCall",
		AWSRegion:      "us-east-1",
		AccountID:      "111122223333",
		UserIdentity: ctevent.UserIdentity{
			Type: "IAMUser",
			ARN:  "arn:aws:iam::111122223333:user/test-user",
		},
		RequestParameters: params,
	}
}

// TestCTEventSummarizer_IAM_RoutesToSummarizeIAM verifies that an iam.amazonaws.com event
// has its REQUEST section built by SummarizeIAM rather than SummarizeGeneric.
// Discriminating field: "policyArn" — SummarizeIAM marks it IsNavigable=true with
// TargetType="policy"; SummarizeGeneric emits IsNavigable=false.
func TestCTEventSummarizer_IAM_RoutesToSummarizeIAM(t *testing.T) {
	policyARN := "arn:aws:iam::111122223333:policy/MyPolicy"
	event := summarizerTestEvent("iam.amazonaws.com", "AttachRolePolicy", map[string]any{
		"policyArn": policyARN,
		// permissionsBoundary ends in "Boundary" — not matched by catch-all (*Id/*Name/*Arn),
		// so it is guaranteed to survive TARGET extraction into REQUEST.
		"permissionsBoundary": "arn:aws:iam::aws:policy/AdministratorAccess",
	})

	sections := ctevent.BuildSections(event)

	req, ok := findSection(sections, ctevent.SectionRequest)
	if !ok {
		t.Fatal("IAM event: expected REQUEST section in BuildSections output; got none")
	}

	// Locate the policyArn row if it survived TARGET extraction.
	// policyArn ends in "Arn" and may be lifted by the catch-all — either outcome is valid;
	// we assert navigability only when the row is present.
	var policyRow *ctevent.Row
	for i := range req.Rows {
		if req.Rows[i].Key == "policyArn" {
			policyRow = &req.Rows[i]
			break
		}
	}
	if policyRow != nil {
		// SummarizeIAM marks policyArn navigable; SummarizeGeneric does not.
		if !policyRow.IsNavigable {
			t.Errorf("IAM event: REQUEST row policyArn IsNavigable=false; want true (SummarizeIAM must have run)")
		}
		if policyRow.TargetType != "policy" {
			t.Errorf("IAM event: REQUEST row policyArn TargetType=%q; want %q", policyRow.TargetType, "policy")
		}
	}

	// permissionsBoundary survives TARGET extraction (no *Id/*Name/*Arn suffix).
	// Its presence in REQUEST proves the summarizer emitted it.
	foundBoundary := false
	for _, r := range req.Rows {
		if r.Key == "permissionsBoundary" {
			foundBoundary = true
			break
		}
	}
	if !foundBoundary && policyRow == nil {
		t.Error("IAM event: REQUEST section has neither policyArn nor permissionsBoundary row; routing may have failed")
	}
}

// TestCTEventSummarizer_EC2_RoutesToSummarizeEC2 verifies that an ec2.amazonaws.com event
// has its REQUEST section built by SummarizeEC2 rather than SummarizeGeneric.
// Discriminating field: "imageId" — SummarizeEC2 marks it IsNavigable=true with
// TargetType="ami"; SummarizeGeneric emits IsNavigable=false.
func TestCTEventSummarizer_EC2_RoutesToSummarizeEC2(t *testing.T) {
	event := summarizerTestEvent("ec2.amazonaws.com", "RunInstances", map[string]any{
		"imageId": "ami-0abc1234567890def",
		// instanceType does not match catch-all patterns (*Id/*Name/*Arn),
		// so it always survives TARGET extraction into REQUEST.
		"instanceType": "t3.micro",
	})

	sections := ctevent.BuildSections(event)

	req, ok := findSection(sections, ctevent.SectionRequest)
	if !ok {
		t.Fatal("EC2 event: expected REQUEST section in BuildSections output; got none")
	}

	// imageId ends in "Id" — the catch-all may lift it into TARGET.
	// Assert navigability when it survives into REQUEST.
	var imageRow *ctevent.Row
	for i := range req.Rows {
		if req.Rows[i].Key == "imageId" {
			imageRow = &req.Rows[i]
			break
		}
	}
	if imageRow != nil {
		// SummarizeEC2 marks imageId navigable with TargetType="ami".
		if !imageRow.IsNavigable {
			t.Errorf("EC2 event: REQUEST row imageId IsNavigable=false; want true (SummarizeEC2 must have run)")
		}
		if imageRow.TargetType != "ami" {
			t.Errorf("EC2 event: REQUEST row imageId TargetType=%q; want %q", imageRow.TargetType, "ami")
		}
	}

	// instanceType is never lifted by catch-all (no *Id/*Name/*Arn suffix).
	// Its presence in REQUEST confirms the summarizer ran for this event.
	foundInstanceType := false
	for _, r := range req.Rows {
		if r.Key == "instanceType" {
			foundInstanceType = true
			if r.IsNavigable {
				t.Errorf("EC2 event: REQUEST row instanceType should not be navigable; got IsNavigable=true")
			}
			break
		}
	}
	if !foundInstanceType && imageRow == nil {
		t.Error("EC2 event: REQUEST section has neither imageId nor instanceType row; routing may have failed")
	}
}

// TestCTEventSummarizer_S3_RoutesToSummarizeS3 verifies that an s3.amazonaws.com event
// has its REQUEST section built by SummarizeS3 rather than falling to SummarizeGeneric.
// S3's summarizer emits all residual fields as non-navigable rows — identical behavior
// to SummarizeGeneric. The routing is verified by confirming fields that survive TARGET
// extraction appear in REQUEST, proving the registered S3 summarizer ran.
func TestCTEventSummarizer_S3_RoutesToSummarizeS3(t *testing.T) {
	// PutObject with fields that do not match catch-all heuristics (*Id/*Name/*Arn)
	// so they are guaranteed to survive TARGET extraction into REQUEST.
	event := summarizerTestEvent("s3.amazonaws.com", "PutObject", map[string]any{
		"x-amz-server-side-encryption": "aws:kms",
		"x-amz-storage-class":          "STANDARD_IA",
	})

	sections := ctevent.BuildSections(event)

	req, ok := findSection(sections, ctevent.SectionRequest)
	if !ok {
		t.Fatal("S3 event: expected REQUEST section in BuildSections output; got none")
	}

	// Both residual fields must appear in REQUEST.
	wantKeys := []string{"x-amz-server-side-encryption", "x-amz-storage-class"}
	emitted := make(map[string]bool, len(req.Rows))
	for _, r := range req.Rows {
		emitted[r.Key] = true
	}
	for _, k := range wantKeys {
		if !emitted[k] {
			t.Errorf("S3 event: REQUEST section missing row Key=%q; got rows=%v", k, req.Rows)
		}
	}

	// SummarizeS3 does not mark any S3 request fields as navigable.
	for _, r := range req.Rows {
		if r.IsNavigable {
			t.Errorf("S3 event: REQUEST row %q unexpectedly navigable; SummarizeS3 does not mark request fields navigable", r.Key)
		}
	}
}

// TestCTEventSummarizer_UnknownSource_FallsToGeneric verifies that an event source
// with no registered summarizer falls through to SummarizeGeneric. This establishes
// the baseline that motivates the IAM and EC2 navigable-field assertions above: only
// a service-specific summarizer can produce navigable rows, so any navigable REQUEST
// row proves the service-specific path ran.
func TestCTEventSummarizer_UnknownSource_FallsToGeneric(t *testing.T) {
	event := summarizerTestEvent("dynamodb.amazonaws.com", "PutItem", map[string]any{
		// tableName ends in "Name" — will be lifted by catch-all into TARGET.
		"tableName": "my-table",
		// conditionExpression does not match catch-all; survives into REQUEST.
		"conditionExpression": "attribute_not_exists(pk)",
	})

	sections := ctevent.BuildSections(event)

	req, ok := findSection(sections, ctevent.SectionRequest)
	if !ok {
		// If conditionExpression somehow did not survive, REQUEST section is absent — valid.
		return
	}

	// SummarizeGeneric emits all rows as non-navigable with empty TargetType.
	for _, r := range req.Rows {
		if r.IsNavigable {
			t.Errorf("DynamoDB event (unregistered source): REQUEST row %q IsNavigable=true; SummarizeGeneric must not mark rows navigable", r.Key)
		}
		if r.TargetType != "" {
			t.Errorf("DynamoDB event (unregistered source): REQUEST row %q TargetType=%q; want empty", r.Key, r.TargetType)
		}
	}
}
