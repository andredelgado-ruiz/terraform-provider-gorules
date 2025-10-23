---
page_title: "gorules_project Resource - gorules"
subcategory: ""
description: |-
  Manages a GoRules project. Projects are the top-level organizational unit in GoRules.
---

# gorules_project (Resource)

Manages a GoRules project. Projects are the top-level organizational unit in GoRules where you can organize your business rules, decision tables, and other rule-related resources.

## Example Usage

```terraform
resource "gorules_project" "my_project" {
  name = "E-commerce Rules"
  key  = "ecommerce-rules"
}
```

## Schema

### Required

- `name` (String) The display name of the project
- `key` (String) The unique key identifier for the project. Must be unique across your GoRules instance.

### Optional

- `description` (String) A description of the project

### Read-Only

- `id` (String) The unique identifier of the project
- `created_at` (String) The timestamp when the project was created
- `updated_at` (String) The timestamp when the project was last updated

## Import

Projects can be imported using their `id`:

```shell
terraform import gorules_project.example "project-id-12345"
```