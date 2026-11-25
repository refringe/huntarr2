# Contributing to Huntarr2

Thanks for your interest in contributing. Huntarr2 is built in the open and welcomes contributions of all kinds: code, documentation, bug reports, feature discussions, and testing.

## Getting started

1. Fork the repository and clone your fork.
2. Run `make hooks` to install the pre-commit hook.
3. Copy `.env.example` to `.env` and configure for local development.
4. Run `make build && make run` to start the server.

## Prerequisites

- Go 1.27+
- Docker and Docker Compose
- Node.js, for the Prettier checks that run through `npx`

templ and golangci-lint are pinned in `go.mod` and run through `go tool`, so neither needs a separate install. The `make migrate-*` targets are the one exception: they call the [goose](https://github.com/pressly/goose) CLI and read `DATABASE_PATH`. The server applies migrations itself at startup, so most contributors never need them.

## Finding work

- Issues labeled `good first issue` are scoped for newcomers.
- Issues labeled `help wanted` are ready for contribution but may require more context.
- Check the project's milestones for current priorities.

## Pull request guidelines

- One concern per pull request. Don't mix refactors with features.
- Write tests for new functionality.
- Run `make test` and `make lint` before submitting.
- Write a clear PR description explaining what changed and why.
- Reference the issue number if one exists.

## Code style

- Run `make fmt` and follow the patterns established in the codebase.

## Reporting bugs

Open a GitHub issue with:
- Steps to reproduce
- Expected behaviour
- Actual behaviour
- Huntarr2 version and deployment method

## Code of conduct

Be constructive, be patient, and be kind. Harassment and discrimination are not tolerated. See [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) for details.
