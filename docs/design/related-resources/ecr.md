# ECR Repositories (ecr) — Related Resources

## Real-World Use Cases

**1. "Which services use images from this repository?"** Before deleting or modifying the repo, you need to find all ECS task definitions, Lambda functions, and EKS pods that pull images from this repo. Deleting a repo doesn't immediately break running containers (they already have the image), but new deployments and scaling events will fail.

**2. "When was the latest image pushed?"** Deployment verification: after CI runs, confirm the expected tag exists in ECR with a recent push timestamp. This is a child view (Images), but the producers (CodeBuild) and consumers (ECS, Lambda, EKS) are cross-resource links.

**3. "Does this repo have vulnerability findings?"** ECR image scanning results show CVE counts by severity. Navigate from a scan finding to the specific image and then to the services running that image.

## Reverse Relationships

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| ECS Task Definitions (not in a9s) | Search task definitions for `containerDefinitions[].image` containing this repo's URI (`{account}.dkr.ecr.{region}.amazonaws.com/{repo-name}`). Requires iterating task definition families. If a9s has ECS service data cached, check their task definitions. | "Which ECS services use images from this repo?" Breaking change if the repo or tag is deleted. | P0 |
| Lambda Functions (lambda) | `lambda:ListFunctions` and filter for `PackageType=Image` with `Code.ImageUri` containing this repo's URI. | "Which Lambda functions use container images from this repo?" | P1 |
| CodeBuild Projects (cb) | Search CodeBuild projects for `environment.image` containing this repo's URI (custom build images from ECR). Also: builds that push to this repo (heuristic — requires parsing buildspecs). | "What builds push to or pull from this repo?" | P1 |
| CloudFormation Stacks (cfn) | Check tags. | "Which stack manages this repo?" | P2 |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| Lifecycle Policy | `ecr:GetLifecyclePolicy` returns rules for automatic image cleanup (e.g., keep only last 10 tagged images, expire untagged after 14 days). Not a resource but critical for understanding why images disappear. | "Why did my image tag vanish?" Lifecycle policies automatically delete old images. | P1 |
| Repository Policy | `ecr:GetRepositoryPolicy` returns the resource policy controlling cross-account access. Parse `Principal` for account IDs. | "Who can pull images from this repo?" Security audit — cross-account pull access must be intentional. | P1 |
| KMS Key (kms) | If encryption is configured, `encryptionConfiguration.kmsKey` — FORWARD. | "Who controls the encryption key?" | P2 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| DeleteRepository | "Who deleted this repository?" All images are lost. Deployments and scaling events fail immediately. |
| PutImage | "Who pushed an image?" The most important ECR audit event for CI pipelines. Shows the image tag, digest, and the actor (usually a CI role). |
| SetRepositoryPolicy | "Who changed cross-account access?" Granting pull access to an untrusted account can leak proprietary software. |
