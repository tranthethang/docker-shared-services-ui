#!/usr/bin/env bash
set -euo pipefail

LABEL="com.tranthethang.dss-ui"
PLIST_PATH="${HOME}/Library/LaunchAgents/${LABEL}.plist"
INSTALL_BIN="${HOME}/.local/bin/dss-ui"
REMOVE_BIN=false

usage() {
  cat <<EOF
Usage: $(basename "$0") [--remove-bin]

Unload and remove the macOS launchd user agent (${LABEL}).
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --remove-bin)
      REMOVE_BIN=true
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

GUI_DOMAIN="gui/$(id -u)"
launchctl bootout "${GUI_DOMAIN}/${LABEL}" >/dev/null 2>&1 || true

if [[ -f "${PLIST_PATH}" ]]; then
  rm -f "${PLIST_PATH}"
  echo "Removed ${PLIST_PATH}"
else
  echo "Plist already absent: ${PLIST_PATH}"
fi

if [[ "${REMOVE_BIN}" == "true" ]] && [[ -f "${INSTALL_BIN}" ]]; then
  rm -f "${INSTALL_BIN}"
  echo "Removed ${INSTALL_BIN}"
fi

echo "Uninstalled ${LABEL}."
