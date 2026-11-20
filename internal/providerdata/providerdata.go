package providerdata

import (
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/jianyuan/terraform-provider-anthropic/internal/apiclient"
)

type ProviderData struct {
	ApiKey          string
	AuthToken       string
	ApiKeyClient    *anthropic.Client
	AuthTokenClient *anthropic.Client

	ServiceAccounts                *apiclient.ListCache[anthropic.ServiceAccount]
	ServiceAccountWorkspaceMembers *apiclient.ListCache[anthropic.ServiceAccountWorkspaceMember]
	FederationRules                *apiclient.ListCache[anthropic.FederationRule]
}
