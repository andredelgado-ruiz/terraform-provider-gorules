---
page_title: "GoRules Provider"
subcategory: ""
description: |-
  The GoRules provider is used to interact with GoRules Business Rules Management System (BRMS). The provider needs to be configured with the proper credentials before it can be used.
---

# GoRules Provider

The GoRules provider is used to interact with [GoRules](https://gorules.io/) Business Rules Management System (BRMS). The provider allows you to manage projects, environments, and groups within your GoRules instance.

## Example Usage

```terraform
terraform {
  required_providers {
    gorules = {
      source  = "andredelgado-ruiz/gorules"
      version = "~> 0.1.0"
    }
  }
}

provider "gorules" {
  base_url = "https://your-gorules-instance.com"
  token    = var.gorules_token  # Personal Access Token
}

# Create a project
resource "gorules_project" "my_project" {
  name = "E-commerce Rules"
  key  = "ecommerce-rules"
}

# Create a group with permissions
resource "gorules_group" "approvers" {
  project_id  = gorules_project.my_project.id
  name        = "Production Approvers"
  description = "Team that can approve production releases"
  permissions = ["documents:view-content", "releases:manage", "releases:deploy"]
}

# Create an environment with approval
resource "gorules_environment" "production" {
  project_id = gorules_project.my_project.id
  name       = "production"
  key        = "prod"
  type       = "production"
  
  approval_mode   = "required"
  approval_groups = [gorules_group.approvers.id]
}
```

## Authentication

The GoRules provider supports authentication via Personal Access Token (PAT).

### Personal Access Token

You can generate a Personal Access Token in your GoRules instance:

1. Navigate to your GoRules dashboard
2. Go to Account Settings > API Tokens
3. Generate a new Personal Access Token
4. Use this token in the provider configuration

```terraform
provider "gorules" {
  base_url = "https://your-gorules-instance.com"
  token    = "your-personal-access-token"
}
```

It's recommended to use environment variables or Terraform variables for sensitive information:

```bash
export GORULES_BASE_URL="https://your-gorules-instance.com"
export GORULES_TOKEN="your-personal-access-token"
```

## Schema

### Required

- `base_url` (String) The base URL of your GoRules instance (e.g., `https://your-gorules-instance.com`)
- `token` (String, Sensitive) Personal Access Token for authentication

### Optional

- `timeout` (Number) HTTP client timeout in seconds. Default: `30`