# Contributing to Mattermost Community Toolkit Plugin

Thank you for your interest in contributing to the Mattermost Community Toolkit Plugin!
This document is a primer for developing in this repository.

## Table of Contents

- [Development Environment Setup](#development-environment-setup)
- [Project Structure](#project-structure)
- [Build System (Make Commands)](#build-system-make-commands)
- [Development Workflow](#development-workflow)
- [Testing](#testing)
- [Code Style and Linting](#code-style-and-linting)
- [Debugging](#debugging)

## Development Environment Setup

### Prerequisites

1. **Go 1.21+**

   ```bash
   go version  # Should show 1.21 or higher
   ```

2. **Node.js** (version specified in `.nvmrc`)

   ```bash
   nvm use  # If using nvm
   node --version
   ```

3. **GNU Make** (Any version at/above 4.3 should be safe)

   ```bash
   make --version
   ```

4. **Mattermost Server** (9.3.0+)
   - Either local installation or Podman container
   - Admin access for plugin management

5. **Git**

### Initial Setup

1. **Clone the repository:**

   ```bash
   git clone https://github.com/rocky-linux/mattermost-plugin-community-toolkit.git
   cd mattermost-plugin-community-toolkit
   ```

2. **Install Go development tools:**

   ```bash
   make install-go-tools
   ```

   This installs:
   - `golangci-lint v2.6.0` - Go code linter
   - `gotestsum v1.13.0` - Enhanced test runner

3. **Configure plugin deployment (optional - for local testing):**

   Create or edit `build/pluginctl` configuration:

   ```bash
   export MM_SERVICESETTINGS_SITEURL=http://localhost:8065
   export MM_ADMIN_TOKEN=your-admin-token-here
   export MM_ADMIN_USERNAME=admin
   export MM_ADMIN_PASSWORD=your-password
   ```

   To get an admin token:

   ```bash
   # Via Mattermost CLI
   mattermost user create --email admin@example.com --username admin --password Admin123!
   mattermost roles system_admin admin
   mattermost token generate admin
   ```

4. **Verify setup:**

   ```bash
   make check-style  # Should run without errors
   make test        # Should pass all tests
   ```

## Project Structure

```
mattermost-plugin-community-toolkit/
├── server/                 # Go backend code
│   └── [plugin Go code]
├── webapp-WIP/            # Frontend (work in progress)
├── build/                 # Build tools
│   ├── manifest/          # Manifest tool
│   └── pluginctl/         # Plugin control utility
├── .github/workflows/     # CI/CD pipelines
├── Makefile               # Build automation
├── go.mod                 # Go dependencies
├── plugin.json            # Plugin manifest
└── .golangci.yml          # Linter configuration
```

## Build System (Make Commands)

The project uses GNU Make for build automation. Here are all available commands:

### Primary Build Commands

| Command       | Description                                        | Usage                                                |
| ------------- | -------------------------------------------------- | ---------------------------------------------------- |
| `make all`    | Complete build pipeline: check-style → test → dist | Use before committing                                |
| `make dist`   | Build production plugin bundle                     | Creates `dist/mattermost-community-toolkit-*.tar.gz` |
| `make server` | Build server binaries only                         | Builds for linux-amd64 and linux-arm64               |
| `make webapp` | Build webapp (currently disabled)                  | N/A - webapp is WIP                                  |
| `make bundle` | Create distribution tarball                        | Packages built artifacts                             |
| `make clean`  | Remove all build artifacts                         | Clean slate rebuild                                  |

### Development Commands

| Command                  | Description                  | Usage                           |
| ------------------------ | ---------------------------- | ------------------------------- |
| `make apply`             | Propagate manifest changes   | Run after editing `plugin.json` |
| `make deploy`            | Build and install to server  | Requires configured `pluginctl` |
| `make watch`             | Auto-rebuild on file changes | For webapp development          |
| `make deploy-from-watch` | Deploy watched changes       | Use with `make watch`           |

### Testing Commands

| Command            | Description                         | Usage                        |
| ------------------ | ----------------------------------- | ---------------------------- |
| `make test`        | Run all tests with race detection   | Standard test run            |
| `make test-ci`     | CI-optimized test with JUnit output | For CI pipelines             |
| `make coverage`    | Generate test coverage report       | Opens HTML report in browser |
| `make check-style` | Run linters (Go + JS)               | Must pass before commit      |

### Plugin Management Commands

| Command           | Description                       | Usage                |
| ----------------- | --------------------------------- | -------------------- |
| `make enable`     | Enable the plugin                 | After deployment     |
| `make disable`    | Disable the plugin                | For testing          |
| `make reset`      | Restart plugin (disable + enable) | Quick restart        |
| `make kill`       | Force kill plugin process         | Emergency stop       |
| `make logs`       | View plugin logs                  | Debugging            |
| `make logs-watch` | Tail plugin logs                  | Real-time monitoring |

### Debugging Commands

| Command                | Description               | Usage                 |
| ---------------------- | ------------------------- | --------------------- |
| `make attach`          | Attach dlv debugger       | Interactive debugging |
| `make attach-headless` | Headless dlv on port 2346 | Remote debugging      |
| `make detach`          | Detach debugger           | Stop debugging        |
| `make setup-attach`    | Find plugin PID           | Internal use          |

### Utility Commands

| Command                 | Description                  | Usage            |
| ----------------------- | ---------------------------- | ---------------- |
| `make install-go-tools` | Install required Go tools    | First-time setup |
| `make i18n-extract`     | Extract translatable strings | For localization |
| `make help`             | Show all available commands  | Quick reference  |

### Environment Variables

You can customize the build process with these variables:

```bash
# Enable debug mode
MM_DEBUG=1 make dist

# Enable local development mode (builds only for current platform)
MM_SERVICESETTINGS_ENABLEDEVELOPER=1 make server

# Custom Go flags
GO_BUILD_FLAGS="-v" make server
GO_TEST_FLAGS="-v -count=1" make test
```

## Development Workflow

### Standard Development Cycle

1. **Create a feature branch:**

   ```bash
   git checkout -b issue-XX-feature-name
   ```

2. **Make your changes:**
   - Edit code in `server/` directory
   - Update tests as needed
   - Add new tests for new functionality

3. **Run quality checks:**

   ```bash
   make check-style  # Fix any linting issues
   make test        # Ensure all tests pass
   ```

4. **Build and test locally:**

   ```bash
   make dist        # Build the plugin
   make deploy      # Deploy to local server
   make enable      # Enable the plugin
   make logs-watch  # Monitor for issues
   ```

5. **Test your changes:**
   - Manual testing in Mattermost
   - Verify all scenarios work
   - Check edge cases

6. **Commit your changes:**

   ```bash
   git add .
   git commit -m "feat: add new feature X"
   ```

### Building for Production

```bash
# Clean build
make clean
make all

# The production bundle is created at:
# dist/mattermost-community-toolkit-*.tar.gz
```

## Testing

### Running Tests

```bash
# Run all tests with verbose output
make test

# Run specific test
cd server && go test -v -run TestMessageWillBePosted ./...

# Run with coverage
make coverage
```

## Code Style and Linting

### Automatic Formatting

The project uses several tools for code quality:

```bash
# Run all linters
make check-style

# Auto-fix some issues
gofmt -w server/
goimports -w -local github.com/rocky-linux/mattermost-plugin-community-toolkit server/
```

### Linting Rules

We use `golangci-lint v2.6.0+` with linting rules configured in `.golangci.yml`

## Debugging

### Using Delve Debugger

1. **Install Delve:**

   ```bash
   go install github.com/go-delve/delve/cmd/dlv@latest
   ```

2. **Attach to running plugin:**

   ```bash
   make attach  # Interactive mode
   # OR
   make attach-headless  # For remote debugging on port 2346
   ```

3. **Common debugging commands:**

   ```bash
   # In dlv prompt
   break server/plugin.go:34  # Set breakpoint
   continue                   # Run to breakpoint
   next                      # Step over
   step                      # Step into
   print variableName        # Inspect variable
   goroutines               # List goroutines
   ```

### Getting Help

1. [Check existing issues](https://github.com/rocky-linux/mattermost-plugin-community-toolkit/issues)
2. [Mattermost Plugin Docs](https://developers.mattermost.com/integrate/plugins/)
3. **Create a new issue:** Include logs, error messages, and steps to reproduce

## Additional Resources

- [Mattermost Plugin Developer Docs](https://developers.mattermost.com/integrate/plugins/)
- [Go Documentation](https://go.dev/doc/)
- [Podman Development Environment](docs/PODMAN_DEVELOPMENT.md)
- [New User Moderation Features](docs/NEW_USER_MODERATION.md)
