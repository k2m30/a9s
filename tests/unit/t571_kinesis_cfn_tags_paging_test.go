package unit_test

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	kinesistypes "github.com/aws/aws-sdk-go-v2/service/kinesis/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// t571KinesisTagFake pages ListTagsForStream the way Kinesis does: tags in key
// order, at most pageSize per call, HasMoreTags when more remain, and the next
// call resumes after ExclusiveStartTagKey. failAfterFirst makes every call
// past the first page fail.
type t571KinesisTagFake struct {
	awsclient.KinesisAPI

	tags           []kinesistypes.Tag
	pageSize       int
	failAfterFirst bool
}

func (f *t571KinesisTagFake) ListTagsForStream(_ context.Context, in *kinesis.ListTagsForStreamInput, _ ...func(*kinesis.Options)) (*kinesis.ListTagsForStreamOutput, error) {
	start := 0
	if k := aws.ToString(in.ExclusiveStartTagKey); k != "" {
		if f.failAfterFirst {
			return nil, errors.New("AccessDeniedException: kinesis:ListTagsForStream denied")
		}
		start = len(f.tags)
		for i, tag := range f.tags {
			if aws.ToString(tag.Key) == k {
				start = i + 1
				break
			}
		}
	}
	end := min(start+f.pageSize, len(f.tags))
	return &kinesis.ListTagsForStreamOutput{
		Tags:        f.tags[start:end],
		HasMoreTags: aws.Bool(end < len(f.tags)),
	}, nil
}

func t571StreamTags(n int, stackAt int) []kinesistypes.Tag {
	tags := make([]kinesistypes.Tag, 0, n)
	for i := range n {
		if i == stackAt {
			tags = append(tags, kinesistypes.Tag{Key: aws.String("aws:cloudformation:stack-name"), Value: aws.String("clickstream-pipeline")})
			continue
		}
		tags = append(tags, kinesistypes.Tag{Key: aws.String("team-" + strconv.Itoa(100+i)), Value: aws.String("data")})
	}
	return tags
}

func t571KinesisCFN(t *testing.T, fake *t571KinesisTagFake) resource.RelatedCheckResult {
	t.Helper()
	cache := resource.ResourceCache{"cfn": resource.ResourceCacheEntry{Resources: []resource.Resource{
		{ID: "clickstream-pipeline", Name: "clickstream-pipeline", Fields: map[string]string{"stack_name": "clickstream-pipeline"}},
		{ID: "unrelated-stack", Name: "unrelated-stack", Fields: map[string]string{"stack_name": "unrelated-stack"}},
	}}}
	stream := resource.Resource{ID: "clickstream-ingest", Name: "clickstream-ingest", Fields: map[string]string{
		"stream_arn": "arn:aws:kinesis:us-east-1:123456789012:stream/clickstream-ingest",
	}}
	return checkerByTarget(t, "kinesis", "cfn")(context.Background(), &awsclient.ServiceClients{Kinesis: fake}, stream, cache)
}

// A stream's aws:cloudformation:stack-name tag may sit on any
// ListTagsForStream page; the pivot follows HasMoreTags until it finds it.
func TestT571_KinesisCFN_StackTagOnALaterPageIsFound(t *testing.T) {
	got := t571KinesisCFN(t, &t571KinesisTagFake{tags: t571StreamTags(9, 7), pageSize: 3})
	if ids := got.ResourceIDs(); len(ids) != 1 || ids[0] != "clickstream-pipeline" {
		t.Fatalf("ResourceIDs = %v (state %v, truncated %v), want [clickstream-pipeline] from the third tag page",
			ids, got.State(), got.Truncated())
	}
	if got.Truncated() {
		t.Error("Truncated = true, want false — every tag page was read")
	}
}

// Every tag read and none names a stack: an exact zero.
func TestT571_KinesisCFN_AllTagsReadWithoutStackIsAnExactZero(t *testing.T) {
	got := t571KinesisCFN(t, &t571KinesisTagFake{tags: t571StreamTags(9, -1), pageSize: 3})
	if got.State() != domain.RelatedResolved || got.Count() != 0 || got.Truncated() {
		t.Errorf("state=%v count=%d truncated=%v, want resolved exact 0", got.State(), got.Count(), got.Truncated())
	}
}

// A tag page that failed may have carried the stack tag, so the pivot cannot
// claim the stream belongs to no stack.
func TestT571_KinesisCFN_LaterPageFailedIsNotAProvenZero(t *testing.T) {
	got := t571KinesisCFN(t, &t571KinesisTagFake{tags: t571StreamTags(9, -1), pageSize: 3, failAfterFirst: true})
	if got.State() == domain.RelatedResolved && !got.Truncated() {
		t.Errorf("state=%v count=%d truncated=false: a stream whose tags were not all read reads as a proven zero",
			got.State(), got.Count())
	}
}

// A stream with more tag pages than PerParentPageCap was read in part.
func TestT571_KinesisCFN_PastPageCapIsNotAProvenZero(t *testing.T) {
	n := awsclient.PerParentPageCap + 3
	got := t571KinesisCFN(t, &t571KinesisTagFake{tags: t571StreamTags(n, n-1), pageSize: 1})
	if got.State() == domain.RelatedResolved && !got.Truncated() {
		t.Errorf("state=%v count=%d truncated=false: tags past the page cap were never read", got.State(), got.Count())
	}
}
