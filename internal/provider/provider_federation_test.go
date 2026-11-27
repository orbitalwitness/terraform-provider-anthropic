package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func unsetAuthEnv(t *testing.T) {
	for _, env := range []string{
		"ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN",
		envFederationRuleId, envOrganizationId, envServiceAccountId, envWorkspaceId, envIdentityToken, envIdentityTokenFile,
	} {
		t.Setenv(env, "")
	}
}

func federationTestServer(t *testing.T, exchangeStatus int, exchanges, apiCalls *atomic.Int32) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/oauth/token":
			exchanges.Add(1)
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			want := map[string]string{
				"grant_type":         "urn:ietf:params:oauth:grant-type:jwt-bearer",
				"assertion":          "scalr-jwt",
				"federation_rule_id": "fdrl_test",
				"organization_id":    "11111111-2222-3333-4444-555555555555",
				"service_account_id": "svac_test",
			}
			for k, v := range want {
				if body[k] != v {
					t.Errorf("token exchange %s = %q, want %q", k, body[k], v)
				}
			}
			w.WriteHeader(exchangeStatus)
			if exchangeStatus != http.StatusOK {
				_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"assertion rejected"}`))
				return
			}
			_, _ = w.Write([]byte(`{"access_token":"sk-ant-oat-exchanged","token_type":"Bearer","expires_in":3600}`))
		case "/v1/organizations/service_accounts":
			apiCalls.Add(1)
			if got := r.Header.Get("Authorization"); got != "Bearer sk-ant-oat-exchanged" || r.Header.Get("X-Api-Key") != "" {
				t.Errorf("expected the exchanged bearer token, got Authorization=%q x-api-key=%q", got, r.Header.Get("X-Api-Key"))
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(`{"data":[],"next_page":null}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func federationTestConfig(baseURL, extra string) string {
	return fmt.Sprintf(`
provider "anthropic" {
  base_url           = %q
  federation_rule_id = "fdrl_test"
  service_account_id = "svac_test"
  identity_token     = "scalr-jwt"
  %s
}

data "anthropic_service_accounts" "test" {}
`, baseURL, extra)
}

const testOrganizationId = `organization_id = "11111111-2222-3333-4444-555555555555"`

func TestProviderFederation(t *testing.T) {
	unsetAuthEnv(t)
	var exchanges, apiCalls atomic.Int32
	server := federationTestServer(t, http.StatusOK, &exchanges, &apiCalls)
	defer server.Close()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: federationTestConfig(server.URL, testOrganizationId)},
		},
	})

	if apiCalls.Load() == 0 {
		t.Error("expected the admin API to be called")
	}
	// Each Terraform command configures a fresh provider, which exchanges once.
	if exchanges.Load() == 0 || exchanges.Load() > apiCalls.Load() {
		t.Errorf("exchanges = %d for %d API calls, want one per provider instance", exchanges.Load(), apiCalls.Load())
	}
}

func TestProviderFederationErrors(t *testing.T) {
	unsetAuthEnv(t)
	var exchanges, apiCalls atomic.Int32
	server := federationTestServer(t, http.StatusUnauthorized, &exchanges, &apiCalls)
	defer server.Close()

	for name, tc := range map[string]struct {
		extra string
		err   string
	}{
		"missing organization id": {"", `Missing: organization_id`},
		"auth token conflict":     {testOrganizationId + "\n  auth_token = \"sk-ant-oat-manual\"", `auth_token cannot be combined with workload identity federation`},
		"invalid workspace id":    {testOrganizationId + "\n  workspace_id = \"ws_test\"", `workspace_id is invalid`},
		"exchange rejected":       {testOrganizationId, `(?s)oauth token request failed \(status 401\);\s+invalid_grant: assertion rejected`},
	} {
		t.Run(name, func(t *testing.T) {
			resource.UnitTest(t, resource.TestCase{
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{
					{Config: federationTestConfig(server.URL, tc.extra), ExpectError: regexp.MustCompile(tc.err)},
				},
			})
		})
	}

	if apiCalls.Load() != 0 {
		t.Errorf("admin API called %d times without a valid token", apiCalls.Load())
	}
}
