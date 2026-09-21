# Terraform Provider for WSL container

This provider manages the lifecycle of WSL containers, via
[`wslc.exe`](https://github.com/MicrosoftDocs/WSL/blob/main/WSL/wsl-container.md)
(the WSL Container CLI).

Requires Terraform running natively on a Windows host with the WSL
container feature available (WSL 2.9.3 or later).

## Status

Early scaffolding. `wslc_container` supports create/read/delete; every
configurable attribute besides `id` and `state` replaces the container on
change, since `wslc.exe` has no in-place update for image, command,
environment, labels, or published ports.

## Development

```powershell
go build ./...
go test ./...
```

See `examples/` for sample configuration.
