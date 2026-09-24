// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"
	"regexp"
	"slices"

	"github.com/aws/aws-sdk-go-v2/service/ec2"

	"github.com/k2m30/a9s/v3/core/resource"
)

// FetchEC2InstancesByIDs fetches specific EC2 instances by instance ID,
// bypassing the paginated fetcher's full-account sweep. Used by the
// related-panel lazy-add path and by the Cost Explorer's RESOURCE_ID drill
// navigation (a cost row names an instance ID that may not be in the
// caller's already-loaded list). The IDs go in batches of ec2MaxIDsPerCall;
// missing IDs are handled by fetchEC2ByIDs.
func FetchEC2InstancesByIDs(ctx context.Context, api EC2DescribeInstancesAPI, ids []string) ([]resource.Resource, error) {
	return fetchEC2ByIDs(ctx, "ec2 FetchByIDs", "InvalidInstanceID.NotFound", "i-", ids, func(ids []string) ([]resource.Resource, error) {
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*ec2.DescribeInstancesOutput, error) {
			return api.DescribeInstances(ctx, &ec2.DescribeInstancesInput{InstanceIds: ids})
		})
		if err != nil {
			return nil, err
		}
		var resources []resource.Resource
		for _, reservation := range out.Reservations {
			for _, inst := range reservation.Instances {
				resources = append(resources, ec2InstanceToResource(inst))
			}
		}
		return resources, nil
	})
}

// fetchEC2ByIDs runs describe over the non-empty ids. An EC2 Describe* call
// given IDs fails whole with notFoundCode when any of them does not exist
// ("The specified … does not exist",
// docs.aws.amazon.com/AWSEC2/latest/APIReference/errors-overview.html), naming
// the missing ID(s) in its message, and returns none of the others. The IDs
// the message names, by their prefix, are dropped and the rest asked for
// again until a call succeeds; every dropped or unreturned ID is reported in
// a composite error beside the rows that were found. An empty ID list is
// never sent, since EC2 reads it as "every resource".
func fetchEC2ByIDs(ctx context.Context, op, notFoundCode, idPrefix string, ids []string, describe func([]string) ([]resource.Resource, error)) ([]resource.Resource, error) {
	requested := without(ids, []string{""})
	if len(requested) == 0 {
		return nil, nil
	}
	idPattern := regexp.MustCompile(regexp.QuoteMeta(idPrefix) + `[0-9a-zA-Z]+`)
	var rows []resource.Resource
	var missing []string
	for batch := range slices.Chunk(requested, ec2MaxIDsPerCall) {
		found, lost, err := describeEC2Batch(batch, notFoundCode, idPattern, describe)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", op, err)
		}
		rows, missing = append(rows, found...), append(missing, lost...)
	}
	return rows, AggregateMissing(op, missing, len(requested))
}

// ec2MaxIDsPerCall is the most IDs one EC2 describe call is sent: "you should
// not specify more than 1,000 IDs in a single call"
// (https://docs.aws.amazon.com/ec2/latest/devguide/ec2-api-pagination.html).
const ec2MaxIDsPerCall = 1000

// describeEC2Batch describes one batch of IDs, dropping the ones a
// notFoundCode failure names until a call succeeds: the rows found and the
// IDs that were not.
func describeEC2Batch(batch []string, notFoundCode string, idPattern *regexp.Regexp, describe func([]string) ([]resource.Resource, error)) (rows []resource.Resource, missing []string, _ error) {
	remaining := batch
	for len(remaining) > 0 {
		found, err := describe(remaining)
		if err == nil {
			for _, id := range remaining {
				if !slices.ContainsFunc(found, func(r resource.Resource) bool { return r.ID == id }) {
					missing = append(missing, id)
				}
			}
			return found, missing, nil
		}
		var bad []string
		for _, id := range idPattern.FindAllString(err.Error(), -1) {
			if slices.Contains(remaining, id) && !slices.Contains(bad, id) {
				bad = append(bad, id)
			}
		}
		if !ErrCodeIs(err, notFoundCode) || len(bad) == 0 {
			return nil, nil, err
		}
		missing = append(missing, bad...)
		remaining = without(remaining, bad)
	}
	return nil, missing, nil
}

// without returns the ids not in drop, in order.
func without(ids, drop []string) []string {
	var kept []string
	for _, id := range ids {
		if !slices.Contains(drop, id) {
			kept = append(kept, id)
		}
	}
	return kept
}
