# Anthropic runtime permission boundaries

Verified against current vendor documentation and this provider on 13 November 2026. Platform pins `orbital.scalr.io/providers/anthropic = 0.4.0-rc`. All necessary exposed controls already exist in that release; this change adds regression tests and an environment-independent example, not a new unsupported argument.

| Control | Selection | Reason and limits |
| --- | --- | --- |
| Service-account organisation role | `developer` | Only non-admin role accepted by the service-account create/update API. Never use `admin` for runtime identities. |
| Workspace membership | `workspace_restricted_developer` | Narrowest documented API-capable workspace role. `workspace_user` is playground-only; restricted developer and developer have the same API capabilities. Restricted developer removes some Console capabilities only. |
| Inference-only federation rule | `workspace:inference` | Messages, streaming, token counting, Models and compatible chat. No Files, Skills or administrative endpoints. |
| Explicit Files-capable profile | `workspace:developer` | Required if a workload actually uploads/deletes Files or uses Skills/Managed Agents. Broader than inference; not the default. |
| Provisioning credential | Separate OAuth bearer with `org:admin` | Required to manage service accounts/membership/rules. Admin API keys are not accepted by those endpoints. Never pass this credential to the runtime. |

## What Terraform and the API can manage

The provider's `anthropic_service_account` supports create/read, organisation-role update, archive and import by ID. `anthropic_service_account_workspace_member` supports create/read, workspace-role update, removal and import by `<workspace_id>/<service_account_id>`. `anthropic_federation_rule` supports create/read, scope/match/target updates, archive and import by rule ID. Role and scope changes are updates, not account replacements. Refresh reads the current role/scope and detects changed managed values. These resources do not enumerate and revoke additional memberships or other rules created separately.

The example takes explicit environment/workload, workspace/issuer IDs, audience and exact subject. Reuse it with separate Joy, pre-prod and prod inputs/state. It uses one workspace rather than all-workspace access and waits for membership before creating the rule. No credentials or hardcoded environment identities are included in its outputs. Supply the administrative bearer through the trusted Scalr provisioning configuration; token automation is tracked separately in CORE-1641.

## Vendor limitations — do not mistake scope for identity permissions

[Workspace roles](https://platform.claude.com/docs/en/manage-claude/workspaces#workspace-roles-and-permissions) grant restricted developers API access **and API-key management**. Their extra restrictions are Console-only. The [service-account membership API](https://platform.claude.com/docs/en/api/http/organization/workspaces/service_accounts/add) exposes admin, developer, restricted developer and user, but no inference-only membership role. It also documents an implicit `workspace_user` membership in the default workspace, which cannot be removed.

The separately documented [RBAC Roles API](https://platform.claude.com/docs/en/api/cli/organization/rbac_roles/permissions/list) is for Claude Enterprise organisations and exposes role/permission reads, not a Console service-account custom-role create/update/assignment control. It does not supply a supported workaround for this API-account setup.

Therefore an underlying service account with working API access cannot currently be made strictly inference-only using these documented role controls. The API does not expose a narrower service-account organisation role than `developer`, or a workspace role that permits API inference but removes all key-management capability. Do not claim the requirement to remove **all underlying account-management permissions** is met. The example selects the narrowest documented API-capable membership and caps its WIF credential; it grants neither organisation nor billing administration. Verify actual key-management denials before relying on any stronger restriction.

[WIF scopes](https://platform.claude.com/docs/en/manage-claude/wif-reference#oauth-scopes) are a ceiling intersected with the account's role and selected workspace membership. They cannot expand the underlying identity. Requests outside the scope return 403. Anthropic documents no per-resource or read/write subdivision of `workspace:developer`; a Files-only addition to inference is not available through this rule API. An OAuth `org:admin` caller can manage only developer/inference rules, not grant an administrative runtime rule. Other credentials and rules targeting the same service account must be audited separately. Scope does not rewrite the account's role or restrict a separately issued key.

## Application operations and profile choice

Source inspected: `orbitalwitness/project100x@7ca6e20ae8e1821f11793add7a6b40272192d10e`.

- `providers/router.py:get_anthropic_model` constructs `CachedAnthropicModel` **without** an image-file resolver, with an explicit inline-image policy. `providers/anthropic_cached.py` only calls its Files resolver when explicitly configured. `services/provider_image_files.py` has upload/delete helpers, but no runtime callers enable that resolver. Thus `needs_files=false` is the current narrowest profile, including the normal Anthropic failover path.
- Streaming, token counting, inline images and function-tool exchanges go through Messages. Application tools execute in our own services/sandbox; they do not imply access to Anthropic Managed Agents or Skills. Native web search is provider-executed within Messages, not organisation administration.
- If a future reviewed change enables image Files reuse or provider-hosted Skills/Managed Agents, select `needs_files=true` for that workload, acknowledging the broader scope. Changing an unrelated provider's permissions must not break failover.

## Validation and platform adoption

Mock-backed tests cover scope changes both ways, scope drift, import and membership role updates/drift. They prove Terraform/SDK HTTP behaviour, not live vendor authorisation. A local representative plan for the example should add three resources: service account, membership and rule, with no replacement. No vendor credentials or live apply are used here.

Platform can use `workspace:inference` and `workspace_restricted_developer` with its existing `0.4.0-rc` pin in a separately reviewed configuration change. No new provider release is required for these controls. Compare all currently enabled runtime operations first; do not replace existing identities merely to change these values. Live acceptance must verify Messages streaming/tool/image input, both failover directions, denial of Files under inference scope and denial of organisation/billing management. The unresolved underlying key-management limitation remains a vendor restriction, not a missing provider feature.
