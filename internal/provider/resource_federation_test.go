package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// Exercise Terraform plans and state against the SDK's real HTTP transport.
func TestFederationResources(t *testing.T) {
	var mu sync.Mutex
	objects := map[string]map[string]any{}
	archives := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("Authorization") != "Bearer sk-ant-oat-test" || r.Header.Get("x-api-key") != "" {
			t.Errorf("expected OAuth authentication, got Authorization=%q x-api-key=%q", r.Header.Get("Authorization"), r.Header.Get("x-api-key"))
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/v1/organizations/"), "/")
		kind := parts[0]
		if kind != "federation_issuers" && kind != "federation_rules" {
			t.Errorf("unexpected endpoint %s", r.URL.Path)
			w.WriteHeader(404)
			return
		}
		switch {
		case r.Method == http.MethodPost && len(parts) == 1:
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			body["id"] = map[string]string{"federation_issuers": "fdis_test", "federation_rules": "fdrl_test"}[kind]
			body["archived_at"] = nil
			if kind == "federation_rules" {
				workspaceID, ok := body["workspace_id"].(string)
				if !ok {
					t.Error("create must send a workspace_id")
					w.WriteHeader(400)
					return
				}
				body["workspace_ids"] = []string{workspaceID}
				body["applies_to_all_workspaces"] = false
				if _, ok := body["description"]; !ok {
					body["description"] = ""
				}
			}
			objects[kind] = body
		case r.Method == http.MethodPost && len(parts) == 3 && parts[2] == "archive":
			if kind == "federation_issuers" && objects["federation_rules"]["archived_at"] == nil {
				t.Error("issuer archived before dependent rule")
				w.WriteHeader(400)
				return
			}
			archives[kind]++
			objects[kind]["archived_at"] = "2026-11-05T13:00:00Z"
		case r.Method == http.MethodPost && len(parts) == 2:
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			if _, ok := body["issuer_id"]; ok {
				t.Error("update must not send immutable issuer_id")
			}
			for k, v := range body {
				objects[kind][k] = v
			}
			if workspace, ok := body["workspace_id"].(string); ok {
				objects[kind]["workspace_ids"] = []string{workspace}
			}
		case r.Method == http.MethodGet && len(parts) == 2:
			if objects[kind] == nil {
				w.WriteHeader(404)
				_, _ = w.Write([]byte(`{"type":"error","error":{"type":"not_found_error","message":"missing"}}`))
				return
			}
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(400)
			return
		}
		if err := json.NewEncoder(w).Encode(objects[kind]); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	config := func(updated bool) string {
		scope := "workspace:inference"
		if updated {
			scope = "workspace:developer"
		}
		issuerName, ruleName, description, jwks, match := "github", "deploy", `description = "deployment"`, "", `claims = { repository = "example/app" }`
		if updated {
			issuerName, ruleName, description, jwks, match = "github-updated", "deploy-updated", "", `jwks_url = "https://example.com/jwks.json"`, `subject_prefix = "repo:example/app:*"`
		}
		return fmt.Sprintf(`
provider "anthropic" {
 base_url = %q
 auth_token = "sk-ant-oat-test"
}

resource "anthropic_federation_issuer" "test" {
 name = %q
 issuer_url = "https://token.actions.githubusercontent.com"
 %s
}
resource "anthropic_federation_rule" "test" {
 name = %q
 issuer_id = anthropic_federation_issuer.test.id
 service_account_id = "svac_test"
 workspace_id = "wrkspc_test"
 oauth_scope = %q
 %s
 match = { %s }
}
`, server.URL, issuerName, jwks, ruleName, scope, description, match)
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: config(false), Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("anthropic_federation_rule.test", "oauth_scope", "workspace:inference"),
				resource.TestCheckResourceAttr("anthropic_federation_issuer.test", "check_jti", "true"),
				resource.TestCheckResourceAttr("anthropic_federation_rule.test", "token_lifetime_seconds", "3600"),
				resource.TestCheckResourceAttr("anthropic_federation_rule.test", "match.claims.repository", "example/app"),
			)},
			{ResourceName: "anthropic_federation_issuer.test", ImportState: true, ImportStateVerify: true},
			{ResourceName: "anthropic_federation_rule.test", ImportState: true, ImportStateVerify: true},
			{Config: config(true), Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("anthropic_federation_rule.test", "oauth_scope", "workspace:developer"),
				resource.TestCheckResourceAttr("anthropic_federation_issuer.test", "jwks_url", "https://example.com/jwks.json"),
				resource.TestCheckNoResourceAttr("anthropic_federation_rule.test", "description"),
				resource.TestCheckNoResourceAttr("anthropic_federation_rule.test", "match.claims"),
			)},
			{Config: config(false)},
			{PreConfig: func() { mu.Lock(); objects["federation_rules"]["oauth_scope"] = "workspace:developer"; mu.Unlock() }, Config: config(false), PlanOnly: true, ExpectNonEmptyPlan: true},
			{Config: config(false)},
		},
	})
	mu.Lock()
	defer mu.Unlock()
	for _, kind := range []string{"federation_issuers", "federation_rules"} {
		if archives[kind] != 1 {
			t.Errorf("%s archives = %d, want 1", kind, archives[kind])
		}
	}
}

