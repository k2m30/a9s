package main

import (
	"context"
	"errors"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	smithy "github.com/aws/smithy-go"
)

// s3Data is the normalized, raw view of S3 buckets the checklist generator reads. It records
// AWS facts only (never a9s's classification): the checklist generator applies the
// docs/resources/s3.md rules to derive expected colors/glyphs/status.
type s3Data struct {
	Buckets []s3Bucket `json:"buckets"`
}

type s3Bucket struct {
	Name         string `json:"name"`
	Region       string `json:"region,omitempty"`
	CreationDate string `json:"creation_date,omitempty"`
	PAB          s3PAB  `json:"pab"`
}

// s3PAB captures the GetPublicAccessBlock outcome per bucket.
//
//	Outcome "configured": the four flags are meaningful.
//	Outcome "none":       no PAB config (NoSuchPublicAccessBlockConfiguration or nil).
//	Outcome "error":      call failed (e.g. cross-region PermanentRedirect).
type s3PAB struct {
	Outcome               string `json:"outcome"`
	ErrorCode             string `json:"error_code,omitempty"`
	BlockPublicAcls       bool   `json:"block_public_acls"`
	IgnorePublicAcls      bool   `json:"ignore_public_acls"`
	BlockPublicPolicy     bool   `json:"block_public_policy"`
	RestrictPublicBuckets bool   `json:"restrict_public_buckets"`
}

// captureS3 lists every bucket (ListBuckets, all pages) and captures the
// GetPublicAccessBlock outcome for each. All buckets are captured — display
// caps (page size, enrichment cap) are screen behavior and belong to the checklist generator.
func captureS3(ctx context.Context, cfg aws.Config) (any, error) {
	client := s3.NewFromConfig(cfg)

	var buckets []s3Bucket
	pager := s3.NewListBucketsPaginator(client, &s3.ListBucketsInput{})
	for pager.HasMorePages() {
		out, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, b := range out.Buckets {
			buckets = append(buckets, s3Bucket{
				Name:         aws.ToString(b.Name),
				Region:       aws.ToString(b.BucketRegion),
				CreationDate: snapFormatTime(b.CreationDate),
			})
		}
	}

	for i := range buckets {
		buckets[i].PAB = capturePAB(ctx, client, buckets[i].Name)
	}

	return s3Data{Buckets: buckets}, nil
}

func capturePAB(ctx context.Context, client *s3.Client, bucket string) s3PAB {
	out, err := client.GetPublicAccessBlock(ctx, &s3.GetPublicAccessBlockInput{Bucket: aws.String(bucket)})
	if err != nil {
		if apiErr, ok := errors.AsType[smithy.APIError](err); ok {
			if apiErr.ErrorCode() == "NoSuchPublicAccessBlockConfiguration" {
				return s3PAB{Outcome: "none"}
			}
			return s3PAB{Outcome: "error", ErrorCode: apiErr.ErrorCode()}
		}
		return s3PAB{Outcome: "error", ErrorCode: err.Error()}
	}
	cfg := out.PublicAccessBlockConfiguration
	if cfg == nil {
		return s3PAB{Outcome: "none"}
	}
	return s3PAB{
		Outcome:               "configured",
		BlockPublicAcls:       aws.ToBool(cfg.BlockPublicAcls),
		IgnorePublicAcls:      aws.ToBool(cfg.IgnorePublicAcls),
		BlockPublicPolicy:     aws.ToBool(cfg.BlockPublicPolicy),
		RestrictPublicBuckets: aws.ToBool(cfg.RestrictPublicBuckets),
	}
}

// snapFormatTime is the single formatTime implementation shared by every
// capture file in this package (each AWS SDK service response uses the same
// *time.Time -> "2006-01-02 15:04" convention).
func snapFormatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02 15:04")
}
