# Contributing

Requires [Go](https://go.dev) 1.25+ and a Windows host with the WSL
container feature available (WSL 2.9.3 or later, `wsl --update
--pre-release`) for live integration checks. Unit tests, build, and vet
run without WSL on both Windows and Linux.

```powershell
go build ./...
go test ./...
go vet ./...
gofmt -l -w .
```

Regenerate documentation under `docs/` after changing any schema
(requires [tfplugindocs](https://github.com/hashicorp/terraform-plugin-docs),
already declared as a `tool` dependency in `go.mod`):

```powershell
go generate ./...
```

## Testing against a local build

Unit tests cover schema validation, state handling, CLI arguments, parsing,
credential-safe diagnostics and deletion races. The test workflow runs build,
vet, formatting and unit tests on Windows and Linux.

There are no live Go-level acceptance tests yet; the way to exercise a change is
to drive a locally built provider with real `terraform plan`/`apply`
commands, using Terraform's own
[development overrides](https://developer.hashicorp.com/terraform/cli/config/config-file#development-overrides-for-provider-developers)
feature -- a one-time setup, since once it's in place plain `terraform
plan`/`apply` (no `terraform init` needed) already do exactly this:

1. `go build -o terraform-provider-wslc.exe .` builds the provider. The
   binary name matters: `dev_overrides` looks for an executable starting
   with `terraform-provider-wslc` in the directory you point it at.
2. Point Terraform at that directory instead of the registry, by creating
   a CLI config file (anywhere; this example keeps it out of the repo)
   containing:

   ```hcl
   provider_installation {
     dev_overrides {
       "jmnote/wslc" = "C:\\path\\to\\terraform-provider-wslc"
     }
     direct {}
   }
   ```

   and pointing the `TF_CLI_CONFIG_FILE` environment variable at it for
   your session, e.g. `$env:TF_CLI_CONFIG_FILE = "C:\path\to\dev.tfrc"`.
3. `cd examples/provider-install-verification` and run `terraform plan`.
   Terraform prints a "provider development overrides are in effect"
   warning to confirm the local build (not the registry) is the one
   responding. Only run `plan` there, never `apply` -- see that file's
   header comment for why.

For a change to a resource's actual behavior (not just its schema), apply a
config against a real `wslc.exe` install the same way and inspect the
result with `wslc inspect <id> --type container --format json` /
`wslc list --all` to confirm it matches.
