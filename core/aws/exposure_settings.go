// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// exposure_settings.go — the account and Region settings AWS applies on top
// of a resource's own configuration when it decides whether the resource is
// reachable or encrypted.
package aws

import (
	"context"
	"errors"
	"slices"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/s3control"
	s3controltypes "github.com/aws/aws-sdk-go-v2/service/s3control/types"
)

type S3ControlAPI interface {
	GetPublicAccessBlock(ctx context.Context, params *s3control.GetPublicAccessBlockInput, optFns ...func(*s3control.Options)) (*s3control.GetPublicAccessBlockOutput, error)
}

type EC2GetSnapshotBlockPublicAccessStateAPI interface {
	GetSnapshotBlockPublicAccessState(ctx context.Context, params *ec2.GetSnapshotBlockPublicAccessStateInput, optFns ...func(*ec2.Options)) (*ec2.GetSnapshotBlockPublicAccessStateOutput, error)
}

type EC2DescribeVpcBlockPublicAccessOptionsAPI interface {
	DescribeVpcBlockPublicAccessOptions(ctx context.Context, params *ec2.DescribeVpcBlockPublicAccessOptionsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcBlockPublicAccessOptionsOutput, error)
}

type EC2DescribeVpcBlockPublicAccessExclusionsAPI interface {
	DescribeVpcBlockPublicAccessExclusions(ctx context.Context, params *ec2.DescribeVpcBlockPublicAccessExclusionsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcBlockPublicAccessExclusionsOutput, error)
}

// EC2VPCBlockPublicAccessAPI is both reads: the exclusions decide only under a
// mode that blocks, so a client without the pair reads the AWS default, off.
type EC2VPCBlockPublicAccessAPI interface {
	EC2DescribeVpcBlockPublicAccessOptionsAPI
	EC2DescribeVpcBlockPublicAccessExclusionsAPI
}

// ExposureSettingsStore holds each account and Region setting read for the
// session, read the first time a row needs it. A failed read is kept beside
// the successful ones, so every row that needs the setting in the session is
// marked with the same cause and the call is not repeated per row; a read the
// context cut off is not kept. The session owns one (core/session/stores.go),
// replaced on rotate and on a refresh of a type that reads it.
type ExposureSettingsStore struct {
	mu    sync.Mutex
	reads map[string]*settingRead
}

type settingRead struct {
	mu     sync.Mutex
	done   bool
	answer any
}

type settingAnswer[T any] struct {
	val T
	err error
}

func NewExposureSettingsStore() *ExposureSettingsStore {
	return &ExposureSettingsStore{reads: map[string]*settingRead{}}
}

// readSetting returns key's value, calling read once per store; concurrent
// callers of one key wait for the first.
func readSetting[T any](ctx context.Context, c *ServiceClients, key string, read func(context.Context) (T, error)) (T, error) {
	s := c.ExposureSettings()
	s.mu.Lock()
	r, ok := s.reads[key]
	if !ok {
		r = &settingRead{}
		s.reads[key] = r
	}
	s.mu.Unlock()

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.done {
		a := r.answer.(settingAnswer[T])
		return a.val, a.err
	}
	v, err := read(ctx)
	if err != nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
		return v, err
	}
	r.done, r.answer = true, settingAnswer[T]{v, err}
	return v, err
}

// errOwnAccountUnknown is what a read keyed by the caller's account returns
// when STS has not named the account.
var errOwnAccountUnknown = errors.New(CheckOwnAccountUnknown)

// s3AccountBlock is the account-level S3 public access block. "Amazon S3
// enforces the most restrictive combination of account-level and bucket-level
// settings", so each flag of a bucket's effective block is the bucket's OR the
// account's. An account that never set one answers
// NoSuchPublicAccessBlockConfiguration, which is the four flags off and is
// returned as nil.
func s3AccountBlock(ctx context.Context, c *ServiceClients) (*s3controltypes.PublicAccessBlockConfiguration, error) {
	account := accountIDFromClients(ctx, c, c.IdentityStore())
	if account == "" {
		return nil, errOwnAccountUnknown
	}
	return readSetting(ctx, c, "s3-account-block/"+account, func(ctx context.Context) (*s3controltypes.PublicAccessBlockConfiguration, error) {
		if c.S3Control == nil {
			return nil, errClientMissing
		}
		out, err := c.S3Control.GetPublicAccessBlock(ctx, &s3control.GetPublicAccessBlockInput{AccountId: aws.String(account)})
		switch {
		case ErrCodeIs(err, "NoSuchPublicAccessBlockConfiguration"):
			return nil, nil
		case err != nil:
			return nil, err
		case out == nil:
			return nil, UnusableAnswerErr{Call: "GetPublicAccessBlock", Field: "public access block configuration"}
		}
		return out.PublicAccessBlockConfiguration, nil
	})
}

