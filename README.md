# wfkit

`wfkit` is a CLI for Webflow code projects.

Use it to:

- scaffold a new project
- develop against your published `.webflow.io` site through a local proxy
- migrate inline Webflow custom code into page and global source files
- publish production bundles back to Webflow

## Install the CLI

Run:

```bash
npm install -g @flowmetelev/wfkit
```

The npm package currently ships native binaries for:

- macOS `x64` and `arm64`
- Linux `x64` and `arm64`
- Windows `x64` and `arm64`

If your package manager blocks `postinstall` scripts, the `wfkit` launcher will download the native binary automatically on first run.

Then:

```bash
wfkit --help
```

## Start a new project

Run:

```bash
wfkit init --name my-site
```

Add `--init-git` if you want the scaffold to initialize a local repository immediately.

This creates a new folder in your current directory:

```text
my-site/
```

Inside it, `wfkit` generates:

- `package.json`
- `wfkit.json`
- `vite.config.js`
- `.gitignore`
- `.prettierrc`
- `.prettierignore`
- `.editorconfig`

`wfkit.json` is the main project config file. That's where you keep your Webflow and GitHub settings.

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
  "globalEntry": "src/global/index.ts",
  "docsEntry": "docs/index.md",
  "docsPageSlug": "docs"
}
```

The generated project is organized for Webflow:

```text
src/
  global/
    index.ts
    modules/
      site.global.ts
  pages/
    home/
      index.ts
  utils/
    dom.ts
build/
  webflow-vite-plugin.js
docs/
  index.md
dist/assets/
  wfkit-manifest.json
