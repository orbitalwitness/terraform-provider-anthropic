package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/anthropics/anthropic-sdk-go/v2/option"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-log/tflogtest"
)

const (
	testIdentityToken = "eyJhbGciOiJSUzI1NiJ9.secret-jwt"
	testAdminKey      = "sk-ant-admin01-secret-admin-key"
)

func mintedToken(n int32) string {
	return fmt.Sprintf("sk-ant-oat01-minted-%d", n)
}

func TestClientsKeepCredentialsOutOfLogs(t *testing.T) {
	unsetAuthEnv(t)
	var exchanges atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == tokenEndpointPath {
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body["assertion"] != testIdentityToken {
				t.Errorf("token exchange assertion = %q (%v)", body["assertion"], err)
			}
			n := exchanges.Add(1)
			_, _ = fmt.Fprintf(w, `{"access_token":%q,"token_type":"Bearer","expires_in":3600}`, mintedToken(n))
			return
		}

		authorization, apiKey := r.Header.Get("Authorization"), r.Header.Get("X-Api-Key")
		switch {
		case apiKey == testAdminKey && authorization == "":
		case authorization == "Bearer "+mintedToken(1) && apiKey == "":
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"type":"error","error":{"type":"authentication_error","message":"token revoked"}}`))
			return
		case authorization == "Bearer "+mintedToken(2) && apiKey == "":
		default:
			t.Errorf("unexpected credentials: Authorization=%q X-Api-Key=%q", authorization, apiKey)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"id":"org","type":"organization","name":"Test"}`))
	}))
	defer server.Close()

	data := &AnthropicProviderModel{
		FederationRuleId: types.StringValue("fdrl_test"),
		OrganizationId:   types.StringValue("11111111-2222-3333-4444-555555555555"),
		IdentityToken:    types.StringValue(testIdentityToken),
	}
	var diags diag.Diagnostics
	federation, ok := federationOption(data, &diags)
	if !ok || federation == nil {
		t.Fatalf("federationOption: %v", diags)
	}
	apiKeyClient, federationClient := newClients(
		[]option.RequestOption{option.WithoutEnvironmentDefaults(), option.WithBaseURL(server.URL)},
		testAdminKey, federation,
	)

	var logs bytes.Buffer
	ctx := tflogtest.RootLogger(context.Background(), &logs)
	if err := federationClient.Get(ctx, "v1/organizations/me", nil, nil); err != nil {
		t.Fatalf("federation client: %v", err)
	}
	if err := apiKeyClient.Get(ctx, "v1/organizations/me", nil, nil); err != nil {
		t.Fatalf("api key client: %v", err)
	}
	if got := exchanges.Load(); got != 2 {
		t.Errorf("exchanges = %d, want 2 (the 401 forces a re-exchange)", got)
	}

	output := logs.String()
	for _, secret := range []string{testIdentityToken, mintedToken(1), mintedToken(2), testAdminKey} {
		if strings.Contains(output, secret) {
			t.Errorf("log output contains %q", secret)
		}
	}
	entries, err := tflogtest.MultilineJSONDecode(&logs)
	if err != nil {
		t.Fatal(err)
	}
	var maskedAuthorization, maskedApiKey, unauthorised int
	for _, entry := range entries {
		if entry["@message"] != "Sending HTTP Request" {
			if entry["tf_http_res_status_code"] == float64(http.StatusUnauthorized) {
				unauthorised++
			}
			continue
		}
		if strings.Contains(fmt.Sprint(entry["tf_http_req_uri"]), tokenEndpointPath) {
			t.Errorf("token exchange was logged: %v", entry)
		}
		if entry["Authorization"] == "***" {
			maskedAuthorization++
		}
		if entry["X-Api-Key"] == "***" {
			maskedApiKey++
		}
	}
	if maskedAuthorization != 2 || maskedApiKey != 1 || unauthorised != 1 {
		t.Errorf("masked Authorization = %d, masked X-Api-Key = %d, logged 401s = %d; want 2, 1, 1\n%s",
			maskedAuthorization, maskedApiKey, unauthorised, output)
	}
}

