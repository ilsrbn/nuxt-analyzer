# Self-Update Execution Plan

Goal: let users know a new release exists and upgrade the binary in-place.

## 1. Stamp version at build time

- Add `var version = "dev"` in `cmd/impact-map/main.go`.
- Update `.goreleaser.yaml` to set ldflags:
  ```yaml
  builds:
    - id: impact-map
      ldflags:
        - -s -w -X main.version={{.Version}}
  ```
- GoReleaser replaces `{{.Version}}` with the release tag during build.

## 2. Add `version` subcommand

- Register `cobra.Command{Use: "version"}` under root.
- Print `impact-map <version>` and exit.
- This validates that the stamp works before any network code is written.

## 3. Add `upgrade` subcommand

Implement a self-update flow in a new `cmd/impact-map/upgrade.go`:

1. Query `https://api.github.com/repos/ilsrbn/nuxt-analyzer/releases/latest` for tag name.
2. If tag equals stamped version, print `already at latest version` and exit.
3. Build download URL from tag + OS/arch:
   ```
   https://github.com/ilsrbn/nuxt-analyzer/releases/download/<tag>/impact-map_<os>_<arch>.tar.gz
   ```
4. Download archive and `checksums.txt` to a temp directory.
5. Verify downloaded archive hash against `checksums.txt`.
6. Extract binary, get the current executable path with `os.Executable()`.
7. Replace binary atomically:
   - Write new binary to `<target>.new`.
   - `os.Rename` over old path.
   - `os.Chmod` to `0755`.
   - Clean up temp files.
8. Print `Upgraded v<old> → v<new>`.

Use only the standard library (`net/http`, `crypto/sha256`, `archive/tar`, `compress/gzip`). Avoid third-party self-update packages to keep the binary small and auditable.

## 4. Add background update check on normal runs

- On every run of `analyze`, fire a lightweight background check.
- Cache the latest version string in `~/.cache/impact-map/latest_version` with a 24-hour TTL (store checked-at timestamp alongside the version).
- If cached value is stale or missing, fetch latest tag from GitHub API in a goroutine. Do not block the command; if the API call takes more than 2 seconds, abandon it.
- If a newer version is found, print a one-line notice to stderr after the command finishes:
  ```
  impact-map v0.1.0: update available v0.2.0. Run `impact-map upgrade`
  ```
- Respect `--no-color` and keep the notice unobtrusive.

## 5. Update `install.sh`

- After installation, print the installed version using the new `version` command.
- Add a commented hint in the script output so first-time installers see the upgrade path:
  ```bash
  info "Upgrade later with: impact-map upgrade"
  ```

## 6. Update README.md

- Add a new section after Installation:
  ```markdown
  ## Upgrading

  Check the installed version:
  ```bash
  impact-map version
  ```

  Upgrade to the latest release:
  ```bash
  impact-map upgrade
  ```
  ```
- Keep the existing curl-based installation instructions unchanged.
- Add `go install` as an alternative:
  ```markdown
  Or install via Go:
  ```bash
  go install github.com/ilsrbn/nuxt-analyzer/cmd/impact-map@latest
  ```
  ```

## Rollout order

1. Version stamp + `version` command.
2. `upgrade` command with checksum verification.
3. Background update check + caching.
4. README and `install.sh` hints.
