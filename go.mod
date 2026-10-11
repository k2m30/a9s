module github.com/k2m30/a9s/v3

go 1.26.9

require (
	charm.land/bubbles/v2 v2.2.1
	charm.land/bubbletea/v2 v2.1.0
	charm.land/lipgloss/v2 v2.0.6
	github.com/atotto/clipboard v0.1.4
	github.com/aws/aws-sdk-go-v2 v1.47.3
	github.com/aws/aws-sdk-go-v2/config v1.33.9
	github.com/aws/aws-sdk-go-v2/credentials v1.20.9
	github.com/aws/aws-sdk-go-v2/service/acm v1.50.4
	github.com/aws/aws-sdk-go-v2/service/apigateway v1.52.1
	github.com/aws/aws-sdk-go-v2/service/apigatewayv2 v1.46.1
	github.com/aws/aws-sdk-go-v2/service/athena v1.66.4
	github.com/aws/aws-sdk-go-v2/service/autoscaling v1.78.4
	github.com/aws/aws-sdk-go-v2/service/backup v1.69.1
	github.com/aws/aws-sdk-go-v2/service/cloudformation v1.82.1
	github.com/aws/aws-sdk-go-v2/service/cloudfront v1.74.3
	github.com/aws/aws-sdk-go-v2/service/cloudtrail v1.65.4
	github.com/aws/aws-sdk-go-v2/service/cloudwatch v1.73.3
	github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs v1.89.3
	github.com/aws/aws-sdk-go-v2/service/codeartifact v1.46.4
	github.com/aws/aws-sdk-go-v2/service/codebuild v1.78.4
	github.com/aws/aws-sdk-go-v2/service/codepipeline v1.55.4
	github.com/aws/aws-sdk-go-v2/service/costexplorer v1.74.1
	github.com/aws/aws-sdk-go-v2/service/docdb v1.57.4
	github.com/aws/aws-sdk-go-v2/service/dynamodb v1.70.3
	github.com/aws/aws-sdk-go-v2/service/ec2 v1.338.4
	github.com/aws/aws-sdk-go-v2/service/ecr v1.66.4
	github.com/aws/aws-sdk-go-v2/service/ecs v1.100.3
	github.com/aws/aws-sdk-go-v2/service/efs v1.49.4
	github.com/aws/aws-sdk-go-v2/service/eks v1.104.1
	github.com/aws/aws-sdk-go-v2/service/elasticache v1.63.3
	github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk v1.43.4
	github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2 v1.63.4
	github.com/aws/aws-sdk-go-v2/service/eventbridge v1.55.3
	github.com/aws/aws-sdk-go-v2/service/glue v1.169.1
	github.com/aws/aws-sdk-go-v2/service/iam v1.64.4
	github.com/aws/aws-sdk-go-v2/service/kafka v1.65.4
	github.com/aws/aws-sdk-go-v2/service/kinesis v1.56.4
	github.com/aws/aws-sdk-go-v2/service/kms v1.61.4
	github.com/aws/aws-sdk-go-v2/service/lambda v1.112.1
	github.com/aws/aws-sdk-go-v2/service/mwaa v1.48.4
	github.com/aws/aws-sdk-go-v2/service/opensearch v1.85.1
	github.com/aws/aws-sdk-go-v2/service/rds v1.130.3
	github.com/aws/aws-sdk-go-v2/service/redshift v1.71.4
	github.com/aws/aws-sdk-go-v2/service/route53 v1.70.4
	github.com/aws/aws-sdk-go-v2/service/s3 v1.114.3
	github.com/aws/aws-sdk-go-v2/service/secretsmanager v1.50.4
	github.com/aws/aws-sdk-go-v2/service/ses v1.42.4
	github.com/aws/aws-sdk-go-v2/service/sesv2 v1.79.1
	github.com/aws/aws-sdk-go-v2/service/sfn v1.51.4
	github.com/aws/aws-sdk-go-v2/service/sns v1.47.5
	github.com/aws/aws-sdk-go-v2/service/sqs v1.52.4
	github.com/aws/aws-sdk-go-v2/service/ssm v1.79.3
	github.com/aws/aws-sdk-go-v2/service/sts v1.51.4
	github.com/aws/aws-sdk-go-v2/service/transfer v1.85.3
	github.com/aws/aws-sdk-go-v2/service/wafv2 v1.83.4
	github.com/aws/smithy-go v1.28.5
	github.com/charmbracelet/x/ansi v0.11.9
	golang.org/x/sync v0.24.0
	golang.org/x/tools v0.52.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.7.22 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.20.3 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.5.6 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.8.6 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.5.6 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.21 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.11.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/endpoint-discovery v1.13.6 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.14.6 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.20.6 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.10.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.38.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.43.4 // indirect
	github.com/charmbracelet/colorprofile v0.4.3 // indirect
	github.com/charmbracelet/ultraviolet v0.0.0-20261008173134-6b8d4baf91b4 // indirect
	github.com/charmbracelet/x/term v0.2.2 // indirect
	github.com/charmbracelet/x/termios v0.1.1 // indirect
	github.com/charmbracelet/x/windows v0.2.2 // indirect
	github.com/clipperhouse/displaywidth v0.11.0 // indirect
	github.com/clipperhouse/uax29/v2 v2.7.0 // indirect
	github.com/lucasb-eyer/go-colorful v1.4.1 // indirect
	github.com/mattn/go-runewidth v0.0.31 // indirect
	github.com/muesli/cancelreader v0.2.2 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/xo/terminfo v1.2.0 // indirect
	golang.org/x/mod v0.42.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
)
