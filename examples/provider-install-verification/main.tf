####################################################################
# This config is for use with `dev_overrides` (see CONTRIBUTING.md,
# "Testing against a local build") to verify a locally built provider
# actually loads and responds -- not a usage example. See examples/
# for those.
#
# Only run `terraform plan` here, never `apply`: planning a resource
# that doesn't exist in state yet only exercises the provider's
# schema/validation logic and never touches wslc.exe, so there's
# nothing here that needs to already exist on your machine. Applying
# would really create this container.
####################################################################

terraform {
  required_providers {
    wslc = {
      source = "jmnote/wslc"
    }
  }
}

provider "wslc" {}

resource "wslc_container" "example" {
  name  = "provider-install-verification"
  image = "nginx:latest"
}
