package demo

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	codeartifacttypes "github.com/aws/aws-sdk-go-v2/service/codeartifact/types"
	"github.com/aws/aws-sdk-go-v2/service/codeartifact"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"
	cptypes "github.com/aws/aws-sdk-go-v2/service/codepipeline/types"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// registerCICDHandlers registers CloudFormation, CodeBuild, CodePipeline, ECR, and CodeArtifact handlers.
func registerCICDHandlers(t *Transport) {
	registerCFNHandlers(t)
	registerCodeBuildHandlers(t)
	registerCodePipelineHandlers(t)
	registerECRHandlers(t)
	registerCodeArtifactHandlers(t)
}

// ---------------------------------------------------------------------------
// CloudFormation (awsquery — XML, service "cloudformation")
// ---------------------------------------------------------------------------

func registerCFNHandlers(t *Transport) {
	t.Handle("cloudformation", "DescribeStacks", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["cfn"]()
		body := buildCFNStacksXML(resources)
		return XMLResponse(body), nil
	})
}

func buildCFNStacksXML(resources []resource.Resource) string {
	var sb strings.Builder
	sb.WriteString(`<Stacks>`)
	for _, r := range resources {
		stackName := r.Fields["stack_name"]
		status := r.Fields["status"]
		description := r.Fields["description"]
		creationTime := r.Fields["creation_time"]
		lastUpdated := r.Fields["last_updated"]

		sb.WriteString(`<member>`)
		fmt.Fprintf(&sb, `<StackName>%s</StackName>`, xmlEscape(stackName))
		fmt.Fprintf(&sb, `<StackStatus>%s</StackStatus>`, xmlEscape(status))
		if description != "" {
			fmt.Fprintf(&sb, `<Description>%s</Description>`, xmlEscape(description))
		}
		if creationTime != "" {
			fmt.Fprintf(&sb, `<CreationTime>%s</CreationTime>`, xmlEscape(creationTime))
		}
		if lastUpdated != "" {
			fmt.Fprintf(&sb, `<LastUpdatedTime>%s</LastUpdatedTime>`, xmlEscape(lastUpdated))
		}
		sb.WriteString(`</member>`)
	}
	sb.WriteString(`</Stacks>`)

	return awsQueryXML("DescribeStacks", "http://cloudformation.amazonaws.com/doc/2010-05-15/", sb.String())
}

// ---------------------------------------------------------------------------
// CodeBuild (awsjson11)
// The deserializer expects lowercase "projects" for both ListProjects and BatchGetProjects.
// ---------------------------------------------------------------------------

func registerCodeBuildHandlers(t *Transport) {
	t.Handle("codebuild", "ListProjects", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["cb"]()
		projects := ExtractSDK[cbtypes.Project](resources)

		names := make([]string, 0, len(projects))
		for _, p := range projects {
			if p.Name != nil {
				names = append(names, *p.Name)
			}
		}

		return JSONResponse(map[string]interface{}{"projects": names})
	})

	t.Handle("codebuild", "BatchGetProjects", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["cb"]()
		_ = ExtractSDK[cbtypes.Project](resources)

		// Build project maps with lowercase keys matching the SDK deserializer.
		projectMaps := make([]map[string]interface{}, 0, len(resources))
		for _, r := range resources {
			m := map[string]interface{}{
				"name": r.ID,
			}
			if r.Fields["source_type"] != "" {
				m["source"] = map[string]interface{}{"type": r.Fields["source_type"]}
			}
			if r.Fields["build_status"] != "" {
				m["lastBuildStatus"] = r.Fields["build_status"]
			}
			projectMaps = append(projectMaps, m)
		}

		return JSONResponse(map[string]interface{}{"projects": projectMaps})
	})
}

// ---------------------------------------------------------------------------
// CodePipeline (awsjson11)
// The deserializer expects lowercase "pipelines".
// ---------------------------------------------------------------------------

func registerCodePipelineHandlers(t *Transport) {
	t.Handle("codepipeline", "ListPipelines", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["pipeline"]()
		pipelines := ExtractSDK[cptypes.PipelineSummary](resources)

		// Build pipeline summaries with lowercase-compatible fields.
		pipelineMaps := make([]map[string]interface{}, 0, len(pipelines))
		for _, p := range pipelines {
			m := map[string]interface{}{
				"name": aws.ToString(p.Name),
			}
			if p.Version != nil {
				m["version"] = *p.Version
			}
			pipelineMaps = append(pipelineMaps, m)
		}

		return JSONResponse(map[string]interface{}{"pipelines": pipelineMaps})
	})
}

// ---------------------------------------------------------------------------
// ECR (awsjson11 — X-Amz-Target routing, service "ecr")
// The deserializer expects lowercase "repositories".
// ---------------------------------------------------------------------------

func registerECRHandlers(t *Transport) {
	t.Handle("ecr", "DescribeRepositories", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["ecr"]()
		repos := ExtractSDK[ecrtypes.Repository](resources)

		repoMaps := make([]map[string]interface{}, 0, len(repos))
		for _, r := range repos {
			m := map[string]interface{}{
				"repositoryName": aws.ToString(r.RepositoryName),
				"repositoryArn":  aws.ToString(r.RepositoryArn),
				"registryId":     aws.ToString(r.RegistryId),
			}
			if r.RepositoryUri != nil {
				m["repositoryUri"] = *r.RepositoryUri
			}
			repoMaps = append(repoMaps, m)
		}

		return JSONResponse(map[string]interface{}{"repositories": repoMaps})
	})
}

// ---------------------------------------------------------------------------
// CodeArtifact (restjson1 — routed by URL path)
// ---------------------------------------------------------------------------

func registerCodeArtifactHandlers(t *Transport) {
	t.Handle("codeartifact", "ListRepositories", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["codeartifact"]()
		repos := ExtractSDK[codeartifacttypes.RepositorySummary](resources)

		out := &codeartifact.ListRepositoriesOutput{
			Repositories: repos,
		}
		return JSONResponseCamelCase(out)
	})
}
