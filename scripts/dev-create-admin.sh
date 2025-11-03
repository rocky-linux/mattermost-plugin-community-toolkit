#!/bin/bash
#
# dev-create-admin.sh - Create admin user and default team in Mattermost
#
# PURPOSE:
#   Automates the creation of an admin user account and default team in a local
#   Mattermost development instance. This saves manual setup time and ensures
#   a consistent development environment.
#
# USAGE:
#   ./scripts/dev-create-admin.sh
#   make dev-create-admin (recommended)
#
# ENVIRONMENT:
#   MM_ADMIN_USERNAME  - Admin username (default: admin)
#   MM_ADMIN_PASSWORD  - Admin password (default: admin123)
#   MM_ADMIN_EMAIL     - Admin email (default: admin@example.com)
#
# EXIT CODES:
#   0 - Admin account and team created/verified successfully
#   1 - Creation failed (container not running, API errors, etc.)
#
# NOTES:
#   - Idempotent: safe to run multiple times, detects existing user/team
#   - Creates default team "mpct" and adds admin as team admin
#   - Uses Mattermost REST API v4
#   - First user can be created without authentication (Mattermost feature)
#   - Subsequent operations require authentication (token or basic auth)
#

set -euo pipefail

readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Source the helper script to verify container is running and load environment
# shellcheck source=/dev/null
source "${SCRIPT_DIR}/dev-check-container.sh"

cd "${PROJECT_ROOT}"

# Check for required tools
for tool in curl grep awk sed; do
    if ! command -v "${tool}" &>/dev/null; then
        echo "Error: Required tool '${tool}' is not installed" >&2
        exit 1
    fi
done

# Configuration with defaults from environment
readonly ADMIN_USERNAME="${MM_ADMIN_USERNAME:-admin}"
readonly ADMIN_PASSWORD="${MM_ADMIN_PASSWORD:-admin123}"
readonly ADMIN_EMAIL="${MM_ADMIN_EMAIL:-admin@example.com}"
readonly MATTERMOST_URL="http://localhost:8065"
readonly TEAM_NAME="mpct"
readonly TEAM_DISPLAY_NAME="MPCT"

# Authentication token (populated after login)
AUTH_TOKEN=""

# Cleanup function to remove temporary files
cleanup() {
    if [[ -n "${COOKIE_JAR:-}" ]] && [[ -f "${COOKIE_JAR}" ]]; then
        rm -f "${COOKIE_JAR}"
    fi
}

# Register cleanup on script exit
trap cleanup EXIT

echo "Setting up admin account: ${ADMIN_USERNAME} / ${ADMIN_EMAIL}"
echo ""

#------------------------------------------------------------------------------
# Helper Functions
#------------------------------------------------------------------------------

# Extract ID from JSON response
# Mattermost API returns JSON like: {"id":"abc123...", ...}
# We extract the first "id" field value using basic text tools
#
# Args:
#   $1 - JSON string to parse
# Returns:
#   ID value (or empty string if not found)
extract_id() {
    local json="$1"
    echo "${json}" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4
}

# Validate Mattermost ID format
# Mattermost uses 26-character alphanumeric IDs
#
# Args:
#   $1 - ID string to validate
# Returns:
#   0 if valid, 1 if invalid
is_valid_id() {
    local id="$1"
    [[ -n "${id}" ]] && echo "${id}" | grep -qE '^[a-zA-Z0-9]{26}$'
}

# Authenticate and retrieve API token
# Tries multiple methods to extract the auth token from login response
#
# Returns:
#   Auth token string (or empty if login failed)
get_auth_token() {
    # Create temporary file for cookies
    COOKIE_JAR="$(mktemp)"

    # Attempt login and capture full response including headers
    local login_response
    login_response="$(curl -s -i -c "${COOKIE_JAR}" -X POST \
        "${MATTERMOST_URL}/api/v4/users/login" \
        -H "Content-Type: application/json" \
        -d "{\"login_id\":\"${ADMIN_USERNAME}\",\"password\":\"${ADMIN_PASSWORD}\"}" 2>&1 || true)"

    # Method 1: Extract from Token header (preferred)
    local token
    token="$(echo "${login_response}" | grep -i "^Token:" | sed 's/^Token: //' | tr -d '\r\n')"

    # Method 2: Extract from Set-Cookie header
    if [[ -z "${token}" ]]; then
        token="$(echo "${login_response}" | grep -i "^Set-Cookie:" | \
                 grep -o "MMAUTHTOKEN=[^;]*" | sed 's/MMAUTHTOKEN=//' | head -1)"
    fi

    # Method 3: Read from cookie jar file
    if [[ -z "${token}" ]] && [[ -f "${COOKIE_JAR}" ]]; then
        token="$(grep -i "MMAUTHTOKEN" "${COOKIE_JAR}" | awk '{print $NF}' | head -1)"
    fi

    echo "${token}"
}

