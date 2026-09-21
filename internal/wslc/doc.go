// Package wslc isolates every interaction with wslc.exe (the WSL Container
// CLI, see https://github.com/MicrosoftDocs/WSL/blob/main/WSL/wsl-container.md)
// behind a small, Terraform-agnostic interface (Client). internal/provider
// depends on this package; this package must never import anything from
// terraform-plugin-framework, so that the Client methods and the output
// parsing they rely on can be unit tested without Terraform, and without a
// real Windows/WSL host, by substituting a fake Runner.
package wslc
