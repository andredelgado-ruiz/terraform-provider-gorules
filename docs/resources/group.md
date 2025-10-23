---
page_title: "gorules_group Resource - gorules"
subcategory: ""
description: |-
  Manages a GoRules group within a project. Groups are used to organize users and manage permissions.
---

# gorules_group (Resource)

Manages a GoRules group within a project. Groups are used to organize users and manage permissions for accessing and modifying rules within a project.

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
```

## Schema

### Required

- `project_id` (String) The ID of the project this group belongs to
- `name` (String) The display name of the group
- `permissions` (Set of String) Set of permissions assigned to this group. See Available Permissions section for valid values.

### Optional

- `description` (String) A description of the group and its purpose

### Read-Only

- `id` (String) The unique identifier of the group
- `created_at` (String) The timestamp when the group was created
- `updated_at` (String) The timestamp when the group was last updated

## Available Permissions

- `owner` - Full ownership of the project
- `documents` - Access to documents
- `releases` - Access to releases
- `releases:manage` - Manage releases
- `releases:deploy` - Deploy releases
- `releases:delete` - Delete releases
- `integrations:manage` - Manage integrations
- `documents:full` - Full access to documents
- `documents:view-content` - View document content
- `documents:edit-content` - Edit document content
- `documents:edit-view` - Edit document views
- `project:manage` - Manage project settings
- `environments` - Access to environments
- `environments:manage` - Manage environments
- `environments:delete` - Delete environments

## Import

Groups can be imported using their `id`:

```shell
terraform import gorules_group.example "group-id-12345"
```