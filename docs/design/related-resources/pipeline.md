# CodePipelines (pipeline) — Related Resources

## Real-World Use Cases

**1. "Where is my deploy?"** The pipeline has stages (Source, Build, Deploy). You need to see which stage is in progress, which failed, and which succeeded. The stage state is a child view, but the resources WITHIN each stage (CodeBuild project, ECS service, CFN stack) are navigable cross-resource links.

**2. "Which CodeBuild project runs the build stage?"** The pipeline definition has action configurations referencing CodeBuild project names. Navigate to CodeBuild to see build logs and history.

**3. "What does this pipeline deploy to?"** Parse the deploy action to find the target — ECS service, Lambda function, S3 bucket, or CloudFormation stack.

## Reverse Relationships

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| (Minimal) | Pipelines are orchestrators. Other resources don't reference pipelines by ARN. | — | — |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| CodeBuild Projects (cb) | Parse pipeline definition's build stage actions. Actions with `ActionTypeId.Provider=CodeBuild` have `Configuration.ProjectName`. Navigate to the CodeBuild project for build logs. | "Which CodeBuild project runs the build?" Navigate to see build history and logs. | P0 |
| Deploy Targets (various) | Parse pipeline definition's deploy stage actions. `Provider=ECS` → `Configuration.ClusterName` and `Configuration.ServiceName` → navigate to ECS service. `Provider=CloudFormation` → `Configuration.StackName` → navigate to CFN stack. `Provider=S3` → `Configuration.BucketName` → navigate to S3. `Provider=Lambda` → `Configuration.FunctionName` → navigate to Lambda. | "What does this pipeline deploy to?" The destination resources — navigate there to verify deployment success. | P0 |
| Source Repository | Parse pipeline definition's source stage. `Provider=GitHub` / `Provider=CodeCommit` / `Provider=S3` with repository/bucket details. Not navigable in a9s (no git repositories), but important context. | "Where does the code come from?" | P1 |
| IAM Role (role) | Pipeline response has `roleArn` — FORWARD. | "What permissions does this pipeline have?" | P1 |
| S3 Artifact Bucket (s3) | Pipeline response has `artifactStore.location` — FORWARD. Navigate to the bucket for artifact management. | "Where are pipeline artifacts stored?" | P2 |
| SNS Topic (sns) | Pipeline notification rules may publish to SNS. `codestar-notifications:ListNotificationRules` with this pipeline's ARN. | "Who gets notified of pipeline events?" | P2 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| UpdatePipeline | "Who changed the pipeline configuration?" Stage/action changes affect the entire deployment flow. |
| DeletePipeline | "Who deleted this pipeline?" Deployments stop entirely. |
| StartPipelineExecution | "Who triggered a pipeline run?" Shows whether it was manual, webhook-triggered, or scheduled. |
