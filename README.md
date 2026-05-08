# Nuxt Analyzer

Nuxt release impact analyzer CLI.

## Installation

Install the latest release:

```bash
curl -fsSL https://raw.githubusercontent.com/ilsrbn/nuxt-analyzer/main/install.sh | bash
```

Or install via Go:

```bash
go install github.com/ilsrbn/nuxt-analyzer/cmd/impact-map@latest
```

Safer inspect-first install:

```bash
curl -fsSL https://raw.githubusercontent.com/ilsrbn/nuxt-analyzer/main/install.sh -o install.sh
bash install.sh
```

Install a specific version:

```bash
VERSION=v0.1.0 bash install.sh
```

Install to a custom directory:

```bash
INSTALL_DIR=/usr/local/bin bash install.sh
```

By default, the installer places the binary at:

```text
~/.local/bin/impact-map
```

## Upgrading

Check the installed version:

```bash
impact-map version
```

Upgrade to the latest release:

```bash
impact-map upgrade
```
