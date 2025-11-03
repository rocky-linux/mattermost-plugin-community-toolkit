# Podman Development Environment

This document describes how to use the Podman Compose-based development environment for testing the Mattermost Community Toolkit plugin locally.

## Overview

The development environment provides a complete, isolated Mattermost instance with PostgreSQL, suitable for testing plugin functionality without affecting a production server.

## Prerequisites

- Podman and Podman Compose installed
- Make utility (standard on Linux/macOS)
- Network access to pull container images

## Quick Start

1. **Start the development environment:**

   ```bash
   make dev-up
   ```

2. **Wait for Mattermost to be ready:**
   The `dev-up` target will wait up to 60 seconds for Mattermost to become ready. You can access it at http://localhost:8065

3. **First-time setup:**
   - Open http://localhost:8065 in your browser
   - Create your admin account (or use the credentials from `podman/env.example`)
   - Complete the initial setup wizard

4. **Configure plugin deployment credentials:**

   You need to set environment variables for plugin deployment. You can either:

   **Option A: Use an admin token (recommended)**

   ```bash
   export MM_ADMIN_TOKEN="your-admin-token-here"
   ```
  
   To get a token:

   - Log into Mattermost
   - Go to Menu > System Console > Integrations > Bot Accounts
   - Choose to enable bot accounts
   - Go to Menu > Integrations > Bot Accounts
   - Add a Bot Account for the plugin (gives us a token to use for auth)
      - Bot Account will need role: `system admin`

   **Option B: Use username/password**

   ```bash
   export MM_ADMIN_USERNAME="admin"
   export MM_ADMIN_PASSWORD="your-password"
   ```

5. **Deploy the plugin:**

   ```bash
   make dev-deploy
   ```

6. **Enable the plugin:**

   ```bash
   make dev-enable
   ```

   Or enable it via System Console > Plugins > Community Toolkit > Enable

7. **Test your plugin:**
   - Access Mattermost at http://localhost:8065
   - Create test users and channels
   - Test plugin functionality

## Make Targets

### Environment Management

- **`make dev-up`** / **`make dev-start`** - Start the Podman Compose stack
  - Starts PostgreSQL and Mattermost containers
  - Waits for Mattermost to be ready
  - Data persists between restarts

- **`make dev-down`** / **`make dev-stop`** - Stop the stack
  - Stops containers but preserves data
  - Use this for daily development (faster than clean)

- **`make dev-restart`** - Restart the stack
  - Equivalent to `dev-down` followed by `dev-up`

- **`make dev-clean`** - Clean everything
  - Stops containers
  - Removes volumes and all data
  - Use this when you want a completely fresh start
  - **Warning:** This deletes all Mattermost data, users, and settings

### Plugin Management

- **`make dev-deploy`** - Build and deploy plugin
  - Builds the plugin bundle
  - Uploads to the running Mattermost instance
  - Requires `MM_ADMIN_TOKEN` or `MM_ADMIN_USERNAME`/`MM_ADMIN_PASSWORD` to be set

- **`make dev-enable`** - Enable the plugin
  - Enables the plugin in the development environment
  - Verifies Mattermost container is running
  - Loads credentials from `podman/.env` if present

- **`make dev-disable`** - Disable the plugin
  - Disables the plugin in the development environment
  - Verifies Mattermost container is running

- **`make dev-reset`** - Reset the plugin
  - Disables and re-enables the plugin (useful for testing)
  - Verifies Mattermost container is running

### Debugging and Monitoring

- **`make dev-logs`** - View Mattermost server logs
  - Shows recent logs from the Mattermost container

- **`make dev-logs-watch`** - Tail logs in real-time
  - Follows Mattermost logs as they are generated
  - Useful for debugging plugin issues
  - Press Ctrl+C to stop

- **`make dev-plugin-logs`** - View plugin logs
  - Shows plugin-specific logs from the development environment
  - Useful for debugging plugin behavior

- **`make dev-plugin-logs-watch`** - Tail plugin logs in real-time
  - Follows plugin logs as they are generated
  - Useful for real-time plugin debugging
  - Press Ctrl+C to stop

- **`make dev-shell`** - Open shell in Mattermost container
  - Provides interactive shell access for debugging
  - Useful for inspecting files, checking processes, etc.

- **`make dev-status`** - Show container status
  - Lists running containers and their status
  - Quick way to verify the stack is running

## Configuration

### Environment Variables

The Podman Compose setup uses environment variables from:

1. `podman/.env` file (if it exists)
2. Shell environment variables
3. Default values in `podman-compose.yml`

To customize the environment:

1. Copy the example file:

   ```bash
   cp podman/env.example podman/.env
   ```

