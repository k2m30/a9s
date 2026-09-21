// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cptypes "github.com/aws/aws-sdk-go-v2/service/codepipeline/types"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
)

// AWS-owned action providers, by the category each is offered under:
// https://docs.aws.amazon.com/codepipeline/latest/userguide/actions-valid-providers.html
//
// A category is the other half of a provider's identity — ECR sources a
// pipeline on an image push and ECRBuildAndPublish builds one, and neither
// name is accepted under the other's category.
var awsPipelineProvidersByCategory = map[cptypes.ActionCategory][]string{
	cptypes.ActionCategorySource: {
		"S3", "ECR", "CodeCommit", "CodeStarSourceConnection",
	},
	cptypes.ActionCategoryBuild: {
		"CodeBuild", "ECRBuildAndPublish", "Commands",
	},
	cptypes.ActionCategoryTest: {
		"CodeBuild", "DeviceFarm",
	},
	cptypes.ActionCategoryDeploy: {
		"S3", "CloudFormation", "CodeDeploy", "EC2Deploy", "ECS", "CodeDeployToECS",
		"EKS", "ElasticBeanstalk", "AppConfig", "OpsWorks", "ServiceCatalog", "Alexa",
	},
	cptypes.ActionCategoryApproval: {
		"Manual",
	},
	cptypes.ActionCategoryInvoke: {
		"Lambda", "StepFunctions", "InspectorScan", "PipelineInvoke",
	},
	cptypes.ActionCategoryCompute: {
		"Commands",
	},
}

// A demo pipeline is the only pipeline most readers of a9s will ever see, and
// an action AWS would reject teaches them a provider that does not exist.
func TestDemoPipelineActionsUseRealAWSProviders(t *testing.T) {
	decls := fixtures.NewCodePipelineFixtures().Declarations
	if len(decls) == 0 {
		t.Fatal("no pipeline declarations in the demo fixtures")
	}

	checked := 0
	for name, decl := range decls {
		for _, stage := range decl.Stages {
			for _, action := range stage.Actions {
				if action.ActionTypeId == nil {
					t.Errorf("%s/%s/%s declares no ActionTypeId",
						name, aws.ToString(stage.Name), aws.ToString(action.Name))
					continue
				}
				if action.ActionTypeId.Owner != cptypes.ActionOwnerAws {
					continue
				}
				checked++
				category := action.ActionTypeId.Category
				provider := aws.ToString(action.ActionTypeId.Provider)
				valid, known := awsPipelineProvidersByCategory[category]
				if !known {
					t.Errorf("%s/%s/%s declares category %q, which CodePipeline does not have",
						name, aws.ToString(stage.Name), aws.ToString(action.Name), category)
					continue
				}
				found := false
				for _, p := range valid {
					if p == provider {
						found = true
					}
				}
				if !found {
					t.Errorf("%s/%s/%s declares provider %q under category %q — AWS offers %v there",
						name, aws.ToString(stage.Name), aws.ToString(action.Name),
						provider, category, valid)
				}
			}
		}
	}

	if checked == 0 {
		t.Fatal("no AWS-owned actions in the demo pipeline declarations — the rule was never exercised")
	}
}
