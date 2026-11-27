module github.com/k2m30/a9s/v3

go 1.26.11

require (
	charm.land/bubbles/v2 v2.2.2
	charm.land/bubbletea/v2 v2.2.0
	charm.land/lipgloss/v2 v2.1.1
	github.com/atotto/clipboard v0.1.4
	github.com/aws/aws-sdk-go-v2 v1.48.1
	github.com/aws/aws-sdk-go-v2/config v1.33.22
	github.com/aws/aws-sdk-go-v2/credentials v1.21.4
	github.com/aws/aws-sdk-go-v2/service/acm v1.52.1
	github.com/aws/aws-sdk-go-v2/service/apigateway v1.53.1
	github.com/aws/aws-sdk-go-v2/service/apigatewayv2 v1.46.10
	github.com/aws/aws-sdk-go-v2/service/athena v1.68.1
	github.com/aws/aws-sdk-go-v2/service/autoscaling v1.82.1
	github.com/aws/aws-sdk-go-v2/service/backup v1.71.1
	github.com/aws/aws-sdk-go-v2/service/cloudformation v1.86.1
	github.com/aws/aws-sdk-go-v2/service/cloudfront v1.78.1
	github.com/aws/aws-sdk-go-v2/service/cloudtrail v1.65.13
	github.com/aws/aws-sdk-go-v2/service/cloudwatch v1.76.1
	github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs v1.92.3
	github.com/aws/aws-sdk-go-v2/service/codeartifact v1.48.7
	github.com/aws/aws-sdk-go-v2/service/codebuild v1.79.1
	github.com/aws/aws-sdk-go-v2/service/codepipeline v1.57.1
	github.com/aws/aws-sdk-go-v2/service/costexplorer v1.75.1
	github.com/aws/aws-sdk-go-v2/service/docdb v1.59.2
	github.com/aws/aws-sdk-go-v2/service/dynamodb v1.74.2
	github.com/aws/aws-sdk-go-v2/service/ec2 v1.350.1
	github.com/aws/aws-sdk-go-v2/service/ecr v1.69.1
	github.com/aws/aws-sdk-go-v2/service/ecs v1.106.1
	github.com/aws/aws-sdk-go-v2/service/efs v1.51.7
	github.com/aws/aws-sdk-go-v2/service/eks v1.105.7
	github.com/aws/aws-sdk-go-v2/service/elasticache v1.66.1
	github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk v1.45.1
	github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2 v1.65.1
	github.com/aws/aws-sdk-go-v2/service/eventbridge v1.55.12
	github.com/aws/aws-sdk-go-v2/service/glue v1.177.1
	github.com/aws/aws-sdk-go-v2/service/iam v1.68.1
	github.com/aws/aws-sdk-go-v2/service/kafka v1.66.1
	github.com/aws/aws-sdk-go-v2/service/kinesis v1.58.1
	github.com/aws/aws-sdk-go-v2/service/kms v1.63.1
	github.com/aws/aws-sdk-go-v2/service/lambda v1.118.1
	github.com/aws/aws-sdk-go-v2/service/mwaa v1.50.1
	github.com/aws/aws-sdk-go-v2/service/opensearch v1.88.2
	github.com/aws/aws-sdk-go-v2/service/rds v1.134.1
	github.com/aws/aws-sdk-go-v2/service/redshift v1.72.1
	github.com/aws/aws-sdk-go-v2/service/route53 v1.73.1
	github.com/aws/aws-sdk-go-v2/service/s3 v1.116.1
	github.com/aws/aws-sdk-go-v2/service/secretsmanager v1.51.1
	github.com/aws/aws-sdk-go-v2/service/ses v1.43.1
	github.com/aws/aws-sdk-go-v2/service/sesv2 v1.80.1
	github.com/aws/aws-sdk-go-v2/service/sfn v1.53.1
	github.com/aws/aws-sdk-go-v2/service/sns v1.48.1
	github.com/aws/aws-sdk-go-v2/service/sqs v1.53.1
	github.com/aws/aws-sdk-go-v2/service/ssm v1.84.1
	github.com/aws/aws-sdk-go-v2/service/sts v1.53.1
	github.com/aws/aws-sdk-go-v2/service/transfer v1.86.9
	github.com/aws/aws-sdk-go-v2/service/wafv2 v1.88.0
	github.com/aws/smithy-go v1.28.11
	github.com/charmbracelet/x/ansi v0.12.0
	golang.org/x/sync v0.25.0
	golang.org/x/tools v0.53.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.7.28 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.20.10 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.5.13 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.8.13 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.5.13 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.26 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.11.14 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/endpoint-discovery v1.13.13 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.14.13 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.20.13 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.14.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.39.6 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.44.6 // indirect
	github.com/charmbracelet/colorprofile v0.4.3 // indirect
	github.com/charmbracelet/ultraviolet v0.0.0-20261125122737-8ed499892fb4 // indirect
	github.com/charmbracelet/x/term v0.2.2 // indirect
	github.com/charmbracelet/x/termios v0.1.1 // indirect
	github.com/charmbracelet/x/windows v0.2.2 // indirect
	github.com/clipperhouse/displaywidth v0.11.0 // indirect
	github.com/clipperhouse/uax29/v2 v2.7.0 // indirect
	github.com/lucasb-eyer/go-colorful v1.4.1 // indirect
	github.com/mattn/go-runewidth v0.0.31 // indirect
	github.com/muesli/cancelreader v0.2.2 // indirect
	github.com/rivo/uniseg v0.4.8-0.20261110175422-3a370411ca06 // indirect
	github.com/xo/terminfo v1.3.1 // indirect
	golang.org/x/mod v0.43.0 // indirect
	golang.org/x/sys v0.50.0 // indirect
)
