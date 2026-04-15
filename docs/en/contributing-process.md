# Contribution Process & PR Checklist

## Workflow

1. **Fork** the repository and clone your fork:
   ```bash
   git clone https://github.com/<your-username>/teenet-wallet.git
   cd teenet-wallet
   ```

2. **Branch** from `main`:
   ```bash
   git checkout -b feature/my-change
   ```

3. **Implement** your changes following the [Coding Standards](coding-standards.md).

4. **Test** locally:
   ```bash
   make lint
   make test
   ```

5. **Push** and open a **pull request** against `main`.

## Commit Message Conventions

Use conventional prefixes:

- `feat:` -- new feature
- `fix:` -- bug fix
- `docs:` -- documentation only
- `refactor:` -- code change that neither fixes a bug nor adds a feature
- `test:` -- adding or updating tests
- `chore:` -- maintenance (CI, deps, build)

Example: `feat: add support for Arbitrum chain`

## PR Description Template

Include in every pull request:

- **What** the change does
- **Why** it is needed
- **How to test** it
- **Breaking changes** or migration steps (if any)

## CI Requirements

CI must pass before merge. The pipeline runs lint, tests (with race detector), and vulnerability scan on every PR.

## Security Vulnerabilities

If you discover a security vulnerability, follow the responsible disclosure process in [SECURITY.md](https://github.com/TEENet-io/teenet-wallet/blob/main/SECURITY.md). Do not open public issues for security reports.

## PR Checklist

Before requesting review, confirm:

- [ ] `make lint` passes
- [ ] `make test` passes (with race detector)
- [ ] New endpoints have tests
- [ ] OpenAPI spec updated if API changed
- [ ] Docs updated if user-facing behavior changed