// snapshotsBlockAllSharing reports whether the Region's block public access
// for snapshots is block-all-sharing, under which AWS treats a snapshot shared
// with the "all" group as private while its attributes still name the group
// (AWS::EC2::SnapshotBlockPublicAccess). block-new-sharing refuses new public
// shares only. A client without the call reads the AWS default,
// unblocked.
func snapshotsBlockAllSharing(ctx context.Context, c *ServiceClients) (bool, error) {
	return readSetting(ctx, c, "snapshot-block/"+c.Region, func(ctx context.Context) (bool, error) {
		api, ok := c.EC2.(EC2GetSnapshotBlockPublicAccessStateAPI)
		if !ok {
			return false, nil
		}
		out, err := api.GetSnapshotBlockPublicAccessState(ctx, &ec2.GetSnapshotBlockPublicAccessStateInput{})
		if err != nil {
			return false, err
		}
		return out != nil && out.State == ec2types.SnapshotBlockPublicAccessStateBlockAllSharing, nil
	})
}

// vpcBlock is the Region's VPC block public access: the mode, and the
// exclusions that are or may still be in force.
type vpcBlock struct {
	mode       ec2types.InternetGatewayBlockMode
	exclusions []ec2types.VpcBlockPublicAccessExclusion
}

// vpcIngressAllowed reports whether VPC block public access lets internet
// traffic in to subnet of vpc. block-ingress: "Block all internet traffic to
// the VPCs in this Region (except for VPCs or subnets which are excluded)";
// block-bidirectional blocks traffic to and from internet gateways with the
// same exception. Only an allow-bidirectional exclusion admits inbound
// traffic: allow-egress "Block[s] inbound internet traffic to the excluded VPCs
// and subnets". The exclusions are read only when the mode blocks. A client
// without the calls reads the AWS default, off.
func vpcIngressAllowed(ctx context.Context, c *ServiceClients, vpc, subnet string) (bool, error) {
	b, err := readSetting(ctx, c, "vpc-block/"+c.Region, func(ctx context.Context) (vpcBlock, error) {
		api, ok := c.EC2.(EC2VPCBlockPublicAccessAPI)
		if !ok {
			return vpcBlock{mode: ec2types.InternetGatewayBlockModeOff}, nil
		}
		opts, err := api.DescribeVpcBlockPublicAccessOptions(ctx, &ec2.DescribeVpcBlockPublicAccessOptionsInput{})
		if err != nil {
			return vpcBlock{}, err
		}
		b := vpcBlock{mode: ec2types.InternetGatewayBlockModeOff}
		if opts != nil && opts.VpcBlockPublicAccessOptions != nil {
			b.mode = opts.VpcBlockPublicAccessOptions.InternetGatewayBlockMode
		}
		if b.mode == ec2types.InternetGatewayBlockModeOff || b.mode == "" {
			return b, nil
		}
		excl, complete, exclErr := PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]ec2types.VpcBlockPublicAccessExclusion, *string, error) {
			out, err := api.DescribeVpcBlockPublicAccessExclusions(ctx, &ec2.DescribeVpcBlockPublicAccessExclusionsInput{NextToken: token})
			if err != nil {
				return nil, nil, err
			}
			return out.VpcBlockPublicAccessExclusions, out.NextToken, nil
		})
		if exclErr == nil && !complete {
			exclErr = errPageCap
		}
		b.exclusions = excl
		return b, exclErr
	})
	if err != nil {
		return false, err
	}
	if b.mode == ec2types.InternetGatewayBlockModeOff || b.mode == "" {
		return true, nil
	}
	return slices.ContainsFunc(b.exclusions, func(e ec2types.VpcBlockPublicAccessExclusion) bool {
		if e.InternetGatewayExclusionMode != ec2types.InternetGatewayExclusionModeAllowBidirectional {
			return false
		}
		switch e.State {
		case ec2types.VpcBlockPublicAccessExclusionStateCreateFailed,
			ec2types.VpcBlockPublicAccessExclusionStateDeleteComplete,
			ec2types.VpcBlockPublicAccessExclusionStateDisableComplete:
			return false
		}
		a, ok := ARNForService(aws.ToString(e.ResourceArn), "ec2")
		return ok && (a.Resource == "vpc/"+vpc || a.Resource == "subnet/"+subnet)
	}), nil
}

// ebsEncryptionByDefault reports whether the Region encrypts every new EBS
// volume: "If you enable it for a Region, you cannot disable it for
// individual volumes or snapshots in that Region" (Amazon EBS User Guide,
// "Enable Amazon EBS encryption by default"). A client without the call reads
// the AWS default, off.
func ebsEncryptionByDefault(ctx context.Context, c *ServiceClients) (bool, error) {
	return readSetting(ctx, c, "ebs-encryption-default/"+c.Region, func(ctx context.Context) (bool, error) {
		api, ok := c.EC2.(EC2GetEbsEncryptionByDefaultAPI)
		if !ok {
			return false, nil
		}
		out, err := api.GetEbsEncryptionByDefault(ctx, &ec2.GetEbsEncryptionByDefaultInput{})
		if err != nil {
			return false, err
		}
		return out != nil && aws.ToBool(out.EbsEncryptionByDefault), nil
	})
}
