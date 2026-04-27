package unit

import (
	"strings"
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/internal/semantics/ctevent"
)

// ---------- helpers ----------

func sectionNames(sections []ctevent.Section) []string {
	names := make([]string, len(sections))
	for i, s := range sections {
		names[i] = s.Name
	}
	return names
}

func findSection(sections []ctevent.Section, name string) (ctevent.Section, bool) {
	for _, s := range sections {
		if s.Name == name {
			return s, true
		}
	}
	return ctevent.Section{}, false
}

func findRow(rows []ctevent.Row, keySubstr string) (ctevent.Row, bool) {
	for _, r := range rows {
		if strings.Contains(r.Key, keySubstr) {
			return r, true
		}
	}
	return ctevent.Row{}, false
}

func allRows(sections []ctevent.Section) []ctevent.Row {
	var rows []ctevent.Row
	for _, s := range sections {
		rows = append(rows, s.Rows...)
	}
	return rows
}

// minimalEvent returns a well-formed base event (Management/AwsApiCall, ct-info)
// suitable for customisation per test.
func minimalEvent() *ctevent.Event {
	return &ctevent.Event{
		EventID:            "e-000000000000",
		EventTime:          time.Date(2026, 4, 7, 14, 0, 0, 0, time.UTC),
		EventSource:        "ec2.amazonaws.com",
		EventName:          "DescribeInstances",
		EventCategory:      "Management",
		EventType:          "AwsApiCall",
		AWSRegion:          "us-east-1",
		SourceIPAddress:    "10.0.0.1",
		AccountID:          "111111111111",
		RecipientAccountID: "111111111111",
		Status:             "ct-info",
		Verb:               "R",
		UserIdentity: ctevent.UserIdentity{
			Type: "AssumedRole",
			ARN:  "arn:aws:sts::111111111111:assumed-role/TestRole/session",
		},
	}
}

// ---------- 1. Section ordering ----------

func TestCTDetailBuildSections_SectionOrdering_SuccessfulApiCall(t *testing.T) {
	event := minimalEvent()
	event.RequestParameters = map[string]any{"filter": "running"}
	event.ResponseElements = map[string]any{"instances": []any{"i-1"}}
	event.Resources = []ctevent.ResourceRef{{ARN: "arn:aws:ec2:us-east-1:111111111111:instance/i-1", Type: "AWS::EC2::Instance"}}

	sections := ctevent.BuildSections(event)
	if sections == nil {
		t.Fatal("BuildSections returned nil")
	}

	names := sectionNames(sections)
	// Must not contain ERROR (no errorCode)
	for _, n := range names {
		if n == ctevent.SectionError {
			t.Errorf("unexpected ERROR section when errorCode is empty; got sections: %v", names)
		}
	}

	// Ordering of present sections: ACTOR before ACTION before CONTEXT
	order := map[string]int{}
	for i, n := range names {
		order[n] = i
	}
	required := []struct{ before, after string }{
		{ctevent.SectionActor, ctevent.SectionAction},
		{ctevent.SectionAction, ctevent.SectionContext},
	}
	for _, pair := range required {
		a, aOK := order[pair.before]
		b, bOK := order[pair.after]
		if aOK && bOK && a > b {
			t.Errorf("section %s (idx %d) must come before %s (idx %d)", pair.before, a, pair.after, b)
		}
	}
}

func TestCTDetailBuildSections_SectionOrdering_WithError(t *testing.T) {
	event := minimalEvent()
	event.ErrorCode = "AccessDenied"
	event.ErrorMessage = "denied"

	sections := ctevent.BuildSections(event)
	if sections == nil {
		t.Fatal("BuildSections returned nil")
	}

	names := sectionNames(sections)
	order := map[string]int{}
	for i, n := range names {
		order[n] = i
	}

	// ERROR must be present
	errIdx, errOK := order[ctevent.SectionError]
	if !errOK {
		t.Fatalf("ERROR section missing when errorCode is set; got: %v", names)
	}

	// ERROR must come after CONTEXT
	if ctxIdx, ctxOK := order[ctevent.SectionContext]; ctxOK {
		if errIdx <= ctxIdx {
			t.Errorf("ERROR (idx %d) must come after CONTEXT (idx %d); got sections: %v", errIdx, ctxIdx, names)
		}
	}

	// ERROR must come before REQUEST (if present)
	if reqIdx, reqOK := order[ctevent.SectionRequest]; reqOK {
		if errIdx >= reqIdx {
			t.Errorf("ERROR (idx %d) must come before REQUEST (idx %d); got sections: %v", errIdx, reqIdx, names)
		}
	}

	// ERROR must come before RESPONSE (if present)
	if respIdx, respOK := order[ctevent.SectionResponse]; respOK {
		if errIdx >= respIdx {
			t.Errorf("ERROR (idx %d) must come before RESPONSE (idx %d); got sections: %v", errIdx, respIdx, names)
		}
	}
}

func TestCTDetailBuildSections_SectionOrdering_FullOrder(t *testing.T) {
	// Build an event that should produce all 7 sections.
	event := minimalEvent()
	event.ErrorCode = "SomeError"
	event.ErrorMessage = "some error message"
	event.Resources = []ctevent.ResourceRef{{ARN: "arn:aws:ec2:us-east-1:111111111111:instance/i-abc", Type: "AWS::EC2::Instance"}}
	event.RequestParameters = map[string]any{"dryRun": false}
	event.ResponseElements = map[string]any{"result": "ok"}

	sections := ctevent.BuildSections(event)
	if sections == nil {
		t.Fatal("BuildSections returned nil")
	}

	// Define canonical order and check any present sections follow it.
	canonicalOrder := []string{
		ctevent.SectionActor,
		ctevent.SectionAction,
		ctevent.SectionTarget,
		ctevent.SectionContext,
		ctevent.SectionError,
		ctevent.SectionRequest,
		ctevent.SectionResponse,
	}

	names := sectionNames(sections)
	pos := map[string]int{}
	for i, n := range names {
		pos[n] = i
	}

	for i := 0; i < len(canonicalOrder)-1; i++ {
		a := canonicalOrder[i]
		b := canonicalOrder[i+1]
		aIdx, aOK := pos[a]
		bIdx, bOK := pos[b]
		if aOK && bOK && aIdx > bIdx {
			t.Errorf("canonical order violation: %s (idx %d) must be before %s (idx %d); sections: %v", a, aIdx, b, bIdx, names)
		}
	}
}

