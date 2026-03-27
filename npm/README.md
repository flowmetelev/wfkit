# wfkit

`wfkit` is a CLI for building and publishing Webflow code projects.

The npm package installs the native Go binary for your platform during setup.

Supported npm targets:

- macOS `x64` and `arm64`
- Linux `x64` and `arm64`
- Windows `x64` and `arm64`

If your platform is outside that matrix, install from source instead of npm.

## Install

Run:

```bash
npm install -g @flowmetelev/wfkit
```

## Quick start

Create a project:

```bash
wfkit init --name my-site
```

Then:

```bash
cd my-site
bun install
bun run start
```

This starts a local proxy on `http://localhost:3000` and injects your Vite scripts into proxied HTML only. The published `.webflow.io` site stays unchanged for everyone else.

To expose the proxy on your local network, run:

```bash
wfkit proxy --host 192.168.1.25
```

When you're ready to ship:

```bash
wfkit publish --env prod
```

To preview the publish plan without changing GitHub or Webflow:

```bash
wfkit publish --env prod --dry-run
```

## Main commands

### `wfkit init`

Create a new project scaffold, including `wfkit.json`.

### `wfkit proxy`

Proxy your published `.webflow.io` site locally and inject local dev scripts.

### `wfkit publish`

Build and publish your code to Webflow.

### `wfkit doctor`

Check config, local tools, auth, and ports.

### `wfkit update`

Check for CLI updates.

## Configure the project

`wfkit.json` is the main project config file.

Example:

```json
{
  "appName": "my-site",
  "siteUrl": "https://my-site.webflow.io",
  "ghUserName": "your-username",
  "repositoryName": "your-repo",
  "packageManager": "bun",
  "branch": "main",
  "buildDir": "dist/assets",
  "devHost": "localhost",
  "devPort": 5173,
  "proxyHost": "localhost",
  "proxyPort": 3000,
  "openBrowser": true,
  "globalEntry": "src/global/index.ts"
}
```

Generated projects follow this structure:

```text
src/
  global/
  pages/
  utils/
build/
dist/assets/
```

`wfkit` builds a `wfkit-manifest.json` file in `dist/assets` so page and global publish flows can resolve the right scripts without guessing filenames.

## Legacy dev mode

`wfkit publish --env dev` still works, but `wfkit proxy` is the recommended development flow.
