# Support

If you need help with `wfkit`, start with the docs:

- Check out the [README](./README.md) for the main workflows
- Run `wfkit doctor` to validate your local setup
- Run commands with `--dry-run` when you want to inspect changes safely

## Where to ask for help

Use GitHub issues when:

- you've found a reproducible bug
- you want to request a feature
- the current behavior is unclear or inconsistent

When opening an issue, include:

- the command you ran
- the output you saw
- your operating system
- your `wfkit` version
- any relevant project config or reproduction steps

## Before opening a bug report

It's worth checking:

1. Is `wfkit.json` present and filled out correctly?
2. Does `wfkit doctor` report any blocking issues?
3. If this is a publish or migrate problem, does `--dry-run` work?
4. If this is a local dev issue, does your Vite app build on its own?

## Security issues

If the issue could expose secrets, authentication data, or unintended access, don't open a public issue first.

Check out [SECURITY.md](./SECURITY.md) for the right reporting path.
