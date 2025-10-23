# GoRules Provider Publishing Guide

This document describes the necessary steps to publish your Terraform provider to the HCP Terraform Private Registry.

## Prerequisites

### 1. Required Tools
- Go (you already have it)
- Git (you already have it)
- GoReleaser: `brew install goreleaser` or `go install github.com/goreleaser/goreleaser@latest`
- GPG for signing releases: `brew install gnupg`

### 2. Configure GPG for signing releases

```bash
# Generate a new GPG key (if you don't have one)
gpg --full-generate-key

# List keys to get the ID
gpg --list-secret-keys --keyid-format LONG

# Export public key (needed for HCP Terraform)
gpg --armor --export YOUR_KEY_ID > public.key

# Export private key (for GitHub Secrets)
gpg --armor --export-secret-keys YOUR_KEY_ID > private.key
```

### 3. Configure GitHub Secrets

In your GitHub repository, go to Settings > Secrets and Variables > Actions and add:

- `GPG_PRIVATE_KEY`: The content of the `private.key` file
- `PASSPHRASE`: Your GPG key passphrase (if it has one)

## Publishing Steps

### 1. Prepare the Release

```bash
# Clean and verify everything compiles
make clean && make build

# Verify GoReleaser configuration
make release-check

# Create a local test release
make release-snapshot
```

### 2. Create a Tag and Release on GitHub

```bash
# Create and push a tag
git tag v0.1.0
git push origin v0.1.0
```

This will automatically trigger the GitHub Actions workflow that:
- Compiles the provider for multiple platforms
- Signs binaries with GPG
- Creates a GitHub release with all necessary files

### 3. Publish to HCP Terraform Private Registry

#### 3.1. Create the provider in HCP Terraform

Create a `provider.json` file:
```json
{
  "data": {
    "type": "registry-providers",
    "attributes": {
      "name": "gorules",
      "namespace": "YOUR_ORGANIZATION",
      "registry-name": "private"
    }
  }
}
```

```bash
curl \
  --header "Authorization: Bearer $TF_CLOUD_TOKEN" \
  --header "Content-Type: application/vnd.api+json" \
  --request POST \
  --data @provider.json \
  https://app.terraform.io/api/v2/organizations/YOUR_ORGANIZATION/registry-providers
```

#### 3.2. Upload your GPG public key

Create a `key.json` file:
```json
{
  "data": {
    "type": "gpg-keys",
    "attributes": {
      "namespace": "YOUR_ORGANIZATION",
      "ascii-armor": "-----BEGIN PGP PUBLIC KEY BLOCK-----\n\n[YOUR_PUBLIC_KEY_CONTENT]\n-----END PGP PUBLIC KEY BLOCK-----\n"
    }
  }
}
```

```bash
curl \
  --header "Authorization: Bearer $TF_CLOUD_TOKEN" \
  --header "Content-Type: application/vnd.api+json" \
  --request POST \
  --data @key.json \
  https://app.terraform.io/api/registry/private/v2/gpg-keys
```

#### 3.3. Create a version

Create a `version.json` file:
```json
{
  "data": {
    "type": "registry-provider-versions",
    "attributes": {
      "version": "0.1.0",
      "key-id": "YOUR_GPG_KEY_ID",
      "protocols": ["5.0"]
    }
  }
}
```

```bash
curl \
  --header "Authorization: Bearer $TF_CLOUD_TOKEN" \
  --header "Content-Type: application/vnd.api+json" \
  --request POST \
  --data @version.json \
  https://app.terraform.io/api/v2/organizations/YOUR_ORGANIZATION/registry-providers/private/YOUR_ORGANIZATION/gorules/versions
```

#### 3.4. Upload release files

The previous response will give you URLs to upload:
- `SHA256SUMS` (from GitHub release)
- `SHA256SUMS.sig` (from GitHub release)

#### 3.5. Create platforms and upload binaries

For each platform (linux/amd64, darwin/amd64, etc.), create a `platform.json` file and upload the corresponding binary.

## Important Files Created

1. **`.goreleaser.yml`**: Configuration for compiling and creating releases
2. **`.github/workflows/release.yml`**: GitHub Actions for automating releases
3. **`.github/workflows/test.yml`**: GitHub Actions for tests
4. **`terraform-registry-manifest.json`**: Registry metadata
5. **`LICENSE`**: Software license
6. **`PUBLISHING_GUIDE.md`**: This guide

## Next Steps

1. Install necessary tools (GoReleaser, GPG)
2. Configure GPG and GitHub Secrets
3. Create your first release with `git tag v0.1.0 && git push origin v0.1.0`
4. Configure your provider in HCP Terraform following the API steps

## Useful Resources

- [Official HashiCorp Documentation](https://developer.hashicorp.com/terraform/cloud-docs/registry/publish-providers)
- [GoReleaser Documentation](https://goreleaser.com/)
- [Terraform Registry Publishing](https://registry.terraform.io/publish)