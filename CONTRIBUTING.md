# Contributing to wfkit

Thanks for taking a look at `wfkit`.

The fastest way to contribute is:

1. Open an issue if you're reporting a bug, proposing a feature, or changing behavior in a meaningful way.
2. Fork the repo and create a focused branch.
3. Make the smallest change that solves the problem well.
4. Run the relevant checks before you open a pull request.

## Use the local workflow

Clone the repo, then run:

```bash
go test ./...
```

If you're working on generated docs or templates, run formatting for the files you changed.

If you're working on the example project or scaffold output, it's also useful to verify the generated workflow manually:

```bash
go run ./cmd/wfkit init --name playground/wfkit --init-git
cd playground/wfkit
bun run build
```

If your change affects project initialization, also verify the scaffold flow itself:

```bash
go run ./cmd/wfkit init --name test-project --init-git
```

## Keep pull requests easy to review

Good pull requests are:

- focused on one problem
- small enough to review without guesswork
- clear about behavior changes
- tested at the level the change deserves

If your change affects CLI behavior, include:

- what changed
- how you verified it
- any tradeoffs or follow-up work

If your change affects `proxy`, `publish`, or `migrate`, include the exact command you ran.

## Match the project style

When contributing:

- keep changes pragmatic
- preserve existing patterns unless there's a strong reason to improve them
- prefer clear names and small functions over clever abstractions
- avoid broad refactors unless the change actually needs them

For docs and templates:

- lead with what people should do
- keep examples short and real
- avoid filler and vague wording

## Before opening a pull request

Run:

```bash
go test ./...
```

Then, depending on what changed, also verify the relevant command flow:

```bash
wfkit doctor
wfkit publish --env prod --dry-run
wfkit migrate --dry-run
```

If you changed open source templates or docs, make sure those files are formatted and readable on GitHub.

Then open a pull request with:

- a short summary
- testing notes
- screenshots or terminal output if the UX changed

## Need help?

If you're not sure whether an idea fits the project, open an issue first.

That's usually the fastest way to avoid wasted work.