# Make authenticated API call to Mattermost
# Supports both token and basic authentication
#
# Args:
#   $1 - HTTP method (GET, POST, PUT, etc.)
#   $2 - Full URL to call
#   $3 - Optional JSON data for request body
# Returns:
#   API response body
api_call() {
    local method="${1:-GET}"
    local url="$2"
    local data="${3:-}"

    # Build curl command based on auth method and whether we have data
    if [[ -n "${AUTH_TOKEN}" ]]; then
        # Use token authentication (preferred)
        if [[ -n "${data}" ]]; then
            curl -s -X "${method}" \
                -H "Authorization: Bearer ${AUTH_TOKEN}" \
                -H "Content-Type: application/json" \
                -d "${data}" \
                "${url}"
        else
            curl -s -X "${method}" \
                -H "Authorization: Bearer ${AUTH_TOKEN}" \
                "${url}"
        fi
    else
        # Fall back to basic authentication
        if [[ -n "${data}" ]]; then
            curl -s -X "${method}" \
                -u "${ADMIN_USERNAME}:${ADMIN_PASSWORD}" \
                -H "Content-Type: application/json" \
                -d "${data}" \
                "${url}"
        else
            curl -s -X "${method}" \
                -u "${ADMIN_USERNAME}:${ADMIN_PASSWORD}" \
                "${url}"
        fi
    fi
}

#------------------------------------------------------------------------------
# Main Script Logic
#------------------------------------------------------------------------------

# Step 1: Create admin user
# First user in Mattermost can be created without authentication
echo "Creating admin user..."
user_create_response="$(curl -s -X POST "${MATTERMOST_URL}/api/v4/users" \
    -H "Content-Type: application/json" \
    -d "{\"email\":\"${ADMIN_EMAIL}\",\"username\":\"${ADMIN_USERNAME}\",\"password\":\"${ADMIN_PASSWORD}\",\"allow_marketing\":false}" \
    2>&1 || true)"

# Determine if creation succeeded, user already exists, or failed
USER_ID=""
if echo "${user_create_response}" | grep -q "\"username\":\"${ADMIN_USERNAME}\""; then
    echo "✓ Admin account created successfully"
    USER_ID="$(extract_id "${user_create_response}")"
    # Allow time for user to be fully processed in the system
    sleep 2
elif echo "${user_create_response}" | grep -qiE "already exists|already registered|unique constraint"; then
    echo "✓ Admin account already exists"
    # Retrieve user ID via API lookup
    user_response="$(api_call GET "${MATTERMOST_URL}/api/v4/users/username/${ADMIN_USERNAME}")"
    USER_ID="$(extract_id "${user_response}")"
else
    echo "Error: Failed to create admin account" >&2
    echo "Response: ${user_create_response}" >&2
    exit 1
fi

# Validate we have a user ID before proceeding
if [[ -z "${USER_ID}" ]]; then
    echo "Error: Could not determine user ID" >&2
    exit 1
fi

# Step 2: Authenticate for subsequent operations
echo "Authenticating..."
AUTH_TOKEN="$(get_auth_token)"

# Retry authentication if it fails (API may need a moment)
readonly MAX_RETRIES=3
retry_count=0
while [[ -z "${AUTH_TOKEN}" ]] && (( retry_count < MAX_RETRIES )); do
    retry_count=$((retry_count + 1))
    if (( retry_count < MAX_RETRIES )); then
        echo "  Retry ${retry_count}/${MAX_RETRIES}..."
        sleep 2
        AUTH_TOKEN="$(get_auth_token)"
    fi
done

# Warn if authentication failed, but continue with basic auth fallback
if [[ -z "${AUTH_TOKEN}" ]]; then
    echo "Warning: Failed to authenticate after ${MAX_RETRIES} attempts" >&2
    echo "  Will attempt basic authentication for remaining operations" >&2
else
    echo "✓ Authentication successful"
fi

echo ""
echo "Setting up default team..."

# Step 3: Check if team exists or create it
team_response="$(api_call GET "${MATTERMOST_URL}/api/v4/teams/name/${TEAM_NAME}" 2>&1 || true)"

TEAM_ID=""
# Validate response contains team name (indicates successful lookup)
if echo "${team_response}" | grep -q "\"name\":\"${TEAM_NAME}\""; then
    TEAM_ID="$(extract_id "${team_response}")"
    if is_valid_id "${TEAM_ID}"; then
        echo "✓ Team '${TEAM_NAME}' already exists"
    else
        # Invalid ID format, treat as not found
        TEAM_ID=""
    fi
fi

