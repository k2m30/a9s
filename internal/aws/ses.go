package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/sesv2"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("ses", []string{"identity_name", "identity_type", "verification_status", "sending_enabled"})
	resource.Register("ses", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchSESIdentities(ctx, c.SESv2)
	})
}

// FetchSESIdentities calls the SES v2 ListEmailIdentities API and converts the
// response into a slice of generic Resource structs.
func FetchSESIdentities(ctx context.Context, api SESv2ListEmailIdentitiesAPI) ([]resource.Resource, error) {
	output, err := api.ListEmailIdentities(ctx, &sesv2.ListEmailIdentitiesInput{})
	if err != nil {
		return nil, fmt.Errorf("fetching SES identities: %w", err)
	}

	var resources []resource.Resource

	for _, identity := range output.EmailIdentities {
		identityName := ""
		if identity.IdentityName != nil {
			identityName = *identity.IdentityName
		}

		identityType := string(identity.IdentityType)
		sendingEnabled := fmt.Sprintf("%t", identity.SendingEnabled)
		verificationStatus := string(identity.VerificationStatus)

		r := resource.Resource{
			ID:     identityName,
			Name:   identityName,
			Status: "",
			Fields: map[string]string{
				"identity_name":       identityName,
				"identity_type":       identityType,
				"sending_enabled":     sendingEnabled,
				"verification_status": verificationStatus,
			},
			RawStruct: identity,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
