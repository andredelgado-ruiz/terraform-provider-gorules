---
page_title: "gorules_role Resource - gorules"
subcategory: ""
description: |-
  Manages a GoRules organisation role. Roles map identity provider groups to project permissions.
---

# gorules_role (Resource)

Manages a GoRules organisation role. With per-project role mapping enabled (`SSO_OAUTH2_ROLES_MAPPING_ENABLED=true`), a role grants project permissions to the members of the mapped identity provider groups.

Role mapping is evaluated at login only when the BRMS uses the OIDC provider (`SSO_OAUTH2_PROVIDER=oidc`); on each login the user's roles are replaced by the roles whose `idp_groups` match the user's groups, so removing a user from the identity provider group revokes the role at the next login.

Users and file-level permissions assigned to the role outside of Terraform, including users added by SSO login, are not managed by this resource and are preserved on update.

## Example Usage

```terraform
resource "gorules_project" "my_project" {
  name = "E-commerce Rules"
  key  = "ecommerce-rules"
}

resource "gorules_role" "ecommerce_viewers" {
  name        = "E-commerce Viewers"
  description = "Read-only access for the e-commerce Okta group"
  idp_groups  = ["okta-ecommerce-viewers"]
  project_ids = [gorules_project.my_project.id]
  permissions = ["documents:view-content"]
}
```

## Schema

### Required

- `name` (String) The display name of the role
- `permissions` (Set of String) Project permissions granted by the role. See Available Permissions section for valid values.

### Optional

- `description` (String) A description of the role
- `idp_groups` (Set of String) Identity provider group names mapped to the role
- `all_projects` (Boolean) Whether the role applies to all projects. Defaults to `false`. When `true`, `project_ids` is ignored by the API.
- `project_ids` (Set of String) IDs of the projects the role applies to

### Read-Only

- `id` (String) The unique identifier of the role

## Available Permissions

- `owner`
- `documents`
- `releases`
- `releases:manage`
- `releases:deploy`
- `releases:delete`
- `integrations:manage`
- `documents:full`
- `documents:view-content`
- `documents:edit-content`
- `documents:edit-view`
- `branches`
- `branches:create`
- `branches:merge`
- `branches:delete`
- `project:manage`
- `environments`
- `environments:manage`
- `environments:delete`

## Import

Roles can be imported using their `id`:

```shell
terraform import gorules_role.example "role-id-12345"
```