# Create team if it doesn't exist
if [[ -z "${TEAM_ID}" ]]; then
    echo "Creating team '${TEAM_NAME}'..."
    team_create_response="$(api_call POST "${MATTERMOST_URL}/api/v4/teams" \
        "{\"name\":\"${TEAM_NAME}\",\"display_name\":\"${TEAM_DISPLAY_NAME}\",\"type\":\"O\"}")"

    if echo "${team_create_response}" | grep -q "\"name\":\"${TEAM_NAME}\""; then
        # Successfully created - extract and validate ID
        TEAM_ID="$(extract_id "${team_create_response}")"
        if is_valid_id "${TEAM_ID}"; then
            echo "✓ Team '${TEAM_NAME}' created successfully"
        else
            echo "Error: Failed to extract valid team ID from response" >&2
            echo "Response: ${team_create_response}" >&2
            exit 1
        fi
    elif echo "${team_create_response}" | grep -qiE "already exists|already registered"; then
        # Race condition: team was created between our check and creation attempt
        echo "✓ Team '${TEAM_NAME}' already exists"
        team_response="$(api_call GET "${MATTERMOST_URL}/api/v4/teams/name/${TEAM_NAME}")"
        if echo "${team_response}" | grep -q "\"name\":\"${TEAM_NAME}\""; then
            TEAM_ID="$(extract_id "${team_response}")"
            if [[ -z "${TEAM_ID}" ]] || ! is_valid_id "${TEAM_ID}"; then
                echo "Error: Failed to extract valid team ID" >&2
                echo "Response: ${team_response}" >&2
                exit 1
            fi
        else
            echo "Error: Failed to retrieve team after creation" >&2
            echo "Response: ${team_response}" >&2
            exit 1
        fi
    else
        echo "Error: Failed to create team" >&2
        echo "Response: ${team_create_response}" >&2
        echo "Note: Admin user may need system admin privileges to create teams" >&2
        exit 1
    fi
fi

# Final validation that we have a team ID
if [[ -z "${TEAM_ID}" ]]; then
    echo "Error: Could not determine team ID" >&2
    exit 1
fi

# Step 4: Add user to team (if not already a member)
member_check_response="$(api_call GET "${MATTERMOST_URL}/api/v4/teams/${TEAM_ID}/members/${USER_ID}" 2>&1 || true)"

if echo "${member_check_response}" | grep -q '"user_id"'; then
    echo "✓ Admin user is already a member of team '${TEAM_NAME}'"
else
    echo "Adding admin user to team '${TEAM_NAME}'..."
    member_response="$(api_call POST "${MATTERMOST_URL}/api/v4/teams/${TEAM_ID}/members" \
        "{\"team_id\":\"${TEAM_ID}\",\"user_id\":\"${USER_ID}\"}")"

    if echo "${member_response}" | grep -q '"user_id"'; then
        echo "✓ Admin user added to team '${TEAM_NAME}'"
    elif echo "${member_response}" | grep -qiE "already exists|already a member"; then
        # Race condition: user was added between check and addition
        echo "✓ Admin user is already a member of team '${TEAM_NAME}'"
    else
        echo "Error: Failed to add admin user to team" >&2
        echo "Response: ${member_response}" >&2
        echo "Note: Admin user may need system admin privileges to add members" >&2
        exit 1
    fi
fi

# Step 5: Set user as team admin
echo "Setting admin user as team admin..."
role_update_response="$(api_call PUT \
    "${MATTERMOST_URL}/api/v4/teams/${TEAM_ID}/members/${USER_ID}/roles" \
    '{"roles":"team_admin team_user"}')"

# API returns {"status":"OK"} on success (case may vary)
if echo "${role_update_response}" | grep -qiE '"status":"ok"'; then
    echo "✓ Admin user set as team admin"
elif echo "${role_update_response}" | grep -qiE "error|failed"; then
    # Not fatal - user can still use Mattermost, just won't have team admin privileges
    echo "Warning: Failed to set team admin role" >&2
    echo "Response: ${role_update_response}" >&2
else
    # Other status values might indicate success (e.g., "success")
    if echo "${role_update_response}" | grep -qi '"status"'; then
        echo "✓ Admin user set as team admin"
    else
        echo "Warning: Unexpected response when setting team admin role" >&2
        echo "Response: ${role_update_response}" >&2
    fi
fi

echo ""
echo "======================================================================"
echo "✓ Admin account and team setup complete!"
echo "======================================================================"
echo ""
echo "Login credentials:"
echo "  Username: ${ADMIN_USERNAME}"
echo "  Email:    ${ADMIN_EMAIL}"
echo "  Password: ${ADMIN_PASSWORD}"
echo ""
echo "Team: ${TEAM_DISPLAY_NAME} (${TEAM_NAME})"
echo ""
echo "Login at: ${MATTERMOST_URL}/login"
echo ""
