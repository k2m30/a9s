module github.com/k2m30/a9s/v3

go 1.26.8

require (
	charm.land/bubbles/v2 v2.2.1
	charm.land/bubbletea/v2 v2.0.9
	charm.land/lipgloss/v2 v2.0.6
	github.com/atotto/clipboard v0.1.4
	github.com/aws/aws-sdk-go-v2 v1.46.0
	github.com/aws/aws-sdk-go-v2/config v1.33.3
	github.com/aws/aws-sdk-go-v2/credentials v1.20.3
	github.com/aws/aws-sdk-go-v2/service/acm v1.49.0
	github.com/aws/aws-sdk-go-v2/service/apigateway v1.46.0
	github.com/aws/aws-sdk-go-v2/service/apigatewayv2 v1.41.0
	github.com/aws/aws-sdk-go-v2/service/athena v1.65.0
	github.com/aws/aws-sdk-go-v2/service/autoscaling v1.77.0
	github.com/aws/aws-sdk-go-v2/service/backup v1.64.0
	github.com/aws/aws-sdk-go-v2/service/cloudformation v1.80.0
	github.com/aws/aws-sdk-go-v2/service/cloudfront v1.72.0
	github.com/aws/aws-sdk-go-v2/service/cloudtrail v1.64.0
	github.com/aws/aws-sdk-go-v2/service/cloudwatch v1.71.0
	github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs v1.86.0
	github.com/aws/aws-sdk-go-v2/service/codeartifact v1.45.0
	github.com/aws/aws-sdk-go-v2/service/codebuild v1.77.0
	github.com/aws/aws-sdk-go-v2/service/codepipeline v1.54.0
	github.com/aws/aws-sdk-go-v2/service/costexplorer v1.71.0
	github.com/aws/aws-sdk-go-v2/service/docdb v1.55.0
	github.com/aws/aws-sdk-go-v2/service/dynamodb v1.67.0
	github.com/aws/aws-sdk-go-v2/service/ec2 v1.330.0
	github.com/aws/aws-sdk-go-v2/service/ecr v1.64.0
	github.com/aws/aws-sdk-go-v2/service/ecs v1.96.0
	github.com/aws/aws-sdk-go-v2/service/efs v1.48.0
	github.com/aws/aws-sdk-go-v2/service/eks v1.98.0
	github.com/aws/aws-sdk-go-v2/service/elasticache v1.60.0
	github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk v1.41.0
	github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2 v1.62.0
	github.com/aws/aws-sdk-go-v2/service/eventbridge v1.53.0
	github.com/aws/aws-sdk-go-v2/service/glue v1.157.0
	github.com/aws/aws-sdk-go-v2/service/iam v1.63.0
	github.com/aws/aws-sdk-go-v2/service/kafka v1.63.0
	github.com/aws/aws-sdk-go-v2/service/kinesis v1.53.0
	github.com/aws/aws-sdk-go-v2/service/kms v1.59.0
	github.com/aws/aws-sdk-go-v2/service/lambda v1.107.0
	github.com/aws/aws-sdk-go-v2/service/mwaa v1.47.0
	github.com/aws/aws-sdk-go-v2/service/opensearch v1.79.0
	github.com/aws/aws-sdk-go-v2/service/rds v1.128.0
	github.com/aws/aws-sdk-go-v2/service/redshift v1.70.0
	github.com/aws/aws-sdk-go-v2/service/route53 v1.69.0
	github.com/aws/aws-sdk-go-v2/service/s3 v1.112.0
	github.com/aws/aws-sdk-go-v2/service/secretsmanager v1.48.0
	github.com/aws/aws-sdk-go-v2/service/ses v1.41.0
	github.com/aws/aws-sdk-go-v2/service/sesv2 v1.72.0
	github.com/aws/aws-sdk-go-v2/service/sfn v1.49.0
	github.com/aws/aws-sdk-go-v2/service/sns v1.46.0
	github.com/aws/aws-sdk-go-v2/service/sqs v1.51.0
	github.com/aws/aws-sdk-go-v2/service/ssm v1.77.0
	github.com/aws/aws-sdk-go-v2/service/sts v1.49.0
	github.com/aws/aws-sdk-go-v2/service/transfer v1.81.0
	github.com/aws/aws-sdk-go-v2/service/wafv2 v1.82.0
	github.com/aws/smithy-go v1.28.1
	github.com/charmbracelet/x/ansi v0.11.8
	golang.org/x/sync v0.23.0
	golang.org/x/tools v0.50.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.7.20 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.19.2 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.5.2 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.8.2 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.5.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.19 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.11.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/endpoint-discovery v1.13.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.14.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.20.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.9.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.37.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.42.0 // indirect
	github.com/charmbracelet/colorprofile v0.4.3 // indirect
	github.com/charmbracelet/ultraviolet v0.0.0-20260903151058-ae99b731b8c5 // indirect
	github.com/charmbracelet/x/term v0.2.2 // indirect
	github.com/charmbracelet/x/termios v0.1.1 // indirect
	github.com/charmbracelet/x/windows v0.2.2 // indirect
	github.com/clipperhouse/displaywidth v0.11.0 // indirect
	github.com/clipperhouse/uax29/v2 v2.7.0 // indirect
	github.com/lucasb-eyer/go-colorful v1.4.1 // indirect
	github.com/mattn/go-runewidth v0.0.29 // indirect
	github.com/muesli/cancelreader v0.2.2 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/xo/terminfo v1.0.0 // indirect
	golang.org/x/mod v0.41.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
)
