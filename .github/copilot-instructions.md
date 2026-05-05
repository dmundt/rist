# GitHubCopilot Project Prompt for rist (CLI + Web-UI)

You are assisting in building a Windows-focused backup orchestration tool called “rist”.

Rist is a deterministic, minimal, secure CLI wrapper around restic, with an optional local Web-UI for configuration and backup control. Rist never stores secrets in plaintext and never executes restic directly from the UI. The CLI is the execution engine; the UI is an orchestrator.

## High-Level Goals

- Provide a clean CLI for managing restic repositories, backup sets, restore flows, and maintenance.
- Provide an optional local Web-UI that edits the YAML config and triggers CLI commands.
- Store all secrets (restic repo passwords) exclusively in the OS credential backend.
- Never store passwords in config files, logs, or environment variables.
- Use restic’s `--password-command` to supply passwords securely.
- Support folder backends only (local paths or UNC paths).
- Use a single YAML config file at: `%APPDATA%/rist/config.yaml`.
- Ensure deterministic behavior, predictable output, and strict error handling.

## Security Requirements

- No plaintext secrets on disk.
- No environment variables required by the user.
- Passwords are stored only in the OS credential backend under targets like `rist-repo-<id>`.
- The CLI provides a subcommand `rist pw <id>` that prints only the repo password to stdout.
- The Web-UI never stores passwords; it only passes them to the CLI for secure storage.
- The config directory must be ACL-restricted to the current user.

## Config File Specification (YAML)

Location: `%APPDATA%/rist/config.yaml`

```yaml
version: 1
repos:
  - id: "<string>"
    path: "<string>" # local or UNC folder backend
    passwordTarget: "rist-repo-<id>" # OS credential target name
    include:
      - "<path>"
    exclude:
      - "<path>"
    options:
      oneFileSystem: <bool>
      verbose: <bool>
```

No secrets appear in this file.
