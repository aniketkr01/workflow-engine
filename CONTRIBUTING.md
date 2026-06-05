# Contributing to Workflow Engine

Thanks for your interest in contributing. We welcome bug reports, feature requests, and PRs that improve the project.

## Getting started

1. Fork the repository and create a feature branch: `git checkout -b feat/my-change`.
2. Write code on your branch and keep changes focused and well-scoped.
3. Run tests and linters locally before submitting a PR.

## Development setup

- Install dependencies:

```bash
go mod download
```

- Run tests:

```bash
go test ./...
```

- Run linters/formatters:

```bash
gofmt -w .
go vet ./...
# golangci-lint if installed
golangci-lint run ./...
```

## Code style

- Follow idiomatic Go style (effective go, gofmt).
- Keep functions small and single-responsibility.
- Prefer clear, descriptive names for variables and functions.

## Testing

- Add unit tests for new behavior and bug fixes.
- Keep tests deterministic and fast when possible.
- Use table-driven tests for multiple cases.

## Pull request process

1. Open a PR against the `main` branch with a clear title and description.
2. Include motivation, summary of changes, and any migration notes.
3. Ensure CI passes (formatting, linting, tests, security scans).
4. Address review comments; squash or rebase commits if requested.

## Commit messages

- Use present-tense, short subject lines (50 characters or less recommended).
- Add a more detailed body if necessary, explaining why the change was made.

## Reporting security issues

If you discover a security vulnerability, please do not open a public issue. Instead, contact the repository owner or use the platform's private security reporting mechanism.

## Code of Conduct

Be respectful and collaborative. Follow the standard GitHub Community Guidelines.

## License

By contributing, you agree that your contributions will be licensed under the repository's MIT license.
