module github.com/k2m30/a9s/v3

go 1.26.8

require (
	charm.land/bubbles/v2 v2.2.1
	charm.land/bubbletea/v2 v2.0.10
	charm.land/lipgloss/v2 v2.0.6
	github.com/atotto/clipboard v0.1.4
	github.com/aws/aws-sdk-go-v2 v1.47.1
	github.com/aws/aws-sdk-go-v2/config v1.33.6
	github.com/aws/aws-sdk-go-v2/credentials v1.20.6
	github.com/aws/aws-sdk-go-v2/service/acm v1.50.1
	github.com/aws/aws-sdk-go-v2/service/apigateway v1.50.0
	github.com/aws/aws-sdk-go-v2/service/apigatewayv2 v1.44.0
	github.com/aws/aws-sdk-go-v2/service/athena v1.66.1
	github.com/aws/aws-sdk-go-v2/service/autoscaling v1.78.1
	github.com/aws/aws-sdk-go-v2/service/backup v1.67.0
	github.com/aws/aws-sdk-go-v2/service/cloudformation v1.81.1
	github.com/aws/aws-sdk-go-v2/service/cloudfront v1.73.1
	github.com/aws/aws-sdk-go-v2/service/cloudtrail v1.65.1
	github.com/aws/aws-sdk-go-v2/service/cloudwatch v1.73.0
	github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs v1.88.1
	github.com/aws/aws-sdk-go-v2/service/codeartifact v1.46.1
	github.com/aws/aws-sdk-go-v2/service/codebuild v1.78.1
	github.com/aws/aws-sdk-go-v2/service/codepipeline v1.55.1
	github.com/aws/aws-sdk-go-v2/service/costexplorer v1.73.1
	github.com/aws/aws-sdk-go-v2/service/docdb v1.57.1
	github.com/aws/aws-sdk-go-v2/service/dynamodb v1.69.1
	github.com/aws/aws-sdk-go-v2/service/ec2 v1.337.0
	github.com/aws/aws-sdk-go-v2/service/ecr v1.66.1
	github.com/aws/aws-sdk-go-v2/service/ecs v1.99.1
	github.com/aws/aws-sdk-go-v2/service/efs v1.49.1
	github.com/aws/aws-sdk-go-v2/service/eks v1.102.0
	github.com/aws/aws-sdk-go-v2/service/elasticache v1.62.0
	github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk v1.43.1
	github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2 v1.63.1
	github.com/aws/aws-sdk-go-v2/service/eventbridge v1.55.0
	github.com/aws/aws-sdk-go-v2/service/glue v1.164.0
	github.com/aws/aws-sdk-go-v2/service/iam v1.64.1
	github.com/aws/aws-sdk-go-v2/service/kafka v1.65.1
	github.com/aws/aws-sdk-go-v2/service/kinesis v1.56.1
	github.com/aws/aws-sdk-go-v2/service/kms v1.61.1
	github.com/aws/aws-sdk-go-v2/service/lambda v1.110.0
	github.com/aws/aws-sdk-go-v2/service/mwaa v1.48.1
	github.com/aws/aws-sdk-go-v2/service/opensearch v1.82.0
	github.com/aws/aws-sdk-go-v2/service/rds v1.129.1
	github.com/aws/aws-sdk-go-v2/service/redshift v1.71.1
	github.com/aws/aws-sdk-go-v2/service/route53 v1.70.1
	github.com/aws/aws-sdk-go-v2/service/s3 v1.113.4
	github.com/aws/aws-sdk-go-v2/service/secretsmanager v1.50.1
	github.com/aws/aws-sdk-go-v2/service/ses v1.42.1
	github.com/aws/aws-sdk-go-v2/service/sesv2 v1.76.0
	github.com/aws/aws-sdk-go-v2/service/sfn v1.51.1
	github.com/aws/aws-sdk-go-v2/service/sns v1.47.2
	github.com/aws/aws-sdk-go-v2/service/sqs v1.52.1
	github.com/aws/aws-sdk-go-v2/service/ssm v1.79.0
	github.com/aws/aws-sdk-go-v2/service/sts v1.51.1
	github.com/aws/aws-sdk-go-v2/service/transfer v1.83.1
	github.com/aws/aws-sdk-go-v2/service/wafv2 v1.83.1
	github.com/aws/smithy-go v1.28.2
	github.com/charmbracelet/x/ansi v0.11.8
	golang.org/x/sync v0.23.0
	golang.org/x/tools v0.50.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.7.20 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.20.1 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.5.4 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.8.4 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.5.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.19 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.11.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/endpoint-discovery v1.13.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.14.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.20.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.10.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.38.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.43.1 // indirect
	github.com/charmbracelet/colorprofile v0.4.3 // indirect
	github.com/charmbracelet/ultraviolet v0.0.0-20260922123528-4e49372c11f9 // indirect
	github.com/charmbracelet/x/term v0.2.2 // indirect
	github.com/charmbracelet/x/termios v0.1.1 // indirect
	github.com/charmbracelet/x/windows v0.2.2 // indirect
	github.com/clipperhouse/displaywidth v0.11.0 // indirect
	github.com/clipperhouse/uax29/v2 v2.7.0 // indirect
	github.com/lucasb-eyer/go-colorful v1.4.1 // indirect
	github.com/mattn/go-runewidth v0.0.30 // indirect
	github.com/muesli/cancelreader v0.2.2 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/xo/terminfo v1.2.0 // indirect
	golang.org/x/mod v0.41.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
)
