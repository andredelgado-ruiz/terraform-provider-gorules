# GoRules Terraform Provider Release Guide

## Prerequisites

1. **GPG Key**: You need a GPG key for signing releases
2. **GitHub Secrets**: Configure the following secrets in your repository:
   - `GPG_PRIVATE_KEY`: Your private GPG key
   - `PASSPHRASE`: Your GPG key passphrase

## Setting up GPG Key

### 1. Generate GPG Key (if you don't have one)

```bash
gpg --full-generate-key
```

Choose:
- RSA and RSA (default)
- 4096 bits
- Key does not expire (or set expiration as needed)
- Real name and email

### 2. Export your GPG key

```bash
# List your keys
gpg --list-secret-keys --keyid-format LONG

# Export private key (for GitHub secrets)
gpg --armor --export-secret-keys YOUR_KEY_ID

# Export public key (for HCP Terraform)
gpg --armor --export YOUR_KEY_ID
```

### 3. Add to GitHub Secrets

1. Go to your repository settings
2. Navigate to "Secrets and variables" > "Actions"
3. Add `GPG_PRIVATE_KEY` with the private key content
4. Add `PASSPHRASE` with your GPG passphrase

## Release Process

### 1. Create a release locally (for testing)

```bash
make release-snapshot
```

### 2. Create a GitHub release

```bash
# Tag your release
git tag v0.1.0
git push origin v0.1.0
```

The GitHub Action will automatically:
- Build binaries for multiple platforms
- Sign the release with your GPG key
- Create SHA256SUMS and signature files
- Publish the release

## Publishing to HCP Terraform Private Registry

After creating a GitHub release, follow these steps:

### 1. Create the provider in HCP Terraform

Create `provider.json`:
```json
{
  "data": {
    "type": "registry-providers",
    "attributes": {
      "name": "gorules",
      "namespace": "YOUR_ORG_NAME",
      "registry-name": "private"
    }
  }
}
```

```bash
curl \
  --header "Authorization: Bearer $HCP_TERRAFORM_TOKEN" \
  --header "Content-Type: application/vnd.api+json" \
  --request POST \
  --data @provider.json \
  https://app.terraform.io/api/v2/organizations/YOUR_ORG_NAME/registry-providers
```

### 2. Add your GPG public key

Create `key.json`:
```json
{
  "data": {
    "type": "gpg-keys",
    "attributes": {
      "namespace": "YOUR_ORG_NAME",
      "ascii-armor": "-----BEGIN PGP PUBLIC KEY BLOCK-----\n\n[YOUR_PUBLIC_KEY_HERE]\n-----END PGP PUBLIC KEY BLOCK-----\n"
    }
  }
}
```

```bash
curl \
  --header "Authorization: Bearer $HCP_TERRAFORM_TOKEN" \
  --header "Content-Type: application/vnd.api+json" \
  --request POST \
  --data @key.json \
  https://app.terraform.io/api/registry/private/v2/gpg-keys
```

### 3. Create a version and upload binaries

Follow the steps in the HCP Terraform documentation to create versions and upload the binaries from your GitHub releases.

## Next Steps

1. Set up GPG key and GitHub secrets
2. Test with `make release-snapshot`
3. Create your first tagged release
4. Configure HCP Terraform private registry