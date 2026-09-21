---
page_title: "Getting Started with the wslc Provider"
subcategory: ""
description: |-
  A first walkthrough: run a container with Terraform.
---

# Getting Started with the wslc Provider

This walks through running a single container with Terraform.

## 1. Configure the provider

```terraform
terraform {
  required_providers {
    wslc = {
      source = "jmnote/wslc"
    }
  }
}

provider "wslc" {}
```

## 2. Declare a container

```terraform
resource "wslc_container" "nginx" {
  name  = "example-nginx"
  image = "nginx:latest"
  pull  = "missing" # "always", "missing" (default), or "never"

  hostname = "nginx"
  cpus     = "1"
  memory   = "512M"

  env = {
    NGINX_ENTRYPOINT_QUIET_LOGS = "1"
  }

  labels = {
    "managed-by" = "terraform"
  }

  ports = [
    {
      host_port      = 18080
      container_port = 80
    }
  ]
}
```

`image` follows the same reference syntax as `wslc create <image>` (e.g.
`"nginx:latest"`). `pull` controls whether it's fetched: `"missing"` (the
default) only pulls if not already present locally, `"always"`
re-checks the registry every time, `"never"` fails instead of pulling.

## 3. Apply

```shell
terraform init
terraform apply
```

This runs `wslc create` followed by `wslc start`.

## 4. See what you created

`terraform apply` itself only reports create/change/destroy counts, not a
resource's attribute values. To see those -- e.g. the container's
assigned ID -- declare an output:

```terraform
# Terraform does not print a resource's attributes after apply on its own;
# an explicit output is how to see them, e.g. the container's assigned ID
# and current run state, without a separate `terraform show`/`state show`
# step.
output "nginx" {
  value = <<-EOT
    ID: ${wslc_container.nginx.id}
    State: ${wslc_container.nginx.state}
  EOT
}
```

`terraform apply` then prints it under `Outputs:`, and `terraform output
nginx` reprints it any time afterward without another apply.

## Next steps

- Full attribute reference: [`wslc_container`](../resources/container.md).
- `terraform destroy`, or changing almost any attribute, permanently
  removes the container; see the resource documentation's
  destructive-delete note before you apply.
