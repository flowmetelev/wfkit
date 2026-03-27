# Release checklist

Use this before cutting a new `wfkit` release.

## Validate the release locally

Run:

```bash
go test ./...
```

Then:

```bash
go build ./cmd/wfkit
```

If the scaffold or example project changed, also verify:

```bash
go run ./cmd/wfkit init --name playground/wfkit --init-git
cd playground/wfkit
bun run build
```

Make sure npm trusted publishing is configured for `@flowmetelev/wfkit` with:

- owner: `flowmetelev`
- repository: `wfkit`
- workflow file: `publish.yml`
- environment name: `main`

## Check versioning

The release version comes from `npm/package.json`.

Before merging a release PR:

- prefer `task release:patch`, `task release:minor`, or `task release:major` to create a dedicated release commit
- or use `task version:patch`, `task version:minor`, or `task version:major` if you only want to bump the version without committing
- make sure the release bump is committed in the PR
- make sure the CLI version and npm version stay aligned
- review any release workflow changes

If you changed update logic, confirm the update check still works with the latest release format.

## Review release content

Check:

- changelog or release notes
- breaking changes
- migration notes, if any
- docs updates for new commands or flags

## Cut the release

Typical flow:

1. Merge the release-ready PR into `main`.
2. The release workflow checks the version in `npm/package.json`.
3. If that version tag does not already exist, GitHub Actions builds, tags, publishes, and creates the release.
4. Verify the workflow succeeds and the npm package and release artifacts are published.

## After the release

Smoke check:

- `wfkit --help`
- `wfkit update`
- `wfkit doctor`

If the release affects publish or migrate behavior, it's also worth doing one real project smoke test before announcing it.
