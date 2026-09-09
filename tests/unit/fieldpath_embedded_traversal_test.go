package unit

// fieldpath_embedded_traversal_test.go — core/fieldpath's path walkers
// (ExtractValue, ExtractFirstListScalar) traverse exported anonymous
// embedded struct fields, so a detail enricher's wrapper RawStruct (e.g.
// awsclient.FunctionEnriched embedding lambdatypes.FunctionConfiguration)
// keeps its detail field list post-enrichment:
// ExtractValue(FunctionEnriched{...}, "FunctionName") resolves. Direct fields
// win over promoted ones, pointer embeds are deref'd, and json:"-" /
// non-struct anonymous fields are not promoted. This file pins that contract
// against every wrapper type plus PolicyEnriched, and a locally-defined
// multi-level embed for the depth/shadowing axes no real wrapper happens to
// exercise.

import (
	"reflect"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	sfntypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/fieldpath"
)

// embeddedTraversalCase pins one wrapper type: wrapper embeds raw, and path
// must resolve to the same value on both — proving the wrapper doesn't
// collapse the raw SDK struct's fields.
type embeddedTraversalCase struct {
	name    string
	wrapper any
	raw     any
	path    string
	want    string // expected formatted scalar (via fieldpath.ExtractScalar sanity check)
}

func embeddedTraversalCases() []embeddedTraversalCase {
	sfnItem := sfntypes.StateMachineListItem{Name: aws.String("order-processing")}
	stack := cfntypes.Stack{StackName: aws.String("prod-vpc-network")}
	fn := lambdatypes.FunctionConfiguration{FunctionName: aws.String("process-payment")}
	instance := ec2types.Instance{InstanceId: aws.String("i-0abc123def4567890")}
	topic := snstypes.Topic{TopicArn: aws.String("arn:aws:sns:us-east-1:123456789012:order-events")}
	bucket := s3types.Bucket{Name: aws.String("acme-app-logs-prod")}
	policy := iamtypes.Policy{PolicyName: aws.String("test-policy")}

	return []embeddedTraversalCase{
		{
			name:    "StateMachineEnriched",
			wrapper: awsclient.StateMachineEnriched{StateMachineListItem: sfnItem, Status: sfntypes.StateMachineStatusActive},
			raw:     sfnItem,
			path:    "Name",
			want:    "order-processing",
		},
		{
			name:    "StackEnriched",
			wrapper: awsclient.StackEnriched{Stack: stack, TemplateBody: map[string]any{"k": "v"}},
			raw:     stack,
			path:    "StackName",
			want:    "prod-vpc-network",
		},
		{
			name:    "FunctionEnriched",
			wrapper: awsclient.FunctionEnriched{FunctionConfiguration: fn},
			raw:     fn,
			path:    "FunctionName",
			want:    "process-payment",
		},
		{
			name:    "InstanceEnriched",
			wrapper: awsclient.InstanceEnriched{Instance: instance, UserData: "#!/bin/bash\necho hi\n"},
			raw:     instance,
			path:    "InstanceId",
			want:    "i-0abc123def4567890",
		},
		{
			name:    "TopicEnriched",
			wrapper: awsclient.TopicEnriched{Topic: topic, Attributes: map[string]any{"DisplayName": "Order Events"}},
			raw:     topic,
			path:    "TopicArn",
			want:    "arn:aws:sns:us-east-1:123456789012:order-events",
		},
		{
			name:    "BucketEnriched",
			wrapper: awsclient.BucketEnriched{Bucket: bucket, Policy: map[string]any{"Version": "2012-10-17"}},
			raw:     bucket,
			path:    "Name",
			want:    "acme-app-logs-prod",
		},
		{
			name:    "PolicyEnriched",
			wrapper: awsclient.PolicyEnriched{Policy: policy, Document: map[string]any{"Version": "2012-10-17"}},
			raw:     policy,
			path:    "PolicyName",
			want:    "test-policy",
		},
	}
}

// TestFieldpathEmbeddedTraversal_WrapperMatchesRaw is the table across all 7
// real detail-enrichment wrapper types: ExtractValue must resolve an
// embedded scalar path through the wrapper identically to the raw SDK
// struct it embeds.
func TestFieldpathEmbeddedTraversal_WrapperMatchesRaw(t *testing.T) {
	for _, tc := range embeddedTraversalCases() {
		t.Run(tc.name, func(t *testing.T) {
			gotRaw, errRaw := fieldpath.ExtractValue(tc.raw, tc.path)
			if errRaw != nil {
				t.Fatalf("sanity check failed: ExtractValue(raw, %q) error: %v — fixture path is wrong", tc.path, errRaw)
			}
			gotWrapper, errWrapper := fieldpath.ExtractValue(tc.wrapper, tc.path)
			if errWrapper != nil {
				t.Fatalf("ExtractValue(%s wrapper, %q) error: %v — wrapper must resolve embedded fields like the raw struct does", tc.name, tc.path, errWrapper)
			}
			if !reflect.DeepEqual(gotWrapper.Interface(), gotRaw.Interface()) {
				t.Errorf("%s: wrapper resolved %v, want identical to raw's %v", tc.name, gotWrapper.Interface(), gotRaw.Interface())
			}

			if got := fieldpath.ExtractScalar(tc.wrapper, tc.path); got != tc.want {
				t.Errorf("%s: ExtractScalar(wrapper, %q) = %q, want %q", tc.name, tc.path, got, tc.want)
			}
		})
	}
}

