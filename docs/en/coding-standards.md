# Coding Standards & CI

## Formatting

- Run `go fmt ./...` before committing.
- Run `go vet ./...` for common mistakes.

## Handler Pattern

One handler file per domain (wallets, contracts, auth, etc.). Every handler follows this flow:

1. **Validate** input
2. **Auth check** (verify caller permissions)
3. **Business logic**
4. **Audit** -- call `writeAuditCtx()` for any state-changing operation
5. **Respond** with JSON

## Auth Groups

Routes are organized into auth groups in `main.go`:

| Group | Auth Required | Use For |
|-------|---------------|---------|
| _(bare router)_ | None | Public endpoints (health, chain list) |
| `auth` | API key OR Passkey | Standard operations (transfers, balance queries) |
| `passkeyOnly` | Passkey session | Sensitive operations (wallet deletion, policy deletion, approve/reject) |
| `approveOnly` | Passkey session | Approval actions (approve, reject pending requests) |

## Approval-Gated Operations

For API key operations that require human approval, use `createPendingApproval` from `handler/helpers.go`. This creates an `ApprovalRequest` and returns HTTP 202 to the caller.

## Testing

- Tests use in-memory SQLite (`file::memory:`) and a nil SDK client.
- Handlers fail gracefully at the signing step -- this is by design. Tests verify behavior up to the point where signing would occur.
- For integration tests that need real cryptographic signing, use the mock consensus server from [teenet-sdk/mock-server](https://github.com/TEENet-io/teenet-sdk/tree/main/mock-server).
- All new features should include tests. See `handler/*_test.go` for examples.

## CI Pipeline

GitHub Actions runs the following on every pull request:

1. **Lint** -- `go vet` and `staticcheck`
2. **Test** -- `go test ./... -race`
3. **Vulnerability scan**

All checks must pass before a PR can be merged.
