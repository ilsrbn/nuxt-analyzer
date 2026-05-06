#!/usr/bin/env bash
set -euo pipefail

REPO="ilsrbn/nuxt-analyzer"
BINARY="impact-map"
INSTALL_DIR="${INSTALL_DIR:-"$HOME/.local/bin"}"
VERSION="${VERSION:-}"
TMP_DIR=""

err() {
  echo "error: $*" >&2
  exit 1
}

info() {
  echo "==> $*"
}

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || err "required command not found: $1"
}

detect_os() {
  case "$(uname -s)" in
    Darwin) echo "darwin" ;;
    Linux) echo "linux" ;;
    *) err "unsupported OS: $(uname -s). Supported: macOS and Linux" ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo "amd64" ;;
    arm64|aarch64) echo "arm64" ;;
    *) err "unsupported architecture: $(uname -m). Supported: amd64 and arm64" ;;
  esac
}

latest_version() {
  local response version

  response="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest")" \
    || err "failed to fetch latest release from GitHub"

  version="$(printf '%s\n' "$response" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)"
  [ -n "$version" ] || err "could not determine latest release version"

  printf '%s\n' "$version"
}

path_contains() {
  case ":${PATH:-}:" in
    *":$1:"*) return 0 ;;
    *) return 1 ;;
  esac
}

shell_config() {
  local shell_name

  shell_name="$(basename "${SHELL:-}")"
  case "$shell_name" in
    zsh) printf '%s\n' "$HOME/.zshrc" ;;
    bash)
      if [ -f "$HOME/.bashrc" ]; then
        printf '%s\n' "$HOME/.bashrc"
      else
        printf '%s\n' "$HOME/.bash_profile"
      fi
      ;;
    fish) printf '%s\n' "$HOME/.config/fish/config.fish" ;;
    *) printf '%s\n' "" ;;
  esac
}

path_line_for_shell() {
  local config="$1"

  case "$config" in
    *.fish)
      if [ "$INSTALL_DIR" = "$HOME/.local/bin" ]; then
        printf '%s\n' "fish_add_path \$HOME/.local/bin"
      else
        printf '%s\n' "fish_add_path $INSTALL_DIR"
      fi
      ;;
    *)
      if [ "$INSTALL_DIR" = "$HOME/.local/bin" ]; then
        printf '%s\n' "export PATH=\"\$HOME/.local/bin:\$PATH\""
      else
        printf '%s\n' "export PATH=\"$INSTALL_DIR:\$PATH\""
      fi
      ;;
  esac
}

append_path_if_needed() {
  local config line dir

  config="$(shell_config)"
  [ -n "$config" ] || {
    info "Could not detect a supported shell config. Add this to your PATH manually:"
    echo "  $INSTALL_DIR"
    return
  }

  line="$(path_line_for_shell "$config")"
  dir="$(dirname "$config")"
  mkdir -p "$dir"
  touch "$config"

  if grep -Fqx "$line" "$config" || grep -Fq "$INSTALL_DIR" "$config"; then
    info "PATH entry already exists in $config"
  elif [ "$INSTALL_DIR" = "$HOME/.local/bin" ] && grep -Fq '$HOME/.local/bin' "$config"; then
    info "PATH entry already exists in $config"
  else
    {
      printf '\n'
      printf '%s\n' "$line"
    } >>"$config"
    info "Added $INSTALL_DIR to PATH in $config"
  fi

  info "Restart your terminal or source $config to update your PATH."
}

prompt_add_path() {
  local answer

  if [ -r /dev/tty ] && [ -w /dev/tty ]; then
    printf 'Add %s to PATH? [y/N] ' "$INSTALL_DIR" >/dev/tty
    read -r answer </dev/tty || answer=""
  elif [ -t 0 ]; then
    printf 'Add %s to PATH? [y/N] ' "$INSTALL_DIR"
    read -r answer || answer=""
  else
    info "$INSTALL_DIR is not in PATH."
    info "Skipping PATH prompt because no interactive terminal is available."
    echo "Installed binary: $INSTALL_DIR/$BINARY"
    return
  fi

  case "$answer" in
    y|Y|yes|YES) append_path_if_needed ;;
    *) echo "Installed binary: $INSTALL_DIR/$BINARY" ;;
  esac
}

find_binary() {
  local dir="$1"
  local candidate

  if [ -f "$dir/$BINARY" ]; then
    printf '%s\n' "$dir/$BINARY"
    return
  fi

  candidate="$(find "$dir" -type f -name "$BINARY" -perm -u+x -print -quit)"
  if [ -z "$candidate" ]; then
    candidate="$(find "$dir" -type f -name "$BINARY" -print -quit)"
  fi

  [ -n "$candidate" ] || err "archive did not contain $BINARY"
  printf '%s\n' "$candidate"
}

cleanup() {
  if [ -n "$TMP_DIR" ]; then
    rm -rf "$TMP_DIR"
  fi
}

main() {
  local os arch version url archive extracted_binary installed_path

  need_cmd curl
  need_cmd tar
  need_cmd uname
  need_cmd sed
  need_cmd find

  os="$(detect_os)"
  arch="$(detect_arch)"
  version="$VERSION"

  if [ -z "$version" ]; then
    info "Fetching latest release version"
    version="$(latest_version)"
  fi

  url="https://github.com/${REPO}/releases/download/${version}/${BINARY}_${os}_${arch}.tar.gz"
  TMP_DIR="$(mktemp -d)"
  archive="$TMP_DIR/${BINARY}.tar.gz"
  installed_path="$INSTALL_DIR/$BINARY"
  trap cleanup EXIT

  info "Installing $BINARY $version for $os/$arch"
  info "Downloading $url"
  curl -fL "$url" -o "$archive" || err "failed to download release archive: $url"

  info "Extracting archive"
  tar -xzf "$archive" -C "$TMP_DIR" || err "failed to extract archive"
  extracted_binary="$(find_binary "$TMP_DIR")"

  info "Installing to $installed_path"
  mkdir -p "$INSTALL_DIR"
  mv "$extracted_binary" "$installed_path" || err "failed to install binary to $installed_path"
  chmod +x "$installed_path"

  if path_contains "$INSTALL_DIR"; then
    info "$INSTALL_DIR is already in PATH"
  else
    prompt_add_path
  fi

  echo
  info "Installed path: $installed_path"
  info "Try: $BINARY --help"
}

main "$@"
