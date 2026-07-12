package aws

import (
	"context"
	"fmt"
	"regexp"

	"github.com/aws/aws-sdk-go-v2/service/ec2"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// ec2InstanceIDPattern matches an EC2 instance ID token ("i-" plus 8 or 17
// hex digits) inside an InvalidInstanceID.NotFound error message, e.g.
// `The instance ID 'i-0a1b2c3d4e5f60001' does not exist` — AWS names the
// exact offending ID(s) in the message, never a partial Reservations list.
var ec2InstanceIDPattern = regexp.MustCompile(`i-[0-9a-fA-F]{8,17}`)

// FetchEC2InstancesByIDs fetches specific EC2 instances by instance ID,
// bypassing the paginated fetcher's full-account sweep. Used by the
// related-panel lazy-add path and by the Cost Explorer's RESOURCE_ID drill
// navigation (a cost row names an instance ID that may not be in the
// caller's already-loaded list). DescribeInstances accepts InstanceIds as a
// batched filter, so this is normally a single API call regardless of how
// many IDs were requested (up to the AWS per-request limit) — but AWS
// rejects the WHOLE batch call with InvalidInstanceID.NotFound when ANY
// requested ID is invalid or terminated-and-purged, never a partial
// Reservations list omitting just the bad one. On that specific error this
// parses the named offending ID(s) out of the message and retries once with
// them excluded, so a batch with one bad ID still recovers the live
// instances instead of losing the whole batch.
//
// The DescribeInstances call is wrapped in RetryOnThrottle. IDs not present
// in the final response (e.g. terminated-and-purged, or simply wrong) are
// collected into a composite error returned alongside the partial results —
// mirrors FetchAMIsByIDs/FetchEBSSnapshotsByIDs.
func FetchEC2InstancesByIDs(ctx context.Context, api EC2DescribeInstancesAPI, ids []string) ([]resource.Resource, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	requested := make([]string, 0, len(ids))
	for _, id := range ids {
		if id != "" {
			requested = append(requested, id)
		}
	}
	if len(requested) == 0 {
		return nil, nil
	}

	resources, unrecovered, err := fetchEC2InstancesByIDsOnce(ctx, api, requested)
	if err != nil {
		code, _, _ := ClassifyAWSError(err)
		if code != "InvalidInstanceID.NotFound" {
			return resources, fmt.Errorf("fetching EC2 instances by id: %w", err)
		}
		bad := ec2InstanceIDPattern.FindAllString(err.Error(), -1)
		retry := make([]string, 0, len(requested))
		badSet := make(map[string]struct{}, len(bad))
		for _, id := range bad {
			badSet[id] = struct{}{}
		}
		for _, id := range requested {
			if _, isBad := badSet[id]; !isBad {
				retry = append(retry, id)
			}
		}
		if len(bad) == 0 || len(retry) == len(requested) {
			// Couldn't identify (or exclude) any offending ID — nothing
			// left to retry with, propagate the original error as-is.
			return resources, fmt.Errorf("fetching EC2 instances by id: %w", err)
		}
		if len(retry) == 0 {
			// Every requested ID was named bad — retry would be called with
			// an empty InstanceIds list, which DescribeInstances treats as
			// an unfiltered, ACCOUNT-WIDE describe rather than "describe
			// nothing." Return the missing aggregate directly instead.
			return resources, AggregateMissing("ec2 FetchByIDs", bad, len(requested))
		}
		resources, unrecovered, err = fetchEC2InstancesByIDsOnce(ctx, api, retry)
		if err != nil {
			return resources, fmt.Errorf("fetching EC2 instances by id: %w", err)
		}
		unrecovered = append(unrecovered, bad...)
	}

	return resources, AggregateMissing("ec2 FetchByIDs", unrecovered, len(requested))
}

// fetchEC2InstancesByIDsOnce issues exactly one (retried-on-throttle)
// DescribeInstances call for ids and diffs requested-vs-returned. err is
// non-nil only for a batch-level failure (e.g. InvalidInstanceID.NotFound);
// unrecovered names ids present in the request but absent from a
// successful response.
func fetchEC2InstancesByIDsOnce(ctx context.Context, api EC2DescribeInstancesAPI, ids []string) (resources []resource.Resource, unrecovered []string, err error) {
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*ec2.DescribeInstancesOutput, error) {
		return api.DescribeInstances(ctx, &ec2.DescribeInstancesInput{InstanceIds: ids})
	})
	if err != nil {
		return nil, nil, err
	}

	returned := make(map[string]struct{}, len(ids))
	for _, reservation := range out.Reservations {
		for _, inst := range reservation.Instances {
			r := ec2InstanceToResource(inst)
			resources = append(resources, r)
			returned[r.ID] = struct{}{}
		}
	}
	for _, id := range ids {
		if _, found := returned[id]; !found {
			unrecovered = append(unrecovered, id)
		}
	}
	return resources, unrecovered, nil
}
