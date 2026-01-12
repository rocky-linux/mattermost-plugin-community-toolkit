#!/bin/bash
#
# dev-pluginctl.sh - Execute pluginctl commands in development environment
#
# PURPOSE:
#   Centralized script for running pluginctl commands against a local Mattermost
#   development environment. This script handles environment setup, container
#   verification, and credential management for all pluginctl operations.
#
# USAGE:
#   ./build/scripts/dev-pluginctl.sh <command>
#
#   Where <command> is one of: enable, disable, reset, logs, logs-watch
#
# EXAMPLES:
#   ./build/scripts/dev-pluginctl.sh enable      # Enable the plugin
#   ./build/scripts/dev-pluginctl.sh disable     # Disable the plugin
#   ./build/scripts/dev-pluginctl.sh reset       # Reset (disable and re-enable) the plugin
#   ./build/scripts/dev-pluginctl.sh logs        # View plugin logs
#   ./build/scripts/dev-pluginctl.sh logs-watch  # Tail plugin logs in real-time
#
# ENVIRONMENT:
#   PLUGIN_ID               - Plugin identifier (default: mattermost-community-toolkit)
#   MM_SERVICESETTINGS_SITEURL - Mattermost server URL (default: http://localhost:8065)
#   MM_ADMIN_TOKEN          - Admin auth token (preferred)
#   MM_ADMIN_USERNAME       - Admin username (fallback auth method)
#   MM_ADMIN_PASSWORD       - Admin password (used with username)
#
# EXIT CODES:
#   0 - Command executed successfully
#   1 - Command failed (invalid command, missing dependencies, container not running, etc.)
#
# NOTES:
#   - Requires Mattermost container to be running (checked via dev-check-container.sh)
#   - Authentication credentials should be set in podman/.env
#   - This script is typically called via Makefile targets (dev-enable, dev-disable, etc.)
#

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)" # split to two lines for SC2155
readonly SCRIPT_DIR
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)" # split to two lines for SC2155
readonly PROJECT_ROOT

# Source the helper script to verify container is running and load environment
# shellcheck source=/dev/null
source "${SCRIPT_DIR}/dev-check-container.sh"

cd "${PROJECT_ROOT}"

# Validate command argument
if [[ $# -ne 1 ]]; then
    echo "Error: Missing command argument" >&2
    echo "Usage: $0 <command>" >&2
    echo "  Commands: enable, disable, reset, logs, logs-watch" >&2
    exit 1
fi

readonly COMMAND="$1"

# Validate command is one of the supported commands
case "${COMMAND}" in
    enable|disable|reset|logs|logs-watch)
        # Valid command, continue
        ;;
    *)
        echo "Error: Invalid command: ${COMMAND}" >&2
        echo "Valid commands: enable, disable, reset, logs, logs-watch" >&2
        exit 1
        ;;
esac

# Check for required tools
if [[ ! -x "./build/bin/pluginctl" ]]; then
    echo "Error: pluginctl not found at ./build/bin/pluginctl" >&2
    echo "Build the project first with: make all" >&2
    exit 1
fi

# Get plugin configuration from environment or use defaults
readonly PLUGIN_ID="${PLUGIN_ID:-mattermost-community-toolkit}"
readonly SITE_URL="${MM_SERVICESETTINGS_SITEURL:-http://localhost:8065}"

# Execute the pluginctl command
MM_SERVICESETTINGS_SITEURL="${SITE_URL}" \
    ./build/bin/pluginctl "${COMMAND}" "${PLUGIN_ID}"

