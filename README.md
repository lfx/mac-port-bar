# mac-port-bar

A lightweight macOS menu bar utility written in Go that automatically scans for and lists locally running HTTP services.

## Features

- **Menu Bar Integration**: Unobtrusively lives in your Mac menu bar.
- **Port Scanning**: Continuously scans for active `TCP:LISTEN` ports.
- **HTTP Verification**: Automatically pings discovered ports to filter out non-HTTP system daemons. Endpoints returning `401 Unauthorized` or `403 Forbidden` are also hidden to reduce noise.
- **Smart Grouping**: Separates working HTTP endpoints (HTTP 200-399) from those returning error codes (HTTP 400+). 
  *Note: Fresh web framework projects (like Django or Rails) often return a `404 Not Found` on their root `/` URL before routes are configured. These will securely appear grouped at the bottom below the menu separator with a ⚠️ warning icon, so they don't clutter up your active production apps!*
- **Process Context**: Displays the exact working directory for each running process.
- **Quick Actions**: Dropdown menu allows you to open the port in your browser, copy the localhost URL, or forcefully stop the holding process.
- **Auto Refresh**: Updates the active list in the background every 10 seconds.

## Installation

### Using Homebrew

Pending...

### Building from Source

Ensure you have Go installed:

```bash
git clone https://github.com/lfx/mac-port-bar.git
cd mac-port-bar
go build -o mac-port-bar
```

## Usage

Simply run the application:

```bash
./mac-port-bar
```

The application icon will appear in your top right menu bar. Click it to view and manage your open HTTP ports.

## Automated Releases

This repository includes a GitHub Actions pipeline hooked up to GoReleaser that automatically builds binaries for macOS (Intel & Silicon) when pushing a new version tag to GitHub.

To create a new automated release, utilize the helper script:

```bash
./release.sh v1.0.0
```

*This script validates the version formatting, sets the git tag, and pushes it to origin, instantly triggering the `CI & Release` workflow.*
