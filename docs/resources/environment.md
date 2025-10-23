---
page_title: "gorules_environment Resource - gorules"
subcategory: ""
description: |-
  Manages a GoRules environment within a project. Environments represent different stages of your rule development lifecycle.
---

# gorules_environment (Resource)

Manages a GoRules environment within a project. Environments represent different stages of your rule development lifecycle, such as development, staging, and production.

## Example Usage

```terraform
resource "gorules_project" "my_project" {
  name = "E-commerce Rules"
  key  = "ecommerce-rules"
}

resource "gorules_group" "approvers" {
  project_id  = gorules_project.my_project.id
  name        = "Production Approvers"
  description = "Team that can approve production releases"
  permissions = ["documents:view-content", "releases:manage", "releases:deploy"]
}

resource "gorules_environment" "production" {
  project_id = gorules_project.my_project.id
  name       = "production"
  key        = "prod"
  type       = "production"
  
  approval_mode   = "required"
  approval_groups = [gorules_group.approvers.id]
  
  depends_on = [gorules_group.approvers]
}
```

## Schema

### Required

- `project_id` (String) The ID of the project this environment belongs to
- `name` (String) The display name of the environment
- `key` (String) The unique key identifier for the environment within the project
- `type` (String) The type of environment. Valid values: `development`, `staging`, `production`

### Optional

- `description` (String) A description of the environment
- `approval_mode` (String) The approval mode for deployments to this environment. Valid values: `none`, `required`
- `approval_groups` (Set of String) List of group IDs that can approve deployments to this environment

### Read-Only

- `id` (String) The unique identifier of the environment
- `created_at` (String) The timestamp when the environment was created
- `updated_at` (String) The timestamp when the environment was last updated

## Import

Environments can be imported using their `id`:

```shell
terraform import gorules_environment.example "environment-id-12345"
```