// ---------- 2. ERROR hoist position ----------

func TestCTDetailBuildSections_ErrorHoist_OnlyWhenErrorCodeSet(t *testing.T) {
	t.Run("no error code - no ERROR section", func(t *testing.T) {
		event := minimalEvent()
		sections := ctevent.BuildSections(event)
		if sections == nil {
			t.Fatal("BuildSections returned nil")
		}
		if _, ok := findSection(sections, ctevent.SectionError); ok {
			t.Error("ERROR section present when errorCode is empty")
		}
	})

	t.Run("with error code - ERROR section present", func(t *testing.T) {
		event := minimalEvent()
		event.ErrorCode = "AccessDenied"
		event.ErrorMessage = "not allowed"
		sections := ctevent.BuildSections(event)
		if sections == nil {
			t.Fatal("BuildSections returned nil")
		}
		errSec, ok := findSection(sections, ctevent.SectionError)
		if !ok {
			t.Fatal("ERROR section missing when errorCode is set")
		}
		// Must have errorCode row
		if _, rowOK := findRow(errSec.Rows, "errorCode"); !rowOK {
			t.Error("ERROR section missing errorCode row")
		}
	})
}

// ---------- 3. Empty section omission ----------

func TestCTDetailBuildSections_EmptySectionOmission_NoResponseElements(t *testing.T) {
	event := minimalEvent()
	event.ResponseElements = nil // explicitly nil → no RESPONSE

	sections := ctevent.BuildSections(event)
	if sections == nil {
		t.Fatal("BuildSections returned nil")
	}
	if _, ok := findSection(sections, ctevent.SectionResponse); ok {
		t.Error("RESPONSE section present when responseElements is nil")
	}
}

func TestCTDetailBuildSections_EmptySectionOmission_NoTarget(t *testing.T) {
	event := minimalEvent()
	event.Resources = nil
	event.RequestParameters = nil // no extractable target

	sections := ctevent.BuildSections(event)
	if sections == nil {
		t.Fatal("BuildSections returned nil")
	}
	if _, ok := findSection(sections, ctevent.SectionTarget); ok {
		t.Error("TARGET section present when no target can be extracted")
	}
}

func TestCTDetailBuildSections_EmptySectionOmission_NoRequest(t *testing.T) {
	event := minimalEvent()
	event.RequestParameters = nil

	sections := ctevent.BuildSections(event)
	if sections == nil {
		t.Fatal("BuildSections returned nil")
	}
	// With no requestParameters there's nothing to summarize → REQUEST omitted
	if _, ok := findSection(sections, ctevent.SectionRequest); ok {
		t.Error("REQUEST section present when requestParameters is nil")
	}
}

func TestCTDetailBuildSections_NonNilReturn(t *testing.T) {
	// Even a completely empty/degenerate event must return non-nil.
	event := &ctevent.Event{
		EventID:       "e-degenerate",
		EventCategory: "Management",
		EventType:     "AwsApiCall",
		Status:        "ct-info",
	}
	sections := ctevent.BuildSections(event)
	if sections == nil {
		t.Fatal("BuildSections must return non-nil slice (contract: possibly empty, never nil)")
	}
}

// ---------- 4. Insight events omit ACTOR ----------

func TestCTDetailBuildSections_InsightOmitsActor(t *testing.T) {
	event := &ctevent.Event{
		EventID:            "e-insight-001",
		EventTime:          time.Date(2026, 4, 7, 9, 14, 0, 0, time.UTC),
		EventSource:        "ec2.amazonaws.com",
		EventName:          "RunInstances",
		EventCategory:      "Insight",
		EventType:          "AwsApiCall",
		AWSRegion:          "us-east-1",
		AccountID:          "999999999999",
		RecipientAccountID: "999999999999",
		Status:             "ct-info",
		Verb:               "R",
		// No UserIdentity ARN → should produce no ACTOR
		UserIdentity: ctevent.UserIdentity{},
		InsightDetails: &ctevent.InsightDetails{
			State:       "Start",
			InsightType: "ApiCallRateInsight",
			EventSource: "ec2.amazonaws.com",
			EventName:   "RunInstances",
		},
	}

	sections := ctevent.BuildSections(event)
	if sections == nil {
		t.Fatal("BuildSections returned nil")
	}

	if _, ok := findSection(sections, ctevent.SectionActor); ok {
		t.Error("Insight event MUST NOT have ACTOR section")
	}

	// Must start with ACTION
	if len(sections) == 0 || sections[0].Name != ctevent.SectionAction {
		names := sectionNames(sections)
		t.Errorf("Insight event must start with ACTION; got sections: %v", names)
	}
}

// ---------- 5. AwsServiceEvent emits Service row in ACTOR ----------