func TestFederationReadMissingAndArchived(t *testing.T) {
	for _, kind := range []string{"issuer", "rule"} {
		for _, status := range []string{"missing", "archived", "forbidden"} {
			t.Run(kind+"/"+status, func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					switch status {
					case "missing":
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"type":"error","error":{"type":"not_found_error","message":"missing"}}`))
					case "forbidden":
						w.WriteHeader(http.StatusForbidden)
						_, _ = w.Write([]byte(`{"type":"error","error":{"type":"permission_error","message":"forbidden"}}`))
					default:
						_, _ = w.Write([]byte(`{"id":"test","archived_at":"2026-11-05T13:00:00Z"}`))
					}
				}))
				defer server.Close()
				client := anthropic.NewClient(option.WithBaseURL(server.URL), option.WithAuthToken("sk-ant-oat-test"), option.WithMaxRetries(0))
				ctx := context.Background()
				var r frameworkresource.Resource
				var model any
				if kind == "issuer" {
					r = &FederationIssuerResource{baseResource: baseResource{base: base{authTokenClient: &client}}}
					model = &FederationIssuerModel{Id: types.StringValue("test")}
				} else {
					r = &FederationRuleResource{baseResource: baseResource{base: base{authTokenClient: &client}}}
					model = &FederationRuleModel{Id: types.StringValue("test")}
				}
				var schemaResp frameworkresource.SchemaResponse
				r.Schema(ctx, frameworkresource.SchemaRequest{}, &schemaResp)
				state := tfsdk.State{Schema: schemaResp.Schema}
				if d := state.Set(ctx, model); d.HasError() {
					t.Fatal(d)
				}
				resp := frameworkresource.ReadResponse{State: state}
				r.Read(ctx, frameworkresource.ReadRequest{State: state}, &resp)
				if status == "forbidden" {
					if !resp.Diagnostics.HasError() || resp.State.Raw.IsNull() {
						t.Fatal("API errors must preserve state and report diagnostics")
					}
				} else if resp.Diagnostics.HasError() || !resp.State.Raw.IsNull() {
					t.Fatalf("expected resource removal, got diagnostics %v and state %v", resp.Diagnostics, resp.State.Raw)
				}
			})
		}
	}
}

func TestFederationRuleRequiresIdentityConstraint(t *testing.T) {
	m := FederationRuleModel{Match: &FederationRuleMatchModel{Audience: types.StringValue("aud"), Claims: types.MapNull(types.StringType)}}
	if _, d := m.ToAPIForCreate(context.Background()); !d.HasError() {
		t.Fatal("audience-only rules must fail")
	}
	m.Match.SubjectPrefix = types.StringValue("repo:example/app:*")
	if _, d := m.ToAPIForCreate(context.Background()); d.HasError() {
		t.Fatal(d)
	}
}
