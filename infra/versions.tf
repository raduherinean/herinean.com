terraform {
  required_version = ">= 1.8.0"
  required_providers {
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "~> 5.0"
    }
  }
}

# Auth: CLOUDFLARE_API_TOKEN from the environment (scripts/env.sh). Never in files.
provider "cloudflare" {}
