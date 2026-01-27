# Contributing to cwalk

Thank you for your interest in contributing to cwalk! This document provides guidelines and instructions for contributing to the project.

## Code of Conduct

Be respectful, inclusive, and professional in all interactions.

## Getting Started

1. **Fork the repository** on GitHub
2. **Clone your fork** locally:
   ```bash
   git clone https://github.com/your-username/cwalk.git
   cd cwalk
   ```
3. **Create a branch** for your changes:
   ```bash
   git checkout -b feature/your-feature-name
   # or
   git checkout -b fix/issue-description
   ```

## Development Setup

### Prerequisites

- Go 1.21 or later
- Git

### Running Tests

```bash
# Run all tests
go test -v ./...

# Run tests with race detection
go test -race ./...

# Run benchmarks
go test -bench=. -benchtime=1x ./...

# Run specific tests
go test -v -run TestWalkBasicTraversal ./...
```

### Code Style

- Follow standard Go conventions (gofmt, goimports)
- Run `gofmt -s -w .` before committing
- Run `go vet ./...` to check for issues
- Write clear, concise commit messages
- Add comments for exported types and functions (GoDoc)

## Making Changes

1. **Create focused commits** - Each commit should address a single concern
2. **Write tests** for new functionality or bug fixes
3. **Update documentation** if your changes affect the public API
4. **Run tests locally** to ensure everything passes:
   ```bash
   go test -v -race ./...
   ```

## Commit Messages

Use clear, descriptive commit messages:

```
Brief summary (50 chars or less)

Longer explanation if needed. Explain what changed and why.
Reference issues if applicable: Fixes #123
```

Examples:
- `Fix: prevent work stealing deadlock in high concurrency scenarios`
- `Feature: add OnError callback for error handling`
- `Docs: improve README examples`
- `Refactor: simplify walkBranch logic`

## Pull Request Process

1. **Push your branch** to your fork
2. **Create a Pull Request** with:
   - Clear title describing the change
   - Description of what changed and why
   - Reference to related issues
3. **Wait for CI checks** to pass (tests must pass on all Go versions)
4. **Respond to feedback** promptly and professionally
5. **Keep the branch updated** if requested

## Testing Requirements

All pull requests must:
- Pass tests on Go 1.21 through 1.24
- Pass race detection tests
- Maintain or improve code coverage
- Pass `go vet` and `gofmt` checks

The CI pipeline will automatically run these checks on:
- Branch: `main` (all PRs and commits)
- Branch: `dev` (development branch)

## Reporting Issues

When reporting issues, please include:
- Go version (`go version`)
- Operating system and version
- Clear description of the problem
- Steps to reproduce (if applicable)
- Expected vs actual behavior
- Any relevant code snippets or error messages

## Questions?

Feel free to open an issue to ask questions or discuss ideas before implementing major changes.

## License

By contributing to cwalk, you agree that your contributions will be licensed under the MIT License.
