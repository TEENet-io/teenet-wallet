# Contributing to TEENet Wallet

Thank you for your interest in contributing to TEENet Wallet. This guide will help you get started.

## Getting Started

1. Fork the repository and clone your fork:
   ```bash
   git clone https://github.com/<your-username>/teenet-wallet.git
   cd teenet-wallet
   ```

2. Install dependencies and build:
   ```bash
   make build
   ```

3. Run the test suite:
   ```bash
   make test
   ```

## Development Setup

### Prerequisites

- **Go 1.24+** -- download from https://go.dev/dl/
- **SQLite3 development headers** -- required for CGo SQLite driver
  - Debian/Ubuntu: `sudo apt-get install libsqlite3-dev`
  - macOS: included with Xcode Command Line Tools
  - Alpine: `apk add sqlite-dev gcc musl-dev`
- **Docker** (optional) -- for container builds

### Make Commands

| Command      | Description                        |
|--------------|------------------------------------|
| `make build` | Compile the binary                 |
| `make test`  | Run all tests with race detector   |
| `make lint`  | Run `go vet` and `staticcheck`     |
| `make docker`| Build the Docker image             |
| `make clean` | Remove the compiled binary         |

## Code Style

- Run `go fmt ./...` before committing.
- Run `go vet ./...` to catch common mistakes.
- Follow the patterns established in existing handlers -- each handler file covers one domain (wallets, contracts, auth, etc.).
- Keep functions focused. If a handler grows beyond ~100 lines, consider extracting helpers.

## Testing

- All new features should include tests. See `handler/*_test.go` for examples.
- Run `go test ./... -race` to check for data races.
- Tests should not require external services. Use mocks or test fixtures where needed.

## Pull Requests

1. Create a feature branch from `main`:
   ```bash
   git checkout -b feature/my-change
   ```

2. Make your changes and commit with a clear message:
   ```bash
   git commit -m "feat: add support for new chain type"
   ```

3. Push to your fork and open a pull request against `main`.

4. In the PR description, include:
   - What the change does and why
   - How to test it
   - Any breaking changes or migration steps

5. Ensure CI passes before requesting review.

## Security

If you discover a security vulnerability, **do not open a public issue**. Please follow the responsible disclosure process described in [SECURITY.md](SECURITY.md).

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
