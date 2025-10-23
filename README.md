# Terraform Provider GoRules

A Terraform provider for managing resources in GoRules Business Rules Management System (BRMS).

## Requirements

- Terraform >= 1.0
- Go >= 1.21 (for development)
- GoRules BRMS instance with API access

## Using the Provider

### Example Configuration

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

# Create a project
resource "gorules_project" "example" {
  name      = "My Project"
  key       = "my-project"
  protected = false
}

# Create an environment
resource "gorules_environment" "staging" {
  project_id    = gorules_project.example.id
  name          = "Staging"
  type          = "brms"
  approval_mode = "none"
}

# Create a group
resource "gorules_group" "developers" {
  project_id  = gorules_project.example.id
  name        = "Developers"
  description = "Development team group"
  permissions = ["read", "write", "deploy"]
}
```

## Authentication

The provider requires authentication via Personal Access Token (PAT). You can obtain a PAT from your GoRules instance admin panel.

### Environment Variables

You can set the following environment variables instead of hardcoding credentials:

```bash
export GORULES_BASE_URL="https://your-gorules-instance.com"
export GORULES_TOKEN="your-personal-access-token"
```

## Resources

### `gorules_project`

Manages GoRules projects.

#### Arguments

- `name` (String, Required) - Project name
- `key` (String, Required) - Unique project key (regex: `^[a-z0-9]{2,}(-[a-z0-9]+)*$`)
- `protected` (Boolean, Optional) - Whether the project is protected
- `copy_content_ref` (String, Optional) - UUID of project to copy content from

#### Attributes

- `id` (String) - Project UUID

### `gorules_environment`

Manages environments within a GoRules project.

#### Arguments

- `project_id` (String, Required) - Parent project ID
- `name` (String, Required) - Environment name
- `key` (String, Optional) - Environment key (defaults to name if not provided)
- `type` (String, Required) - Environment type (`brms` or `deployment`)
- `approval_mode` (String, Optional) - Approval mode (`none`, `require_one_per_team`, `none_create_request`, `require_any`)
- `approval_groups` (List of String, Optional) - List of group names that can approve

#### Attributes

- `id` (String) - Environment UUID

### `gorules_group`

Manages groups within a GoRules project.

#### Arguments

- `project_id` (String, Required) - Parent project ID
- `name` (String, Required) - Group name
- `description` (String, Optional) - Group description
- `permissions` (List of String, Required) - List of permissions for the group

#### Attributes

- `id` (String) - Group UUID

## Development

### Building the Provider

```bash
make build
```

### Installing Locally

```bash
make install
```

### Running Tests

```bash
make test
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Run the test suite
6. Submit a pull request

## License

This project is licensed under the Mozilla Public License 2.0 - see the [LICENSE](LICENSE) file for details.

## Support

For issues and questions:
- GitHub Issues: [Issues](https://github.com/andredelgadoruiz/terraform-provider-gorules/issues)
- Documentation: [GoRules Documentation](https://gorules.io/docs)