```

How it works:

- `src/global/index.ts` is the one explicit global entry
- `src/global/modules/*` contains reusable global behaviors that you choose to import from the global entry
- `src/pages/*/index.ts` is for per-page code
- `src/utils/*` is shared code that can be used from both global and page entries
- `docs/index.md` is the default markdown entry for the docs hub page
- `dist/assets/wfkit-manifest.json` is generated during build and used by `wfkit` to resolve the correct output files

## Publish a docs hub page

`wfkit` can render one markdown file into a dedicated Webflow docs page.

The default scaffold creates:

```text
docs/index.md
```

Publish it with:

```bash
wfkit docs
```

By default this targets the Webflow page with slug `docs`.

The command injects a managed docs block into that page's custom code and mounts the rendered content into:

- `[data-wf-docs-root]`, if present
- otherwise `main`

If you want a specific mount point, add an element with `data-wf-docs-root` to the Webflow page.

## Develop through the local proxy

The recommended dev flow is `proxy`, not `publish --env dev`.

`proxy` leaves the live Webflow site alone. It proxies your published `.webflow.io` site locally, removes managed production scripts from proxied HTML, and injects your local Vite scripts only in the browser you use for development.

Run:

```bash
cd my-site
```

Then:

```bash
bun install
```

Then:

```bash
bun run start
```

This starts:

- Vite on `http://localhost:5173`
- the Webflow proxy on `http://localhost:3000`

Open:

```text
http://localhost:3000
```

Use `wfkit proxy` directly if you want to override ports or hosts.

If you want to open the proxied site from another device on your network, run:

```bash
wfkit proxy --host 192.168.1.25
```

This uses the provided host in local script URLs and binds the proxy/dev server so the site can be reached outside `localhost`.

## Publish production code

When you're ready to ship, run:

```bash
wfkit publish --env prod
```

This flow:

1. builds your project
2. pushes the build to GitHub
3. updates the Webflow custom code
4. republishes the site

If you want to preview the publish plan without changing GitHub or Webflow, run:

```bash
wfkit publish --env prod --dry-run
```

This still builds the project and checks Webflow, but it won't push, update code, or republish the site.

## Migrate existing Webflow code

If your Webflow project already has inline custom code, you can migrate it into the generated file structure:

```bash
wfkit migrate --dry-run
```

Then apply it:

```bash
wfkit migrate
```

What `migrate` does:

- reads page-level inline scripts from Webflow
- creates page files using page slugs, for example `src/pages/about/index.js`
- moves migratable global inline code into `src/global/modules/webflow.migrated.js`
- builds the project and publishes managed jsDelivr script references back to Webflow

Use `--force` if a target page or global migration file already exists and you want to overwrite it.

## Release the CLI

`wfkit` uses the version in [`npm/package.json`](./npm/package.json) as the single release version.

The simplest release flow is:

```bash
task release:patch
```

Or:

```bash
task release:minor
task release:major
```

These commands already:

1. validate the release version metadata
2. run the npm installer test
3. run `go test ./...`
4. require a clean working tree before bumping
5. bump the version in `npm/package.json`
6. create a commit like `chore: release vX.Y.Z`

If you only want to bump the version without creating a commit, use:

```bash
task version:patch
task version:minor
task version:major
```

After that:

1. push your branch
2. open and merge the PR into `main`
3. GitHub Actions creates the release automatically if that version tag does not already exist

The npm publish step uses npm trusted publishing via GitHub Actions OIDC.
It does not require `NPM_TOKEN` when npm trusted publishing is configured correctly.

Trusted publisher settings for `@flowmetelev/wfkit`:

- owner: `flowmetelev`
- repository: `wfkit`
- workflow file: `publish.yml`
- environment name: `main`

If the version in `npm/package.json` hasn't changed, the release workflow skips itself.

Typical flow:

1. make and commit your feature changes in a branch
2. run one command: `task release:patch`, `task release:minor`, or `task release:major`
3. push the branch and open a pull request
4. merge into `main`
5. let GitHub Actions cut the release automatically

## Check your setup

Run:

```bash
wfkit doctor
```

This checks:

- `wfkit.json`
- `package.json`
- package manager and `git`
- configured ports
- Webflow authentication
- whether the local dev server is already reachable

## Use the commands

### `wfkit init`

Create a new project scaffold.

Options:

- `--name` Project name
- `--pages-dir` Directory for page scripts
- `--global-entry` Global entry file
- `--global-var` Global variable name
- `--init-git` Initialize a local git repository
- `--types` Generate TypeScript types
- `--package-manager` Package manager: `bun`, `npm`, `yarn`, `pnpm`

### `wfkit proxy`

Run the local reverse proxy for your published site.

Options:

- `--site-url` Published `.webflow.io` URL
- `--script-url` Custom local script URL
- `--dev-port` Local Vite port
- `--dev-host` Local Vite host
- `--proxy-port` Local proxy port
- `--proxy-host` Local proxy host
- `--open` Open the proxied site automatically

### `wfkit publish`

Publish code to Webflow.

Options:

- `--env` `prod` or `dev`
- `--by-page` Publish scripts per page
- `--dry-run` Show what would change without pushing or updating Webflow
- `--script-url` Override the generated script URL
- `--dev-port` Local dev server port for legacy `dev` mode
- `--dev-host` Local dev server host for legacy `dev` mode
- `--custom-commit` Custom Git commit message
- `--branch` Git branch for CDN URLs
- `--build-dir` Build output directory
- `--notify` Show a desktop notification and play a sound when finished
- `--update` Check for CLI updates before publish

### `wfkit docs`

Render markdown and publish it to a dedicated Webflow docs page.

Options:

- `--file` Markdown entry file for the docs hub
- `--page-slug` Target Webflow page slug
- `--selector` Selector used to mount the rendered docs content
- `--dry-run` Show what would be changed without updating Webflow
- `--publish` Publish the site after updating the docs page
- `--notify` Show a desktop notification and play a sound when finished

### `wfkit migrate`

Migrate inline Webflow custom code into local source files and publish managed script references back to Webflow.

Options:

- `--dry-run` Show the migration plan without writing files or updating Webflow
- `--force` Overwrite existing generated migration targets
- `--custom-commit` Custom Git commit message for the generated migration commit
- `--branch` Git branch for CDN URLs
- `--build-dir` Build output directory
- `--notify` Show a desktop notification and play a sound when finished
- `--update` Check for CLI updates before migration

### `wfkit update`

Check whether a newer CLI version is available.

### `wfkit doctor`

Validate your local environment and Webflow auth.

## Legacy dev mode

`wfkit publish --env dev` is still available, but it's now the advanced path.

Use it only if you explicitly want `wfkit` to inject a dev script into Webflow and roll it back later. For everyday development, `proxy` is safer because other people visiting the site won't see your local setup.

## Contributing

Take a look at [CONTRIBUTING.md](CONTRIBUTING.md).

## Thanks

`wfkit` uses [kooky](https://github.com/browserutils/kooky) to load browser cookies for Webflow authentication.

Thanks to the `browserutils/kooky` maintainers for the library this workflow builds on.