func TestCTDetailBuildSections_AwsServiceEvent_ServiceRowInActor(t *testing.T) {
	event := &ctevent.Event{
		EventID:            "e-d4e5f6a7",
		EventTime:          time.Date(2026, 4, 7, 2, 0, 7, 0, time.UTC),
		EventSource:        "kms.amazonaws.com",
		EventName:          "RotateKey",
		EventCategory:      "Management",
		EventType:          "AwsServiceEvent",
		AWSRegion:          "us-east-1",
		SourceIPAddress:    "AWS Internal",
		AccountID:          "444444444444",
		RecipientAccountID: "444444444444",
		Status:             "ct-attention",
		Verb:               "W",
		UserIdentity: ctevent.UserIdentity{
			Type:      "AWSService",
			InvokedBy: "kms.amazonaws.com",
			// No ARN for a service event
		},
	}

	sections := ctevent.BuildSections(event)
	if sections == nil {
		t.Fatal("BuildSections returned nil")
	}

	actorSec, ok := findSection(sections, ctevent.SectionActor)
	if !ok {
		t.Fatal("ACTOR section missing for AwsServiceEvent")
	}

	serviceRow, rowOK := findRow(actorSec.Rows, "Service")
	if !rowOK {
		t.Errorf("ACTOR section must contain a 'Service' row for AwsServiceEvent; got rows: %+v", actorSec.Rows)
	} else if serviceRow.Value == "" {
		t.Error("Service row value must be non-empty")
	}

	// Must NOT have a Principal row (no ARN)
	if _, principalOK := findRow(actorSec.Rows, "Principal"); principalOK {
		t.Error("AwsServiceEvent ACTOR must not have a Principal row when no ARN is present")
	}
}

// ---------- 6. Event row carries Severity ----------

func TestCTDetailBuildSections_EventRowSeverity(t *testing.T) {
	cases := []struct {
		status string
	}{
		{"ct-info"},
		{"ct-attention"},
		{"ct-danger"},
	}

	for _, tc := range cases {
		t.Run(tc.status, func(t *testing.T) {
			event := minimalEvent()
			event.Status = tc.status

			sections := ctevent.BuildSections(event)
			if sections == nil {
				t.Fatal("BuildSections returned nil")
			}

			actionSec, ok := findSection(sections, ctevent.SectionAction)
			if !ok {
				t.Fatal("ACTION section missing")
			}

			eventRow, rowOK := findRow(actionSec.Rows, "Event")
			if !rowOK {
				t.Fatal("ACTION section must contain an 'Event' row")
			}

			if eventRow.Severity != tc.status {
				t.Errorf("Event row Severity = %q; want %q", eventRow.Severity, tc.status)
			}
		})
	}
}

func TestCTDetailBuildSections_OnlyEventRowHasSeverity(t *testing.T) {
	// Test all three severity tiers — no other row may have non-empty Severity.
	statuses := []string{"ct-info", "ct-attention", "ct-danger"}

	for _, status := range statuses {
		t.Run(status, func(t *testing.T) {
			event := minimalEvent()
			event.Status = status
			event.ErrorCode = "SomeError"
			event.ErrorMessage = "an error occurred"
			event.Resources = []ctevent.ResourceRef{{ARN: "arn:aws:ec2:us-east-1:111111111111:instance/i-abc", Type: "AWS::EC2::Instance"}}
			event.RequestParameters = map[string]any{"param": "value"}
			event.ResponseElements = map[string]any{"result": "done"}

			sections := ctevent.BuildSections(event)
			if sections == nil {
				t.Fatal("BuildSections returned nil")
			}

			for _, sec := range sections {
				for _, row := range sec.Rows {
					isEventRow := sec.Name == ctevent.SectionAction && strings.Contains(row.Key, "Event")
					if !isEventRow && row.Severity != "" {
						t.Errorf("section %s row %q has non-empty Severity=%q; only ACTION/Event row may set Severity",
							sec.Name, row.Key, row.Severity)
					}
				}
			}
		})
	}
}

// ---------- 7. Cross-account row in CONTEXT ----------

func TestCTDetailBuildSections_CrossAccount_RecipientRowPresent(t *testing.T) {
	event := minimalEvent()
	event.AccountID = "888888888888"
	event.RecipientAccountID = "777777777777" // different → cross-account
	event.UserIdentity.ARN = "arn:aws:sts::888888888888:assumed-role/CiBuildRole/build-4821"

	sections := ctevent.BuildSections(event)
	if sections == nil {
		t.Fatal("BuildSections returned nil")
	}

	ctxSec, ok := findSection(sections, ctevent.SectionContext)
	if !ok {
		t.Fatal("CONTEXT section missing")
	}

	recipientRow, rowOK := findRow(ctxSec.Rows, "Recipient")
	if !rowOK {
		t.Errorf("CONTEXT must contain a Recipient row when accountId != recipientAccountId; rows: %+v", ctxSec.Rows)
	} else {
		if !strings.Contains(recipientRow.Value, "777777777777") {
			t.Errorf("Recipient row value must contain recipient account ID 777777777777; got %q", recipientRow.Value)
		}
		if !strings.Contains(recipientRow.Value, "cross-account") {
			t.Errorf("Recipient row value must contain 'cross-account'; got %q", recipientRow.Value)
		}
	}
}

func TestCTDetailBuildSections_CrossAccount_NoRecipientRowWhenSameAccount(t *testing.T) {
	event := minimalEvent()
	event.AccountID = "111111111111"
	event.RecipientAccountID = "111111111111" // same → no cross-account

	sections := ctevent.BuildSections(event)
	if sections == nil {
		t.Fatal("BuildSections returned nil")
	}

	ctxSec, ok := findSection(sections, ctevent.SectionContext)
	if !ok {
		t.Fatal("CONTEXT section missing")
	}

	if _, rowOK := findRow(ctxSec.Rows, "Recipient"); rowOK {
		t.Error("CONTEXT must NOT contain Recipient row when accountId == recipientAccountId")
	}
}

