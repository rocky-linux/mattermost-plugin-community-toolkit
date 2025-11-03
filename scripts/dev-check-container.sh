#!/bin/bash
#
# dev-check-container.sh - Verify Mattermost development container status
#
# PURPOSE:
#   This script ensures the Mattermost development container is running before
#   other scripts attempt to interact with it. It also loads environment variables
#   from the podman/.env file to make credentials and configuration available to
#   calling scripts.
#
# USAGE:
#   This script is typically sourced by other development scripts:
#       source scripts/dev-check-container.sh
#
# ENVIRONMENT:
#   PODMAN_COMPOSE_FILE - Path to podman-compose file (default: podman-compose.yml)
#
# EXIT CODES:
#   0 - Container is running and ready
#   1 - Container is not running or dependencies missing
#
# NOTES:
#   - This script changes to PROJECT_ROOT directory
#   - Environment variables from podman/.env are exported to calling shell
#   - Use 'set -a' pattern to automatically export variables when sourcing .env
#

set -euo pipefail

# Determine script and project directories
# Only set as readonly if not already defined (allows sourcing by other scripts)
if [[ -z "${SCRIPT_DIR:-}" ]]; then
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    readonly SCRIPT_DIR
fi

if [[ -z "${PROJECT_ROOT:-}" ]]; then
    PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
    readonly PROJECT_ROOT
fi

if [[ -z "${PODMAN_COMPOSE_FILE:-}" ]]; then
    PODMAN_COMPOSE_FILE="podman-compose.yml"
    readonly PODMAN_COMPOSE_FILE
fi

# Check for required tools
if ! command -v podman-compose &>/dev/null; then
    echo "Error: podman-compose is not installed or not in PATH" >&2
    echo "Install it with: pip install podman-compose" >&2
    exit 1
fi

# Change to project root for consistent behavior
cd "${PROJECT_ROOT}"

# Verify Mattermost container is running
# We check for "Up" or "starting" status in the podman-compose output
if ! podman-compose -f "${PODMAN_COMPOSE_FILE}" ps | grep -qE "mattermost.*(Up|starting)"; then
    echo "Error: Mattermost container is not running" >&2
    echo "Start it with: make dev-up" >&2
    exit 1
fi

# Load environment variables from podman/.env if present
# Using 'set -a' causes all variables to be exported automatically
readonly ENV_FILE="podman/.env"
if [[ -f "${ENV_FILE}" ]]; then
    set -a
    # shellcheck source=/dev/null
    source "${ENV_FILE}"
    set +a
fi

