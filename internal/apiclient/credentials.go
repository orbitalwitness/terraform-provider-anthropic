package apiclient

import "github.com/anthropics/anthropic-sdk-go/v2/option"

// WithAdminAPIKey sends an Admin API key as the X-Api-Key header, as the v1
// SDK did. The v2 SDK's option.WithAPIKey sends it as a bearer token instead.
func WithAdminAPIKey(key string) option.RequestOption {
	if key == "" {
		return option.WithHeaderDel("X-Api-Key")
	}
	return option.WithHeader("X-Api-Key", key)
}