2. Edit `podman/.env` with your preferences:
   - Database credentials
   - Admin user credentials
   - Site URL (default: http://localhost:8065)

3. The `.env` file is gitignored, so your local settings won't be committed.

### Default Configuration

- **Site URL:** http://localhost:8065
- **Database:** PostgreSQL 14
- **Mattermost Version:** 9.3 (minimum required for plugin)
- **Plugin Uploads:** Enabled
- **Developer Mode:** Enabled (for debugging)

## Common Workflows

### Daily Development Workflow

```bash
# Start environment (if not already running)
make dev-up

# Make code changes...

# Rebuild and redeploy plugin
make dev-deploy

# Enable plugin if needed
make dev-enable

# View logs to check for issues
make dev-logs-watch

# Test in browser at http://localhost:8065
```

### Clean Start Workflow

```bash
# Stop and clean everything
make dev-clean

# Start fresh
make dev-up

# Set up Mattermost (first-time setup)
# Open http://localhost:8065 in browser

# Configure deployment credentials
export MM_ADMIN_TOKEN="your-token"

# Deploy and enable plugin
make dev-deploy
make dev-enable
```

### Troubleshooting Permission Issues

The setup process automatically creates the required directories with proper permissions via the `dev-setup` target. However, if you encounter permission issues with the config or data directories, you can manually fix them:

```bash
chmod -R 777 podman/config podman/data/mattermost
```

### Debugging Workflow

```bash
# View logs
make dev-logs-watch

# Check container status
make dev-status

# Access container shell
make dev-shell

# Inside the shell, you can:
# - Check plugin files: ls -la /mattermost/plugins/
# - View Mattermost config: cat /mattermost/config/config.json
# - Check logs: tail -f /mattermost/logs/mattermost.log
```

## Troubleshooting

### Mattermost Won't Start

**Symptoms:** `make dev-up` completes but Mattermost isn't accessible

**Solutions:**

1. Check container status: `make dev-status`
2. View logs: `make dev-logs`
3. Check if port 8065 is already in use:
   ```bash
   lsof -i :8065
   ```
4. Restart the stack: `make dev-restart`

### Plugin Deployment Fails

**Symptoms:** `make dev-deploy` fails with authentication error

**Solutions:**

1. Verify credentials are set:
   ```bash
   echo $MM_ADMIN_TOKEN
   # or
   echo $MM_ADMIN_USERNAME
   ```
2. Check if Mattermost is ready:
   ```bash
   curl http://localhost:8065/api/v4/system/ping
   ```
3. Verify admin user exists and credentials are correct
4. Generate a new token from System Console if needed

### Database Connection Issues

**Symptoms:** Mattermost logs show database connection errors

**Solutions:**

1. Check PostgreSQL container is running: `make dev-status`
2. Restart the stack: `make dev-restart`
3. If issues persist, try a clean start: `make dev-clean` then `make dev-start`

### Port Already in Use

**Symptoms:** Podman Compose fails with "port already allocated"

**Solutions:**

1. Find what's using the port:
   ```bash
   lsof -i :8065
   ```
2. Stop the conflicting service, or
3. Modify `podman-compose.yml` to use a different port:
   ```yaml
   ports:
     - "8066:8065" # Use 8066 instead of 8065
   ```
   Then update `MM_SERVICESETTINGS_SITEURL` accordingly

### Plugin Not Loading

**Symptoms:** Plugin appears uploaded but doesn't activate

**Solutions:**

1. Check plugin compatibility with Mattermost version
2. View logs for errors: `make dev-logs`
3. Verify plugin was built correctly: `make dist`
4. Check plugin permissions in container:
   ```bash
   make dev-shell
   ls -la /mattermost/plugins/
   ```

### Data Persistence Issues

**Symptoms:** Changes disappear after restart

**Solutions:**

1. Verify volumes are mounted: `podman volume ls`
2. Check data directories exist: `ls -la podman/data/`
3. Ensure you're using `dev-down` (not `dev-clean`) for normal stops

## File Structure

```
.
├── podman-compose.yml          # Podman Compose configuration
├── podman/
│   ├── env.example             # Example environment variables
│   ├── .env                    # Local environment overrides (gitignored)
│   ├── config/                 # Mattermost config files (gitignored)
│   └── data/                   # Persistent data (gitignored)
│       ├── mattermost/         # Mattermost data and plugins
│       └── postgres/           # PostgreSQL data
```

## Accessing Mattermost

- **Web UI:** http://localhost:8065
- **API:** http://localhost:8065/api/v4
- **Admin Console:** System Console (access via hamburger menu when logged in as admin)

## Integration with Existing Workflow

The Podman development environment works alongside existing deployment methods:

- **`make deploy`** - Deploys to server configured via environment variables (works with any Mattermost instance)
- **`make dev-deploy`** - Specifically deploys to the local Podman stack

You can use either method depending on your needs. The Podman stack is ideal for:

- Isolated testing
- CI/CD pipelines
- Reproducible test environments
- Development without affecting production

## Best Practices

1. **Use `dev-down` for daily stops** - Preserves your test data and configuration
2. **Use `dev-clean` sparingly** - Only when you need a completely fresh environment
3. **Set up environment variables** - Create `podman/.env` for consistent configuration
4. **Monitor logs during development** - Keep `make dev-logs-watch` running in a separate terminal
5. **Version control** - Don't commit `podman/.env` or `podman/data/` (already in `.gitignore`)

## Additional Resources

- [Mattermost Plugin Development Documentation](https://developers.mattermost.com/integrate/plugins/)
- [Podman Compose Documentation](https://github.com/containers/podman-compose)
- [Mattermost Docker Hub](https://hub.docker.com/r/mattermost/mattermost-team-edition)
