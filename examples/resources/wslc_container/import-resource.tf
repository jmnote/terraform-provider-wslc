# Only name and image are included here, since those (along with the
# computed id/state) are the only attributes `terraform import` can
# populate from `wslc inspect`. Adding any other attribute (env, ports,
# cpus, etc.) after import forces a replace on the next apply, since
# there is nothing in state to compare it against -- see this resource's
# Import section above.
resource "wslc_container" "foo" {
  name  = "foo"
  image = "nginx:latest"
}
