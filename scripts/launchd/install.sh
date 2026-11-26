#!/usr/bin/env bash
set -euo pipefail

LABEL="com.tranthethang.dss-ui"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
TMPL_PATH="${SCRIPT_DIR}/${LABEL}.plist.tmpl"

REPO_PATH="${DSS_REPO_PATH:-}"
ADDR="127.0.0.1:8787"
SOURCE_BIN="${PROJECT_ROOT}/bin/dashboard"
INSTALL_BIN="${HOME}/.local/bin/dss-ui"
PLIST_DIR="${HOME}/Library/LaunchAgents"
PLIST_PATH="${PLIST_DIR}/${LABEL}.plist"
LOG_DIR="${HOME}/Library/Logs"
LOG_PATH="${LOG_DIR}/dss-ui.log"

usage() {
  cat <<EOF
Usage: $(basename "$0") --repo <path> [--addr <host:port>] [--bin <path>]

Install and start the macOS launchd user agent (${LABEL}).
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --repo)
      REPO_PATH="${2:-}"
      shift 2
      ;;
    --addr)
      ADDR="${2:-}"
      shift 2
      ;;
    --bin)
      SOURCE_BIN="${2:-}"
      shift 2
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

if [[ -z "${REPO_PATH}" ]]; then
  echo "Error: --repo <path> or DSS_REPO_PATH is required." >&2
  exit 1
fi

if [[ ! -d "${REPO_PATH}" ]]; then
  echo "Error: repository directory does not exist: ${REPO_PATH}" >&2
  exit 1
fi
REPO_PATH="$(cd "${REPO_PATH}" && pwd)"

if [[ ! -f "${SOURCE_BIN}" ]]; then
  echo "Error: dashboard binary not found at ${SOURCE_BIN}. Run 'make build' first." >&2
  exit 1
fi

if [[ ! -f "${TMPL_PATH}" ]]; then
  echo "Error: plist template not found at ${TMPL_PATH}." >&2
  exit 1
fi

mkdir -p "$(dirname "${INSTALL_BIN}")" "${PLIST_DIR}" "${LOG_DIR}"
cp "${SOURCE_BIN}" "${INSTALL_BIN}"
chmod 755 "${INSTALL_BIN}"

escape_sed() {
  printf '%s' "$1" | sed -e 's/[&|\\]/\\&/g'
}

BIN_ESC="$(escape_sed "${INSTALL_BIN}")"
REPO_ESC="$(escape_sed "${REPO_PATH}")"
ADDR_ESC="$(escape_sed "${ADDR}")"
LOG_ESC="$(escape_sed "${LOG_PATH}")"

sed \
  -e "s|__BIN_PATH__|${BIN_ESC}|g" \
  -e "s|__REPO_PATH__|${REPO_ESC}|g" \
  -e "s|__ADDR__|${ADDR_ESC}|g" \
  -e "s|__LOG_PATH__|${LOG_ESC}|g" \
  "${TMPL_PATH}" > "${PLIST_PATH}"

plutil -lint "${PLIST_PATH}" >/dev/null

GUI_DOMAIN="gui/$(id -u)"
launchctl bootout "${GUI_DOMAIN}/${LABEL}" >/dev/null 2>&1 || true
launchctl bootstrap "${GUI_DOMAIN}" "${PLIST_PATH}"

echo "Installed ${LABEL}:"
echo "  Binary: ${INSTALL_BIN}"
echo "  Plist:  ${PLIST_PATH}"
echo "  Log:    ${LOG_PATH}"
echo "  URL:    http://${ADDR}"
