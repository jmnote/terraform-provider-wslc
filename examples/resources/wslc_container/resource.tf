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