// ---------- 8. MFA row only when true ----------

func TestCTDetailBuildSections_MFARow_OnlyWhenTrue(t *testing.T) {
	t.Run("mfa true - row present", func(t *testing.T) {
		event := minimalEvent()
		event.UserIdentity.SessionContext = &ctevent.SessionContext{
			Attributes: ctevent.SessionAttributes{MFAAuthenticated: true},
		}

		sections := ctevent.BuildSections(event)
		if sections == nil {
			t.Fatal("BuildSections returned nil")
		}

		actorSec, ok := findSection(sections, ctevent.SectionActor)
		if !ok {
			t.Fatal("ACTOR section missing")
		}

		mfaRow, rowOK := findRow(actorSec.Rows, "MFA")
		if !rowOK {
			t.Error("ACTOR must contain MFA row when mfaAuthenticated=true")
		} else if mfaRow.Value != "yes" {
			t.Errorf("MFA row value = %q; want %q", mfaRow.Value, "yes")
		}
	})

	t.Run("mfa false - no row", func(t *testing.T) {
		event := minimalEvent()
		event.UserIdentity.SessionContext = &ctevent.SessionContext{
			Attributes: ctevent.SessionAttributes{MFAAuthenticated: false},
		}

		sections := ctevent.BuildSections(event)
		if sections == nil {
			t.Fatal("BuildSections returned nil")
		}

		actorSec, ok := findSection(sections, ctevent.SectionActor)
		if !ok {
			// No ACTOR is also acceptable (e.g. empty identity), skip MFA check
			return
		}

		if _, rowOK := findRow(actorSec.Rows, "MFA"); rowOK {
			t.Error("ACTOR must NOT contain MFA row when mfaAuthenticated=false")
		}
	})

	t.Run("no session context - no mfa row", func(t *testing.T) {
		event := minimalEvent()
		event.UserIdentity.SessionContext = nil

		sections := ctevent.BuildSections(event)
		if sections == nil {
			t.Fatal("BuildSections returned nil")
		}

		actorSec, ok := findSection(sections, ctevent.SectionActor)
		if !ok {
			return
		}

		if _, rowOK := findRow(actorSec.Rows, "MFA"); rowOK {
			t.Error("ACTOR must NOT contain MFA row when SessionContext is nil")
		}
	})
}

// ---------- 9. Drop-boring-defaults ----------

func TestCTDetailBuildSections_DroppedBoringKeys(t *testing.T) {
	// These keys must never appear in any section regardless of input.
	neverKeys := []string{"Verb", "Read only", "Identity type", "Source"}

	event := minimalEvent()
	event.RequestParameters = map[string]any{
		"param1":          "value1",
		"Verb":            "W",
		"Read only":       "false",
		"SourceIPAddress": "10.0.0.1",
	}
	event.ResponseElements = map[string]any{"result": "ok"}

	sections := ctevent.BuildSections(event)
	if sections == nil {
		t.Fatal("BuildSections returned nil")
	}

	rows := allRows(sections)
	for _, forbidden := range neverKeys {
		for _, row := range rows {
			if row.Key == forbidden {
				t.Errorf("forbidden key %q found in section output (drop-boring-defaults violated)", forbidden)
			}
		}
	}
}

func TestCTDetailBuildSections_NoStandaloneAccountRow(t *testing.T) {
	event := minimalEvent()

	sections := ctevent.BuildSections(event)
	if sections == nil {
		t.Fatal("BuildSections returned nil")
	}

	rows := allRows(sections)
	for _, row := range rows {
		if row.Key == "Account" {
			t.Errorf("standalone 'Account' row found (drop-boring-defaults: account is encoded in ARN)")
		}
	}
}

// ---------- 10. Wireframe cases A–I ----------

