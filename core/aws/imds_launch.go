// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/core/domain"
)

// imdsLaunch is the instance metadata settings one launch spec states — a
// launch template version or a launch configuration — and the row its verdict
// lands on. An empty Endpoint or Tokens is a value the spec leaves unset.
type imdsLaunch struct {
	ID       string
	ImageID  string
	Endpoint string
	Tokens   string
}

// raiseIMDSv1 is the one place a9s decides whether instances a launch spec
// starts answer IMDSv1, and raises code on every row whose spec lets them.
// Each setting follows the order of precedence the EC2 User Guide states
// ("Order of precedence for instance metadata options"): the value the spec
// sets, then the account-level default for the Region
// (GetInstanceMetadataDefaults), then, for HttpTokens alone, the AMI's
// ImdsSupport, where v2.0 means required. A disabled metadata endpoint is no
// exposure whatever the tokens say. An account that enforces IMDSv2
// (HttpTokensEnforced) fails every launch that resolves to optional, so no
// instance it starts answers IMDSv1.
//
// The account defaults are one read, made only when a spec leaves a setting
// they could decide; a failed read leaves every such row not inspected. Every
// AMI a spec leaving HttpTokens unset names is read in one DescribeImages
// walk. A row whose AMI that walk does not return, or whose image is chosen at
// launch (a resolve:ssm: reference, or none), is not inspected. With no EC2
// client every such row is not inspected; a client without
// GetInstanceMetadataDefaults reads the account as stating no preference.
// rows builds the finding's supporting rows.
func raiseIMDSv1(ctx context.Context, api EC2DescribeImagesAPI, launches []imdsLaunch, result *IssueEnricherResult, code domain.FindingCode, rows func(imdsLaunch) []domain.DetailRow) error {
	disabled := string(ec2types.InstanceMetadataEndpointStateDisabled)
	var open []imdsLaunch
	for _, l := range launches {
		if l.Endpoint != disabled && l.Tokens != string(ec2types.HttpTokensStateRequired) {
			open = append(open, l)
		}
	}
	if len(open) == 0 {
		return nil
	}
	if api == nil {
		for _, l := range open {
			markUninspected(result, l.ID, imdsCheckNoClient)
		}
		return nil
	}
	account := readIMDSAccount(ctx, api)
	var amis []string
	for _, l := range open {
		if l.Tokens == "" && strings.HasPrefix(l.ImageID, "ami-") && !slices.Contains(amis, l.ImageID) {
			amis = append(amis, l.ImageID)
		}
	}
	slices.Sort(amis)
	imds, unreadErr := imdsAMIDefaults(ctx, api, amis)

	var accountFailures, amiFailures []Failure
	for _, l := range open {
		support, read := imds[l.ImageID]
		switch {
		case account.err != nil:
			MarkSkipped(result, l.ID, &accountFailures, account.err)
		case account.defaults.HttpTokensEnforced == ec2types.HttpTokensEnforcedStateEnabled:
		case cmp.Or(l.Endpoint, string(account.defaults.HttpEndpoint)) == disabled:
		case l.Tokens != "", account.defaults.HttpTokens == ec2types.HttpTokensStateOptional:
			setWave2Finding(result, l.ID, code, rows(l))
		case account.defaults.HttpTokens == ec2types.HttpTokensStateRequired:
		case read && support == ec2types.ImdsSupportValuesV20:
		case read:
			setWave2Finding(result, l.ID, code, rows(l))
		case !strings.HasPrefix(l.ImageID, "ami-"):
			markUninspected(result, l.ID, imdsCheckAMIAtLaunch)
		case unreadErr[l.ImageID] != nil:
			MarkSkipped(result, l.ID, &amiFailures, unreadErr[l.ImageID])
		default:
			markUninspected(result, l.ID, imdsCheckAMINotFound)
		}
	}
	return errors.Join(AggregateFailures("GetInstanceMetadataDefaults", accountFailures, len(open)), AggregateFailures("DescribeImages", amiFailures, len(open)))
}

// imdsAccount is the Region's account-level instance metadata defaults, or
// the error reading them returned. An empty value is no preference.
type imdsAccount struct {
	defaults ec2types.InstanceMetadataDefaultsResponse
	err      error
}

// readIMDSAccount reads the Region's account-level instance metadata
// defaults. raiseIMDSv1 reads them only when a launch spec leaves a setting
// they could decide. A client without the call reads no preference.
func readIMDSAccount(ctx context.Context, api EC2DescribeImagesAPI) imdsAccount {
	defaultsAPI, ok := api.(EC2GetInstanceMetadataDefaultsAPI)
	if !ok {
		return imdsAccount{}
	}
	out, err := defaultsAPI.GetInstanceMetadataDefaults(ctx, &ec2.GetInstanceMetadataDefaultsInput{})
	if err != nil {
		return imdsAccount{err: err}
	}
	if out.AccountLevel == nil {
		// no finding: an answer without account-level settings states no
		// preference, which is what an empty value says.
		return imdsAccount{}
	}
	return imdsAccount{defaults: *out.AccountLevel}
}

// imdsAMIDefaults reads the ImdsSupport of every AMI in ids, sorted, in one
// DescribeImages walk per imdsImageFilterValues ids. An AMI a refused or
// capped request asked about is in unread with that request's error.
func imdsAMIDefaults(ctx context.Context, api EC2DescribeImagesAPI, ids []string) (imds map[string]ec2types.ImdsSupportValues, unread map[string]error) {
	imds = make(map[string]ec2types.ImdsSupportValues, len(ids))
	unread = map[string]error{}
	for chunk := range slices.Chunk(ids, imdsImageFilterValues) {
		// The image-id filter answers an id it cannot find with nothing,
		// where ImageIds fails the whole call on it.
		images, complete, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]ec2types.Image, *string, error) {
			out, err := api.DescribeImages(ctx, &ec2.DescribeImagesInput{
				Filters:           []ec2types.Filter{{Name: aws.String("image-id"), Values: chunk}},
				IncludeDeprecated: aws.Bool(true),
				IncludeDisabled:   aws.Bool(true),
				NextToken:         token,
			})
			if err != nil {
				return nil, nil, err
			}
			return out.Images, out.NextToken, nil
		})
		if err == nil && !complete {
			err = errPageCap
		}
		if err != nil {
			for _, id := range chunk {
				unread[id] = err
			}
			continue
		}
		for _, img := range images {
			imds[aws.ToString(img.ImageId)] = img.ImdsSupport
		}
	}
	return imds, unread
}

// imdsImageFilterValues is the most values one DescribeImages request is
// sent: EC2 accepts "up to 200 total filter values in a single request"
// (EC2 User Guide, "List and filter using the command line").
const imdsImageFilterValues = 200

// The checks a row records when a launch spec's account- or AMI-level default
// could not be read.
const (
	imdsCheckNoClient    = "no EC2 client"
	imdsCheckAMINotFound = "AMI not returned by DescribeImages"
	imdsCheckAMIAtLaunch = "AMI chosen at launch"
)
