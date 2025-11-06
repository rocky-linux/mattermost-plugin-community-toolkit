#!/bin/bash
#
# dev-deploy.sh - Deploy plugin to local Mattermost development environment
#
# PURPOSE:
#   Builds and deploys the Mattermost plugin to a running Podman development
#   stack. This script locates the latest plugin bundle and uses pluginctl
#   to install it on the local Mattermost instance.
#
# USAGE:
#   ./scripts/dev-deploy.sh
#   make deploy (recommended - ensures build happens first)
#
# ENVIRONMENT:
#   PLUGIN_ID           - Plugin identifier (default: mattermost-community-toolkit)
#   BUNDLE_NAME         - Path to plugin bundle, or auto-detect latest in dist/
#   MM_ADMIN_TOKEN      - Admin auth token (preferred)
#   MM_ADMIN_USERNAME   - Admin username (fallback auth method)
#   MM_ADMIN_PASSWORD   - Admin password (used with username)
#
# EXIT CODES:
#   0 - Plugin deployed successfully
#   1 - Deployment failed (missing dependencies, container not running, etc.)
#
# NOTES:
#   - Requires Mattermost container to be running (checked via dev-check-container.sh)
#   - Authentication credentials should be set in podman/.env
#   - After deployment, plugin must be enabled via 'make dev-enable' or System Console
#

set -euo pipefail

readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Source the helper script to verify container is running and load environment
# shellcheck source=/dev/null
source "${SCRIPT_DIR}/dev-check-container.sh"

cd "${PROJECT_ROOT}"

# Check for required tools
if [[ ! -x "./build/bin/pluginctl" ]]; then
    echo "Error: pluginctl not found at ./build/bin/pluginctl" >&2
    echo "Build the project first with: make all" >&2
    exit 1
fi

echo "Deploying plugin to development environment..."

# Validate deployment credentials are configured
if [[ -z "${MM_ADMIN_TOKEN:-}" ]] && [[ -z "${MM_ADMIN_USERNAME:-}" ]]; then
    echo "Warning: No authentication credentials configured" >&2
    echo "  Set MM_ADMIN_TOKEN or MM_ADMIN_USERNAME/MM_ADMIN_PASSWORD" >&2
    echo "  in podman/.env (see podman/env.example for template)" >&2
    echo "" >&2
    echo "Deployment may fail without proper authentication." >&2
    # Continue anyway - pluginctl will fail with a clear error if auth is required
fi

# Get plugin configuration from environment or use defaults
readonly PLUGIN_ID="${PLUGIN_ID:-mattermost-community-toolkit}"
BUNDLE_NAME="${BUNDLE_NAME:-}"

# Auto-detect latest bundle if not explicitly specified
if [[ -z "${BUNDLE_NAME}" ]]; then
    # Find most recently modified .tar.gz file in dist/
    BUNDLE_NAME="$(find dist -maxdepth 1 -name "*.tar.gz" -type f -printf "%T@ %p\n" 2>/dev/null | \
                   sort -rn | head -1 | cut -d' ' -f2-)"

    if [[ -z "${BUNDLE_NAME}" ]]; then
        echo "Error: No plugin bundle found in dist/" >&2
        echo "Build the plugin first with: make dist" >&2
        exit 1
    fi

    echo "Using bundle: ${BUNDLE_NAME}"
fi

# Verify bundle file exists and is readable
if [[ ! -f "${BUNDLE_NAME}" ]]; then
    echo "Error: Bundle file not found: ${BUNDLE_NAME}" >&2
    exit 1
fi

if [[ ! -r "${BUNDLE_NAME}" ]]; then
    echo "Error: Bundle file not readable: ${BUNDLE_NAME}" >&2
    exit 1
fi

# Deploy the plugin using pluginctl
# MM_SERVICESETTINGS_SITEURL tells pluginctl where to find the Mattermost API
MM_SERVICESETTINGS_SITEURL="http://localhost:8065" \
    ./build/bin/pluginctl deploy "${PLUGIN_ID}" "${BUNDLE_NAME}"

echo ""
echo "Plugin deployed successfully!"
echo "Next steps:"
echo "  - Enable: make dev-enable"
echo "  - Or enable via System Console at http://localhost:8065"