func TestCTDetailBuildSections_WireframeCases(t *testing.T) {
	type sectionCheck struct {
		sectionName string
		rowKeyHint  string // empty means just check section exists
		valueHint   string // empty means skip value check
	}
	type wireCase struct {
		name           string
		event          *ctevent.Event
		expectSections []string // must all be present
		noSections     []string // must not be present
		firstSection   string   // if non-empty, sections[0].Name must match
		checks         []sectionCheck
	}

	cases := []wireCase{
		{
			// Case A — Karpenter ec2:DescribeInstances (read, success → ct-info)
			name: "A_Karpenter_DescribeInstances_ctinfo",
			event: &ctevent.Event{
				EventID:            "e-a1b2c3d4",
				EventTime:          time.Date(2026, 4, 7, 14, 2, 11, 0, time.UTC),
				EventSource:        "ec2.amazonaws.com",
				EventName:          "DescribeInstances",
				EventCategory:      "Management",
				EventType:          "AwsApiCall",
				AWSRegion:          "us-east-1",
				SourceIPAddress:    "10.0.14.221",
				UserAgent:          "aws-sdk-go-v2/1.30.3",
				AccountID:          "111111111111",
				RecipientAccountID: "111111111111",
				Status:             "ct-info",
				Verb:               "R",
				UserIdentity: ctevent.UserIdentity{
					Type:        "AssumedRole",
					ARN:         "arn:aws:sts::111111111111:assumed-role/KarpenterNodeRole/karpenter-1759",
					AccessKeyID: "ASIAY44QH8DCKARPEXMP",
				},
				RequestParameters: map[string]any{
					"filters":    []any{map[string]any{"Name": "instance-state-name", "Values": []any{"running"}}},
					"maxResults": 1000,
				},
			},
			expectSections: []string{ctevent.SectionActor, ctevent.SectionAction, ctevent.SectionContext, ctevent.SectionRequest},
			noSections:     []string{ctevent.SectionError},
			checks: []sectionCheck{
				{ctevent.SectionAction, "Event", "ec2:DescribeInstances"},
				{ctevent.SectionActor, "Principal", "KarpenterNodeRole"},
			},
		},
		{
			// Case B — SSO ec2:TerminateInstances (D verb, MFA → ct-danger)
			name: "B_SSO_TerminateInstances_ctdanger",
			event: &ctevent.Event{
				EventID:            "e-b2c3d4e5",
				EventTime:          time.Date(2026, 4, 7, 14, 7, 42, 0, time.UTC),
				EventSource:        "ec2.amazonaws.com",
				EventName:          "TerminateInstances",
				EventCategory:      "Management",
				EventType:          "AwsApiCall",
				AWSRegion:          "eu-west-1",
				SourceIPAddress:    "AWS Internal",
				UserAgent:          "Console (AWS Internal)",
				AccountID:          "222222222222",
				RecipientAccountID: "222222222222",
				Status:             "ct-danger",
				Verb:               "D",
				UserIdentity: ctevent.UserIdentity{
					Type:        "AssumedRole",
					ARN:         "arn:aws:sts::222222222222:assumed-role/AWSReservedSSO_AdminAccess_3c4d5e6f7a8b9c0d/alice@corp",
					AccessKeyID: "ASIAZK7L9PQRSSOXEXMP",
					SessionContext: &ctevent.SessionContext{
						Attributes: ctevent.SessionAttributes{MFAAuthenticated: true},
					},
				},
				Resources: []ctevent.ResourceRef{
					{ARN: "arn:aws:ec2:eu-west-1:222222222222:instance/i-0f1e2d3c4b5a69788", Type: "AWS::EC2::Instance"},
					{ARN: "arn:aws:ec2:eu-west-1:222222222222:instance/i-0f1e2d3c4b5a69789", Type: "AWS::EC2::Instance"},
				},
				ResponseElements: map[string]any{
					"terminating": []any{"i-0f1e2d3c4b5a69788", "i-0f1e2d3c4b5a69789"},
				},
			},
			expectSections: []string{ctevent.SectionActor, ctevent.SectionAction, ctevent.SectionTarget, ctevent.SectionContext, ctevent.SectionResponse},
			noSections:     []string{ctevent.SectionError, ctevent.SectionRequest},
			checks: []sectionCheck{
				{ctevent.SectionAction, "Event", "ec2:TerminateInstances"},
				{ctevent.SectionActor, "MFA", "yes"},
				{ctevent.SectionTarget, "", ""},
			},
		},
		{
			// Case C — s3:PutObject AccessDenied (ERROR hoisted after CONTEXT)
			name: "C_S3_PutObject_AccessDenied_ctdanger",
			event: &ctevent.Event{
				EventID:            "e-c3d4e5f6",
				EventTime:          time.Date(2026, 4, 7, 14, 11, 3, 0, time.UTC),
				EventSource:        "s3.amazonaws.com",
				EventName:          "PutObject",
				EventCategory:      "Management",
				EventType:          "AwsApiCall",
				AWSRegion:          "us-east-1",
				SourceIPAddress:    "198.51.100.42",
				UserAgent:          "aws-cli/2.17.9 Python/3.12.4 Darwin/24.1.0",
				AccountID:          "333333333333",
				RecipientAccountID: "333333333333",
				Status:             "ct-danger",
				Verb:               "W",
				ErrorCode:          "AccessDenied",
				ErrorMessage:       "User: arn:aws:iam::333333333333:user/bob is not authorized to perform: s3:PutObject",
				UserIdentity: ctevent.UserIdentity{
					Type:        "IAMUser",
					ARN:         "arn:aws:iam::333333333333:user/bob",
					AccessKeyID: "AKIAIOSFODNN7BOB1XMP",
				},
				RequestParameters: map[string]any{
					"bucketName": "prod-logs",
					"key":        "prod-logs/2026/04/07/app.log",
				},
			},
			expectSections: []string{ctevent.SectionActor, ctevent.SectionAction, ctevent.SectionContext, ctevent.SectionError},
			checks: []sectionCheck{
				{ctevent.SectionAction, "Event", "s3:PutObject"},
				{ctevent.SectionError, "errorCode", "AccessDenied"},
			},
		},
		{
			// Case D — KMS kms:RotateKey (AwsServiceEvent → ct-attention)
			name: "D_KMS_RotateKey_AwsServiceEvent_ctattention",
			event: &ctevent.Event{
				EventID:            "e-d4e5f6a7",
				EventTime:          time.Date(2026, 4, 7, 2, 0, 7, 0, time.UTC),
				EventSource:        "kms.amazonaws.com",
				EventName:          "RotateKey",
				EventCategory:      "Management",
				EventType:          "AwsServiceEvent",
				AWSRegion:          "us-east-1",
				SourceIPAddress:    "AWS Internal",
				AccountID:          "444444444444",
				RecipientAccountID: "444444444444",
				Status:             "ct-attention",
				Verb:               "W",
				UserIdentity: ctevent.UserIdentity{
					Type:      "AWSService",
					InvokedBy: "kms.amazonaws.com",
				},
				RequestParameters: map[string]any{
					"keyId":        "arn:aws:kms:us-east-1:444444444444:key/2f7e9a5b-8c1d-4e3f-9a0b-1c2d3e4f5a6b",
					"rotationType": "AUTOMATIC",
					"backingKey":   true,
				},
			},
			expectSections: []string{ctevent.SectionActor, ctevent.SectionAction, ctevent.SectionContext},
			checks: []sectionCheck{
				{ctevent.SectionActor, "Service", "kms.amazonaws.com"},
				{ctevent.SectionAction, "Event", "kms:RotateKey"},
				{ctevent.SectionAction, "Category", "AwsServiceEvent"},
			},
		},
		{
			// Case E — Root s3:PutBucketPolicy (Root + W → ct-attention)
			name: "E_Root_PutBucketPolicy_ctattention",
			event: &ctevent.Event{
				EventID:            "e-e5f6a7b8",
				EventTime:          time.Date(2026, 4, 7, 3, 42, 18, 0, time.UTC),
				EventSource:        "s3.amazonaws.com",
				EventName:          "PutBucketPolicy",
				EventCategory:      "Management",
				EventType:          "AwsApiCall",
				AWSRegion:          "us-east-1",
				SourceIPAddress:    "203.0.113.17",
				UserAgent:          "Console (Mozilla/5.0 Safari/605.1.15)",
				AccountID:          "555555555555",
				RecipientAccountID: "555555555555",
				Status:             "ct-attention",
				Verb:               "W",
				UserIdentity: ctevent.UserIdentity{
					Type: "Root",
					ARN:  "arn:aws:iam::555555555555:root",
				},
				RequestParameters: map[string]any{
					"bucketName": "prod-artifacts",
					"policy":     `{"Version":"2012-10-17","Statement":[]}`,
				},
			},
			expectSections: []string{ctevent.SectionActor, ctevent.SectionAction, ctevent.SectionContext},
			noSections:     []string{ctevent.SectionError},
			checks: []sectionCheck{
				{ctevent.SectionActor, "Principal", "root"},
				{ctevent.SectionAction, "Event", "s3:PutBucketPolicy"},
			},
		},
		{
			// Case F — IRSA s3:GetObject (WebIdentityUser, R → ct-info)
			name: "F_IRSA_GetObject_ctinfo",
			event: &ctevent.Event{
				EventID:            "e-f6a7b8c9",
				EventTime:          time.Date(2026, 4, 7, 14, 20, 21, 0, time.UTC),
				EventSource:        "s3.amazonaws.com",
				EventName:          "GetObject",
				EventCategory:      "Management",
				EventType:          "AwsApiCall",
				AWSRegion:          "eu-west-1",
				SourceIPAddress:    "10.42.3.18",
				UserAgent:          "aws-sdk-go-v2/1.30.3",
				AccountID:          "666666666666",
				RecipientAccountID: "666666666666",
				Status:             "ct-info",
				Verb:               "R",
				UserIdentity: ctevent.UserIdentity{
					Type: "WebIdentityUser",
					ARN:  "arn:aws:sts::666666666666:assumed-role/eks-checkout-svc-sa/1717156821993453824",
					SessionContext: &ctevent.SessionContext{
						WebIDFederationData: &ctevent.WebIDFederationData{
							FederatedProvider: "oidc.eks.eu-west-1.amazonaws.com/id/EXAMPLE0D8C",
						},
					},
				},
				RequestParameters: map[string]any{
					"bucketName": "checkout-config",
					"key":        "checkout-config/prod/config.json",
				},
			},
			expectSections: []string{ctevent.SectionActor, ctevent.SectionAction, ctevent.SectionContext},
			noSections:     []string{ctevent.SectionError},
			checks: []sectionCheck{
				{ctevent.SectionActor, "Federation", "oidc.eks.eu-west-1.amazonaws.com"},
				{ctevent.SectionAction, "Event", "s3:GetObject"},
			},
		},
		{
			// Case G — Cross-account s3:PutObject (W + cross-acct → ct-attention)
			name: "G_CrossAccount_PutObject_ctattention",
			event: &ctevent.Event{
				EventID:            "e-a7b8c9d0",
				EventTime:          time.Date(2026, 4, 7, 14, 31, 55, 0, time.UTC),
				EventSource:        "s3.amazonaws.com",
				EventName:          "PutObject",
				EventCategory:      "Management",
				EventType:          "AwsApiCall",
				AWSRegion:          "us-east-2",
				SourceIPAddress:    "52.14.88.201",
				UserAgent:          "aws-cli/2.17.9",
				AccountID:          "888888888888",
				RecipientAccountID: "777777777777",
				Status:             "ct-attention",
				Verb:               "W",
				UserIdentity: ctevent.UserIdentity{
					Type:        "AssumedRole",
					ARN:         "arn:aws:sts::888888888888:assumed-role/CiBuildRole/build-4821",
					AccessKeyID: "ASIAQF3M2N8KCIB1XMPL",
				},
				RequestParameters: map[string]any{
					"bucketName": "shared-artifacts",
					"key":        "shared-artifacts/build-4821.tar.gz",
				},
			},
			expectSections: []string{ctevent.SectionActor, ctevent.SectionAction, ctevent.SectionContext},
			noSections:     []string{ctevent.SectionError},
			checks: []sectionCheck{
				{ctevent.SectionContext, "Recipient", "777777777777"},
				{ctevent.SectionAction, "Event", "s3:PutObject"},
			},
		},
		{
			// Case H — Insight ApiCallRateInsight (no ACTOR, starts at ACTION)
			name: "H_Insight_ApiCallRateInsight_noactor",
			event: &ctevent.Event{
				EventID:            "e-b8c9d0e1",
				EventTime:          time.Date(2026, 4, 7, 9, 14, 0, 0, time.UTC),
				EventSource:        "ec2.amazonaws.com",
				EventName:          "RunInstances",
				EventCategory:      "Insight",
				EventType:          "AwsApiCall",
				AWSRegion:          "us-east-1",
				AccountID:          "999999999999",
				RecipientAccountID: "999999999999",
				Status:             "ct-info",
				Verb:               "R",
				UserIdentity:       ctevent.UserIdentity{}, // empty
				InsightDetails: &ctevent.InsightDetails{
					State:       "Start",
					InsightType: "ApiCallRateInsight",
					EventSource: "ec2.amazonaws.com",
					EventName:   "RunInstances",
				},
			},
			expectSections: []string{ctevent.SectionAction, ctevent.SectionContext},
			noSections:     []string{ctevent.SectionActor, ctevent.SectionError},
			firstSection:   ctevent.SectionAction,
			checks: []sectionCheck{
				{ctevent.SectionAction, "Insight type", "ApiCallRateInsight"},
				{ctevent.SectionAction, "State", "Start"},
				{ctevent.SectionAction, "Category", "Insight"},
			},
		},
		{
			// Case I — NetworkActivity VPCE deny (errorCode → ct-danger, ERROR hoisted)
			name: "I_NetworkActivity_VPCEDeny_ctdanger",
			event: &ctevent.Event{
				EventID:            "e-c9d0e1f2",
				EventTime:          time.Date(2026, 4, 7, 14, 44, 17, 0, time.UTC),
				EventSource:        "s3.amazonaws.com",
				EventName:          "PutObject",
				EventCategory:      "NetworkActivity",
				EventType:          "AwsVpceEvent",
				AWSRegion:          "eu-central-1",
				SourceIPAddress:    "10.12.4.77",
				AccountID:          "111111111111",
				RecipientAccountID: "111111111111",
				Status:             "ct-danger",
				Verb:               "W",
				ErrorCode:          "VpceAccessDenied",
				ErrorMessage:       "The VPC endpoint policy denies the s3:PutObject action",
				UserIdentity: ctevent.UserIdentity{
					Type: "AssumedRole",
					ARN:  "arn:aws:sts::111111111111:assumed-role/DataPipelineRole/dp-0719",
				},
				RequestParameters: map[string]any{
					"bucketName": "prod-lake",
					"key":        "prod-lake/landing/2026/04/07/batch-0719.parquet",
				},
			},
			expectSections: []string{ctevent.SectionActor, ctevent.SectionAction, ctevent.SectionContext, ctevent.SectionError},
			checks: []sectionCheck{
				{ctevent.SectionAction, "Event", "s3:PutObject"},
				{ctevent.SectionAction, "Category", "NetworkActivity"},
				{ctevent.SectionError, "errorCode", "VpceAccessDenied"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sections := ctevent.BuildSections(tc.event)
			if sections == nil {
				t.Fatalf("BuildSections returned nil for case %s", tc.name)
			}

			names := sectionNames(sections)

			// Check expected sections are present
			for _, exp := range tc.expectSections {
				if _, ok := findSection(sections, exp); !ok {
					t.Errorf("expected section %s not found; got sections: %v", exp, names)
				}
			}

			// Check sections that must NOT be present
			for _, no := range tc.noSections {
				if _, ok := findSection(sections, no); ok {
					t.Errorf("section %s must not be present; got sections: %v", no, names)
				}
			}

			// Check first section
			if tc.firstSection != "" && len(sections) > 0 && sections[0].Name != tc.firstSection {
				t.Errorf("first section = %q; want %q; got sections: %v", sections[0].Name, tc.firstSection, names)
			}

			// Check specific rows
			for _, chk := range tc.checks {
				sec, ok := findSection(sections, chk.sectionName)
				if !ok {
					if chk.rowKeyHint == "" {
						// Just checking section presence, already done above
						continue
					}
					t.Errorf("section %s not found for row check %q", chk.sectionName, chk.rowKeyHint)
					continue
				}
				if chk.rowKeyHint == "" {
					continue
				}
				row, rowOK := findRow(sec.Rows, chk.rowKeyHint)
				if !rowOK {
					t.Errorf("[%s] section %s missing row with key hint %q; rows: %+v", tc.name, chk.sectionName, chk.rowKeyHint, sec.Rows)
					continue
				}
				if chk.valueHint != "" && !strings.Contains(row.Value, chk.valueHint) {
					t.Errorf("[%s] section %s row %q: value %q does not contain %q", tc.name, chk.sectionName, row.Key, row.Value, chk.valueHint)
				}
			}

			// For all cases: verify only ACTION/Event row has Severity set
			for _, sec := range sections {
				for _, row := range sec.Rows {
					isEventRow := sec.Name == ctevent.SectionAction && strings.Contains(row.Key, "Event")
					if !isEventRow && row.Severity != "" {
						t.Errorf("[%s] non-Event row %s/%s has non-empty Severity=%q", tc.name, sec.Name, row.Key, row.Severity)
					}
				}
			}

			// For cases with errorCode: verify ERROR comes after CONTEXT and before REQUEST/RESPONSE
			if tc.event.ErrorCode != "" {
				order := map[string]int{}
				for i, n := range names {
					order[n] = i
				}
				errIdx, errOK := order[ctevent.SectionError]
				if !errOK {
					t.Errorf("[%s] ERROR section missing despite errorCode=%q", tc.name, tc.event.ErrorCode)
				} else {
					if ctxIdx, ctxOK := order[ctevent.SectionContext]; ctxOK && errIdx <= ctxIdx {
						t.Errorf("[%s] ERROR (idx %d) must come after CONTEXT (idx %d)", tc.name, errIdx, ctxIdx)
					}
					if reqIdx, reqOK := order[ctevent.SectionRequest]; reqOK && errIdx >= reqIdx {
						t.Errorf("[%s] ERROR (idx %d) must come before REQUEST (idx %d)", tc.name, errIdx, reqIdx)
					}
				}
			}
		})
	}
}

// ---------- Regression: ERROR hoist position (FR-005 / design §2.5) ----------

// TestCTDetailBuildSections_Regression_ErrorHoistPosition is a named regression
// guard for the ERROR section hoist contract: ERROR must always sit after CONTEXT
// and before REQUEST/RESPONSE.  The name anchors future changes to FR-005 so that
// if TestCTDetailBuildSections_SectionOrdering_WithError is ever refactored away,
// this explicit contract survives.
func TestCTDetailBuildSections_Regression_ErrorHoistPosition(t *testing.T) {
	t.Run("RequestPresent", func(t *testing.T) {
		// Event with ErrorCode set, non-empty requestParameters (non-TARGET keys
		// survive de-dup so REQUEST section is produced), and non-nil responseElements.
		event := minimalEvent()
		event.ErrorCode = "AccessDenied"
		event.ErrorMessage = "User is not authorized"
		// Use a key that ExtractTarget will not consume as a target identifier
		// so it survives into cleanedParams → REQUEST section is produced.
		event.RequestParameters = map[string]any{"maxResults": 100, "nextToken": "abc"}
		event.ResponseElements = map[string]any{"instances": []any{"i-111"}}

		sections := ctevent.BuildSections(event)
		if sections == nil {
			t.Fatal("BuildSections returned nil")
		}

		names := sectionNames(sections)
		pos := map[string]int{}
		for i, n := range names {
			pos[n] = i
		}

		errIdx, errOK := pos[ctevent.SectionError]
		if !errOK {
			t.Fatalf("ERROR section missing despite ErrorCode set; sections: %v", names)
		}

		ctxIdx, ctxOK := pos[ctevent.SectionContext]
		if !ctxOK {
			t.Fatalf("CONTEXT section missing; sections: %v", names)
		}
		if errIdx <= ctxIdx {
			t.Errorf("regression FR-005: ERROR (idx %d) must come after CONTEXT (idx %d); sections: %v",
				errIdx, ctxIdx, names)
		}

		if reqIdx, reqOK := pos[ctevent.SectionRequest]; reqOK {
			if errIdx >= reqIdx {
				t.Errorf("regression FR-005: ERROR (idx %d) must come before REQUEST (idx %d); sections: %v",
					errIdx, reqIdx, names)
			}
		} else {
			t.Log("REQUEST section absent (de-dup consumed all params); REQUEST ordering constraint skipped")
		}

		if respIdx, respOK := pos[ctevent.SectionResponse]; respOK {
			if errIdx >= respIdx {
				t.Errorf("regression FR-005: ERROR (idx %d) must come before RESPONSE (idx %d); sections: %v",
					errIdx, respIdx, names)
			}
		} else {
			t.Log("RESPONSE section absent; RESPONSE ordering constraint skipped")
		}
	})

	t.Run("RequestAbsent", func(t *testing.T) {
		// Event with ErrorCode set, nil requestParameters, nil responseElements.
		// REQUEST and RESPONSE must both be omitted; ERROR must be the last section
		// and must still come after CONTEXT.
		event := minimalEvent()
		event.ErrorCode = "NoSuchBucket"
		event.ErrorMessage = "The specified bucket does not exist"
		event.RequestParameters = nil
		event.ResponseElements = nil

		sections := ctevent.BuildSections(event)
		if sections == nil {
			t.Fatal("BuildSections returned nil")
		}

		names := sectionNames(sections)
		pos := map[string]int{}
		for i, n := range names {
			pos[n] = i
		}

		errIdx, errOK := pos[ctevent.SectionError]
		if !errOK {
			t.Fatalf("ERROR section missing despite ErrorCode set; sections: %v", names)
		}

		ctxIdx, ctxOK := pos[ctevent.SectionContext]
		if !ctxOK {
			t.Fatalf("CONTEXT section missing; sections: %v", names)
		}
		if errIdx <= ctxIdx {
			t.Errorf("regression FR-005: ERROR (idx %d) must come after CONTEXT (idx %d); sections: %v",
				errIdx, ctxIdx, names)
		}

		// REQUEST and RESPONSE must be absent (nil inputs → empty rows → omitted).
		if _, reqOK := pos[ctevent.SectionRequest]; reqOK {
			t.Errorf("regression: REQUEST section present despite nil requestParameters; sections: %v", names)
		}
		if _, respOK := pos[ctevent.SectionResponse]; respOK {
			t.Errorf("regression: RESPONSE section present despite nil responseElements; sections: %v", names)
		}

		// ERROR must be the last section.
		wantLastIdx := len(sections) - 1
		if errIdx != wantLastIdx {
			t.Errorf("regression FR-005: ERROR (idx %d) must be last section (idx %d) when REQUEST+RESPONSE absent; sections: %v",
				errIdx, wantLastIdx, names)
		}
	})
}

// ---------- Event row value format ----------

func TestCTDetailBuildSections_EventRowValue_ServiceColonEventName(t *testing.T) {
	// ACTION Event: row value should be "<service>:<eventName>" format
	event := minimalEvent()
	// ec2.amazonaws.com → service prefix "ec2", event "DescribeInstances"
	// expected value contains "ec2" and "DescribeInstances"

	sections := ctevent.BuildSections(event)
	if sections == nil {
		t.Fatal("BuildSections returned nil")
	}

	actionSec, ok := findSection(sections, ctevent.SectionAction)
	if !ok {
		t.Fatal("ACTION section missing")
	}

	eventRow, rowOK := findRow(actionSec.Rows, "Event")
	if !rowOK {
		t.Fatal("ACTION section missing Event row")
	}

	if !strings.Contains(eventRow.Value, "DescribeInstances") {
		t.Errorf("Event row value %q must contain event name 'DescribeInstances'", eventRow.Value)
	}
	if !strings.Contains(eventRow.Value, "ec2") {
		t.Errorf("Event row value %q must contain service prefix 'ec2'", eventRow.Value)
	}
}
