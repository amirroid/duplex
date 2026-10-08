#!/usr/bin/env bash
set -euo pipefail

# duplex installation script
# Cross-platform installer for macOS and Linux

APP_NAME="duplex"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

echo "==> Installing ${APP_NAME}..."

# 1. Build binary if Go is available, or use existing binary
if command -v go >/dev/null 2>&1; then
    echo "==> Building ${APP_NAME} using Go toolchain..."
    cd "${ROOT_DIR}"
    GOPROXY="https://proxy.golang.org,direct" GOSUMDB=off go build -ldflags="-s -w" -o "${ROOT_DIR}/${APP_NAME}" cmd/duplex/main.go
elif [[ -f "${ROOT_DIR}/${APP_NAME}" ]]; then
    echo "==> Using pre-built ${APP_NAME} binary..."
else
    echo "Error: Neither 'go' compiler nor pre-built '${APP_NAME}' binary was found." >&2
    exit 1
fi

# 2. Determine installation destination
TARGET_DIR=""
if [[ -w "/usr/local/bin" ]]; then
    TARGET_DIR="/usr/local/bin"
elif [[ $EUID -eq 0 ]]; then
    TARGET_DIR="/usr/local/bin"
else
    # Non-root user without write access to /usr/local/bin -> install to ~/.local/bin
    TARGET_DIR="${HOME}/.local/bin"
    mkdir -p "${TARGET_DIR}"
fi

TARGET_BIN="${TARGET_DIR}/${APP_NAME}"

echo "==> Installing binary to ${TARGET_BIN}..."
cp -f "${ROOT_DIR}/${APP_NAME}" "${TARGET_BIN}"
chmod 755 "${TARGET_BIN}"

# 3. Ensure TARGET_DIR is on PATH
PATH_CHECK=0
IFS=':' read -ra PATH_ARRAY <<< "$PATH"
for p in "${PATH_ARRAY[@]}"; do
    if [[ "$p" == "${TARGET_DIR}" ]]; then
        PATH_CHECK=1
        break
    fi
done

if [[ ${PATH_CHECK} -eq 0 ]]; then
    echo "==> Adding ${TARGET_DIR} to your shell PATH..."
    SHELL_NAME="$(basename "${SHELL:-/bin/bash}")"
    RC_FILE=""
    if [[ "${SHELL_NAME}" == "zsh" ]]; then
        RC_FILE="${HOME}/.zshrc"
    else
        RC_FILE="${HOME}/.bashrc"
    fi

    if ! grep -q "${TARGET_DIR}" "${RC_FILE}" 2>/dev/null; then
        echo "export PATH=\"${TARGET_DIR}:\$PATH\"" >> "${RC_FILE}"
        echo "==> Appended export PATH to ${RC_FILE}"
    fi
fi

# 4. Verify installation
echo "==> Verifying installation..."
if "${TARGET_BIN}" --version; then
    echo ""
    echo "🎉 Successfully installed ${APP_NAME}!"
    echo "You can now run '${APP_NAME} <file.pdf>' from any directory."
else
    echo "Warning: Verification failed." >&2
fi
