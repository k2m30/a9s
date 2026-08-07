module github.com/k2m30/a9s/v3

go 1.26.5

require (
	charm.land/bubbles/v2 v2.1.0
	charm.land/bubbletea/v2 v2.0.6
	charm.land/lipgloss/v2 v2.0.3
	github.com/atotto/clipboard v0.1.4
	github.com/aws/aws-sdk-go-v2 v1.43.3
	github.com/aws/aws-sdk-go-v2/config v1.32.34
	github.com/aws/aws-sdk-go-v2/credentials v1.19.33
	github.com/aws/aws-sdk-go-v2/service/acm v1.43.3
	github.com/aws/aws-sdk-go-v2/service/apigateway v1.42.3
	github.com/aws/aws-sdk-go-v2/service/apigatewayv2 v1.37.3
	github.com/aws/aws-sdk-go-v2/service/athena v1.60.3
	github.com/aws/aws-sdk-go-v2/service/autoscaling v1.70.3
	github.com/aws/aws-sdk-go-v2/service/backup v1.59.3
	github.com/aws/aws-sdk-go-v2/service/cloudformation v1.76.0
	github.com/aws/aws-sdk-go-v2/service/cloudfront v1.67.3
	github.com/aws/aws-sdk-go-v2/service/cloudtrail v1.58.3
	github.com/aws/aws-sdk-go-v2/service/cloudwatch v1.66.2
	github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs v1.81.0
	github.com/aws/aws-sdk-go-v2/service/codeartifact v1.41.3
	github.com/aws/aws-sdk-go-v2/service/codebuild v1.72.3
	github.com/aws/aws-sdk-go-v2/service/codepipeline v1.49.3
	github.com/aws/aws-sdk-go-v2/service/costexplorer v1.67.3
	github.com/aws/aws-sdk-go-v2/service/docdb v1.51.3
	github.com/aws/aws-sdk-go-v2/service/dynamodb v1.62.3
	github.com/aws/aws-sdk-go-v2/service/ec2 v1.318.1
	github.com/aws/aws-sdk-go-v2/service/ecr v1.60.3
	github.com/aws/aws-sdk-go-v2/service/ecs v1.89.3
	github.com/aws/aws-sdk-go-v2/service/efs v1.44.3
	github.com/aws/aws-sdk-go-v2/service/eks v1.90.3
	github.com/aws/aws-sdk-go-v2/service/elasticache v1.56.3
	github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk v1.37.3
	github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2 v1.58.4
	github.com/aws/aws-sdk-go-v2/service/eventbridge v1.48.3
	github.com/aws/aws-sdk-go-v2/service/glue v1.151.1
	github.com/aws/aws-sdk-go-v2/service/iam v1.57.1
	github.com/aws/aws-sdk-go-v2/service/kafka v1.57.1
	github.com/aws/aws-sdk-go-v2/service/kinesis v1.46.3
	github.com/aws/aws-sdk-go-v2/service/kms v1.55.3
	github.com/aws/aws-sdk-go-v2/service/lambda v1.101.1
	github.com/aws/aws-sdk-go-v2/service/mwaa v1.43.3
	github.com/aws/aws-sdk-go-v2/service/opensearch v1.75.3
	github.com/aws/aws-sdk-go-v2/service/rds v1.124.0
	github.com/aws/aws-sdk-go-v2/service/redshift v1.65.3
	github.com/aws/aws-sdk-go-v2/service/route53 v1.65.5
	github.com/aws/aws-sdk-go-v2/service/s3 v1.106.4
	github.com/aws/aws-sdk-go-v2/service/secretsmanager v1.44.3
	github.com/aws/aws-sdk-go-v2/service/ses v1.37.3
	github.com/aws/aws-sdk-go-v2/service/sesv2 v1.66.3
	github.com/aws/aws-sdk-go-v2/service/sfn v1.45.3
	github.com/aws/aws-sdk-go-v2/service/sns v1.42.3
	github.com/aws/aws-sdk-go-v2/service/sqs v1.46.3
	github.com/aws/aws-sdk-go-v2/service/ssm v1.73.3
	github.com/aws/aws-sdk-go-v2/service/sts v1.45.3
	github.com/aws/aws-sdk-go-v2/service/transfer v1.75.3
	github.com/aws/aws-sdk-go-v2/service/wafv2 v1.77.2
	github.com/aws/smithy-go v1.27.6
	github.com/charmbracelet/x/ansi v0.11.7
	golang.org/x/sync v0.22.0
	golang.org/x/tools v0.48.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.7.16 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.18.34 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.34 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.34 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.4.35 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.15 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.9.27 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/endpoint-discovery v1.12.11 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.34 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.19.35 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.5.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.33.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.38.3 // indirect
	github.com/charmbracelet/colorprofile v0.4.3 // indirect
	github.com/charmbracelet/ultraviolet v0.0.0-20260416155717-489999b90468 // indirect
	github.com/charmbracelet/x/term v0.2.2 // indirect
	github.com/charmbracelet/x/termios v0.1.1 // indirect
	github.com/charmbracelet/x/windows v0.2.2 // indirect
	github.com/clipperhouse/displaywidth v0.11.0 // indirect
	github.com/clipperhouse/uax29/v2 v2.7.0 // indirect
	github.com/lucasb-eyer/go-colorful v1.4.0 // indirect
	github.com/mattn/go-runewidth v0.0.23 // indirect
	github.com/muesli/cancelreader v0.2.2 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/xo/terminfo v0.0.0-20220910002029-abceb7e1c41e // indirect
	golang.org/x/mod v0.38.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
)
