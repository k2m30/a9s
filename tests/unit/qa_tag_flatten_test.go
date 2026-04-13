package unit

// qa_tag_flatten_test.go verifies that AWS tag sections in the detail view
// render as flat "Key: Value" pairs instead of verbose "- Key: ... / Value: ..."
// struct output. Only detail views are affected — YAML/JSON/copy stay raw.

import (
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// tagDetailModel creates a sized DetailModel for the given resource and type.
func tagDetailModel(res resource.Resource, resType string) views.DetailModel {
	cfg := config.DefaultConfig()
	k := keys.Default()
	d := views.NewDetail(res, resType, cfg, k)
	d.SetSize(120, 40)
	return d
}

// ---------------------------------------------------------------------------
// Test 1: EC2 tags render as flat Key: Value pairs
// ---------------------------------------------------------------------------

func TestQA_TagFlatten_EC2_TagsRenderAsKeyValue(t *testing.T) {
	inst := ec2types.Instance{
		InstanceId:   aws.String("i-0tag1234567890abc"),
		InstanceType: ec2types.InstanceTypeT3Micro,
		State: &ec2types.InstanceState{
			Name: ec2types.InstanceStateNameRunning,
			Code: aws.Int32(16),
		},
		Tags: []ec2types.Tag{
			{Key: aws.String("Environment"), Value: aws.String("production")},
			{Key: aws.String("Team"), Value: aws.String("platform")},
		},
	}
	res := resource.Resource{ID: "i-0tag1234567890abc", Name: "tag-test", RawStruct: inst}
	m := tagDetailModel(res, "ec2")

	plain := stripANSI(m.View())

	if !strings.Contains(plain, "Environment") {
		t.Errorf("detail view should contain tag key 'Environment'; got:\n%s", plain)
	}
	if !strings.Contains(plain, "production") {
		t.Errorf("detail view should contain tag value 'production'; got:\n%s", plain)
	}
	// Verbose struct format must NOT appear.
	if strings.Contains(plain, "- Key: Environment") {
		t.Errorf("detail view must NOT render tags as '- Key: Environment'; got:\n%s", plain)
	}
	if strings.Contains(plain, "  Value: production") {
		t.Errorf("detail view must NOT render 'Value: production' sub-field; got:\n%s", plain)
	}
}

// ---------------------------------------------------------------------------
// Test 2: Tags preserve original slice order
// ---------------------------------------------------------------------------

func TestQA_TagFlatten_MultipleTags_PreservesOrder(t *testing.T) {
	inst := ec2types.Instance{
		InstanceId:   aws.String("i-0order123456789ab"),
		InstanceType: ec2types.InstanceTypeT3Small,
		State: &ec2types.InstanceState{
			Name: ec2types.InstanceStateNameRunning,
			Code: aws.Int32(16),
		},
		Tags: []ec2types.Tag{
			{Key: aws.String("Alpha"), Value: aws.String("first")},
			{Key: aws.String("Beta"), Value: aws.String("second")},
			{Key: aws.String("Gamma"), Value: aws.String("third")},
		},
	}
	res := resource.Resource{ID: "i-0order123456789ab", Name: "order-test", RawStruct: inst}
	m := tagDetailModel(res, "ec2")

	plain := stripANSI(m.View())

	alphaIdx := strings.Index(plain, "Alpha")
	betaIdx := strings.Index(plain, "Beta")
	gammaIdx := strings.Index(plain, "Gamma")

	if alphaIdx < 0 || betaIdx < 0 || gammaIdx < 0 {
		t.Fatalf("all tag keys must appear; Alpha=%d Beta=%d Gamma=%d\n%s", alphaIdx, betaIdx, gammaIdx, plain)
	}
	if !(alphaIdx < betaIdx && betaIdx < gammaIdx) {
		t.Errorf("tags must appear in original order; Alpha=%d Beta=%d Gamma=%d", alphaIdx, betaIdx, gammaIdx)
	}
}

// ---------------------------------------------------------------------------
// Test 3: Empty tags — no verbose sub-fields
// ---------------------------------------------------------------------------

func TestQA_TagFlatten_EmptyTags_NoSubFields(t *testing.T) {
	inst := ec2types.Instance{
		InstanceId:   aws.String("i-0notags123456789a"),
		InstanceType: ec2types.InstanceTypeT3Nano,
		State: &ec2types.InstanceState{
			Name: ec2types.InstanceStateNameRunning,
			Code: aws.Int32(16),
		},
		Tags: []ec2types.Tag{},
	}
	res := resource.Resource{ID: "i-0notags123456789a", Name: "notags", RawStruct: inst}
	m := tagDetailModel(res, "ec2")

	plain := stripANSI(m.View())

	if strings.Contains(plain, "- Key:") {
		t.Errorf("detail view must NOT render '- Key:' for empty tags; got:\n%s", plain)
	}
}

// ---------------------------------------------------------------------------
// Test 4: Tag with nil Value — no panic, key still appears
// ---------------------------------------------------------------------------

func TestQA_TagFlatten_NilValue_RendersEmpty(t *testing.T) {
	inst := ec2types.Instance{
		InstanceId:   aws.String("i-0nilval123456789a"),
		InstanceType: ec2types.InstanceTypeT3Nano,
		State: &ec2types.InstanceState{
			Name: ec2types.InstanceStateNameRunning,
			Code: aws.Int32(16),
		},
		Tags: []ec2types.Tag{
			{Key: aws.String("Orphan"), Value: nil},
		},
	}
	res := resource.Resource{ID: "i-0nilval123456789a", Name: "nil-val", RawStruct: inst}
	m := tagDetailModel(res, "ec2")

	// Must not panic.
	plain := stripANSI(m.View())

	if !strings.Contains(plain, "Orphan") {
		t.Errorf("detail view should show tag key 'Orphan' even with nil Value; got:\n%s", plain)
	}
	if strings.Contains(plain, "- Key: Orphan") {
		t.Errorf("detail view must NOT render '- Key: Orphan'; got:\n%s", plain)
	}
}

// ---------------------------------------------------------------------------
// Test 5: Duplicate keys — both rendered
// ---------------------------------------------------------------------------

func TestQA_TagFlatten_DuplicateKeys_BothRendered(t *testing.T) {
	inst := ec2types.Instance{
		InstanceId:   aws.String("i-0dupkey1234567890"),
		InstanceType: ec2types.InstanceTypeT3Micro,
		State: &ec2types.InstanceState{
			Name: ec2types.InstanceStateNameRunning,
			Code: aws.Int32(16),
		},
		Tags: []ec2types.Tag{
			{Key: aws.String("Repeated"), Value: aws.String("first-value")},
			{Key: aws.String("Repeated"), Value: aws.String("second-value")},
		},
	}
	res := resource.Resource{ID: "i-0dupkey1234567890", Name: "dupkey", RawStruct: inst}
	m := tagDetailModel(res, "ec2")

	plain := stripANSI(m.View())

	if !strings.Contains(plain, "first-value") {
		t.Errorf("detail view should contain 'first-value'; got:\n%s", plain)
	}
	if !strings.Contains(plain, "second-value") {
		t.Errorf("detail view should contain 'second-value'; got:\n%s", plain)
	}
}

// ---------------------------------------------------------------------------
// Test 6: Non-tag section (SecurityGroups) is NOT flattened
// ---------------------------------------------------------------------------

func TestQA_TagFlatten_NonTagSection_NotFlattened(t *testing.T) {
	inst := ec2types.Instance{
		InstanceId:   aws.String("i-0sgtest1234567890"),
		InstanceType: ec2types.InstanceTypeT3Micro,
		State: &ec2types.InstanceState{
			Name: ec2types.InstanceStateNameRunning,
			Code: aws.Int32(16),
		},
		SecurityGroups: []ec2types.GroupIdentifier{
			{GroupId: aws.String("sg-0abc11223344"), GroupName: aws.String("app-sg")},
		},
	}
	res := resource.Resource{ID: "i-0sgtest1234567890", Name: "sg-test", RawStruct: inst}
	m := tagDetailModel(res, "ec2")

	plain := stripANSI(m.View())

	if !strings.Contains(plain, "sg-0abc11223344") {
		t.Errorf("SecurityGroup GroupId should be visible; got:\n%s", plain)
	}
	if !strings.Contains(plain, "app-sg") {
		t.Errorf("SecurityGroup GroupName should be visible; got:\n%s", plain)
	}
}

// ---------------------------------------------------------------------------
// Test 7: RDS tags also flatten (multi-type coverage)
// ---------------------------------------------------------------------------

func TestQA_TagFlatten_RDS_TagsRenderAsKeyValue(t *testing.T) {
	dbi := rdstypes.DBInstance{
		DBInstanceIdentifier: aws.String("rds-tag-test-01"),
		DBInstanceClass:      aws.String("db.t3.micro"),
		Engine:               aws.String("mysql"),
		DBInstanceStatus:     aws.String("available"),
		TagList: []rdstypes.Tag{
			{Key: aws.String("Environment"), Value: aws.String("staging")},
			{Key: aws.String("Owner"), Value: aws.String("backend-team")},
		},
	}
	res := resource.Resource{ID: "rds-tag-test-01", Name: "rds-tag-test-01", RawStruct: dbi}
	m := tagDetailModel(res, "dbi")

	plain := stripANSI(m.View())

	if !strings.Contains(plain, "Environment") {
		t.Errorf("RDS detail should contain tag key 'Environment'; got:\n%s", plain)
	}
	if !strings.Contains(plain, "staging") {
		t.Errorf("RDS detail should contain tag value 'staging'; got:\n%s", plain)
	}
	if strings.Contains(plain, "- Key: Environment") {
		t.Errorf("RDS detail must NOT render '- Key: Environment'; got:\n%s", plain)
	}
}

// ---------------------------------------------------------------------------
// Test 8: ASG rich tags (PropagateAtLaunch) are NOT flattened
// ---------------------------------------------------------------------------

func TestQA_TagFlatten_ASG_RichTags_NotFlattened(t *testing.T) {
	asg := asgtypes.AutoScalingGroup{
		AutoScalingGroupName: aws.String("my-asg"),
		MinSize:              aws.Int32(1),
		MaxSize:              aws.Int32(10),
		DesiredCapacity:      aws.Int32(3),
		Tags: []asgtypes.TagDescription{
			{
				Key:               aws.String("Environment"),
				Value:             aws.String("production"),
				PropagateAtLaunch: aws.Bool(true),
				ResourceId:        aws.String("my-asg"),
				ResourceType:      aws.String("auto-scaling-group"),
			},
		},
	}
	res := resource.Resource{ID: "my-asg", Name: "my-asg", RawStruct: asg}
	m := tagDetailModel(res, "asg")

	plain := stripANSI(m.View())

	// PropagateAtLaunch must remain visible — NOT stripped by flattening.
	if !strings.Contains(plain, "PropagateAtLaunch") {
		t.Errorf("ASG tag metadata 'PropagateAtLaunch' should be preserved; got:\n%s", plain)
	}
}

// ---------------------------------------------------------------------------
// Test 9: YAML view stays raw (unflattened)
// ---------------------------------------------------------------------------

func TestQA_TagFlatten_YAMLView_StaysRaw(t *testing.T) {
	inst := ec2types.Instance{
		InstanceId:   aws.String("i-0yamlraw123456789"),
		InstanceType: ec2types.InstanceTypeT3Micro,
		State: &ec2types.InstanceState{
			Name: ec2types.InstanceStateNameRunning,
			Code: aws.Int32(16),
		},
		Tags: []ec2types.Tag{
			{Key: aws.String("Stage"), Value: aws.String("canary")},
		},
	}
	res := resource.Resource{ID: "i-0yamlraw123456789", Name: "yaml-raw", RawStruct: inst}
	m := tagDetailModel(res, "ec2")

	raw := m.RawYAML()

	if raw == "" {
		t.Fatal("RawYAML should return non-empty YAML")
	}
	if !strings.Contains(raw, "Stage") {
		t.Errorf("RawYAML should contain tag key 'Stage'; got:\n%s", raw)
	}
	if !strings.Contains(raw, "canary") {
		t.Errorf("RawYAML should contain tag value 'canary'; got:\n%s", raw)
	}
}
