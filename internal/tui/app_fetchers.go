package tui

import (
	"context"
	"fmt"

	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

// fetchResources returns a tea.Cmd that calls the appropriate AWS fetcher
// using the resource registry. Child resource types (S3 objects, R53 records)
// are handled by fetchChildResources instead.
func (m *Model) fetchResources(resourceType string) tea.Cmd {
	if m.demoMode {
		return m.fetchDemoResources(resourceType)
	}
	clients := m.clients
	return func() tea.Msg {
		if clients == nil {
			return messages.APIErrorMsg{
				ResourceType: resourceType,
				Err:          fmt.Errorf("AWS clients not initialized"),
			}
		}

		ctx := context.Background()
		fetcher := resource.GetFetcher(resourceType)
		if fetcher == nil {
			return messages.APIErrorMsg{
				ResourceType: resourceType,
				Err:          fmt.Errorf("unsupported resource type: %s", resourceType),
			}
		}
		resources, err := fetcher(ctx, clients)
		if err != nil {
			return messages.APIErrorMsg{ResourceType: resourceType, Err: err}
		}
		return messages.ResourcesLoadedMsg{ResourceType: resourceType, Resources: resources}
	}
}

// fetchChildResources returns a tea.Cmd that calls the appropriate child fetcher.
// Uses the child fetcher registry to look up the fetcher by child type name.
func (m *Model) fetchChildResources(childType string, parentCtx map[string]string) tea.Cmd {
	if m.demoMode {
		return m.fetchDemoChildResources(childType, parentCtx)
	}
	clients := m.clients
	return func() tea.Msg {
		if clients == nil {
			return messages.APIErrorMsg{
				ResourceType: childType,
				Err:          fmt.Errorf("AWS clients not initialized"),
			}
		}

		fetcher := resource.GetChildFetcher(childType)
		if fetcher == nil {
			return messages.APIErrorMsg{
				ResourceType: childType,
				Err:          fmt.Errorf("unsupported child type: %s", childType),
			}
		}

		ctx := context.Background()
		pc := resource.ParentContext(parentCtx)
		resources, err := fetcher(ctx, clients, pc)
		if err != nil {
			return messages.APIErrorMsg{ResourceType: childType, Err: err}
		}
		return messages.ResourcesLoadedMsg{ResourceType: childType, Resources: resources}
	}
}

// fetchDemoChildResources returns synthetic fixture data for child views in demo mode.
func (m *Model) fetchDemoChildResources(childType string, parentCtx map[string]string) tea.Cmd {
	return func() tea.Msg {
		resources, ok := demo.GetChildResources(childType, parentCtx)
		if !ok {
			resources = nil
		}
		return messages.ResourcesLoadedMsg{
			ResourceType: childType,
			Resources:    resources,
		}
	}
}

// fetchDemoResources returns a tea.Cmd that provides synthetic fixture data
// instead of calling AWS APIs. Maintains the async message contract.
func (m *Model) fetchDemoResources(resourceType string) tea.Cmd {
	return func() tea.Msg {
		// Resolve alias to canonical short name
		canonicalType := resourceType
		rt := resource.FindResourceType(resourceType)
		if rt != nil {
			canonicalType = rt.ShortName
		}
		resources, _ := demo.GetResources(canonicalType)
		return messages.ResourcesLoadedMsg{
			ResourceType: resourceType,
			Resources:    resources,
		}
	}
}

func (m *Model) fetchIdentity() tea.Cmd {
	if m.demoMode {
		return func() tea.Msg {
			return messages.IdentityLoadedMsg{
				Identity: &awsclient.CallerIdentity{
					AccountID:     "123456789012",
					AccountAlias:  "demo-account",
					Arn:           "arn:aws:sts::123456789012:assumed-role/demo-admin/session",
					RoleName:      "demo-admin",
					SessionName:   "session",
					IdentityName:  "demo-admin",
					IsAssumedRole: true,
				},
			}
		}
	}
	clients := m.clients
	return func() tea.Msg {
		if clients == nil {
			return messages.IdentityErrorMsg{Err: "AWS clients not initialized"}
		}
		identity, err := awsclient.FetchCallerIdentity(context.Background(), clients.STS, clients.IAM)
		if err != nil {
			return messages.IdentityErrorMsg{Err: err.Error()}
		}
		return messages.IdentityLoadedMsg{Identity: identity}
	}
}

func (m *Model) fetchProfiles() tea.Cmd {
	return func() tea.Msg {
		configPath := awsclient.DefaultConfigPath()
		profiles, err := awsclient.ListProfiles(configPath)
		if err != nil {
			return messages.FlashMsg{Text: "failed to list profiles: " + err.Error(), IsError: true}
		}
		if len(profiles) == 0 {
			return messages.FlashMsg{Text: "no AWS profiles found", IsError: true}
		}
		return profilesLoadedMsg{profiles: profiles}
	}
}

type profilesLoadedMsg struct {
	profiles []string
}

func (m *Model) fetchSecretValue(secretName string) tea.Cmd {
	clients := m.clients
	return func() tea.Msg {
		if clients == nil {
			return messages.FlashMsg{Text: "AWS clients not initialized", IsError: true}
		}
		ctx := context.Background()
		value, err := awsclient.RevealSecret(ctx, clients.SecretsManager, secretName)
		if err != nil {
			return messages.FlashMsg{Text: "failed to reveal secret: " + err.Error(), IsError: true}
		}
		return messages.SecretRevealedMsg{SecretName: secretName, Value: value}
	}
}

func (m *Model) connectAWS(profile, region string) tea.Cmd {
	// Resolve region fallback BEFORE the async closure so that NewAWSSession
	// always receives a non-empty region. Without this, the SDK fails with
	// "Missing Region" when ~/.aws/config is absent (issue #82).
	if region == "" {
		configPath := awsclient.DefaultConfigPath()
		region = awsclient.GetDefaultRegion(configPath, profile)
	}
	return func() tea.Msg {
		cfg, err := awsclient.NewAWSSession(profile, region)
		if err != nil {
			return messages.ClientsReadyMsg{Err: err}
		}
		clients := awsclient.CreateServiceClients(cfg)
		return messages.ClientsReadyMsg{Clients: clients}
	}
}
