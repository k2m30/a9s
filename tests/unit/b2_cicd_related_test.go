package unit_test

import (
	"context"
	"slices"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cptypes "github.com/aws/aws-sdk-go-v2/service/codepipeline/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// An S3 source action names the bucket it polls under S3Bucket, where a deploy
// action names its target under BucketName.
func TestRelated_Pipeline_S3_SourceAction(t *testing.T) {
	const pipelineName = "s3-source-pipeline"
	const sourceBucket = "acme-release-drops"
	decl := &cptypes.PipelineDeclaration{
		Name: aws.String(pipelineName),
		Stages: []cptypes.StageDeclaration{{
			Name: aws.String("Source"),
			Actions: []cptypes.ActionDeclaration{{
				Name: aws.String("S3Source"),
				ActionTypeId: &cptypes.ActionTypeId{
					Category: cptypes.ActionCategorySource,
					Owner:    cptypes.ActionOwnerAws,
					Provider: aws.String("S3"),
					Version:  aws.String("1"),
				},
				Configuration: map[string]string{
					"S3Bucket":    sourceBucket,
					"S3ObjectKey": "app/release.zip",
				},
			}},
		}},
	}
	clients := &awsclient.ServiceClients{
		CodePipeline: newFakeCodePipelineWithDeclarations(map[string]*cptypes.PipelineDeclaration{
			pipelineName: decl,
		}),
	}
	src := resource.Resource{ID: pipelineName, Name: pipelineName, Fields: map[string]string{}}

	result := pipelineCheckerByTarget(t, "s3")(context.Background(), clients, src, resource.ResourceCache{})

	if want := []string{sourceBucket}; !slices.Equal(result.ResourceIDs(), want) {
		t.Errorf("ResourceIDs = %v, want %v", result.ResourceIDs(), want)
	}
}
