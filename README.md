# Rist

Rist is a Windows-focused, deterministic CLI wrapper around restic with an optional local Web UI.

## Security Model

- Passwords are stored in the OS credential backend.
- Config contains no secrets.
- restic is always called with `--password-command "<rist-exe> pw <id>"`.
- UI orchestrates CLI calls and never executes restic directly.

## Credential Backends

- Windows: Credential Manager (`wincred`)
- Linux: Secret Service/libsecret via keyring backend
- macOS: Keychain Services via keyring backend
- Other OSes: no credential backend (unsupported)

## Config

- Path: `%APPDATA%/rist/config.yaml`
- Versioned YAML schema.
- Folder backends only: absolute local paths or UNC paths.

## Quick Start

1. Build the CLI:
    - `go build -o rist.exe ./cmd/rist`
2. Initialize config:
    - `./rist.exe config init`
3. Add repo:
    - `./rist.exe repo add --id main --path "C:\\Backups\\restic"`
4. Store password from stdin:
    - `"your-password" | ./rist.exe pw set main`
5. Initialize restic repo:
    - `./rist.exe repo init main`
6. Add include paths and run backup:
    - `./rist.exe repo include add main "C:\\Users\\me\\Documents"`
    - `./rist.exe backup main`
7. Preview actions without changing the repository:
    - `./rist.exe backup main --dry-run`
    - `./rist.exe forget main --keep-last 5 --dry-run`
    - `./rist.exe restore main --snapshot latest --target "C:\restore" --dry-run`

## Layout

- CLI entrypoint: `./cmd/rist`
- Internal packages: `./internal/*`
- UI template: `./internal/ui/page.html`

## Core Commands

- `rist config init`
- `rist config validate`
- `rist config migrate`
- `rist config show`
- `rist repo add --id <id> --path <path> [--password-target <target>]`
- `rist repo remove <id>`
- `rist repo list`
- `rist repo init <id>`
- `rist repo include add <id> <path>`
- `rist repo include remove <id> <path>`
- `rist repo include list <id>`
- `rist repo exclude add <id> <path>`
- `rist repo exclude remove <id> <path>`
- `rist repo exclude list <id>`
- `rist repo options show <id>`
- `rist repo options set <id> [--one-file-system=true|false] [--verbose=true|false]`
- `rist pw set <id>` (password from stdin)
- `rist pw <id>`
- `rist pw clear <id>`
- `rist backup <id> [--dry-run]`
- `rist snapshots <id>`
- `rist check <id>`
- `rist prune <id>`
- `rist stats <id>`
- `rist forget <id> [--keep-* N] [--prune] [--dry-run]`
- `rist restore <id> --snapshot <snapshot> --target <path> [--dry-run]`
- `rist serve [--addr 127.0.0.1:8787]`

## Build

- `go test ./...`
- `go build ./...`

## Troubleshooting

- `credential backend unavailable`:
  - Linux: ensure a Secret Service backend is running and user session D-Bus is available.
  - macOS: unlock login keychain and allow access.
- `restic command failed (wrong-password)`:
  - Reset password with `rist pw set <id>` and retry.
- `restic command failed (repository-damaged)`:
  - Run `rist check <id>` and inspect repository health.
- Config schema/version errors:
  - Run `rist config migrate` to normalize current schema version.

## License

This project is licensed under the MIT License. See [LICENSE](LICENSE).
