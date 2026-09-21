# Releasing

This is a maintainer process; regular contributors won't need it.

[`.goreleaser.yml`](.goreleaser.yml) is configured to build
`windows_amd64`/`windows_arm64` binaries, `SHA256SUMS`, a GPG signature over
the checksum file, and a GitHub Release, matching the
[Terraform Registry publishing requirements](https://developer.hashicorp.com/terraform/registry/providers/publishing).

**No GitHub Actions workflow wires this up to a tagged push yet** -- unlike
some sibling providers, this repository does not currently have a
`.github/workflows/release.yml`. Until one is added, a release is cut by
running [GoReleaser](https://goreleaser.com) locally against a `v*` tag,
with the following environment variables set (a maintainer's real GPG key,
not configured by this codebase):

| Variable            | Purpose                                                   |
| -------------------- | --------------------------------------------------------- |
| `GPG_FINGERPRINT`   | Fingerprint of the key `goreleaser` signs `SHA256SUMS` with |
| `GPG_PASSPHRASE`    | Passphrase for that key (may be empty for a passphrase-less key) |

The corresponding public key must also be uploaded to the provider's
signing key settings in the Terraform Registry before the registry will
accept a release.