func TestIdentityTokenSource(t *testing.T) {
	file := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(file, []byte(testIdentityToken+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Run("environment token", func(t *testing.T) {
		unsetAuthEnv(t)
		t.Setenv(envIdentityToken, " "+testIdentityToken+" ")
		source, err := identityTokenSource(&AnthropicProviderModel{})
		if err != nil || source == nil {
			t.Fatalf("identityTokenSource = %v, %v", source, err)
		}
		if token, err := source(context.Background()); err != nil || token != testIdentityToken {
			t.Errorf("token = %q, %v", token, err)
		}
	})

	for name, tc := range map[string]struct {
		data AnthropicProviderModel
		env  map[string]string
		err  string
	}{
		"both attributes": {
			data: AnthropicProviderModel{IdentityToken: types.StringValue(testIdentityToken), IdentityTokenFile: types.StringValue(file)},
			err:  "identity_token and identity_token_file cannot both be set",
		},
		"both environment variables": {
			env: map[string]string{envIdentityToken: testIdentityToken, envIdentityTokenFile: file},
			err: "ANTHROPIC_IDENTITY_TOKEN and ANTHROPIC_IDENTITY_TOKEN_FILE cannot both be set",
		},
		"token attribute and file environment variable": {
			data: AnthropicProviderModel{IdentityToken: types.StringValue(testIdentityToken)},
			env:  map[string]string{envIdentityTokenFile: file},
			err:  "identity_token and ANTHROPIC_IDENTITY_TOKEN_FILE cannot both be set",
		},
		"file attribute and token environment variable": {
			data: AnthropicProviderModel{IdentityTokenFile: types.StringValue(file)},
			env:  map[string]string{envIdentityToken: testIdentityToken},
			err:  "ANTHROPIC_IDENTITY_TOKEN and identity_token_file cannot both be set",
		},
	} {
		t.Run(name, func(t *testing.T) {
			unsetAuthEnv(t)
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			if _, err := identityTokenSource(&tc.data); err == nil || err.Error() != tc.err {
				t.Errorf("error = %v, want %q", err, tc.err)
			}
		})
	}
}

func configureWithUnknownRule(t *testing.T, deferralAllowed bool) *provider.ConfigureResponse {
	t.Helper()
	unsetAuthEnv(t)
	t.Setenv(envFederationRuleId, "fdrl_from_env")
	t.Setenv(envOrganizationId, "11111111-2222-3333-4444-555555555555")
	t.Setenv(envIdentityToken, testIdentityToken)

	ctx := context.Background()
	p := New("test")()
	var schemaResp provider.SchemaResponse
	p.Schema(ctx, provider.SchemaRequest{}, &schemaResp)
	schemaType, ok := schemaResp.Schema.Type().TerraformType(ctx).(tftypes.Object)
	if !ok {
		t.Fatal("provider schema is not an object")
	}
	values := map[string]tftypes.Value{}
	for name := range schemaType.AttributeTypes {
		values[name] = tftypes.NewValue(tftypes.String, nil)
	}
	values["federation_rule_id"] = tftypes.NewValue(tftypes.String, tftypes.UnknownValue)

	req := provider.ConfigureRequest{
		Config:             tfsdk.Config{Schema: schemaResp.Schema, Raw: tftypes.NewValue(schemaType, values)},
		ClientCapabilities: provider.ConfigureProviderClientCapabilities{DeferralAllowed: deferralAllowed},
	}
	var resp provider.ConfigureResponse
	p.Configure(ctx, req, &resp)
	if resp.ResourceData != nil || resp.DataSourceData != nil {
		t.Error("clients were configured from the environment despite an unknown federation_rule_id")
	}
	return &resp
}

func TestProviderConfigureUnknown(t *testing.T) {
	t.Run("deferral allowed", func(t *testing.T) {
		resp := configureWithUnknownRule(t, true)
		if resp.Diagnostics.HasError() {
			t.Errorf("unexpected diagnostics: %v", resp.Diagnostics)
		}
		if resp.Deferred == nil || resp.Deferred.Reason != provider.DeferredReasonProviderConfigUnknown {
			t.Errorf("Deferred = %v, want provider config unknown", resp.Deferred)
		}
	})

	t.Run("deferral not allowed", func(t *testing.T) {
		resp := configureWithUnknownRule(t, false)
		if resp.Deferred != nil {
			t.Errorf("Deferred = %v, want nil", resp.Deferred)
		}
		if !resp.Diagnostics.HasError() || resp.Diagnostics[0].Summary() != "Unknown provider configuration" ||
			!strings.Contains(resp.Diagnostics[0].Detail(), "federation_rule_id") {
			t.Errorf("diagnostics = %v, want an unknown federation_rule_id error", resp.Diagnostics)
		}
	})
}
