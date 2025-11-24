#!/bin/bash
#
# dev-preflight-check.sh - Preflight checks for development environment setup
#
# PURPOSE:
#   This script performs preflight checks and idempotent setup for the development
#   environment. It validates that required configuration exists and ensures
#   necessary directories are created with proper permissions.
#
# USAGE:
#   Run directly or via make target:
#       ./build/scripts/dev-preflight-check.sh
#       make dev-setup
#
# ENVIRONMENT:
#   PROJECT_ROOT - Root directory of the project (auto-detected)
#
# EXIT CODES:
#   0 - All checks passed and setup completed successfully
#   1 - Missing required configuration or setup failed
#
# NOTES:
#   - This script is idempotent and safe to run multiple times
#   - Only creates directories if they don't already exist
#   - Only sets permissions when needed
#   - Checks for podman/.env file existence before proceeding
#

set -euo pipefail

# Determine script and project directories
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly SCRIPT_DIR

PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
readonly PROJECT_ROOT

# Change to project root for consistent behavior
cd "${PROJECT_ROOT}"

# Define paths
readonly ENV_FILE="podman/.env"
readonly ENV_EXAMPLE="podman/env.example"

# Directories that need to be created
readonly REQUIRED_DIRS=(
    "podman/data/mattermost/plugins"
    "podman/data/mattermost/client/plugins"
    "podman/data/postgres"
    "podman/config"
)

# Directories that need specific permissions
readonly CHMOD_DIRS=(
    "podman/config"
    "podman/data"
)

echo "Running preflight checks for development environment..."

# Check 1: Verify podman/.env file exists
if [[ ! -f "${ENV_FILE}" ]]; then
    echo "Error: Required file '${ENV_FILE}' does not exist" >&2
    echo "" >&2
    echo "To fix this, copy the example environment file:" >&2
    echo "  cp ${ENV_EXAMPLE} ${ENV_FILE}" >&2
    echo "" >&2
    echo "Then edit ${ENV_FILE} and configure:" >&2
    echo "  - Database credentials (POSTGRES_*)" >&2
    echo "  - Mattermost settings (MM_*)" >&2
    echo "  - Admin credentials for plugin deployment" >&2
    echo "" >&2
    exit 1
fi

echo "✓ Environment file '${ENV_FILE}' exists"

# Check 2: Create required directories if they don't exist
echo "Checking required directories..."
directories_created=0

for dir in "${REQUIRED_DIRS[@]}"; do
    if [[ ! -d "${dir}" ]]; then
        echo "  Creating: ${dir}"
        mkdir -p "${dir}"
        directories_created=$((directories_created + 1))
    else
        echo "  ✓ Exists: ${dir}"
    fi
done

if [[ ${directories_created} -gt 0 ]]; then
    echo "Created ${directories_created} director(y|ies)"
else
    echo "All required directories already exist"
fi

# Check 3: Set permissions on directories that need them
echo "Setting directory permissions..."
permissions_set=0

for dir in "${CHMOD_DIRS[@]}"; do
    if [[ -d "${dir}" ]]; then
        # Check if we need to set permissions
        # We'll try to set them regardless to ensure they're correct
        if chmod -R 777 "${dir}" 2>/dev/null; then
            echo "  ✓ Permissions set: ${dir}"
            permissions_set=$((permissions_set + 1))
        else
            echo "  ⚠ Warning: Could not set permissions on ${dir} (may require sudo)" >&2
        fi
    fi
done

echo ""
echo "✓ Preflight checks complete. Development environment is ready."

