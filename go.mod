module github.com/k2m30/a9s/v3

go 1.26.3

require (
	charm.land/bubbles/v2 v2.0.0
	charm.land/bubbletea/v2 v2.0.2
	charm.land/lipgloss/v2 v2.0.2
	github.com/atotto/clipboard v0.1.4
	github.com/aws/aws-sdk-go-v2 v1.41.8
	github.com/aws/aws-sdk-go-v2/config v1.32.19
	github.com/aws/aws-sdk-go-v2/credentials v1.19.18
	github.com/aws/aws-sdk-go-v2/service/acm v1.39.1
	github.com/aws/aws-sdk-go-v2/service/apigateway v1.40.1
	github.com/aws/aws-sdk-go-v2/service/apigatewayv2 v1.35.1
	github.com/aws/aws-sdk-go-v2/service/athena v1.57.7
	github.com/aws/aws-sdk-go-v2/service/autoscaling v1.66.3
	github.com/aws/aws-sdk-go-v2/service/backup v1.57.1
	github.com/aws/aws-sdk-go-v2/service/cloudformation v1.71.12
	github.com/aws/aws-sdk-go-v2/service/cloudfront v1.64.1
	github.com/aws/aws-sdk-go-v2/service/cloudtrail v1.55.12
	github.com/aws/aws-sdk-go-v2/service/cloudwatch v1.57.1
	github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs v1.74.1
	github.com/aws/aws-sdk-go-v2/service/codeartifact v1.39.1
	github.com/aws/aws-sdk-go-v2/service/codebuild v1.68.17
	github.com/aws/aws-sdk-go-v2/service/codepipeline v1.46.24
	github.com/aws/aws-sdk-go-v2/service/docdb v1.48.16
	github.com/aws/aws-sdk-go-v2/service/dynamodb v1.57.5
	github.com/aws/aws-sdk-go-v2/service/ec2 v1.304.1
	github.com/aws/aws-sdk-go-v2/service/ecr v1.57.3
	github.com/aws/aws-sdk-go-v2/service/ecs v1.81.1
	github.com/aws/aws-sdk-go-v2/service/efs v1.41.17
	github.com/aws/aws-sdk-go-v2/service/eks v1.84.1
	github.com/aws/aws-sdk-go-v2/service/elasticache v1.52.3
	github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk v1.34.5
	github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2 v1.54.13
	github.com/aws/aws-sdk-go-v2/service/eventbridge v1.46.1
	github.com/aws/aws-sdk-go-v2/service/glue v1.142.1
	github.com/aws/aws-sdk-go-v2/service/iam v1.53.11
	github.com/aws/aws-sdk-go-v2/service/kafka v1.52.1
	github.com/aws/aws-sdk-go-v2/service/kinesis v1.43.8
	github.com/aws/aws-sdk-go-v2/service/kms v1.52.1
	github.com/aws/aws-sdk-go-v2/service/lambda v1.90.2
	github.com/aws/aws-sdk-go-v2/service/opensearch v1.70.1
	github.com/aws/aws-sdk-go-v2/service/rds v1.118.3
	github.com/aws/aws-sdk-go-v2/service/redshift v1.62.9
	github.com/aws/aws-sdk-go-v2/service/route53 v1.62.8
	github.com/aws/aws-sdk-go-v2/service/s3 v1.102.1
	github.com/aws/aws-sdk-go-v2/service/secretsmanager v1.41.8
	github.com/aws/aws-sdk-go-v2/service/ses v1.34.25
	github.com/aws/aws-sdk-go-v2/service/sesv2 v1.61.1
	github.com/aws/aws-sdk-go-v2/service/sfn v1.41.1
	github.com/aws/aws-sdk-go-v2/service/sns v1.39.18
	github.com/aws/aws-sdk-go-v2/service/sqs v1.42.28
	github.com/aws/aws-sdk-go-v2/service/ssm v1.68.7
	github.com/aws/aws-sdk-go-v2/service/sts v1.42.2
	github.com/aws/aws-sdk-go-v2/service/wafv2 v1.71.6
	github.com/aws/smithy-go v1.25.1
	github.com/charmbracelet/x/ansi v0.11.6
	github.com/stretchr/testify v1.11.1
	golang.org/x/sync v0.19.0
	gopkg.in/ini.v1 v1.67.2
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.7.10 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.18.24 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.24 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.24 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.4.25 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.9 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.9.17 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/endpoint-discovery v1.12.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.24 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.19.24 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.1.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.30.18 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.36.1 // indirect
	github.com/charmbracelet/colorprofile v0.4.2 // indirect
	github.com/charmbracelet/ultraviolet v0.0.0-20260205113103-524a6607adb8 // indirect
	github.com/charmbracelet/x/term v0.2.2 // indirect
	github.com/charmbracelet/x/termios v0.1.1 // indirect
	github.com/charmbracelet/x/windows v0.2.2 // indirect
	github.com/clipperhouse/displaywidth v0.11.0 // indirect
	github.com/clipperhouse/uax29/v2 v2.7.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/lucasb-eyer/go-colorful v1.3.0 // indirect
	github.com/mattn/go-runewidth v0.0.20 // indirect
	github.com/muesli/cancelreader v0.2.2 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/xo/terminfo v0.0.0-20220910002029-abceb7e1c41e // indirect
	golang.org/x/sys v0.42.0 // indirect
)
