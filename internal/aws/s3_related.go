// s3_related.go contains S3 bucket related-resource checker functions.
package aws

import (
	"context"
	"strings"

	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkS3Trail searches the trail cache for trails whose S3BucketName matches
// this bucket name. S3 is a reverse-lookup hub — trails reference S3 buckets,
// not the other way around.
func checkS3Trail(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	bucketName := res.ID
	if bucketName == "" {
		return resource.RelatedCheckResult{TargetType: "trail", Count: 0}
	}

	trailList, truncated, err := s3RelatedResources(ctx, clients, cache, "trail")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "trail", Count: -1, Err: err}
	}
	if trailList == nil {
		return resource.RelatedCheckResult{TargetType: "trail", Count: -1}
	}

	var ids []string
	for _, trailRes := range trailList {
		trail, ok := assertStruct[cloudtrailtypes.Trail](trailRes.RawStruct)
		if !ok {
			continue
		}
		if trail.S3BucketName == nil || *trail.S3BucketName == "" {
			continue
		}
		if *trail.S3BucketName == bucketName {
			ids = append(ids, trailRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "trail", Count: -1}
	}
	return relatedResult("trail", ids)
}

// checkS3CF searches the CloudFront cache for distributions with origins that
// reference this S3 bucket. Origin DomainName formats:
//   - {bucket}.s3.amazonaws.com
//   - {bucket}.s3-website.{region}.amazonaws.com
//   - {bucket}.s3.{region}.amazonaws.com
func checkS3CF(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	bucketName := res.ID
	if bucketName == "" {
		return resource.RelatedCheckResult{TargetType: "cf", Count: 0}
	}

	cfList, truncated, err := s3RelatedResources(ctx, clients, cache, "cf")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "cf", Count: -1, Err: err}
	}
	if cfList == nil {
		return resource.RelatedCheckResult{TargetType: "cf", Count: -1}
	}

	var ids []string
	for _, cfRes := range cfList {
		dist, ok := assertStruct[cftypes.DistributionSummary](cfRes.RawStruct)
		if !ok {
			continue
		}
		if dist.Origins == nil {
			continue
		}
		for _, origin := range dist.Origins.Items {
			if origin.DomainName == nil {
				continue
			}
			if strings.Contains(*origin.DomainName, bucketName+".s3") {
				ids = append(ids, cfRes.ID)
				break
			}
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "cf", Count: -1}
	}
	return relatedResult("cf", ids)
}

// s3RelatedResources returns the resource list for target from cache or by
// fetching the first page via the registered paginated fetcher.
func s3RelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}

func init() {
	resource.RegisterRelated("s3", []resource.RelatedDef{
		{TargetType: "trail", DisplayName: "CloudTrail Trails", Checker: checkS3Trail, NeedsTargetCache: true},
		{TargetType: "cf", DisplayName: "CloudFront", Checker: checkS3CF, NeedsTargetCache: true},
		{TargetType: "lambda", DisplayName: "Lambda (notifications)", Checker: nil, NeedsTargetCache: false},
		{TargetType: "cfn", DisplayName: "CloudFormation", Checker: nil, NeedsTargetCache: true},
	})
}
