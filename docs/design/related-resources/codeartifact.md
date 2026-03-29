# CodeArtifact Repositories (codeartifact) — Related Resources

## Real-World Use Cases

**1. "Which build projects pull packages from this repo?"** CodeArtifact is a private package repository (npm, PyPI, Maven). CI/CD builds authenticate to it for dependency resolution. The repo has no consumer list.

**2. "What upstream sources feed this repo?"** CodeArtifact repos can have upstream connections to public registries (npmjs, PyPI, Maven Central) or other CodeArtifact repos. Understanding the upstream chain explains where packages come from.

**3. "Who has access to this repo?"** The repository policy and domain policy control read/write access.

## Reverse Relationships

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| CodeBuild Projects (cb) | Heuristic: search CodeBuild project environment variables for the CodeArtifact domain and repo names, or for `codeartifact login` commands in buildspecs. No direct API link. | "Which builds use this repo?" Best-effort lookup. | P1 |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| Upstream Repositories | `codeartifact:DescribeRepository` returns `upstreams[]` with other CodeArtifact repository names that this repo pulls from. Also `externalConnections[]` for public registry connections (npm, PyPI, Maven). | "Where do packages come from?" Understanding the dependency resolution chain. | P1 |
| Domain | Repository belongs to a CodeArtifact domain. The domain controls cross-account access and KMS encryption. `codeartifact:DescribeDomain`. | "What domain governs this repo?" | P1 |
| Repository Policy | `codeartifact:GetRepositoryPermissionsPolicy` returns the resource policy. | "Who can read/publish packages?" | P1 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| DeleteRepository | "Who deleted this package repo?" Build dependency resolution breaks. |
| PutRepositoryPermissionsPolicy | "Who changed access permissions?" Granting publish access to untrusted principals enables supply chain attacks. |
| PublishPackageVersion | "Who published a package?" Audit trail for internal package releases. |