// TestFieldpathEmbeddedTraversal_ExtractFirstListScalar_InstanceEnriched
// verifies ExtractFirstListScalar resolves a list-valued embedded path
// (SecurityGroups.GroupId, transparently indexing element 0) through
// InstanceEnriched identically to the raw ec2types.Instance.
func TestFieldpathEmbeddedTraversal_ExtractFirstListScalar_InstanceEnriched(t *testing.T) {
	raw := ec2types.Instance{
		InstanceId: aws.String("i-0abc123def4567890"),
		SecurityGroups: []ec2types.GroupIdentifier{
			{GroupId: aws.String("sg-0abc123"), GroupName: aws.String("web-sg")},
			{GroupId: aws.String("sg-0def456"), GroupName: aws.String("db-sg")},
		},
	}
	wrapper := awsclient.InstanceEnriched{Instance: raw, UserData: "#!/bin/bash\necho hi\n"}

	want := fieldpath.ExtractFirstListScalar(raw, "SecurityGroups.GroupId")
	if want != "sg-0abc123" {
		t.Fatalf("sanity check failed: ExtractFirstListScalar(raw, SecurityGroups.GroupId) = %q, want %q — fixture is wrong", want, "sg-0abc123")
	}

	got := fieldpath.ExtractFirstListScalar(wrapper, "SecurityGroups.GroupId")
	if got != want {
		t.Errorf("ExtractFirstListScalar(InstanceEnriched, SecurityGroups.GroupId) = %q, want %q (identical to raw)", got, want)
	}
}

// embedInner/embedMiddle/embedOuter are locally-defined test-only types
// exercising two axes no real wrapper happens to cover: resolution through
// TWO levels of anonymous embedding, and a directly-declared field shadowing
// a promoted embedded field of the same name (mirrors Go's own field
// promotion rule and encoding/json's: shallower depth wins).
type embedInner struct {
	InnerScalar string
	Shared      string
}

type embedMiddle struct {
	embedInner
	MiddleScalar string
}

type embedOuter struct {
	embedMiddle
	Shared      string
	OuterScalar string
}

func TestFieldpathEmbeddedTraversal_MultiLevelEmbedAndShadowing(t *testing.T) {
	v := embedOuter{
		embedMiddle: embedMiddle{
			embedInner:   embedInner{InnerScalar: "deep-value", Shared: "inner-shared"},
			MiddleScalar: "mid-value",
		},
		Shared:      "outer-shared",
		OuterScalar: "outer-value",
	}

	if got := fieldpath.ExtractScalar(v, "InnerScalar"); got != "deep-value" {
		t.Errorf("ExtractScalar(v, InnerScalar) = %q, want %q (two levels deep: embedOuter -> embedMiddle -> embedInner)", got, "deep-value")
	}
	if got := fieldpath.ExtractScalar(v, "MiddleScalar"); got != "mid-value" {
		t.Errorf("ExtractScalar(v, MiddleScalar) = %q, want %q (one level deep: embedOuter -> embedMiddle)", got, "mid-value")
	}
	if got := fieldpath.ExtractScalar(v, "OuterScalar"); got != "outer-value" {
		t.Errorf("ExtractScalar(v, OuterScalar) = %q, want %q (direct field, no embedding)", got, "outer-value")
	}
	if got := fieldpath.ExtractScalar(v, "Shared"); got != "outer-shared" {
		t.Errorf("ExtractScalar(v, Shared) = %q, want the outer (shallower) field %q — a directly-declared "+
			"field must win over a promoted embedded field of the same name", got, "outer-shared")
	}
}

// TestFieldpathEmbeddedTraversal_UnknownPath_StillErrors verifies embedded
// traversal doesn't turn ExtractValue into an always-succeeds lookup: a
// path matching no field anywhere — own or embedded — must still error.
func TestFieldpathEmbeddedTraversal_UnknownPath_StillErrors(t *testing.T) {
	wrapper := awsclient.FunctionEnriched{
		FunctionConfiguration: lambdatypes.FunctionConfiguration{FunctionName: aws.String("process-payment")},
	}
	if _, err := fieldpath.ExtractValue(wrapper, "NoSuchFieldAnywhere"); err == nil {
		t.Fatal("expected an error for a path matching no field anywhere (own or embedded), got nil")
	}
}

// TestFieldpathExtractSubtree_BigIntInJSONStringField_RoundTripsLosslessly:
// tryParseJSON (extract.go) decodes with json.Decoder.UseNumber, so
// ExtractSubtree's JSON-string-field YAML rendering keeps an integer above
// 2^53 at full precision instead of rounding it the way a plain
// json.Unmarshal-into-any would (9007199254740993 -> 9007199254740992,
// tryParseJSON's own doc comment). No real wrapper type has a plain string
// field holding raw JSON, so a minimal local type stands in, mirroring
// embedInner/embedMiddle/embedOuter above.
func TestFieldpathExtractSubtree_BigIntInJSONStringField_RoundTripsLosslessly(t *testing.T) {
	const bigInt = "9007199254740993"
	type withJSONStringField struct {
		Document string
	}
	v := withJSONStringField{Document: `{"id":` + bigInt + `}`}

	got := fieldpath.ExtractSubtree(v, "Document")
	if !strings.Contains(got, bigInt) {
		t.Errorf("ExtractSubtree(Document) = %q, want it to contain the full-precision digits %q (a float64 would render \"9007199254740992\")", got, bigInt)
	}
}
