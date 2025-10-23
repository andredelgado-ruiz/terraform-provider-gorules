# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2025-10-22

### Added
- Initial release of the GoRules Terraform Provider
- Support for managing GoRules projects
- Support for managing GoRules environments  
- Support for managing GoRules groups
- Basic authentication with GoRules API using Personal Access Token
- Provider configuration with base_url and token
- Comprehensive documentation and examples

### Features
- **Project Resource**: Create, read, update, and delete GoRules projects
- **Environment Resource**: Manage project environments
- **Group Resource**: Manage user groups and permissions
- **HTTP Client**: Robust HTTP client with proper error handling
- **Terraform Framework**: Built using the modern Terraform Plugin Framework

### Technical Details
- Built with Go 1.24+ and Terraform Plugin Framework v1.16.1
- Support for multiple platforms (Linux, Windows, macOS) and architectures (amd64, arm64)
- Proper semantic versioning and release management with GoReleaser
- Automated GitHub Actions for testing and releases
- GPG-signed releases for security

### Added
- Initial release of the GoRules Terraform Provider
- Support for managing GoRules projects with `gorules_project` resource
- Support for managing GoRules environments with `gorules_environment` resource  
- Support for managing GoRules groups with `gorules_group` resource
- Authentication via Personal Access Token (PAT)
- Comprehensive error handling and validation
- Support for approval workflows in environments
- Group permission management
- Project protection and content cloning capabilities

### Features
- **Project Management**: Create, read, update, and delete GoRules projects
- **Environment Management**: Full lifecycle management of project environments
- **Group Management**: Create and manage user groups with configurable permissions
- **Approval Workflows**: Support for different approval modes in environments
- **Flexible Configuration**: Optional and computed attributes for all resources
- **Robust Error Handling**: Detailed error messages and graceful failure handling

### Technical Details
- Built with Terraform Plugin Framework v1.16.1
- Go 1.24+ compatibility
- RESTful API integration with GoRules BRMS
- Proper state management and drift detection
- Resource import capabilities

[Unreleased]: https://github.com/andredelgadoruiz/terraform-provider-gorules/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/andredelgadoruiz/terraform-provider-gorules/releases/tag/v0.1.0