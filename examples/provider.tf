# GoRules Provider Configuration Examples

## Basic Configuration

```hcl
terraform {
  required_providers {
    gorules = {
      source  = "andredelgadoruiz/gorules"
      version = "~> 0.1.0"
    }
  }
}

provider "gorules" {
  base_url = "https://your-gorules-instance.com"
  token    = var.gorules_token  # Personal Access Token
}
```

## Complete Example

```hcl
# Configure the GoRules provider
terraform {
  required_providers {
    gorules = {
      source  = "andredelgadoruiz/gorules"
      version = "~> 0.1.0"
    }
  }
}

# Provider configuration
provider "gorules" {
  base_url = var.gorules_base_url
  token    = var.gorules_token
}

# Variables
variable "gorules_base_url" {
  description = "GoRules instance base URL"
  type        = string
  default     = "https://my-gorules.com"
}

variable "gorules_token" {
  description = "GoRules Personal Access Token"
  type        = string
  sensitive   = true
}

# Create a project
resource "gorules_project" "example" {
  name        = "My Terraform Project"
  description = "Project managed by Terraform"
}

# Create an environment within the project
resource "gorules_environment" "development" {
  project_id  = gorules_project.example.id
  name        = "development"
  description = "Development environment"
}

# Create a group (if you have group resources)
resource "gorules_group" "api_team" {
  name        = "API Team"
  description = "Team responsible for API development"
}

# Output values
output "project_id" {
  description = "The ID of the created project"
  value       = gorules_project.example.id
}

output "environment_id" {
  description = "The ID of the created environment"
  value       = gorules_environment.development.id
}
```

## Environment Variables

You can also configure the provider using environment variables:

```bash
export GORULES_BASE_URL="https://your-gorules-instance.com"
export GORULES_TOKEN="your-personal-access-token"
```

## Multiple Environments

```hcl
# Production provider
provider "gorules" {
  alias    = "prod"
  base_url = var.gorules_prod_url
  token    = var.gorules_prod_token
}

# Development provider
provider "gorules" {
  alias    = "dev"
  base_url = var.gorules_dev_url
  token    = var.gorules_dev_token
}

# Production project
resource "gorules_project" "prod_project" {
  provider    = gorules.prod
  name        = "Production Project"
  description = "Production environment project"
}

# Development project
resource "gorules_project" "dev_project" {
  provider    = gorules.dev
  name        = "Development Project"
  description = "Development environment project"
}
```