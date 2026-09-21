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
