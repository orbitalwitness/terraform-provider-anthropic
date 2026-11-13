package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestRuntimeWorkspaceMembership(t *testing.T) {
	var mu sync.Mutex
	role := ""
	creates := 0
	removed := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("Authorization") != "Bearer sk-ant-oat-test" || r.Header.Get("x-api-key") != "" {
			t.Error("wrong provisioning credential")
			w.WriteHeader(401)
			return
		}
		base := "/v1/organizations/workspaces/wrkspc_test/service_accounts"
		if !strings.HasPrefix(r.URL.Path, base) {
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(404)
			return
		}
		switch r.Method {
		case http.MethodPost:
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if r.URL.Path == base {
				creates++
				if body["service_account_id"] != "svac_test" {
					t.Error("missing account ID")
				}
			}
			role, _ = body["workspace_role"].(string)
		case http.MethodGet:
			if role == "" {
				w.WriteHeader(404)
				_, _ = w.Write([]byte(`{"type":"error","error":{"type":"not_found_error","message":"missing"}}`))
				return
			}
		case http.MethodDelete:
			removed = true
			_, _ = w.Write([]byte(`{"type":"service_account_workspace_member_deleted","service_account_id":"svac_test","workspace_id":"wrkspc_test"}`))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"type": "service_account_workspace_member", "workspace_id": "wrkspc_test", "service_account_id": "svac_test", "workspace_role": role, "implicit": false, "created_by_actor_id": "user_admin"})
	}))
	defer server.Close()
	config := func(role string) string {
		return fmt.Sprintf(`provider "anthropic" {
 base_url = %q
 auth_token = "sk-ant-oat-test"
}
resource "anthropic_service_account_workspace_member" "runtime" {
 workspace_id = "wrkspc_test"
 service_account_id = "svac_test"
 workspace_role = %q
}`, server.URL, role)
	}
	rn := "anthropic_service_account_workspace_member.runtime"
	resource.UnitTest(t, resource.TestCase{ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: config("workspace_developer")},
		{Config: config("workspace_restricted_developer"), Check: resource.TestCheckResourceAttr(rn, "workspace_role", "workspace_restricted_developer")},
		{ResourceName: rn, ImportState: true, ImportStateId: "wrkspc_test/svac_test", ImportStateVerify: true, ImportStateVerifyIdentifierAttribute: "service_account_id"},
		{PreConfig: func() { mu.Lock(); role = "workspace_admin"; mu.Unlock() }, Config: config("workspace_restricted_developer"), PlanOnly: true, ExpectNonEmptyPlan: true},
		{Config: config("workspace_restricted_developer"), Check: resource.TestCheckResourceAttr(rn, "workspace_role", "workspace_restricted_developer")},
	}})
	if creates != 1 || !removed {
		t.Errorf("membership replaced or not removed: creates=%d removed=%t", creates, removed)
	}
}
