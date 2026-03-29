# CodeBuild Projects (cb) — Related Resources

## Real-World Use Cases

**1. "Is the build passing and what broke?"** The project config shows source, environment, and buildspec. But you need recent builds (a child view) and, for failures, the build logs in CloudWatch. Build → CloudWatch Log stream is the critical cross-service link.

**2. "Which pipeline uses this build project?"** During debugging, trace backwards from the CodeBuild project to the CodePipeline that triggers it. The project doesn't know about pipelines.

**3. "What does this build produce?"** Check artifacts configuration — does it push Docker images to ECR? Upload to S3? Feed into a deploy stage?

## Reverse Relationships

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| CodePipeline (pipeline) | Search pipeline definitions for build stage actions with `Configuration.ProjectName` matching this project name. If a9s has pipeline data cached, search in-memory. | "Which pipeline triggers this build?" Navigate to the pipeline to understand the full CI/CD flow. | P0 |
| CloudWatch Alarms (alarm) | Search alarms with `ProjectName` dimension. | "What monitoring watches this build project?" | P2 |
| CloudFormation Stacks (cfn) | Check tags. | "Which stack manages this project?" | P2 |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| CloudWatch Log Group (logs) | Project config has `logsConfig.cloudWatchLogs.groupName` — FORWARD. Default is `/aws/codebuild/{project-name}`. Navigate to the log group to search across all builds. Individual builds have specific stream names in `logs.streamName`. | "Where are all build logs?" The log group contains logs from every build of this project. | P0 |
| S3 Bucket — Logs (s3) | Project config `logsConfig.s3Logs.location` — FORWARD (if S3 logging enabled). | "Where are archived build logs?" | P1 |
| S3 Bucket — Artifacts (s3) | Project config `artifacts.location` — FORWARD (for S3 artifact type). | "Where does the build output go?" | P1 |
| ECR Repository (ecr) | Build environment `image` field may reference ECR (custom build image). Also, builds that produce Docker images typically push to ECR — check the buildspec for `docker push` commands to `{account}.dkr.ecr.{region}.amazonaws.com/{repo}`. Heuristic — requires parsing buildspec. | "Which ECR repo does this build push to?" Navigate to ECR to verify image tags and scan results. | P1 |
| IAM Role (role) | Project has `serviceRole` — FORWARD. Navigate to the role to check what permissions the build has (S3 access, ECR push, secrets reading, etc.). | "Why is the build getting AccessDenied?" Service role permissions. | P1 |
| VPC / Subnets / SGs | Project config `vpcConfig` — FORWARD (if VPC-enabled). VPC builds need network access to private resources (databases, internal registries). | "Why can't this build reach the internal registry?" Check VPC config and SG rules. | P2 |
| Source Repository | Project config `source.location` — FORWARD. GitHub URL, CodeCommit repo, S3 bucket. Not navigable in a9s for git repos, but S3 source is navigable. | "Where does the source code come from?" | P2 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| UpdateProject | "Who changed the build configuration?" Buildspec, environment, or VPC config changes can break builds. |
| DeleteProject | "Who deleted this build project?" Pipeline builds will fail. |
| StartBuild / StopBuild | "Who triggered or stopped a build?" For audit and to understand why a build ran at an unexpected time. |
