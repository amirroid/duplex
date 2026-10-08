#!/usr/bin/env bash
set -euo pipefail

# duplex uninstallation script

APP_NAME="duplex"

echo "==> Uninstalling ${APP_NAME}..."

LOCATIONS=(
    "/usr/local/bin/${APP_NAME}"
    "${HOME}/.local/bin/${APP_NAME}"
)

REMOVED=0
for loc in "${LOCATIONS[@]}"; do
    if [[ -f "${loc}" ]]; then
        echo "Removing ${loc}..."
        rm -f "${loc}"
        REMOVED=1
    fi
done

if [[ ${REMOVED} -eq 1 ]]; then
    echo "✓ ${APP_NAME} has been completely uninstalled."
else
    echo "No installed ${APP_NAME} binary found in standard locations."
fi
